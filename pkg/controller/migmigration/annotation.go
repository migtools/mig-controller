package migmigration

import (
	"context"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Velero Plugin Annotations
const (
	StageOrFinalMigrationAnnotation = "migration.openshift.io/migmigration-type" // (stage|final)
	StageMigration                  = "stage"
	FinalMigration                  = "final"
	PvActionAnnotation              = "openshift.io/migrate-type"          // (move|copy)
	PvStorageClassAnnotation        = "openshift.io/target-storage-class"  // storageClassName
	PvAccessModeAnnotation          = "openshift.io/target-access-mode"    // accessMode
	PvCopyMethodAnnotation          = "migration.openshift.io/copy-method" // (snapshot|filesystem)
	QuiesceAnnotation               = "openshift.io/migrate-quiesce-pods"  // (true|false)
	QuiesceNodeSelector             = "migration.openshift.io/quiesceDaemonSet"
	SuspendAnnotation               = "migration.openshift.io/preQuiesceSuspend"
	ReplicasAnnotation              = "migration.openshift.io/preQuiesceReplicas"
	NodeSelectorAnnotation          = "migration.openshift.io/preQuiesceNodeSelector"
	StagePodImageAnnotation         = "migration.openshift.io/stage-pod-image"
)

// Restic Annotations
const (
	ResticPvBackupAnnotation = "backup.velero.io/backup-volumes" // comma-separated list of volume names
	ResticPvVerifyAnnotation = "backup.velero.io/verify-volumes" // comma-separated list of volume names
)

// Labels.
const (
	// Resources included in the stage backup.
	// Referenced by the Backup.LabelSelector. The value is the Task.UID().
	IncludedInStageBackupLabel = "migration-included-stage-backup"
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
	// Identifies associated directvolumemigration resource
	// The value is the Task.UID()
	DirectVolumeMigrationLabel = "migration-direct-volume"
	// Identifies the resource as migrated by us
	// for easy search or application rollback.
	// The value is the Task.UID().
	MigMigrationLabel = "migration.openshift.io/migrated-by-migmigration" // (migmigration UID)
	// Identifies associated migmigration
	// to assist manual debugging
	// The value is Task.Owner.Name
	MigMigrationDebugLabel = "migration.openshift.io/migmigration-name"
	// Identifies associated migplan
	// to assist manual debugging
	// The value is Task.Owner.Spec.migPlanRef.Name
	MigPlanDebugLabel = "migration.openshift.io/migplan-name"
	// Identifies associated migplan
	// to allow migplan restored resources rollback
	// The value is Task.PlanResources.MigPlan.UID
	MigPlanLabel = "migration.openshift.io/migrated-by-migplan" // (migplan UID)
	// Identifies Pod as a stage pod to allow
	// for cleanup at migration start and rollback
	// The value is always "true" if set.
	StagePodLabel = "migration.openshift.io/is-stage-pod"
)

// Set of Service Accounts.
// Keyed by namespace (name) with value of map keyed by SA name.
type ServiceAccounts map[string]map[string]bool

// Add annotations and labels.
// The PvActionAnnotation annotation is added to PV & PVC as needed by the velero plugin.
// The PvStorageClassAnnotation annotation is added to PVC as needed by the velero plugin.
// The PvAccessModeAnnotation annotation is added to PVC as needed by the velero plugin.
// The PvCopyMethodAnnotation annotation is added to PVC as needed by the velero plugin.
// The ResticPvBackupAnnotation is added to Pods as needed by Restic.
// The ResticPvVerifyAnnotation is added to Pods as needed by Restic.
// The IncludedInStageBackupLabel label is added to Pods, PVs, PVCs, ImageStreams and
// is referenced by the velero.Backup label selector.
// The IncludedInStageBackupLabel label is added to Namespaces to prevent the
// velero.Backup from being empty which causes the Restore to fail.
func (t *Task) annotateStageResources() error {
	sourceClient, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	// Namespaces
	err = t.labelNamespaces(sourceClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Pods
	serviceAccounts, err := t.annotatePods(sourceClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	// PV & PVCs
	err = t.annotatePVs(sourceClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Service accounts used by stage pods.
	err = t.labelServiceAccounts(sourceClient, serviceAccounts)
	if err != nil {
		return liberr.Wrap(err)
	}

	err = t.labelImageStreams(sourceClient)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Gets a list of restic volumes and restic verify volumes for a pod
func (t *Task) getResticVolumes(client k8sclient.Client, pod corev1.Pod) ([]string, []string, error) {
	volumes := []string{}
	verifyVolumes := []string{}
	pvs := t.getPVs()
	for _, pv := range pod.Spec.Volumes {
		claim := pv.VolumeSource.PersistentVolumeClaim
		if claim == nil {
			continue
		}
		pvc := corev1.PersistentVolumeClaim{}
		err := client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Namespace: pod.Namespace,
				Name:      claim.ClaimName,
			},
			&pvc)
		if err != nil {
			return nil, nil, liberr.Wrap(err)
		}
		pvAction := findPVAction(pvs, pvc.Spec.VolumeName)
		if pvAction == migapi.PvCopyAction {
			// Add to Restic volume list if copyMethod is "filesystem"
			if findPVCopyMethod(pvs, pvc.Spec.VolumeName) == migapi.PvFilesystemCopyMethod {
				volumes = append(volumes, pv.Name)
				if findPVVerify(pvs, pvc.Spec.VolumeName) {
					verifyVolumes = append(verifyVolumes, pv.Name)
				}
			}
		}
	}
	return volumes, verifyVolumes, nil
}

// Add label to namespaces
func (t *Task) labelNamespaces(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
		namespace := corev1.Namespace{}
		err := client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Name: ns,
			},
			&namespace)
		if err != nil {
			return liberr.Wrap(err)
		}
		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string)
		}
		namespace.Labels[IncludedInStageBackupLabel] = t.UID()
		err = client.Update(context.TODO(), &namespace)
		if err != nil {
			return liberr.Wrap(err)
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
// The ResticPvVerifyAnnotation is added to Pods as needed by Restic.
// The IncludedInStageBackupLabel label is added to Pods and is referenced
// by the velero.Backup label selector.
// Returns a set of referenced service accounts.
func (t *Task) annotatePods(client k8sclient.Client) (ServiceAccounts, error) {
	serviceAccounts := ServiceAccounts{}
	list := corev1.PodList{}
	// include stage pods only
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	err := client.List(context.TODO(), options, &list)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, pod := range list.Items {
		// Annotate PVCs.
		volumes, verifyVolumes, err := t.getResticVolumes(client, pod)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		// Restic annotation used to specify volumes.
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[ResticPvBackupAnnotation] = strings.Join(volumes, ",")
		pod.Annotations[ResticPvVerifyAnnotation] = strings.Join(verifyVolumes, ",")
		// Update
		err = client.Update(context.TODO(), &pod)
		if err != nil {
			return nil, liberr.Wrap(err)
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

	return serviceAccounts, nil
}

// Add annotations and labels to PVs and PVCs.
// The PvActionAnnotation annotation is added to PVs and PVCs as needed by the velero plugin.
// The PvStorageClassAnnotation annotation is added to PVs and PVCs as needed by the velero plugin.
// The IncludedInStageBackupLabel label is added to PVs and PVCs and is referenced
// by the velero.Backup label selector.
// The PvAccessModeAnnotation annotation is added to PVC as needed by the velero plugin.
// The PvCopyMethodAnnotation annotation is added to PV and PVC as needed by the velero plugin.
func (t *Task) annotatePVs(client k8sclient.Client) error {
	pvs := t.getPVs()
	for _, pv := range pvs.List {
		pvResource := corev1.PersistentVolume{}
		err := client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Name: pv.Name,
			},
			&pvResource)
		if err != nil {
			return liberr.Wrap(err)
		}
		if pvResource.Annotations == nil {
			pvResource.Annotations = make(map[string]string)
		}
		// PV action (move|copy) needed by the velero plugin.
		pvResource.Annotations[PvActionAnnotation] = pv.Selection.Action
		// PV storageClass annotation needed by the velero plugin.
		pvResource.Annotations[PvStorageClassAnnotation] = pv.Selection.StorageClass
		if pv.Selection.Action == migapi.PvCopyAction {
			// PV copyMethod annotation needed by the velero plugin.
			pvResource.Annotations[PvCopyMethodAnnotation] = pv.Selection.CopyMethod
		}
		// Add label used by stage backup label selector.
		if pvResource.Labels == nil {
			pvResource.Labels = make(map[string]string)
		}
		pvResource.Labels[IncludedInStageBackupLabel] = t.UID()
		// Update
		err = client.Update(context.TODO(), &pvResource)
		if err != nil {
			return liberr.Wrap(err)
		}
		log.Info(
			"PV annotations/labels added.",
			"name",
			pv.Name)

		pvcResource := corev1.PersistentVolumeClaim{}
		err = client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Namespace: pv.PVC.Namespace,
				Name:      pv.PVC.Name,
			},
			&pvcResource)
		if err != nil {
			return liberr.Wrap(err)
		}
		if pvcResource.Annotations == nil {
			pvcResource.Annotations = make(map[string]string)
		}
		// PV action (move|copy) needed by the velero plugin.
		pvcResource.Annotations[PvActionAnnotation] = pv.Selection.Action
		// Add label used by stage backup label selector.
		if pvcResource.Labels == nil {
			pvcResource.Labels = make(map[string]string)
		}
		pvcResource.Labels[IncludedInStageBackupLabel] = t.UID()
		if pv.Selection.Action == migapi.PvCopyAction {
			// PV storageClass annotation needed by the velero plugin.
			pvcResource.Annotations[PvStorageClassAnnotation] = pv.Selection.StorageClass
			// PV copyMethod annotation needed by the velero plugin.
			pvcResource.Annotations[PvCopyMethodAnnotation] = pv.Selection.CopyMethod
			// PV accessMode annotation needed by the velero plugin, if present on the PV.
			if pv.Selection.AccessMode != "" {
				pvcResource.Annotations[PvAccessModeAnnotation] = string(pv.Selection.AccessMode)
			}
		}
		// Update
		err = client.Update(context.TODO(), &pvcResource)
		if err != nil {
			return liberr.Wrap(err)
		}
		log.Info(
			"PVC annotations/labels added.",
			"ns",
			pv.PVC.Namespace,
			"name",
			pv.PVC.Name)
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
			return liberr.Wrap(err)
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
				return liberr.Wrap(err)
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

// Add label to ImageStreeams
func (t *Task) labelImageStreams(client compat.Client) error {
	for _, ns := range t.sourceNamespaces() {
		imageStreamList := imagev1.ImageStreamList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &imageStreamList)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, is := range imageStreamList.Items {
			if is.Labels == nil {
				is.Labels = map[string]string{}
			}
			is.Labels[IncludedInStageBackupLabel] = t.UID()
			err = client.Update(context.Background(), &is)
			if err != nil {
				return liberr.Wrap(err)
			}
			log.Info(
				"ImageStream labels added.",
				"ns",
				is.Namespace,
				"name",
				is.Name)
		}
	}

	return nil
}

// Delete temporary annotations and labels added.
func (t *Task) deleteAnnotations() error {
	clients, namespaceList, err := t.getBothClientsWithNamespaces()
	if err != nil {
		return liberr.Wrap(err)
	}

	for i, client := range clients {
		err = t.deletePVCAnnotations(client, namespaceList[i])
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.deletePVAnnotations(client)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.deletePodAnnotations(client, namespaceList[i])
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.deleteNamespaceLabels(client, namespaceList[i])
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.deleteServiceAccountLabels(client)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.deleteImageStreamLabels(client, namespaceList[i])
		if err != nil {
			return liberr.Wrap(err)
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
			return liberr.Wrap(err)
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
				if _, found := pod.Annotations[ResticPvVerifyAnnotation]; found {
					delete(pod.Annotations, ResticPvVerifyAnnotation)
					needsUpdate = true
				}
			}
			if pod.Labels != nil {
				if _, found := pod.Labels[IncludedInStageBackupLabel]; found {
					delete(pod.Labels, IncludedInStageBackupLabel)
					needsUpdate = true
				}
			}
			if !needsUpdate {
				continue
			}
			err = client.Update(context.TODO(), &pod)
			if err != nil {
				return liberr.Wrap(err)
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
			k8sclient.ObjectKey{
				Name: ns,
			},
			&namespace)
		// Check if namespace doesn't exist. This will happpen during prepare phase
		// since destination cluster doesn't have the migrated namespaces yet
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return liberr.Wrap(err)
		}
		delete(namespace.Labels, IncludedInStageBackupLabel)
		err = client.Update(context.TODO(), &namespace)
		if err != nil {
			return liberr.Wrap(err)
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
			return liberr.Wrap(err)
		}
		for _, pvc := range pvcList.Items {
			if pvc.Spec.VolumeName == "" {
				continue
			}
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
				return liberr.Wrap(err)
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
		return liberr.Wrap(err)
	}
	for _, pv := range pvList.Items {
		delete(pv.Labels, IncludedInStageBackupLabel)
		delete(pv.Annotations, PvActionAnnotation)
		delete(pv.Annotations, PvStorageClassAnnotation)
		err = client.Update(context.TODO(), &pv)
		if err != nil {
			return liberr.Wrap(err)
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
		return liberr.Wrap(err)
	}
	for _, sa := range pvList.Items {
		delete(sa.Labels, IncludedInStageBackupLabel)
		err = client.Update(context.TODO(), &sa)
		if err != nil {
			return liberr.Wrap(err)
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

// Delete ImageStream labels
func (t *Task) deleteImageStreamLabels(client k8sclient.Client, namespaceList []string) error {
	for _, ns := range namespaceList {
		imageStreamList := imagev1.ImageStreamList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &imageStreamList)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, is := range imageStreamList.Items {
			delete(is.Labels, IncludedInStageBackupLabel)
			err = client.Update(context.Background(), &is)
			if err != nil {
				return liberr.Wrap(err)
			}
			log.Info(
				"ImageStream labels removed.",
				"ns",
				is.Namespace,
				"name",
				is.Name)
		}
	}
	return nil
}
