package velerorunner

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
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
	PlanResources   *migapi.PlanResources
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
