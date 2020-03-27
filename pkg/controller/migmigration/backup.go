package migmigration

import (
	"context"
	"fmt"
	"reflect"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the initial backup on the source cluster has been created
// and has the proper settings.
func (t *Task) ensureInitialBackup() (*velero.Backup, error) {
	newBackup, err := t.buildBackup(nil)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	newBackup.Labels[InitialBackupLabel] = t.UID()
	newBackup.Spec.ExcludedResources = excludedInitialResources
	delete(newBackup.Annotations, QuiesceAnnotation)
	foundBackup, err := t.getInitialBackup()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	if foundBackup == nil {
		client, err := t.getSourceClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		err = client.Create(context.TODO(), newBackup)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		return newBackup, nil
	}
	if !t.equalsBackup(newBackup, foundBackup) {
		client, err := t.getSourceClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		t.updateBackup(foundBackup)
		err = client.Update(context.TODO(), foundBackup)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
	}

	return foundBackup, nil
}

// Get the initial backup on the source cluster.
func (t *Task) getInitialBackup() (*velero.Backup, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[InitialBackupLabel] = t.UID()
	return t.getBackup(labels)
}

// Ensure the second backup on the source cluster has been created and
// has the proper settings.
func (t *Task) ensureStageBackup() (*velero.Backup, error) {
	newBackup, err := t.buildBackup(nil)
	if err != nil {
		return nil, err
	}
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			IncludedInStageBackupLabel: t.UID(),
		},
	}
	newBackup.Labels[StageBackupLabel] = t.UID()
	newBackup.Spec.IncludedResources = stagingResources
	newBackup.Spec.LabelSelector = &labelSelector
	foundBackup, err := t.getStageBackup()
	if err != nil {
		return nil, err
	}
	if foundBackup == nil {
		client, err := t.getSourceClient()
		if err != nil {
			return nil, err
		}
		err = client.Create(context.TODO(), newBackup)
		if err != nil {
			return nil, err
		}
		return newBackup, nil
	}
	if !t.equalsBackup(newBackup, foundBackup) {
		client, err := t.getSourceClient()
		if err != nil {
			return nil, err
		}
		err = client.Update(context.TODO(), foundBackup)
		if err != nil {
			return nil, err
		}
	}

	return foundBackup, nil
}

// Get the stage backup on the source cluster.
func (t *Task) getStageBackup() (*velero.Backup, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[StageBackupLabel] = t.UID()
	return t.getBackup(labels)
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
func (t Task) getBackup(labels map[string]string) (*velero.Backup, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	list := velero.BackupList{}
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

// Get whether a backup has completed on the source cluster.
func (t Task) hasBackupCompleted(backup *velero.Backup) (bool, []string) {
	completed := false
	reasons := []string{}
	switch backup.Status.Phase {
	case velero.BackupPhaseCompleted:
		completed = true
	case velero.BackupPhaseFailed:
		completed = true
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup: %s/%s partially failed.",
				backup.Namespace,
				backup.Name))
	case velero.BackupPhasePartiallyFailed:
		completed = true
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup: %s/%s partially failed.",
				backup.Namespace,
				backup.Name))
	case velero.BackupPhaseFailedValidation:
		reasons = backup.Status.ValidationErrors
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup: %s/%s validation failed.",
				backup.Namespace,
				backup.Name))
		completed = true
	}

	return completed, reasons
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
func (t *Task) buildBackup(includeClusterResources *bool) (*velero.Backup, error) {
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
		Spec: velero.BackupSpec{
			IncludeClusterResources: includeClusterResources,
		},
	}
	err = t.updateBackup(backup)
	return backup, err
}

// Update a Backups as desired for the source cluster.
func (t *Task) updateBackup(backup *velero.Backup) error {
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
		IncludedNamespaces:      t.sourceNamespaces(),
		ExcludedNamespaces:      []string{},
		IncludedResources:       t.BackupResources,
		ExcludedResources:       []string{},
		Hooks: velero.BackupHooks{
			Resources: []velero.BackupResourceHookSpec{},
		},
		IncludeClusterResources: backup.Spec.IncludeClusterResources,
	}

	return nil
}

func (t *Task) deleteBackups() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}

	labels := t.Owner.GetCorrelationLabels()
	labels["migmigration"] = string(t.Owner.GetUID())
	list := velero.BackupList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		log.Trace(err)
		return err
	}

	for _, backup := range list.Items {
		request := buildDeleteBackupRequest(backup.Name, backup.UID)
		if err := client.Create(context.TODO(), request); err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

func buildDeleteBackupRequest(name string, uid types.UID) *velero.DeleteBackupRequest {
	return &velero.DeleteBackupRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    migapi.VeleroNamespace,
			GenerateName: name + "-",
			Labels: map[string]string{
				velero.BackupNameLabel: label.GetValidName(name),
				velero.BackupUIDLabel:  string(uid),
			},
		},
		Spec: velero.DeleteBackupRequestSpec{
			BackupName: name,
		},
	}
}

// Determine whether backups are replicated by velero on the destination cluster.
func (t *Task) isBackupReplicated(backup *velero.Backup) (bool, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}
	replicated := velero.Backup{}
	err = client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		&replicated)
	if err == nil {
		return true, nil
	}
	if k8serrors.IsNotFound(err) {
		err = nil
	}

	return false, err
}

func findPVAction(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.Action
		}
	}
	return ""
}

func findPVStorageClass(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.StorageClass
		}
	}
	return ""
}

func findPVAccessMode(pvList migapi.PersistentVolumes, pvName string) corev1.PersistentVolumeAccessMode {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.AccessMode
		}
	}
	return ""
}

func findPVCopyMethod(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.CopyMethod
		}
	}
	return ""
}
