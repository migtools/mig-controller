package velerorunner

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var VeleroNamespace = "velero"

// A Velero task that provides the complete backup & restore workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A MigStage or MigMigration resource.
// PlanResources - A PlanRefResources.
// BackupResources - Resource types to be included in the backup.
// Backup - A Backup created on the source cluster.
// Restore - A Restore created on the destination cluster.
// ClientCache - Client cache keyed by cluster.
type Task struct {
	Log             logr.Logger
	Client          k8sclient.Client
	Owner           migapi.MigResource
	PlanResources   *migapi.PlanRefResources
	BackupResources []string
	Backup          *velero.Backup
	Restore         *velero.Restore
	ClientCache     map[*migapi.MigCluster]k8sclient.Client
}

// Reconcile() Example:
//
// task := velerorunner.Task{
//     Log: log,
//     Client: r,
//     Owner: migration,
//     PlanResources: plan.GetPlanResources(),
// }
//
// completed, err := task.Run()
//

// Run the task.
// Return `true` when run to completion.
func (t *Task) Run() (bool, error) {
	// Backup
	err := t.ensureBackup()
	if err != nil {
		return false, err
	}
	if t.Backup.Status.Phase != velero.BackupPhaseCompleted {
		t.Log.Info(
			"Waiting for backup to complete.",
			"owner",
			t.Owner.GetName(),
			"backup",
			t.Backup.Name)
		return false, nil
	}
	t.Log.Info(
		"Backup has completed.",
		"owner",
		t.Owner.GetName(),
		"backup",
		t.Backup.Name)

	backup, err := t.getReplicatedBackup()
	if err != nil {
		return false, err
	}
	if backup == nil {
		t.Log.Info(
			"Waiting for backup to be replicated to the destination.",
			"owner",
			t.Owner.GetName(),
			"backup",
			t.Backup.Name)
		return false, nil
	}
	// Restore
	err = t.ensureRestore()
	if err != nil {
		return false, err
	}
	if t.Restore.Status.Phase != velero.RestorePhaseCompleted {
		t.Log.Info(
			"Waiting for restore to complete.",
			"owner",
			t.Owner.GetName(),
			"restore",
			t.Restore.Name)
		return false, nil
	}
	t.Log.Info(
		"Restore has completed.",
		"owner",
		t.Owner.GetName(),
		"restore",
		t.Restore.Name)

	return true, nil
}

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

	return location, nil
}

// Build a Backups as desired for the source cluster.
func (t *Task) buildBackup() (*velero.Backup, error) {
	backup := &velero.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    VeleroNamespace,
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
	backup.Spec = velero.BackupSpec{
		StorageLocation:         backupLocation.Name,
		VolumeSnapshotLocations: []string{"aws-default"},
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

// Ensure the restore on the destination cluster has been
// created  and has the proper settings.
func (t *Task) ensureRestore() error {
	newRestore := t.buildRestore()
	foundRestore, err := t.getRestore()
	if err != nil {
		return err
	}
	if foundRestore == nil {
		t.Restore = newRestore
		client, err := t.getDestinationClient()
		if err != nil {
			return err
		}
		err = client.Create(context.TODO(), newRestore)
		if err != nil {
			return err
		}
		return nil
	}
	t.Restore = foundRestore
	if !t.equalsRestore(newRestore, foundRestore) {
		t.updateRestore(foundRestore)
		client, err := t.getDestinationClient()
		if err != nil {
			return err
		}
		err = client.Update(context.TODO(), foundRestore)
		if err != nil {
			return err
		}
	}

	return nil
}

// Get whether the two Restores are equal.
func (t *Task) equalsRestore(a, b *velero.Restore) bool {
	match := a.Spec.BackupName == b.Spec.BackupName &&
		*a.Spec.RestorePVs == *b.Spec.RestorePVs
	return match
}

// Get an existing Restore on the destination cluster.
func (t Task) getRestore() (*velero.Restore, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	list := velero.RestoreList{}
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

// Build a Restore as desired for the destination cluster.
func (t *Task) buildRestore() *velero.Restore {
	restore := &velero.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    VeleroNamespace,
		},
	}
	t.updateRestore(restore)
	return restore
}

// Update a Restore as desired for the destination cluster.
func (t *Task) updateRestore(restore *velero.Restore) {
	restorePVs := true
	restore.Spec = velero.RestoreSpec{
		BackupName: t.Backup.Name,
		RestorePVs: &restorePVs,
	}
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

// Get a client for the source cluster using the cache.
func (t *Task) getSourceClient() (k8sclient.Client, error) {
	return t.getClient(t.PlanResources.SrcMigCluster)
}

// Get a client for the destination cluster using the cache.
func (t *Task) getDestinationClient() (k8sclient.Client, error) {
	return t.getClient(t.PlanResources.DestMigCluster)
}

// Get a client for the cluster using the cache.
func (t *Task) getClient(cluster *migapi.MigCluster) (k8sclient.Client, error) {
	if t.ClientCache == nil {
		t.ClientCache = map[*migapi.MigCluster]k8sclient.Client{}
	}
	client, found := t.ClientCache[cluster]
	if found {
		return client, nil
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	} else {
		t.ClientCache[cluster] = client
	}

	return client, nil
}
