package migmigration

import (
	"context"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Requeue
var FastReQ = time.Duration(time.Millisecond * 100)
var PollReQ = time.Duration(time.Second * 3)
var NoReQ = time.Duration(0)

// Phases
const (
	Created                         = ""
	Started                         = "Started"
	Prepare                         = "Prepare"
	EnsureCloudSecretPropagated     = "EnsureCloudSecretPropagated"
	PreBackupHooks                  = "PreBackupHooks"
	PostBackupHooks                 = "PostBackupHooks"
	PreRestoreHooks                 = "PreRestoreHooks"
	PostRestoreHooks                = "PostRestoreHooks"
	PreBackupHooksFailed            = "PreBackupHooksFailed"
	PostBackupHooksFailed           = "PostBackupHooksFailed"
	PreRestoreHooksFailed           = "PreRestoreHooksFailed"
	PostRestoreHooksFailed          = "PostRestoreHooksFailed"
	EnsureInitialBackup             = "EnsureInitialBackup"
	InitialBackupCreated            = "InitialBackupCreated"
	InitialBackupFailed             = "InitialBackupFailed"
	AnnotateResources               = "AnnotateResources"
	EnsureStagePodsFromRunning      = "EnsureStagePodsFromRunning"
	EnsureStagePodsFromTemplates    = "EnsureStagePodsFromTemplates"
	EnsureStagePodsFromOrphanedPVCs = "EnsureStagePodsFromOrphanedPVCs"
	StagePodsCreated                = "StagePodsCreated"
	StagePodsFailed                 = "StagePodsFailed"
	RestartRestic                   = "RestartRestic"
	ResticRestarted                 = "ResticRestarted"
	QuiesceApplications             = "QuiesceApplications"
	EnsureQuiesced                  = "EnsureQuiesced"
	UnQuiesceApplications           = "UnQuiesceApplications"
	EnsureStageBackup               = "EnsureStageBackup"
	StageBackupCreated              = "StageBackupCreated"
	StageBackupFailed               = "StageBackupFailed"
	EnsureInitialBackupReplicated   = "EnsureInitialBackupReplicated"
	EnsureStageBackupReplicated     = "EnsureStageBackupReplicated"
	EnsureStageRestore              = "EnsureStageRestore"
	StageRestoreCreated             = "StageRestoreCreated"
	StageRestoreFailed              = "StageRestoreFailed"
	EnsureFinalRestore              = "EnsureFinalRestore"
	FinalRestoreCreated             = "FinalRestoreCreated"
	FinalRestoreFailed              = "FinalRestoreFailed"
	Verification                    = "Verification"
	EnsureStagePodsDeleted          = "EnsureStagePodsDeleted"
	EnsureStagePodsTerminated       = "EnsureStagePodsTerminated"
	EnsureAnnotationsDeleted        = "EnsureAnnotationsDeleted"
	EnsureMigratedDeleted           = "EnsureMigratedDeleted"
	DeleteMigrated                  = "DeleteMigrated"
	DeleteBackups                   = "DeleteBackups"
	DeleteRestores                  = "DeleteRestores"
	MigrationFailed                 = "MigrationFailed"
	Canceling                       = "Canceling"
	Canceled                        = "Canceled"
	Rollback                        = "Rollback"
	Completed                       = "Completed"
)

// Flags
const (
	Quiesce      = 0x01 // Only when QuiescePods (true).
	HasStagePods = 0x02 // Only when stage pods created.
	HasPVs       = 0x04 // Only when PVs migrated.
	HasVerify    = 0x08 // Only when the plan has enabled verification
	HasISs       = 0x10 // Only when ISs migrated
)

type Itinerary struct {
	Name  string
	Steps []Step
}

var StageItinerary = Itinerary{
	Name: "Stage",
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: EnsureCloudSecretPropagated},
		{phase: EnsureStagePodsFromRunning, all: HasPVs},
		{phase: EnsureStagePodsFromTemplates, all: HasPVs},
		{phase: EnsureStagePodsFromOrphanedPVCs, all: HasPVs},
		{phase: StagePodsCreated, all: HasStagePods},
		{phase: AnnotateResources, any: HasPVs | HasISs},
		{phase: RestartRestic, all: HasStagePods},
		{phase: ResticRestarted, all: HasStagePods},
		{phase: QuiesceApplications, all: Quiesce},
		{phase: EnsureQuiesced, all: Quiesce},
		{phase: EnsureStageBackup, any: HasPVs | HasISs},
		{phase: StageBackupCreated, any: HasPVs | HasISs},
		{phase: EnsureStageBackupReplicated, any: HasPVs | HasISs},
		{phase: EnsureStageRestore, any: HasPVs | HasISs},
		{phase: StageRestoreCreated, any: HasPVs | HasISs},
		{phase: EnsureStagePodsDeleted, all: HasStagePods},
		{phase: EnsureStagePodsTerminated, all: HasStagePods},
		{phase: EnsureAnnotationsDeleted, any: HasPVs | HasISs},
		{phase: Completed},
	},
}

var FinalItinerary = Itinerary{
	Name: "Final",
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: EnsureCloudSecretPropagated},
		{phase: PreBackupHooks},
		{phase: EnsureInitialBackup},
		{phase: InitialBackupCreated},
		{phase: EnsureStagePodsFromRunning, all: HasPVs},
		{phase: EnsureStagePodsFromTemplates, all: HasPVs},
		{phase: EnsureStagePodsFromOrphanedPVCs, all: HasPVs},
		{phase: StagePodsCreated, all: HasStagePods},
		{phase: AnnotateResources, any: HasPVs | HasISs},
		{phase: RestartRestic, all: HasStagePods},
		{phase: ResticRestarted, all: HasStagePods},
		{phase: QuiesceApplications, all: Quiesce},
		{phase: EnsureQuiesced, all: Quiesce},
		{phase: EnsureStageBackup, any: HasPVs | HasISs},
		{phase: StageBackupCreated, any: HasPVs | HasISs},
		{phase: EnsureStageBackupReplicated, any: HasPVs | HasISs},
		{phase: EnsureStageRestore, any: HasPVs | HasISs},
		{phase: StageRestoreCreated, any: HasPVs | HasISs},
		{phase: EnsureStagePodsDeleted, all: HasStagePods},
		{phase: EnsureStagePodsTerminated, all: HasStagePods},
		{phase: EnsureAnnotationsDeleted, any: HasPVs | HasISs},
		{phase: EnsureInitialBackupReplicated},
		{phase: PostBackupHooks},
		{phase: PreRestoreHooks},
		{phase: EnsureFinalRestore},
		{phase: FinalRestoreCreated},
		{phase: PostRestoreHooks},
		{phase: Verification, all: HasVerify},
		{phase: Completed},
	},
}

var CancelItinerary = Itinerary{
	Name: "Cancel",
	Steps: []Step{
		{phase: Canceling},
		{phase: DeleteBackups},
		{phase: DeleteRestores},
		{phase: EnsureStagePodsDeleted, all: HasStagePods},
		{phase: EnsureAnnotationsDeleted, any: HasPVs | HasISs},
		{phase: Canceled},
		{phase: Completed},
	},
}

var FailedItinerary = Itinerary{
	Name: "Failed",
	Steps: []Step{
		{phase: MigrationFailed},
		{phase: EnsureStagePodsDeleted, all: HasStagePods},
		{phase: EnsureAnnotationsDeleted, any: HasPVs | HasISs},
		{phase: Completed},
	},
}

var RollbackItinerary = Itinerary{
	Name: "Rollback",
	Steps: []Step{
		{phase: Rollback},
		{phase: DeleteBackups},
		{phase: DeleteRestores},
		{phase: EnsureStagePodsDeleted, all: HasStagePods},
		{phase: EnsureAnnotationsDeleted, any: HasPVs | HasISs},
		{phase: DeleteMigrated},
		{phase: EnsureMigratedDeleted},
		{phase: UnQuiesceApplications, all: Quiesce},
		{phase: Completed},
	},
}

// Step
type Step struct {
	// A phase name.
	phase string
	// Step included when ALL flags evaluate true.
	all uint8
	// Step included when ANY flag evaluates true.
	any uint8
}

// Get a progress report.
// Returns: phase, n, total.
func (r Itinerary) progressReport(phase string) (string, int, int) {
	n := 0
	total := len(r.Steps)
	for i, step := range r.Steps {
		if step.phase == phase {
			n = i + 1
			break
		}
	}

	return phase, n, total
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
// Itinerary - The phase itinerary.
// Errors - Migration errors.
// Failed - Task phase has failed.
type Task struct {
	Log             logr.Logger
	Client          k8sclient.Client
	Owner           *migapi.MigMigration
	PlanResources   *migapi.PlanResources
	Annotations     map[string]string
	BackupResources mapset.Set
	Phase           string
	Requeue         time.Duration
	Itinerary       Itinerary
	Errors          []string
}

// Run the task.
// Each call will:
//   1. Run the current phase.
//   2. Update the phase to the next phase.
//   3. Set the Requeue (as appropriate).
//   4. Return.
func (t *Task) Run() error {
	t.Log.Info("[RUN]", "stage", t.stage(), "phase", t.Phase)

	err := t.init()
	if err != nil {
		return err
	}

	// Run the current phase.
	switch t.Phase {
	case Created, Started, Rollback:
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case Prepare:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			return liberr.Wrap(err)
		}
		err = t.deleteAnnotations()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureCloudSecretPropagated:
		count := 0
		for _, cluster := range t.getBothClusters() {
			propagated, err := t.veleroPodCredSecretPropagated(cluster)
			if err != nil {
				return liberr.Wrap(err)
			}
			if propagated {
				count++
			} else {
				break
			}
		}
		if count == 2 {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case PreBackupHooks:
		status, err := t.runHooks(migapi.PreBackupHookPhase)
		if err != nil {
			t.fail(PreBackupHooksFailed, []string{err.Error()})
			return liberr.Wrap(err)
		}
		if status {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case EnsureInitialBackup:
		_, err := t.ensureInitialBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case InitialBackupCreated:
		backup, err := t.getInitialBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		completed, reasons := t.hasBackupCompleted(backup)
		if completed {
			if len(reasons) > 0 {
				t.fail(InitialBackupFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.Requeue = NoReQ
		}
	case AnnotateResources:
		err := t.annotateStageResources()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromRunning:
		err := t.ensureStagePodsFromRunning()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromTemplates:
		err := t.ensureStagePodsFromTemplates()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromOrphanedPVCs:
		err := t.ensureStagePodsFromOrphanedPVCs()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StagePodsCreated:
		report, err := t.ensureStagePodsStarted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if report.failed {
			t.fail(StagePodsFailed, report.reasons)
			break
		}
		if report.started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case RestartRestic:
		err := t.restartResticPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = PollReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case ResticRestarted:
		started, err := t.haveResticPodsStarted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case QuiesceApplications:
		err := t.quiesceApplications()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureQuiesced:
		quiesced, err := t.ensureQuiescedPodsTerminated()
		if err != nil {
			return liberr.Wrap(err)
		}
		if quiesced {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case UnQuiesceApplications:
		err := t.unQuiesceApplications()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStageBackup:
		_, err := t.ensureStageBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StageBackupCreated:
		backup, err := t.getStageBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		completed, reasons := t.hasBackupCompleted(backup)
		if completed {
			if len(reasons) > 0 {
				t.fail(StageBackupFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.Requeue = NoReQ
		}
	case EnsureStageBackupReplicated:
		backup, err := t.getStageBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		replicated, err := t.isBackupReplicated(backup)
		if err != nil {
			return liberr.Wrap(err)
		}
		if replicated {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case PostBackupHooks:
		status, err := t.runHooks(migapi.PostBackupHookPhase)
		if err != nil {
			t.fail(PostBackupHooksFailed, []string{err.Error()})
			return liberr.Wrap(err)
		}
		if status {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case PreRestoreHooks:
		status, err := t.runHooks(migapi.PreRestoreHookPhase)
		if err != nil {
			t.fail(PreRestoreHooksFailed, []string{err.Error()})
			return liberr.Wrap(err)
		}
		if status {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case EnsureStageRestore:
		_, err := t.ensureStageRestore()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StageRestoreCreated:
		restore, err := t.getStageRestore()
		if err != nil {
			return liberr.Wrap(err)
		}
		if restore == nil {
			return errors.New("Restore not found")
		}
		completed, reasons := t.hasRestoreCompleted(restore)
		if completed {
			t.setResticConditions(restore)
			if len(reasons) > 0 {
				t.fail(StageRestoreFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.Requeue = NoReQ
		}
	case EnsureStagePodsDeleted:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsTerminated:
		terminated, err := t.ensureStagePodsTerminated()
		if err != nil {
			return liberr.Wrap(err)
		}
		if terminated {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case EnsureAnnotationsDeleted:
		if !t.keepAnnotations() {
			err := t.deleteAnnotations()
			if err != nil {
				return liberr.Wrap(err)
			}
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureInitialBackupReplicated:
		backup, err := t.getInitialBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		replicated, err := t.isBackupReplicated(backup)
		if err != nil {
			return liberr.Wrap(err)
		}
		if replicated {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case EnsureFinalRestore:
		backup, err := t.getInitialBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		if backup == nil {
			return errors.New("Backup not found")
		}
		_, err = t.ensureFinalRestore()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case FinalRestoreCreated:
		restore, err := t.getFinalRestore()
		if err != nil {
			return liberr.Wrap(err)
		}
		if restore == nil {
			return errors.New("Restore not found")
		}
		completed, reasons := t.hasRestoreCompleted(restore)
		if completed {
			if len(reasons) > 0 {
				t.fail(FinalRestoreFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.Requeue = NoReQ
		}
	case PostRestoreHooks:
		status, err := t.runHooks(migapi.PostRestoreHookPhase)
		if err != nil {
			t.fail(PostRestoreHooksFailed, []string{err.Error()})
			return liberr.Wrap(err)
		}
		if status {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = NoReQ
		}
	case Verification:
		completed, err := t.VerificationCompleted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if completed {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case Canceling:
		// Skip directly to Completed if the Cancel was set on a Rollback migration.
		if t.rollback() {
			t.Phase = Completed
		}
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     Canceling,
			Status:   True,
			Reason:   Cancel,
			Category: Advisory,
			Message:  CancelInProgressMessage,
			Durable:  true,
		})
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}

	case MigrationFailed:
		t.Phase = Completed

	case DeleteMigrated:
		err := t.deleteMigrated()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureMigratedDeleted:
		deleted, err := t.ensureMigratedResourcesDeleted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if deleted {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case DeleteBackups:
		if err := t.deleteBackups(); err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DeleteRestores:
		if err := t.deleteRestores(); err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case Canceled:
		t.Owner.Status.DeleteCondition(Canceling)
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     Canceled,
			Status:   True,
			Reason:   Cancel,
			Category: Advisory,
			Message:  CanceledMessage,
			Durable:  true,
		})
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case Completed:
	default:
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	}

	if t.Phase == Completed {
		t.Requeue = NoReQ
		t.Log.Info("[COMPLETED]")
	}

	return nil
}

// Initialize.
func (t *Task) init() error {
	t.Requeue = FastReQ
	if t.failed() {
		t.Itinerary = FailedItinerary
	} else if t.canceled() {
		t.Itinerary = CancelItinerary
	} else if t.rollback() {
		t.Itinerary = RollbackItinerary
	} else if t.stage() {
		t.Itinerary = StageItinerary
	} else {
		t.Itinerary = FinalItinerary
	}
	if t.Owner.Status.Itinerary != t.Itinerary.Name {
		t.Phase = t.Itinerary.Steps[0].phase
	}

	hasImageStreams, err := t.hasImageStreams()
	if err != nil {
		return err
	}

	if t.stage() && (!t.hasPVs() && !hasImageStreams) {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StageNoOp,
			Status:   True,
			Category: migapi.Warn,
			Message:  StageNoOpMessage,
			Durable:  true,
		})
	}
	return nil
}

// Advance the task to the next phase.
func (t *Task) next() error {
	current := -1
	for i, step := range t.Itinerary.Steps {
		if step.phase != t.Phase {
			continue
		}
		current = i
		break
	}
	if current == -1 {
		t.Phase = Completed
		return nil
	}
	for n := current + 1; n < len(t.Itinerary.Steps); n++ {
		next := t.Itinerary.Steps[n]
		flag, err := t.allFlags(next)
		if err != nil {
			return liberr.Wrap(err)
		}
		if !flag {
			continue
		}
		flag, err = t.anyFlags(next)
		if err != nil {
			return liberr.Wrap(err)
		}
		if !flag {
			continue
		}
		t.Phase = next.phase
		return nil
	}
	t.Phase = Completed
	return nil
}

// Evaluate `all` flags.
func (t *Task) allFlags(step Step) (bool, error) {
	if step.all&HasPVs != 0 && !t.hasPVs() {
		return false, nil
	}
	if step.all&HasStagePods != 0 && !t.Owner.Status.HasCondition(StagePodsCreated) {
		return false, nil
	}
	if step.all&Quiesce != 0 && !t.quiesce() {
		return false, nil
	}
	if step.all&HasVerify != 0 && !t.hasVerify() {
		return false, nil
	}
	hasImageStream, err := t.hasImageStreams()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if step.all&HasISs != 0 && hasImageStream {
		return false, nil
	}

	return true, nil
}

// Evaluate `any` flags.
func (t *Task) anyFlags(step Step) (bool, error) {
	if step.any&HasPVs != 0 && t.hasPVs() {
		return true, nil
	}
	if step.any&HasStagePods != 0 && t.Owner.Status.HasCondition(StagePodsCreated) {
		return true, nil
	}
	if step.any&Quiesce != 0 && t.quiesce() {
		return true, nil
	}
	if step.any&HasVerify != 0 && t.hasVerify() {
		return true, nil
	}
	hasImageStream, err := t.hasImageStreams()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if step.any&HasISs != 0 && hasImageStream {
		return true, nil
	}
	return step.any == uint8(0), nil
}

// Phase fail.
func (t *Task) fail(nextPhase string, reasons []string) {
	t.addErrors(reasons)
	t.Owner.AddErrors(t.Errors)
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     Failed,
		Status:   True,
		Reason:   t.Phase,
		Category: Advisory,
		Message:  FailedMessage,
		Durable:  true,
	})
	t.Phase = nextPhase
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	for _, error := range errors {
		t.Errors = append(t.Errors, error)
	}
}

// Migration UID.
func (t *Task) UID() string {
	return string(t.Owner.UID)
}

// Get whether the migration has failed
func (t *Task) failed() bool {
	return t.Owner.HasErrors() || t.Owner.Status.HasCondition(Failed)
}

// Get whether the migration is cancelled.
func (t *Task) canceled() bool {
	return t.Owner.Spec.Canceled || t.Owner.Status.HasAnyCondition(Canceled, Canceling)
}

// Get whether the migration is rollback.
func (t *Task) rollback() bool {
	return t.Owner.Spec.Rollback
}

// Get whether the migration is stage.
func (t *Task) stage() bool {
	return t.Owner.Spec.Stage
}

// Get the migration namespaces with mapping.
func (t *Task) namespaces() []string {
	return t.PlanResources.MigPlan.Spec.Namespaces
}

// Get the migration source namespaces without mapping.
func (t *Task) sourceNamespaces() []string {
	return t.PlanResources.MigPlan.GetSourceNamespaces()
}

// Get the migration source namespaces without mapping.
func (t *Task) destinationNamespaces() []string {
	return t.PlanResources.MigPlan.GetDestinationNamespaces()
}

// Get whether to quiesce pods.
func (t *Task) quiesce() bool {
	return t.Owner.Spec.QuiescePods
}

// Get whether to retain annotations
func (t *Task) keepAnnotations() bool {
	return t.Owner.Spec.KeepAnnotations
}

// Get a client for the source cluster.
func (t *Task) getSourceClient() (compat.Client, error) {
	return t.PlanResources.SrcMigCluster.GetClient(t.Client)
}

// Get a client for the destination cluster.
func (t *Task) getDestinationClient() (compat.Client, error) {
	return t.PlanResources.DestMigCluster.GetClient(t.Client)
}

// Get the persistent volumes included in the plan which are not skipped.
func (t *Task) getPVs() migapi.PersistentVolumes {
	volumes := []migapi.PV{}
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action != migapi.PvSkipAction {
			volumes = append(volumes, pv)
		}
	}
	pvList := t.PlanResources.MigPlan.Spec.PersistentVolumes.DeepCopy()
	pvList.List = volumes
	return *pvList
}

// Get the persistentVolumeClaims / action mapping included in the plan which are not skipped.
func (t *Task) getPVCs() map[k8sclient.ObjectKey]migapi.PV {
	claims := map[k8sclient.ObjectKey]migapi.PV{}
	for _, pv := range t.getPVs().List {
		claimKey := k8sclient.ObjectKey{
			Name:      pv.PVC.Name,
			Namespace: pv.PVC.Namespace,
		}
		claims[claimKey] = pv
	}
	return claims
}

// Get whether the associated plan lists not skipped PVs.
func (t *Task) hasPVs() bool {
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action != migapi.PvSkipAction {
			return true
		}
	}
	return false
}

// Get whether the associated plan has imagestreams to be migrated
func (t *Task) hasImageStreams() (bool, error) {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return false, err
	}
	for _, ns := range t.sourceNamespaces() {
		imageStreamList := imagev1.ImageStreamList{}
		err := client.List(context.Background(), &k8sclient.ListOptions{Namespace: ns}, &imageStreamList)
		if err != nil {
			log.Trace(err)
			return false, err
		}
		if len(imageStreamList.Items) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// Get whether the verification is desired
func (t *Task) hasVerify() bool {
	return t.Owner.Spec.Verify
}

// Get both source and destination clusters.
func (t *Task) getBothClusters() []*migapi.MigCluster {
	return []*migapi.MigCluster{
		t.PlanResources.SrcMigCluster,
		t.PlanResources.DestMigCluster}
}

// Get both source and destination clients.
func (t *Task) getBothClients() ([]k8sclient.Client, error) {
	list := []k8sclient.Client{}
	for _, cluster := range t.getBothClusters() {
		client, err := cluster.GetClient(t.Client)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		list = append(list, client)
	}

	return list, nil
}

// Get both source and destination clients with associated namespaces.
func (t *Task) getBothClientsWithNamespaces() ([]k8sclient.Client, [][]string, error) {
	clientList, err := t.getBothClients()
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	namespaceList := [][]string{t.sourceNamespaces(), t.destinationNamespaces()}

	return clientList, namespaceList, nil
}
