package migmigration

import (
	"context"
	"strings"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Velero Plugin Annotations
const (
	StageOrFinalAnnotation   = "openshift.io/migrate-copy-phase"   // (stage|final)
	PvActionAnnotation       = "openshift.io/migrate-type"         // (move|copy)
	PvStorageClassAnnotation = "openshift.io/target-storage-class" // storageClassName
	PvAccessModeAnnotation   = "openshift.io/target-access-mode"   // accessMode
	QuiesceAnnotation        = "openshift.io/migrate-quiesce-pods" // (true|false)
)

// Restic Annotations
const (
	ResticPvBackupAnnotation = "backup.velero.io/backup-volumes" // (true|false)
)

// Labels.
const (
	// Resources included in the stage backup.
	// Referenced by the Backup.LabelSelector. The value is the Task.UID().
	IncludedInStageBackupLabel = "migration-included-stage-backup"
	// Stage pods requiring restic/stage backups.
	// Used to create stage pods. The value is the Task.UID()
	StagePodLabel = "migration-stage-pod"
	// Designated as an `initial` Backup.
	// The value is the Task.UID().
	InitialBackupLabel = "migration-initial-backup"
	// Designated as an `stage` Backup.
	// The value is the Task.UID().
	StageBackupLabel = "migration-stage-backup"
	// Designated as an `stage` Restore.
	// The value is the Task.UID().
	StageRestoreLabel = "migration-stage-restore"
	// Designated as a `final` Restore.
	// The value is the Task.UID().
	FinalRestoreLabel = "migration-final-restore"
)

// Set of Service Accounts.
// Keyed by namespace (name) with value of map keyed by SA name.
type ServiceAccounts map[string]map[string]bool

// Add annotations and labels.
// The PvActionAnnotation annotation is added to PV & PVC as needed by the velero plugin.
// The PvStorageClassAnnotation annotation is added to PVC as needed by the velero plugin.
// The PvAccessModeAnnotation annotation is added to PVC as needed by the velero plugin.
// The ResticPvBackupAnnotation is added to Pods as needed by Restic.
// The IncludedInStageBackupLabel label is added to Pods, PVs, PVCs and is referenced
// by the velero.Backup label selector.
// The IncludedInStageBackupLabel label is added to Namespaces to prevent the
// velero.Backup from being empty which causes the Restore to fail.
func (t *Task) annotateStageResources() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	// Namespaces
	err = t.labelNamespaces(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	// Pods
	serviceAccounts, err := t.annotatePods(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	// PV & PVCs
	err = t.annotatePVs(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	// Service accounts used by stage pods.
	err = t.labelServiceAccounts(client, serviceAccounts)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Add annotations and labels to PVCs.
// The PvActionAnnotation annotation is added to PVCs as needed by the velero plugin.
// The PvStorageClassAnnotation annotation is added to PVC as needed by the velero plugin.
// The PvAccessModeAnnotation annotation is added to PVC as needed by the velero plugin.
// The IncludedInStageBackupLabel label is added to PVCs and is referenced
// by the velero.Backup label selector.
func (t *Task) annotatePVCs(client k8sclient.Client, pod corev1.Pod) ([]string, error) {
	volumes := []string{}
	pvs := t.getPVs()
	for _, pv := range pod.Spec.Volumes {
		claim := pv.VolumeSource.PersistentVolumeClaim
		if claim == nil {
			continue
		}
		pvc := corev1.PersistentVolumeClaim{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      claim.ClaimName,
			},
			&pvc)
		// PV action (move|copy) annotation needed by the velero plugin.
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvAction := findPVAction(pvs, pvc.Spec.VolumeName)
		pvc.Annotations[PvActionAnnotation] = pvAction
		// Add label used by stage backup label selector.
		if pvc.Labels == nil {
			pvc.Labels = make(map[string]string)
		}
		pvc.Labels[IncludedInStageBackupLabel] = t.UID()
		if pvAction == migapi.PvCopyAction {
			// Add to Restic volume list if copyMethod is "filesystem"
			if findPVCopyMethod(pvs, pvc.Spec.VolumeName) == migapi.PvFilesystemCopyMethod {
				volumes = append(volumes, pv.Name)
			}
			// PV storageClass annotation needed by the velero plugin.
			storageClass := findPVStorageClass(pvs, pvc.Spec.VolumeName)
			pvc.Annotations[PvStorageClassAnnotation] = storageClass
			// PV accessMode annotation needed by the velero plugin, if present on the PV.
			accessMode := findPVAccessMode(pvs, pvc.Spec.VolumeName)
			if accessMode != "" {
				pvc.Annotations[PvAccessModeAnnotation] = string(accessMode)
			}
		}
		// Update
		err = client.Update(context.TODO(), &pvc)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		log.Info(
			"PVC annotations/labels added.",
			"ns",
			pvc.Namespace,
			"name",
			pvc.Name)
	}

	return volumes, nil
}

// Add label to namespaces
func (t *Task) labelNamespaces(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
		namespace := corev1.Namespace{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{
				Name: ns,
			},
			&namespace)
		if err != nil {
			log.Trace(err)
			return err
		}
		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string)
		}
		namespace.Labels[IncludedInStageBackupLabel] = t.UID()
		err = client.Update(context.TODO(), &namespace)
		if err != nil {
			log.Trace(err)
			return err
		}
		log.Info(
			"NS annotations/labels added.",
			"name",
			namespace.Name)
	}
	return nil
}

// Add annotations and labels to Pods.
// The ResticPvBackupAnnotation is added to Pods as needed by Restic.
// The IncludedInStageBackupLabel label is added to Pods and is referenced
// by the velero.Backup label selector.
// Returns a set of referenced service accounts.
func (t *Task) annotatePods(client k8sclient.Client) (ServiceAccounts, error) {
	serviceAccounts := ServiceAccounts{}
	for _, ns := range t.sourceNamespaces() {
		list := corev1.PodList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &list)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		for _, pod := range list.Items {
			if pod.Labels == nil {
				pod.Labels = make(map[string]string)
			}
			// Skip stateless pods.
			if len(pod.Spec.Volumes) == 0 {
				continue
			}
			// Annotate PVCs.
			volumes, err := t.annotatePVCs(client, pod)
			if err != nil {
				log.Trace(err)
				return nil, err
			}
			if len(volumes) == 0 {
				continue
			}
			// Restic annotation used to specify volumes.
			if pod.Annotations == nil {
				pod.Annotations = make(map[string]string)
			}
			pod.Annotations[ResticPvBackupAnnotation] = strings.Join(volumes, ",")
			// Add label used by stage backup label selector.
			if pod.Labels == nil {
				pod.Labels = make(map[string]string)
			}

			pod.Labels[StagePodLabel] = t.UID()
			pod.Labels[IncludedInStageBackupLabel] = t.UID()
			// Update
			err = client.Update(context.TODO(), &pod)
			if err != nil {
				log.Trace(err)
				return nil, err
			}
			log.Info(
				"Pod annotations/labels added.",
				"ns",
				pod.Namespace,
				"name",
				pod.Name)
			sa := pod.Spec.ServiceAccountName
			names, found := serviceAccounts[pod.Namespace]
			if !found {
				serviceAccounts[pod.Namespace] = map[string]bool{sa: true}
			} else {
				names[sa] = true
			}
		}
	}

	return serviceAccounts, nil
}

// Add annotations and labels to PVs.
// The PvActionAnnotation annotation is added to PVs as needed by the velero plugin.
// The PvStorageClassAnnotation annotation is added to PVs as needed by the velero plugin.
// The IncludedInStageBackupLabel label is added to PVs and is referenced
// by the velero.Backup label selector.
func (t *Task) annotatePVs(client k8sclient.Client) error {
	pvs := t.getPVs()
	for _, pv := range pvs.List {
		resource := corev1.PersistentVolume{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{
				Name: pv.Name,
			},
			&resource)
		if err != nil {
			log.Trace(err)
			return err
		}
		if resource.Annotations == nil {
			resource.Annotations = make(map[string]string)
		}
		// PV action (move|copy) needed by the velero plugin.
		resource.Annotations[PvActionAnnotation] = pv.Selection.Action
		// PV storageClass annotation needed by the velero plugin.
		resource.Annotations[PvStorageClassAnnotation] = pv.Selection.StorageClass
		// Add label used by stage backup label selector.
		if resource.Labels == nil {
			resource.Labels = make(map[string]string)
		}
		resource.Labels[IncludedInStageBackupLabel] = t.UID()
		// Update
		err = client.Update(context.TODO(), &resource)
		if err != nil {
			log.Trace(err)
			return err
		}
		log.Info(
			"PV annotations/labels added.",
			"name",
			pv.Name)
	}

	return nil
}

// Add label to service accounts.
func (t *Task) labelServiceAccounts(client k8sclient.Client, serviceAccounts ServiceAccounts) error {
	for _, ns := range t.sourceNamespaces() {
		names, found := serviceAccounts[ns]
		if !found {
			continue
		}
		list := corev1.ServiceAccountList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, sa := range list.Items {
			if _, found := names[sa.Name]; !found {
				continue
			}
			if sa.Labels == nil {
				sa.Labels = make(map[string]string)
			}
			sa.Labels[IncludedInStageBackupLabel] = t.UID()
			err = client.Update(context.TODO(), &sa)
			if err != nil {
				log.Trace(err)
				return err
			}
			log.Info(
				"SA annotations/labels added.",
				"ns",
				sa.Namespace,
				"name",
				sa.Name)
		}
	}

	return nil
}

// Delete temporary annotations and labels added.
func (t *Task) deleteAnnotations() error {
	clients, namespaceList, err := t.getBothClientsWithNamespaces()
	if err != nil {
		log.Trace(err)
		return err
	}

	for i, client := range clients {
		err = t.deletePVCAnnotations(client, namespaceList[i])
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.deletePVAnnotations(client)
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.deletePodAnnotations(client, namespaceList[i])
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.deleteNamespaceLabels(client, namespaceList[i])
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.deleteServiceAccountLabels(client)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Delete Pod stage annotations and labels.
func (t *Task) deletePodAnnotations(client k8sclient.Client, namespaceList []string) error {
	for _, ns := range namespaceList {
		options := k8sclient.InNamespace(ns)
		podList := corev1.PodList{}
		err := client.List(context.TODO(), options, &podList)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, pod := range podList.Items {
			if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
				continue
			}
			needsUpdate := false
			if pod.Annotations != nil {
				if _, found := pod.Annotations[ResticPvBackupAnnotation]; found {
					delete(pod.Annotations, ResticPvBackupAnnotation)
					needsUpdate = true
				}
			}
			if pod.Labels != nil {
				if _, found := pod.Labels[IncludedInStageBackupLabel]; found {
					delete(pod.Labels, IncludedInStageBackupLabel)
					needsUpdate = true
				}
				if _, found := pod.Labels[StagePodLabel]; found {
					delete(pod.Labels, StagePodLabel)
					needsUpdate = true
				}
			}
			if !needsUpdate {
				continue
			}
			err = client.Update(context.TODO(), &pod)
			if err != nil {
				log.Trace(err)
				return err
			}
			log.Info(
				"Pod annotations/labels removed.",
				"ns",
				pod.Namespace,
				"name",
				pod.Name)
		}
	}

	return nil
}

// Delete stage label from namespaces
func (t *Task) deleteNamespaceLabels(client k8sclient.Client, namespaceList []string) error {
	for _, ns := range namespaceList {
		namespace := corev1.Namespace{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{
				Name: ns,
			},
			&namespace)
		// Check if namespace doesn't exist. This will happpen during prepare phase
		// since destination cluster doesn't have the migrated namespaces yet
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			log.Trace(err)
			return err
		}
		delete(namespace.Labels, IncludedInStageBackupLabel)
		err = client.Update(context.TODO(), &namespace)
		if err != nil {
			log.Trace(err)
			return err
		}
	}
	return nil
}

// Delete PVC stage annotations and labels.
func (t *Task) deletePVCAnnotations(client k8sclient.Client, namespaceList []string) error {
	for _, ns := range namespaceList {
		options := k8sclient.InNamespace(ns)
		pvcList := corev1.PersistentVolumeClaimList{}
		err := client.List(context.TODO(), options, &pvcList)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, pvc := range pvcList.Items {
			needsUpdate := false
			if pvc.Annotations != nil {
				if _, found := pvc.Annotations[PvActionAnnotation]; found {
					delete(pvc.Annotations, PvActionAnnotation)
					needsUpdate = true
				}
				if _, found := pvc.Annotations[PvStorageClassAnnotation]; found {
					delete(pvc.Annotations, PvStorageClassAnnotation)
					needsUpdate = true
				}
				if _, found := pvc.Annotations[PvAccessModeAnnotation]; found {
					delete(pvc.Annotations, PvAccessModeAnnotation)
					needsUpdate = true
				}
			}
			if pvc.Labels != nil {
				if _, found := pvc.Labels[IncludedInStageBackupLabel]; found {
					delete(pvc.Labels, IncludedInStageBackupLabel)
					needsUpdate = true
				}
			}
			if !needsUpdate {
				continue
			}
			err = client.Update(context.TODO(), &pvc)
			if err != nil {
				log.Trace(err)
				return err
			}
			log.Info(
				"PVC annotations/labels removed.",
				"ns",
				pvc.Namespace,
				"name",
				pvc.Name)
		}
	}

	return nil
}

// Delete PV stage annotations and labels.
func (t *Task) deletePVAnnotations(client k8sclient.Client) error {
	labels := map[string]string{
		IncludedInStageBackupLabel: t.UID(),
	}
	options := k8sclient.MatchingLabels(labels)
	pvList := corev1.PersistentVolumeList{}
	err := client.List(context.TODO(), options, &pvList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, pv := range pvList.Items {
		delete(pv.Labels, IncludedInStageBackupLabel)
		delete(pv.Annotations, PvActionAnnotation)
		delete(pv.Annotations, PvStorageClassAnnotation)
		err = client.Update(context.TODO(), &pv)
		if err != nil {
			log.Trace(err)
			return err
		}
		log.Info(
			"PV annotations/labels removed.",
			"name",
			pv.Name)
	}

	return nil
}

// Delete service account labels.
func (t *Task) deleteServiceAccountLabels(client k8sclient.Client) error {
	labels := map[string]string{
		IncludedInStageBackupLabel: t.UID(),
	}
	options := k8sclient.MatchingLabels(labels)
	pvList := corev1.ServiceAccountList{}
	err := client.List(context.TODO(), options, &pvList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, sa := range pvList.Items {
		delete(sa.Labels, IncludedInStageBackupLabel)
		err = client.Update(context.TODO(), &sa)
		if err != nil {
			log.Trace(err)
			return err
		}
		log.Info(
			"SA annotations/labels removed.",
			"ns",
			sa.Namespace,
			"name",
			sa.Name)
	}

	return nil
}
