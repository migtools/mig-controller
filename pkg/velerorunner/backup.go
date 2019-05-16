package velerorunner

import (
	"context"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// Ensure the backup on the source cluster has been created
// and has the proper settings.
func (t *Task) ensureBackup() error {
	newBackup, err := t.buildBackup()
	if err != nil {
		return err
	}
	foundBackup, err := t.getBackup()
	if err != nil {
		return err
	}
	if foundBackup == nil {
		t.Backup = newBackup
		client, err := t.getSourceClient()
		if err != nil {
			return err
		}
		err = client.Create(context.TODO(), newBackup)
		if err != nil {
			return err
		}
		return nil
	}
	t.Backup = foundBackup
	if !t.equalsBackup(newBackup, foundBackup) {
		client, err := t.getSourceClient()
		if err != nil {
			return err
		}
		t.updateBackup(foundBackup)
		err = client.Update(context.TODO(), foundBackup)
		if err != nil {
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
	storage := t.PlanResources.MigStorage
	location, err := storage.GetBSL(client)
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
	storage := t.PlanResources.MigStorage
	location, err := storage.GetVSL(client)
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
	backup := &velero.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    VeleroNamespace,
			Annotations:  t.Annotations,
		},
	}
	err := t.updateBackup(backup)
	return backup, err
}

// Update a Backups as desired for the source cluster.
func (t *Task) updateBackup(backup *velero.Backup) error {
	namespaces := t.PlanResources.MigAssets.Spec.Namespaces
	backupLocation, err := t.getBSL()
	if err != nil {
		return err
	}
	snapshotLocation, err := t.getVSL()
	if err != nil {
		return err
	}
	backup.Spec = velero.BackupSpec{
		StorageLocation:         backupLocation.Name,
		VolumeSnapshotLocations: []string{snapshotLocation.Name},
		TTL:                metav1.Duration{Duration: 720 * time.Hour},
		IncludedNamespaces: namespaces,
		ExcludedNamespaces: []string{},
		IncludedResources:  t.BackupResources,
		ExcludedResources:  []string{},
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
