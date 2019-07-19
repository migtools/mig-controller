package migmigration

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// Phases
const (
	Created                       = ""
	Started                       = "Started"
	Prepare                       = "Prepare"
	EnsureInitialBackup           = "EnsureInitialBackup"
	InitialBackupCreated          = "InitialBackupCreated"
	InitialBackupFailed           = "InitialBackupFailed"
	AnnotateResources             = "AnnotateResources"
	EnsureStagePods               = "EnsureStagePods"
	StagePodsCreated              = "StagePodsCreated"
	RestartRestic                 = "RestartRestic"
	ResticRestarted               = "ResticRestarted"
	QuiesceApplications           = "QuiesceApplications"
	EnsureStageBackup             = "EnsureStageBackup"
	StageBackupCreated            = "StageBackupCreated"
	StageBackupFailed             = "StageBackupFailed"
	EnsureInitialBackupReplicated = "EnsureInitialBackupReplicated"
	EnsureStageBackupReplicated   = "EnsureStageBackupReplicated"
	EnsureStageRestore            = "EnsureStageRestore"
	StageRestoreCreated           = "StageRestoreCreated"
	StageRestoreFailed            = "StageRestoreFailed"
	EnsureFinalRestore            = "EnsureFinalRestore"
	FinalRestoreCreated           = "FinalRestoreCreated"
	FinalRestoreFailed            = "FinalRestoreFailed"
	EnsureStagePodsDeleted        = "EnsureStagePodsDeleted"
	EnsureAnnotationsDeleted      = "EnsureAnnotationsDeleted"
	Completed                     = "Completed"
)

// Steps
var Step = []string{
	Created,
	Started,
	Prepare,
	EnsureInitialBackup,
	InitialBackupCreated,
	InitialBackupFailed,
	AnnotateResources,
	EnsureStagePods,
	StagePodsCreated,
	RestartRestic,
	ResticRestarted,
	QuiesceApplications,
	EnsureStageBackup,
	StageBackupCreated,
	StageBackupFailed,
	EnsureInitialBackupReplicated,
	EnsureStageBackupReplicated,
	EnsureStageRestore,
	StageRestoreCreated,
	StageRestoreFailed,
	EnsureFinalRestore,
	FinalRestoreCreated,
	FinalRestoreFailed,
	EnsureStagePodsDeleted,
	EnsureAnnotationsDeleted,
	Completed,
}

// End phases.
var EndPhase = map[string]bool{
	InitialBackupFailed: true,
	StageBackupFailed:   true,
	FinalRestoreFailed:  true,
	StageRestoreFailed:  true,
	Completed:           true,
}

// Error phases.
var ErrorPhase = map[string]bool{
	InitialBackupFailed: true,
	StageBackupFailed:   true,
	FinalRestoreFailed:  true,
	StageRestoreFailed:  true,
}

// A Velero task that provides the complete backup & restore workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A MigMigration resource.
// PlanResources - A PlanRefResources.
// Annotations - Map of annotations to applied to the backup & restore
// BackupResources - Resource types to be included in the backup.
// Phase - The task phase.
// Requeue - The requeueAfter duration. 0 indicates no requeue.
// Errors - Migration errors.
type Task struct {
	Log             logr.Logger
	Client          k8sclient.Client
	Owner           *migapi.MigMigration
	PlanResources   *migapi.PlanResources
	Annotations     map[string]string
	BackupResources []string
	Phase           string
	Requeue         time.Duration
	Errors          []string
}

// Run the task.
// Each call will:
//   1. Run the current phase.
//   2. Update the phase to the next phase.
//   3. Set the Requeue (as appropriate).
//   4. Return.
func (t *Task) Run() error {
	t.Log.Info(
		"Migration [RUN]",
		"name",
		t.Owner.Name,
		"stage",
		t.stage(),
		"phase",
		t.Phase)

	t.Requeue = time.Millisecond * 100

	switch t.Phase {
	case Created:
		t.Phase = Started
	case Started:
		t.Phase = Prepare
	case Prepare:
		err := t.deleteAnnotations()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.ensureStagePodsDeleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if t.stage() {
			if t.hasPVs() {
				t.Phase = AnnotateResources
			} else {
				t.Phase = Completed
			}
		} else {
			t.Phase = EnsureInitialBackup
		}
	case EnsureInitialBackup:
		_, err := t.ensureInitialBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = InitialBackupCreated
		t.Requeue = 0
	case InitialBackupCreated:
		backup, err := t.ensureInitialBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		completed, reasons := t.hasBackupCompleted(backup)
		if err != nil {
			log.Trace(err)
			return err
		}
		if completed {
			if len(reasons) > 0 {
				t.addErrors(reasons)
				t.Phase = InitialBackupFailed
			} else {
				if t.hasPVs() {
					t.Phase = AnnotateResources
				} else {
					t.Phase = EnsureInitialBackupReplicated
				}
			}
		} else {
			t.Requeue = 0
		}
	case AnnotateResources:
		err := t.annotateStageResources()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = EnsureStagePods
	case EnsureStagePods:
		count, err := t.ensureStagePodsCreated()
		if err != nil {
			log.Trace(err)
			return err
		}
		if count > 0 {
			t.Phase = StagePodsCreated
			t.Requeue = 0
		} else {
			if t.quiesce() {
				t.Phase = QuiesceApplications
			} else {
				t.Phase = EnsureStageBackup
			}
		}
	case StagePodsCreated:
		started, err := t.ensureStagePodsStarted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if started {
			t.Phase = RestartRestic
		} else {
			t.Requeue = 0
		}
	case RestartRestic:
		err := t.restartResticPod()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = ResticRestarted
	case ResticRestarted:
		started, err := t.hasResticPodStarted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if started {
			if t.quiesce() {
				t.Phase = QuiesceApplications
			} else {
				t.Phase = EnsureStageBackup
			}
		} else {
			t.Requeue = time.Second * 3
		}
	case QuiesceApplications:
		err := t.quiesceApplications()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = EnsureStageBackup
	case EnsureStageBackup:
		_, err := t.ensureStageBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = StageBackupCreated
		t.Requeue = 0
	case StageBackupCreated:
		backup, err := t.ensureStageBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		completed, reasons := t.hasBackupCompleted(backup)
		if err != nil {
			log.Trace(err)
			return err
		}
		if completed {
			if len(reasons) > 0 {
				t.addErrors(reasons)
				t.Phase = StageBackupFailed
			} else {
				t.Phase = EnsureStageBackupReplicated
			}
		} else {
			t.Requeue = 0
		}
	case EnsureStageBackupReplicated:
		backup, err := t.getStageBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		replicated, err := t.isBackupReplicated(backup)
		if err != nil {
			log.Trace(err)
			return err
		}
		if replicated {
			if t.stage() {
				t.Phase = EnsureStageRestore
			} else {
				t.Phase = EnsureStageRestore
			}
		} else {
			t.Requeue = 0
		}
	case EnsureStageRestore:
		backup, err := t.getStageBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		_, err = t.ensureStageRestore()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = StageRestoreCreated
		t.Requeue = 0
	case StageRestoreCreated:
		restore, err := t.ensureStageRestore()
		if err != nil {
			log.Trace(err)
			return err
		}
		if restore == nil {
			return errors.New("Restore not found")
		}
		completed, reasons := t.hasRestoreCompleted(restore)
		if err != nil {
			log.Trace(err)
			return err
		}
		if completed {
			if len(reasons) > 0 {
				t.addErrors(reasons)
				t.Phase = StageRestoreFailed
			} else {
				t.Phase = EnsureStagePodsDeleted
			}
		} else {
			t.Requeue = 0
		}
	case EnsureStagePodsDeleted:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = EnsureAnnotationsDeleted
	case EnsureAnnotationsDeleted:
		err := t.deleteAnnotations()
		if err != nil {
			log.Trace(err)
			return err
		}
		if t.stage() {
			t.Phase = Completed
		} else {
			t.Phase = EnsureInitialBackupReplicated
		}
	case EnsureInitialBackupReplicated:
		backup, err := t.getInitialBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		replicated, err := t.isBackupReplicated(backup)
		if err != nil {
			log.Trace(err)
			return err
		}
		if replicated {
			t.Phase = EnsureFinalRestore
		} else {
			t.Requeue = 0
		}
	case EnsureFinalRestore:
		backup, err := t.getInitialBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		_, err = t.ensureFinalRestore()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = FinalRestoreCreated
		t.Requeue = 0
	case FinalRestoreCreated:
		restore, err := t.ensureFinalRestore()
		if err != nil {
			log.Trace(err)
			return err
		}
		if restore == nil {
			return errors.New("Restore not found")
		}
		completed, reasons := t.hasRestoreCompleted(restore)
		if err != nil {
			log.Trace(err)
			return err
		}
		if completed {
			if len(reasons) > 0 {
				t.addErrors(reasons)
				t.Phase = FinalRestoreFailed
			} else {
				t.Phase = Completed
			}
		} else {
			t.Requeue = 0
		}
	case StageBackupFailed, StageRestoreFailed:
		err := t.deleteAnnotations()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.ensureStagePodsDeleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Phase = Completed
	case InitialBackupFailed, FinalRestoreFailed:
		t.Phase = Completed
	case Completed:
		t.Requeue = 0
	}

	if t.Phase == Completed {
		t.Log.Info(
			"Migration [COMPLETED]",
			"name",
			t.Owner.Name)
	}

	return nil
}

// Migration UID.
func (t *Task) UID() string {
	return string(t.Owner.UID)
}

// Get whether the migration is stage.
func (t *Task) stage() bool {
	return t.Owner.Spec.Stage
}

// Get the migration namespaces.
func (t *Task) namespaces() []string {
	return t.PlanResources.MigPlan.Spec.Namespaces
}

// Get whether to quiesce pods.
func (t *Task) quiesce() bool {
	return t.Owner.Spec.QuiescePods
}

// Get a client for the source cluster.
func (t *Task) getSourceClient() (k8sclient.Client, error) {
	return t.PlanResources.SrcMigCluster.GetClient(t.Client)
}

// Get a client for the destination cluster.
func (t *Task) getDestinationClient() (k8sclient.Client, error) {
	return t.PlanResources.DestMigCluster.GetClient(t.Client)
}

// Get the persistent volumes included in the plan.
func (t *Task) getPVs() migapi.PersistentVolumes {
	return t.PlanResources.MigPlan.Spec.PersistentVolumes
}

// Get whether the associated plan lists any PVs.
func (t *Task) hasPVs() bool {
	return len(t.getPVs().List) > 0
}

// Get both source and destination clients.
func (t *Task) getBothClients() ([]k8sclient.Client, error) {
	list := []k8sclient.Client{}
	// Source
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	list = append(list, client)
	// Destination
	client, err = t.getDestinationClient()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	list = append(list, client)

	return list, nil
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	for _, error := range errors {
		t.Errors = append(t.Errors, error)
	}
}

// Get the current step.
// Returns: name, n, total.
func (t *Task) getStep() (string, int, int) {
	n := -1
	total := len(Step)
	for i, step := range Step {
		if step == t.Phase {
			n = i + 1
		}
	}

	return t.Phase, n, total
}
