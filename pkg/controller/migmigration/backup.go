package migmigration

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var resticPodPrefix = "restic-"

// Ensure the backup on the source cluster has been created
// and has the proper settings.
func (t *Task) ensureBackup() error {
	newBackup, err := t.buildBackup()
	if err != nil {
		log.Trace(err)
		return err
	}
	foundBackup, err := t.getBackup()
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundBackup == nil {
		t.Backup = newBackup
		client, err := t.getSourceClient()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = client.Create(context.TODO(), newBackup)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	t.Backup = foundBackup
	if !t.equalsBackup(newBackup, foundBackup) {
		client, err := t.getSourceClient()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.updateBackup(foundBackup)
		err = client.Update(context.TODO(), foundBackup)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Get whether the two Backups are equal.
func (t *Task) equalsBackup(a, b *velero.Backup) bool {
	match := a.Spec.StorageLocation == b.Spec.StorageLocation &&
		reflect.DeepEqual(a.Spec.VolumeSnapshotLocations, b.Spec.VolumeSnapshotLocations) &&
		reflect.DeepEqual(a.Spec.IncludedNamespaces, b.Spec.IncludedNamespaces) &&
		reflect.DeepEqual(a.Spec.IncludedResources, b.Spec.IncludedResources) &&
		a.Spec.TTL == b.Spec.TTL
	return match
}

// Get an existing Backup on the source cluster.
func (t Task) getBackup() (*velero.Backup, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	list := velero.BackupList{}
	labels := t.Owner.GetCorrelationLabels()
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Get the existing BackupStorageLocation on the source cluster.
func (t *Task) getBSL() (*velero.BackupStorageLocation, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	plan := t.PlanResources.MigPlan
	location, err := plan.GetBSL(client)
	if err != nil {
		return nil, err
	}
	if location == nil {
		return nil, errors.New("BSL not found")
	}

	return location, nil
}

// Get the existing VolumeSnapshotLocation on the source cluster
func (t *Task) getVSL() (*velero.VolumeSnapshotLocation, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	plan := t.PlanResources.MigPlan
	location, err := plan.GetVSL(client)
	if err != nil {
		return nil, err
	}
	if location == nil {
		return nil, errors.New("VSL not found")
	}

	return location, nil
}

// Build a Backups as desired for the source cluster.
func (t *Task) buildBackup() (*velero.Backup, error) {
	// Get client of source cluster
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	annotations, err := t.getAnnotations(client)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	backup := &velero.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    migapi.VeleroNamespace,
			Annotations:  annotations,
		},
	}
	err = t.updateBackup(backup)
	return backup, err
}

// Update a Backups as desired for the source cluster.
func (t *Task) updateBackup(backup *velero.Backup) error {
	namespaces := t.PlanResources.MigPlan.Spec.Namespaces
	backupLocation, err := t.getBSL()
	if err != nil {
		log.Trace(err)
		return err
	}
	snapshotLocation, err := t.getVSL()
	if err != nil {
		log.Trace(err)
		return err
	}
	backup.Spec = velero.BackupSpec{
		StorageLocation:         backupLocation.Name,
		VolumeSnapshotLocations: []string{snapshotLocation.Name},
		TTL:                     metav1.Duration{Duration: 720 * time.Hour},
		IncludedNamespaces:      namespaces,
		ExcludedNamespaces:      []string{},
		IncludedResources:       t.BackupResources,
		ExcludedResources:       []string{},
		Hooks: velero.BackupHooks{
			Resources: []velero.BackupResourceHookSpec{},
		},
	}

	return nil
}

// Get a Backup that has been replicated by velero on the destination cluster.
func (t *Task) getReplicatedBackup() (*velero.Backup, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	list := velero.BackupList{}
	labels := t.Owner.GetCorrelationLabels()
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Delete the running restic pod in velero namespace
// This function is used to get around mount propagation requirements
func (t *Task) bounceResticPod() error {
	// If restart already completed, return
	if !t.Phase.Equals(WaitOnResticRestart) && !t.Phase.Equals(Started) {
		return nil
	} else if t.Phase.Equals(WaitOnResticRestart) {
		t.Log.Info("Waiting for restic pod to be ready")
	}

	// Get client of source cluster
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}

	// List all pods in velero namespace
	list := corev1.PodList{}
	err = client.List(
		context.TODO(),
		&k8sclient.ListOptions{
			Namespace: migapi.VeleroNamespace,
		},
		&list)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Loop through all pods in velero namespace
	for _, pod := range list.Items {
		if !strings.HasPrefix(pod.Name, resticPodPrefix) {
			continue
		}
		// Check if pod is running
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		// If restart already started, mark it as completed
		if t.Phase.Equals(WaitOnResticRestart) {
			t.Phase.Set(ResticRestartCompleted)
			t.Log.Info("Restic pod successfully restarted")
			return nil
		}
		// Delete pod since restart never started
		t.Log.Info(
			"Deleting restic pod",
			"name",
			pod.Name)
		err = client.Delete(
			context.TODO(),
			&pod)
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase.Set(WaitOnResticRestart)
		return nil
	}

	return nil
}

// Annotate all resources with PV action data
func (t *Task) annotateStorageResources() error {
	// Get client of source cluster
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	uniqueBackupLabelKey := fmt.Sprintf("%s-%s", pvBackupLabelKey, t.PlanResources.MigPlan.UID)
	namespaces := t.PlanResources.MigPlan.Spec.Namespaces
	pvs := t.PlanResources.MigPlan.Spec.PersistentVolumes
	for _, pv := range pvs.List {
		// Update PVs with their action
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
		resource.Annotations[pvAnnotationKey] = pv.Action
		if resource.Labels == nil {
			resource.Labels = make(map[string]string)
		}
		resource.Labels[uniqueBackupLabelKey] = pvBackupLabelValue
		client.Update(context.TODO(), &resource)
	}

	for _, ns := range namespaces {
		// Find all pods in our target namespaces
		list := corev1.PodList{}
		options := k8sclient.InNamespace(ns)
		err = client.List(context.TODO(), options, &list)
		if err != nil {
			log.Trace(err)
			return err
		}
		// Loop through all pods to find all volume claims
		for _, pod := range list.Items {
			resticVolumes := []string{}
			for _, volume := range pod.Spec.Volumes {
				claim := volume.VolumeSource.PersistentVolumeClaim
				if claim == nil {
					continue
				}
				pvc := corev1.PersistentVolumeClaim{}
				ref := types.NamespacedName{
					Namespace: ns,
					Name:      claim.ClaimName,
				}
				// Get PVC and update annotation
				err = client.Get(context.TODO(), ref, &pvc)
				if pvc.Annotations == nil {
					pvc.Annotations = make(map[string]string)
				}
				pvName := pvc.Spec.VolumeName
				action := findPVAction(pvs, pvName)
				pvc.Annotations[pvAnnotationKey] = action
				if pvc.Labels == nil {
					pvc.Labels = make(map[string]string)
				}
				pvc.Labels[uniqueBackupLabelKey] = pvBackupLabelValue
				err = client.Update(context.TODO(), &pvc)
				if action == migapi.PvCopyAction {
					resticVolumes = append(resticVolumes, volume.Name)
				}
			}
			if len(resticVolumes) > 0 {
				if pod.Annotations == nil {
					pod.Annotations = make(map[string]string)
				}
				pod.Annotations[resticPvBackupAnnotationKey] = strings.Join(resticVolumes[:], ",")
				if pod.Labels == nil {
					pod.Labels = make(map[string]string)
				}
				pod.Labels[uniqueBackupLabelKey] = pvBackupLabelValue
				err = client.Update(context.TODO(), &pod)
			}
		}
	}

	return nil
}

// Removes temporary annotations and labels used before backing up storage resources
func (t *Task) removeStorageResourceAnnotations() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	uniqueBackupLabelKey := fmt.Sprintf("%s-%s", pvBackupLabelKey, t.PlanResources.MigPlan.UID)
	labelSelector := map[string]string{
		uniqueBackupLabelKey: pvBackupLabelValue,
	}
	pvcList := corev1.PersistentVolumeClaimList{}
	options := k8sclient.MatchingLabels(labelSelector)
	err = client.List(context.TODO(), options, &pvcList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, pvc := range pvcList.Items {
		if pvc.Annotations != nil {
			delete(pvc.Annotations, pvAnnotationKey)
		}
		if pvc.Labels != nil {
			delete(pvc.Labels, uniqueBackupLabelKey)
		}
		err = client.Update(context.TODO(), &pvc)
	}
	pvList := corev1.PersistentVolumeList{}
	err = client.List(context.TODO(), options, &pvList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, pv := range pvList.Items {
		if pv.Annotations != nil {
			delete(pv.Annotations, pvAnnotationKey)
		}
		if pv.Labels != nil {
			delete(pv.Labels, uniqueBackupLabelKey)
		}
		err = client.Update(context.TODO(), &pv)
	}
	podList := corev1.PodList{}
	err = client.List(context.TODO(), options, &podList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, pod := range podList.Items {
		if pod.Annotations != nil {
			delete(pod.Annotations, resticPvBackupAnnotationKey)
		}
		if pod.Labels != nil {
			delete(pod.Labels, uniqueBackupLabelKey)
		}
		err = client.Update(context.TODO(), &pod)
	}
	return nil
}

func findPVAction(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Action
		}
	}
	return ""
}
