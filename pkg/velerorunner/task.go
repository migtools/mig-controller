package velerorunner

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var VeleroNamespace = "velero"

// Phases
const (
	Started                 = ""
	WaitOnResticRestart     = "WaitOnResticRestart"
	ResticRestartCompleted  = "ResticRestartCompleted"
	BackupStarted           = "BackupStarted"
	BackupCompleted         = "BackupCompleted"
	BackupFailed            = "BackupFailed"
	WaitOnBackupReplication = "WaitOnBackupReplication"
	BackupReplicated        = "BackupReplicated"
	RestoreStarted          = "RestoreStarted"
	RestoreCompleted        = "RestoreCompleted"
	RestoreFailed           = "RestoreFailed"
	Completed               = "Completed"
)

// A Velero task that provides the complete backup & restore workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A MigStage or MigMigration resource.
// PlanResources - A PlanRefResources.
// BackupResources - Resource types to be included in the backup.
// Phase - The task phase.
// Backup - A Backup created on the source cluster.
// Restore - A Restore created on the destination cluster.
type Task struct {
	Log             logr.Logger
	Client          k8sclient.Client
	Owner           migapi.MigResource
	PlanResources   *migapi.PlanResources
	BackupResources []string
	Phase           string
	Backup          *velero.Backup
	Restore         *velero.Restore
	Annotations     map[string]string
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
// err := task.Run()
// switch task.Phase {
//     case Complete:
//         ...
// }
//

// Run the task.
// Return `true` when run to completion.
func (t *Task) Run() error {
	t.logEnter()
	defer t.logExit()

	// Backup
	err := t.ensureBackup()
	if err != nil {
		return err
	}
	if t.Backup.Status.Phase != velero.BackupPhaseCompleted {
		t.Phase = BackupStarted
		return nil
	} else {
		t.Phase = BackupCompleted
	}

	// Wait on Backup replication.
	t.Phase = WaitOnBackupReplication
	backup, err := t.getReplicatedBackup()
	if err != nil {
		return err
	}
	if backup != nil {
		t.Phase = BackupReplicated
	} else {
		return nil
	}

	// Restore
	err = t.ensureRestore()
	if err != nil {
		return err
	}
	if t.Restore.Status.Phase != velero.RestorePhaseCompleted {
		t.Phase = RestoreStarted
		return nil
	} else {
		t.Phase = RestoreCompleted
	}

	// Done
	t.Phase = Completed

	return nil
}

// Get a client for the source cluster.
func (t *Task) getSourceClient() (k8sclient.Client, error) {
	return t.PlanResources.SrcMigCluster.GetClient(t.Client)
}

// Get a client for the destination cluster.
func (t *Task) getDestinationClient() (k8sclient.Client, error) {
	return t.PlanResources.DestMigCluster.GetClient(t.Client)
}

// Log task start/resumed.
func (t *Task) logEnter() {
	if t.Phase == Started {
		t.Log.Info("Task started.")
		return
	}
	t.Log.Info("Task resumed.", "phase", t.Phase)
}

// Log task exit/interrupted.
func (t *Task) logExit() {
	if t.Phase == Completed {
		t.Log.Info("Task completed.")
		return
	}
	backup := ""
	restore := ""
	if t.Backup != nil {
		backup = t.Backup.Name
	}
	if t.Restore != nil {
		restore = t.Restore.Name
	}
	t.Log.Info(
		"Task interrupted.",
		"phase", t.Phase,
		"backup",
		backup,
		"restore",
		restore)
}
