package migmigration

import (
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Annotation Keys
const MigQuiesceAnnotationKey = "openshift.io/migrate-quiesce-pods"

// Phases
const (
	Started                 = "Started"
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
// Owner - A MigMigration resource.
// PlanResources - A PlanRefResources.
// Annotations - Map of annotations to applied to the backup & restore
// BackupResources - Resource types to be included in the backup.
// Phase - The task phase.
// Errors - Migration errors.
// Backup - A Backup created on the source cluster.
// Restore - A Restore created on the destination cluster.
type Task struct {
	Log             logr.Logger
	Client          k8sclient.Client
	Owner           *migapi.MigMigration
	PlanResources   *migapi.PlanResources
	Annotations     map[string]string
	BackupResources []string
	Phase           string
	Errors          []string
	Backup          *velero.Backup
	Restore         *velero.Restore
}

// Reconcile() Example:
//
// task := Task{
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

	// Started
	if t.Phase == "" {
		t.Phase = Started
	}

	// Mount propagation workaround
	// TODO: Only bounce restic pod if cluster version is 3.7-3.9,
	// would require passing in cluster version to the controller.
	err := t.bounceResticPod()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Annotate persistent storage resources with actions
	err = t.annotateStorageResources()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Return unless restic restart has finished
	if t.Phase == Started || t.Phase == WaitOnResticRestart {
		return nil
	}

	// Backup
	err = t.ensureBackup()
	if err != nil {
		log.Trace(err)
		return err
	}
	switch t.Backup.Status.Phase {
	case velero.BackupPhaseCompleted:
		t.Phase = BackupCompleted
	case velero.BackupPhaseFailed:
		reason := fmt.Sprintf(
			"Backup: %s/%s failed.",
			t.Backup.Namespace,
			t.Backup.Name)
		t.addErrors([]string{reason})
		t.Phase = BackupFailed
		return nil
	case velero.BackupPhaseFailedValidation:
		t.addErrors(t.Backup.Status.ValidationErrors)
		t.Phase = BackupFailed
		return nil
	default:
		t.Phase = BackupStarted
		return nil
	}

	t.Phase = BackupCompleted

	// Delete storage annotations
	err = t.removeStorageResourceAnnotations()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Wait on Backup replication.
	t.Phase = WaitOnBackupReplication
	backup, err := t.getReplicatedBackup()
	if err != nil {
		log.Trace(err)
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
		log.Trace(err)
		return err
	}
	switch t.Restore.Status.Phase {
	case velero.RestorePhaseCompleted:
		t.Phase = RestoreCompleted
	case velero.RestorePhaseFailedValidation:
		t.addErrors(t.Restore.Status.ValidationErrors)
		t.Phase = RestoreFailed
		return nil
	case velero.RestorePhaseFailed:
		reason := fmt.Sprintf(
			"Restore: %s/%s failed.",
			t.Restore.Namespace,
			t.Restore.Name)
		t.addErrors([]string{reason})
		t.Phase = RestoreFailed
		return nil
	default:
		t.Phase = RestoreStarted
		return nil
	}
	t.Phase = RestoreCompleted

	// Delete stage Pods
	if t.stage() {
		err = t.deleteStagePods()
		if err != nil {
			log.Trace(err)
			return err
		}
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

// Get whether the migration is stage.
func (t *Task) stage() bool {
	return t.Owner.Spec.Stage
}

// Get whether to quiesce pods.
func (t *Task) quiesce() bool {
	return t.Owner.Spec.QuiescePods
}

// Log task start/resumed.
func (t *Task) logEnter() {
	if t.Phase == Started {
		t.Log.Info(
			"Migration: started.",
			"name",
			t.Owner.Name,
			"stage",
			t.stage())
		return
	}
	t.Log.Info(
		"Migration: resumed.",
		"name",
		t.Owner.Name,
		"phase",
		t.Phase)
}

// Log task exit/interrupted.
func (t *Task) logExit() {
	if t.Phase == Completed {
		t.Log.Info("Migration completed.")
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
		"Migration: waiting.",
		"name",
		t.Owner.Name,
		"phase", t.Phase,
		"backup",
		backup,
		"restore",
		restore)
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	for _, error := range errors {
		t.Errors = append(t.Errors, error)
	}
}
