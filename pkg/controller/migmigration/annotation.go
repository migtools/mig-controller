package migmigration

import (
	"context"
	"fmt"
	"path"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Bucket limit for number of items annotated in one Reconcile
const AnnotationsPerReconcile = 50

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
func (t *Task) annotateStageResources() (bool, error) {
	t.Log.V(4).Info("Starting annotation of PVs, PVCs, Pods Namespaces")
	sourceClient, err := t.getSourceClient()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	itemsUpdated := 0
	// Namespaces
	itemsUpdated, err = t.labelNamespaces(sourceClient, itemsUpdated)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if itemsUpdated > AnnotationsPerReconcile {
		return false, nil
	}
	// Pods
	itemsUpdated, serviceAccounts, err := t.annotatePods(sourceClient, itemsUpdated)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if itemsUpdated > AnnotationsPerReconcile {
		return false, nil
	}
	// PV & PVCs
	itemsUpdated, err = t.annotatePVs(sourceClient, itemsUpdated)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if itemsUpdated > AnnotationsPerReconcile {
		return false, nil
	}
	// Service accounts used by stage pods.
	itemsUpdated, err = t.labelServiceAccounts(sourceClient, serviceAccounts, itemsUpdated)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if itemsUpdated > AnnotationsPerReconcile {
		t.Log.Info("Completed [%v] annotations/label operations this reconcile, requeueing.")
		return false, nil
	}

	if t.indirectImageMigration() {
		itemsUpdated, err = t.labelImageStreams(sourceClient, itemsUpdated)
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if itemsUpdated > AnnotationsPerReconcile {
			return false, nil
		}
	}
	return true, nil
}

// Gets a list of restic volumes and restic verify volumes for a pod
func (t *Task) getResticVolumes(client k8sclient.Client, pod corev1.Pod) ([]string, []string, error) {
	volumes := []string{}
	verifyVolumes := []string{}
	pvs := t.getStagePVs()
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
func (t *Task) labelNamespaces(client k8sclient.Client, itemsUpdated int) (int, error) {
	total := len(t.sourceNamespaces())
	for i, ns := range t.sourceNamespaces() {
		namespace := corev1.Namespace{}
		err := client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Name: ns,
			},
			&namespace)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}
		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string)
		}
		if namespace.GetLabels()[migapi.IncludedInStageBackupLabel] == t.UID() {
			continue
		}
		namespace.Labels[migapi.IncludedInStageBackupLabel] = t.UID()
		log.Info("Adding migration annotations/labels to source cluster namespace.",
			"namespace", namespace.Name)
		err = client.Update(context.TODO(), &namespace)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}
		itemsUpdated++
		if itemsUpdated > AnnotationsPerReconcile {
			t.setProgress([]string{fmt.Sprintf("%v/%v Namespaces labeled", i, total)})
			return itemsUpdated, nil
		}
	}
	return itemsUpdated, nil
}

// Add annotations and labels to Pods.
// The ResticPvBackupAnnotation is added to Pods as needed by Restic.
// The ResticPvVerifyAnnotation is added to Pods as needed by Restic.
// The IncludedInStageBackupLabel label is added to Pods and is referenced
// by the velero.Backup label selector.
// Returns a set of referenced service accounts.
func (t *Task) annotatePods(client k8sclient.Client, itemsUpdated int) (int, ServiceAccounts, error) {
	serviceAccounts := ServiceAccounts{}
	list := corev1.PodList{}
	// include stage pods only
	options := k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels())
	err := client.List(context.TODO(), &list, options)
	if err != nil {
		return itemsUpdated, nil, liberr.Wrap(err)
	}
	total := len(list.Items)
	for i, pod := range list.Items {
		// Annotate PVCs.
		volumes, verifyVolumes, err := t.getResticVolumes(client, pod)
		if err != nil {
			return itemsUpdated, nil, liberr.Wrap(err)
		}
		// Restic annotation used to specify volumes.
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}

		if _, exist := pod.Annotations[migapi.ResticPvBackupAnnotation]; exist {
			if _, exist := pod.Annotations[migapi.ResticPvVerifyAnnotation]; exist {
				continue
			}
			continue
		}

		pod.Annotations[migapi.ResticPvBackupAnnotation] = strings.Join(volumes, ",")
		pod.Annotations[migapi.ResticPvVerifyAnnotation] = strings.Join(verifyVolumes, ",")

		// Update
		log.Info("Adding annotations/labels to source cluster Pod.",
			"pod", path.Join(pod.Namespace, pod.Name))
		err = client.Update(context.TODO(), &pod)
		if err != nil {
			return itemsUpdated, nil, liberr.Wrap(err)
		}
		itemsUpdated++
		sa := pod.Spec.ServiceAccountName
		names, found := serviceAccounts[pod.Namespace]
		if !found {
			serviceAccounts[pod.Namespace] = map[string]bool{sa: true}
		} else {
			names[sa] = true
		}

		if itemsUpdated > AnnotationsPerReconcile {
			t.setProgress([]string{fmt.Sprintf("%v/%v Pod annotations/labels added.", i, total)})
			return itemsUpdated, serviceAccounts, nil
		}
	}
	return itemsUpdated, serviceAccounts, nil
}

// Add annotations and labels to PVs and PVCs.
// The PvActionAnnotation annotation is added to PVs and PVCs as needed by the velero plugin.
// The PvStorageClassAnnotation annotation is added to PVs and PVCs as needed by the velero plugin.
// The IncludedInStageBackupLabel label is added to PVs and PVCs and is referenced
// by the velero.Backup label selector.
// The PvAccessModeAnnotation annotation is added to PVC as needed by the velero plugin.
// The PvCopyMethodAnnotation annotation is added to PV and PVC as needed by the velero plugin.
func (t *Task) annotatePVs(client k8sclient.Client, itemsUpdated int) (int, error) {
	pvs := t.getStagePVs()
	total := len(pvs.List)
	for i, pv := range pvs.List {
		pvResource := corev1.PersistentVolume{}
		err := client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Name: pv.Name,
			},
			&pvResource)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}
		if pvResource.Annotations == nil {
			pvResource.Annotations = make(map[string]string)
		}
		if pvResource.Labels[migapi.IncludedInStageBackupLabel] == t.UID() {
			continue
		}
		// PV action (move|copy) needed by the velero plugin.
		pvResource.Annotations[migapi.PvActionAnnotation] = pv.Selection.Action
		// PV storageClass annotation needed by the velero plugin.
		pvResource.Annotations[migapi.PvStorageClassAnnotation] = pv.Selection.StorageClass
		if pv.Selection.Action == migapi.PvCopyAction {
			// PV copyMethod annotation needed by the velero plugin.
			pvResource.Annotations[migapi.PvCopyMethodAnnotation] = pv.Selection.CopyMethod
		}
		// Add label used by stage backup label selector.
		if pvResource.Labels == nil {
			pvResource.Labels = make(map[string]string)
		}
		pvResource.Labels[migapi.IncludedInStageBackupLabel] = t.UID()

		// Update
		log.Info("Adding annotations/labels to source cluster PersistentVolume.",
			"persistentVolume", pv.Name)
		err = client.Update(context.TODO(), &pvResource)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}

		pvcResource := corev1.PersistentVolumeClaim{}
		err = client.Get(
			context.TODO(),
			k8sclient.ObjectKey{
				Namespace: pv.PVC.Namespace,
				Name:      pv.PVC.Name,
			},
			&pvcResource)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}
		if pvcResource.Annotations == nil {
			pvcResource.Annotations = make(map[string]string)
		}
		// PV action (move|copy) needed by the velero plugin.
		pvcResource.Annotations[migapi.PvActionAnnotation] = pv.Selection.Action
		// Add label used by stage backup label selector.
		if pvcResource.Labels == nil {
			pvcResource.Labels = make(map[string]string)
		}
		if pvcResource.Labels[migapi.IncludedInStageBackupLabel] == t.UID() {
			continue
		}
		pvcResource.Labels[migapi.IncludedInStageBackupLabel] = t.UID()
		if pv.Selection.Action == migapi.PvCopyAction {
			// PV storageClass annotation needed by the velero plugin.
			pvcResource.Annotations[migapi.PvStorageClassAnnotation] = pv.Selection.StorageClass
			// PV copyMethod annotation needed by the velero plugin.
			pvcResource.Annotations[migapi.PvCopyMethodAnnotation] = pv.Selection.CopyMethod
			// PV accessMode annotation needed by the velero plugin, if present on the PV.
			if pv.Selection.AccessMode != "" {
				pvcResource.Annotations[migapi.PvAccessModeAnnotation] = string(pv.Selection.AccessMode)
			}
		}
		// Update
		log.Info("Adding annotations/labels to source cluster PersistentVolumeClaim.",
			"persistentVolumeClaim", path.Join(pv.PVC.Namespace, pv.PVC.Name))
		err = client.Update(context.TODO(), &pvcResource)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}

		itemsUpdated++
		if itemsUpdated > AnnotationsPerReconcile {
			t.setProgress([]string{fmt.Sprintf("%v/%v PV annotations/labels added.", i, total), fmt.Sprintf("%v/%v PVC annotations/labels added.", i, total)})
			return itemsUpdated, nil
		}
	}
	return itemsUpdated, nil
}

// Add label to service accounts.
func (t *Task) labelServiceAccounts(client k8sclient.Client, serviceAccounts ServiceAccounts, itemsUpdated int) (int, error) {

	for _, ns := range t.sourceNamespaces() {
		names, found := serviceAccounts[ns]
		if !found {
			continue
		}
		list := corev1.ServiceAccountList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), &list, options)
		if err != nil {
			return itemsUpdated, liberr.Wrap(err)
		}
		total := len(list.Items)
		for i, sa := range list.Items {
			if _, found := names[sa.Name]; !found {
				continue
			}
			if sa.Labels == nil {
				sa.Labels = make(map[string]string)
			}
			if sa.Labels[migapi.IncludedInStageBackupLabel] == t.UID() {
				continue
			}
			sa.Labels[migapi.IncludedInStageBackupLabel] = t.UID()
			err = client.Update(context.TODO(), &sa)
			if err != nil {
				return itemsUpdated, liberr.Wrap(err)
			}
			log.Info("Added annotations/labels to source cluster Service Account.",
				"serviceAccount", path.Join(sa.Namespace, sa.Name))
			itemsUpdated++
			if itemsUpdated > AnnotationsPerReconcile {
				t.setProgress([]string{fmt.Sprintf("%v/%v SA annotations/labels added in the namespace: %s", i, total, sa.Namespace)})
				return itemsUpdated, nil
			}
		}
	}
	return itemsUpdated, nil
}

// Add label to ImageStreeams
func (t *Task) labelImageStreams(client compat.Client, itemsUpdated int) (int, error) {
	for _, ns := range t.sourceNamespaces() {
		imageStreamList := imagev1.ImageStreamList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), &imageStreamList, options)
		if err != nil {
			log.Trace(err)
			return itemsUpdated, err
		}
		total := len(imageStreamList.Items)
		for i, is := range imageStreamList.Items {
			if is.Labels == nil {
				is.Labels = map[string]string{}
			}
			if is.Labels[migapi.IncludedInStageBackupLabel] == t.UID() {
				continue
			}
			is.Labels[migapi.IncludedInStageBackupLabel] = t.UID()

			log.Info("Adding labels to source cluster ImageStream.",
				"imageStream", path.Join(is.Namespace, is.Name))
			err = client.Update(context.Background(), &is)
			if err != nil {
				return itemsUpdated, liberr.Wrap(err)
			}
			itemsUpdated++
			if itemsUpdated > AnnotationsPerReconcile {
				t.setProgress([]string{fmt.Sprintf("%v/%v ImageStream labels added. in the namespace: %s", i, total, is.Namespace)})
				return itemsUpdated, nil
			}
		}
	}

	return itemsUpdated, nil
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
		err := client.List(context.TODO(), &podList, options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, pod := range podList.Items {
			if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
				continue
			}
			needsUpdate := false
			if pod.Annotations != nil {
				if _, found := pod.Annotations[migapi.ResticPvBackupAnnotation]; found {
					delete(pod.Annotations, migapi.ResticPvBackupAnnotation)
					needsUpdate = true
				}
				if _, found := pod.Annotations[migapi.ResticPvVerifyAnnotation]; found {
					delete(pod.Annotations, migapi.ResticPvVerifyAnnotation)
					needsUpdate = true
				}
			}
			if pod.Labels != nil {
				if _, found := pod.Labels[migapi.IncludedInStageBackupLabel]; found {
					delete(pod.Labels, migapi.IncludedInStageBackupLabel)
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
			log.Info("Velero Annotations/Labels removed on Pod.",
				"pod", path.Join(pod.Namespace, pod.Name))
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
		delete(namespace.Labels, migapi.IncludedInStageBackupLabel)
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
		err := client.List(context.TODO(), &pvcList, options)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, pvc := range pvcList.Items {
			if pvc.Spec.VolumeName == "" {
				continue
			}
			needsUpdate := false
			if pvc.Annotations != nil {
				if _, found := pvc.Annotations[migapi.PvActionAnnotation]; found {
					delete(pvc.Annotations, migapi.PvActionAnnotation)
					needsUpdate = true
				}
				if _, found := pvc.Annotations[migapi.PvStorageClassAnnotation]; found {
					delete(pvc.Annotations, migapi.PvStorageClassAnnotation)
					needsUpdate = true
				}
				if _, found := pvc.Annotations[migapi.PvAccessModeAnnotation]; found {
					delete(pvc.Annotations, migapi.PvAccessModeAnnotation)
					needsUpdate = true
				}
			}
			if pvc.Labels != nil {
				if _, found := pvc.Labels[migapi.IncludedInStageBackupLabel]; found {
					delete(pvc.Labels, migapi.IncludedInStageBackupLabel)
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
			log.Info("Velero Annotations/Labels removed on PersistentVolumeClaim.",
				"persistentVolumeClaim", path.Join(pvc.Namespace, pvc.Name))
		}
	}

	return nil
}

// Delete PV stage annotations and labels.
func (t *Task) deletePVAnnotations(client k8sclient.Client) error {
	labels := map[string]string{
		migapi.IncludedInStageBackupLabel: t.UID(),
	}
	options := k8sclient.MatchingLabels(labels)
	pvList := corev1.PersistentVolumeList{}
	err := client.List(context.TODO(), &pvList, options)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, pv := range pvList.Items {
		delete(pv.Labels, migapi.IncludedInStageBackupLabel)
		delete(pv.Annotations, migapi.PvActionAnnotation)
		delete(pv.Annotations, migapi.PvStorageClassAnnotation)
		err = client.Update(context.TODO(), &pv)
		if err != nil {
			return liberr.Wrap(err)
		}
		log.Info("Velero Annotations/Labels removed on PersistentVolume.",
			"persistentVolume", path.Join(pv.Namespace, pv.Name))
	}

	return nil
}

// Delete service account labels.
func (t *Task) deleteServiceAccountLabels(client k8sclient.Client) error {
	labels := map[string]string{
		migapi.IncludedInStageBackupLabel: t.UID(),
	}
	options := k8sclient.MatchingLabels(labels)
	pvList := corev1.ServiceAccountList{}
	err := client.List(context.TODO(), &pvList, options)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, sa := range pvList.Items {
		delete(sa.Labels, migapi.IncludedInStageBackupLabel)
		err = client.Update(context.TODO(), &sa)
		if err != nil {
			return liberr.Wrap(err)
		}
		log.Info("Velero Annotations/Labels removed on ServiceAccount.",
			"serviceAccount", path.Join(sa.Namespace, sa.Name))
	}
	return nil
}

// Delete ImageStream labels
func (t *Task) deleteImageStreamLabels(client k8sclient.Client, namespaceList []string) error {
	labels := map[string]string{
		migapi.IncludedInStageBackupLabel: t.UID(),
	}
	options := k8sclient.MatchingLabels(labels)
	imageStreamList := imagev1.ImageStreamList{}
	err := client.List(context.TODO(), &imageStreamList, options)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, is := range imageStreamList.Items {
		delete(is.Labels, migapi.IncludedInStageBackupLabel)
		err = client.Update(context.Background(), &is)
		if err != nil {
			return liberr.Wrap(err)
		}
		log.Info("Velero Annotations/Labels removed on ImageStream.",
			"imageStream", path.Join(is.Namespace, is.Name))
	}
	return nil
}
