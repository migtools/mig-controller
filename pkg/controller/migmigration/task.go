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
	Created                 = ""
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

var PhaseOrder = map[string]int{
	Created:                 00, // 0-499 normal
	Started:                 1,
	WaitOnResticRestart:     10,
	ResticRestartCompleted:  11,
	BackupStarted:           20,
	BackupCompleted:         21,
	WaitOnBackupReplication: 30,
	BackupReplicated:        31,
	RestoreStarted:          40,
	RestoreCompleted:        41,
	BackupFailed:            501, // 500-999 errors
	RestoreFailed:           510,
	Completed:               1000, // Succeeded
}

// Phase Error
type PhaseNotValid struct {
	Name string
}

func (p PhaseNotValid) Error() string {
	return fmt.Sprintf("Phase %s not valid.", p.Name)
}

// Phase
type Phase struct {
	Name string
}

// Validate the phase.
func (p *Phase) Validate() error {
	_, found := PhaseOrder[p.Name]
	if !found {
		return &PhaseNotValid{Name: p.Name}
	}

	return nil
}

// Set the phase.
// Prevents rewind or set to invalid value.
func (p *Phase) Set(name string) {
	if !p.Before(name) {
		return
	}
	_, found := PhaseOrder[p.Name]
	if !found {
		log.Trace(&PhaseNotValid{Name: name})
		return
	}
	p.Name = name
}

// Phase is equal.
func (p Phase) Equals(other string) bool {
	return p.Name == other
}

// Phase is after `other`.
func (p Phase) After(other string) bool {
	nA, found := PhaseOrder[p.Name]
	if !found {
		log.Trace(&PhaseNotValid{Name: p.Name})
	}
	nB, found := PhaseOrder[other]
	if !found {
		log.Trace(&PhaseNotValid{Name: other})
	}
	return nA > nB
}

// The `other` phase is done.
// Same as p.Phase.Equals(other) || p.Phase.After(other)
func (p Phase) EqAfter(other string) bool {
	nA, found := PhaseOrder[p.Name]
	if !found {
		log.Trace(&PhaseNotValid{Name: p.Name})
	}
	nB, found := PhaseOrder[other]
	if !found {
		log.Trace(&PhaseNotValid{Name: other})
	}
	return nA >= nB
}

// Phase is before `other`.
func (p Phase) Before(other string) bool {
	nA, found := PhaseOrder[p.Name]
	if !found {
		log.Trace(&PhaseNotValid{Name: p.Name})
	}
	nB, found := PhaseOrder[other]
	if !found {
		log.Trace(&PhaseNotValid{Name: other})
	}
	return nA < nB
}

// Phase is final.
func (p Phase) Final() bool {
	n, found := PhaseOrder[p.Name]
	if !found {
		log.Trace(&PhaseNotValid{Name: p.Name})
	}
	return n >= 500
}

// Phase is final.
func (p Phase) Failed() bool {
	n, found := PhaseOrder[p.Name]
	if !found {
		log.Trace(&PhaseNotValid{Name: p.Name})
	}
	return n >= 500 && n < 1000
}

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
	Phase           Phase
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

	// Validate phase.
	err := t.Phase.Validate()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Started
	t.Phase.Set(Started)

	// Mount propagation workaround
	// TODO: Only bounce restic pod if cluster version is 3.7-3.9,
	// would require passing in cluster version to the controller.
	err = t.bounceResticPod()
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
	if t.Phase.Equals(Started) || t.Phase.Equals(WaitOnResticRestart) {
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
		t.Phase.Set(BackupCompleted)
	case velero.BackupPhaseFailed:
		reason := fmt.Sprintf(
			"Backup: %s/%s failed.",
			t.Backup.Namespace,
			t.Backup.Name)
		t.addErrors([]string{reason})
		t.Phase.Set(BackupFailed)
		return nil
	case velero.BackupPhasePartiallyFailed:
		reason := fmt.Sprintf(
			"Backup: %s/%s partially failed.",
			t.Backup.Namespace,
			t.Backup.Name)
		t.addErrors([]string{reason})
		t.Phase.Set(BackupFailed)
		return nil
	case velero.BackupPhaseFailedValidation:
		t.addErrors(t.Backup.Status.ValidationErrors)
		t.Phase.Set(BackupFailed)
		return nil
	default:
		t.Phase.Set(BackupStarted)
		return nil
	}

	t.Phase.Set(BackupCompleted)

	// Delete storage annotations
	err = t.removeStorageResourceAnnotations()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Wait on Backup replication.
	t.Phase.Set(WaitOnBackupReplication)
	backup, err := t.getReplicatedBackup()
	if err != nil {
		log.Trace(err)
		return err
	}
	if backup != nil {
		t.Phase.Set(BackupReplicated)
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
		t.Phase.Set(RestoreCompleted)
	case velero.RestorePhaseFailedValidation:
		t.addErrors(t.Restore.Status.ValidationErrors)
		t.Phase.Set(RestoreFailed)
		return nil
	case velero.RestorePhaseFailed:
		reason := fmt.Sprintf(
			"Restore: %s/%s failed.",
			t.Restore.Namespace,
			t.Restore.Name)
		t.addErrors([]string{reason})
		t.Phase.Set(RestoreFailed)
		return nil
	case velero.RestorePhasePartiallyFailed:
		reason := fmt.Sprintf(
			"Restore: %s/%s partially failed.",
			t.Restore.Namespace,
			t.Restore.Name)
		t.addErrors([]string{reason})
		t.Phase.Set(RestoreFailed)
		return nil
	default:
		t.Phase.Set(RestoreStarted)
		return nil
	}
	t.Phase.Set(RestoreCompleted)

	// Delete stage Pods
	if t.stage() {
		err = t.deleteStagePods()
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	// Done
	t.Phase.Set(Completed)

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
	if t.Phase.Equals(Started) {
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
	if t.Phase.Equals(Completed) {
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
