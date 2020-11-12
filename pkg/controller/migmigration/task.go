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
	Created                                = ""
	Started                                = "Started"
	CleanStaleAnnotations                  = "CleanStaleAnnotations"
	CleanStaleStagePods                    = "CleanStaleStagePods"
	WaitForStaleStagePodsTerminated        = "WaitForStaleStagePodsTerminated"
	StartRefresh                           = "StartRefresh"
	WaitForRefresh                         = "WaitForRefresh"
	CreateRegistries                       = "CreateRegistries"
	CreateDirectImageMigration             = "CreateDirectImageMigration"
	DirectImageMigrationStarted            = "DirectImageMigrationStarted"
	WaitForDirectImageMigrationToComplete  = "WaitForDirectImageMigrationToComplete"
	EnsureCloudSecretPropagated            = "EnsureCloudSecretPropagated"
	PreBackupHooks                         = "PreBackupHooks"
	PostBackupHooks                        = "PostBackupHooks"
	PreRestoreHooks                        = "PreRestoreHooks"
	PostRestoreHooks                       = "PostRestoreHooks"
	PreBackupHooksFailed                   = "PreBackupHooksFailed"
	PostBackupHooksFailed                  = "PostBackupHooksFailed"
	PreRestoreHooksFailed                  = "PreRestoreHooksFailed"
	PostRestoreHooksFailed                 = "PostRestoreHooksFailed"
	EnsureInitialBackup                    = "EnsureInitialBackup"
	InitialBackupCreated                   = "InitialBackupCreated"
	InitialBackupFailed                    = "InitialBackupFailed"
	AnnotateResources                      = "AnnotateResources"
	EnsureStagePodsFromRunning             = "EnsureStagePodsFromRunning"
	EnsureStagePodsFromTemplates           = "EnsureStagePodsFromTemplates"
	EnsureStagePodsFromOrphanedPVCs        = "EnsureStagePodsFromOrphanedPVCs"
	StagePodsCreated                       = "StagePodsCreated"
	StagePodsFailed                        = "StagePodsFailed"
	SourceStagePodsFailed                  = "SourceStagePodsFailed"
	RestartRestic                          = "RestartRestic"
	ResticRestarted                        = "ResticRestarted"
	QuiesceApplications                    = "QuiesceApplications"
	EnsureQuiesced                         = "EnsureQuiesced"
	UnQuiesceApplications                  = "UnQuiesceApplications"
	WaitForRegistriesReady                 = "WaitForRegistriesReady"
	EnsureStageBackup                      = "EnsureStageBackup"
	StageBackupCreated                     = "StageBackupCreated"
	StageBackupFailed                      = "StageBackupFailed"
	EnsureInitialBackupReplicated          = "EnsureInitialBackupReplicated"
	EnsureStageBackupReplicated            = "EnsureStageBackupReplicated"
	EnsureStageRestore                     = "EnsureStageRestore"
	StageRestoreCreated                    = "StageRestoreCreated"
	StageRestoreFailed                     = "StageRestoreFailed"
	CreateDirectVolumeMigration            = "CreateDirectVolumeMigration"
	DirectVolumeMigrationStarted           = "DirectVolumeMigrationStarted"
	WaitForDirectVolumeMigrationToComplete = "WaitForDirectVolumeMigrationToComplete"
	DirectVolumeMigrationFailed            = "DirectVolumeMigrationFailed"
	EnsureFinalRestore                     = "EnsureFinalRestore"
	FinalRestoreCreated                    = "FinalRestoreCreated"
	FinalRestoreFailed                     = "FinalRestoreFailed"
	Verification                           = "Verification"
	EnsureStagePodsDeleted                 = "EnsureStagePodsDeleted"
	EnsureStagePodsTerminated              = "EnsureStagePodsTerminated"
	EnsureAnnotationsDeleted               = "EnsureAnnotationsDeleted"
	EnsureMigratedDeleted                  = "EnsureMigratedDeleted"
	DeleteRegistries                       = "DeleteRegistries"
	DeleteMigrated                         = "DeleteMigrated"
	DeleteBackups                          = "DeleteBackups"
	DeleteRestores                         = "DeleteRestores"
	MigrationFailed                        = "MigrationFailed"
	Canceling                              = "Canceling"
	Canceled                               = "Canceled"
	Rollback                               = "Rollback"
	Completed                              = "Completed"
)

// Flags
const (
	Quiesce        = 0x001 // Only when QuiescePods (true).
	HasStagePods   = 0x002 // Only when stage pods created.
	HasPVs         = 0x004 // Only when PVs migrated.
	HasVerify      = 0x008 // Only when the plan has enabled verification
	HasISs         = 0x010 // Only when ISs migrated
	DirectImage    = 0x020 // Only when using direct image migration
	IndirectImage  = 0x040 // Only when using indirect image migration
	DirectVolume   = 0x080 // Only when using direct volume migration
	IndirectVolume = 0x100 // Only when using indirect volume migration
	HasStageBackup = 0x200 // True when stage backup is needed
)

// Migration steps
const (
	StepPrepare      = "Prepare"
	StepDirectImage  = "DirectImage"
	StepDirectVolume = "DirectVolume"
	StepBackup       = "Backup"
	StepStageBackup  = "StageBackup"
	StepStageRestore = "StageRestore"
	StepRestore      = "Restore"
	StepFinal        = "Final"
)

// Itinerary defines itinerary
type Itinerary struct {
	Name   string
	Phases []Phase
}

var StageItinerary = Itinerary{
	Name: "Stage",
	Phases: []Phase{
		{Name: Created, Step: StepPrepare},
		{Name: Started, Step: StepPrepare},
		{Name: StartRefresh, Step: StepPrepare},
		{Name: WaitForRefresh, Step: StepPrepare},
		{Name: CleanStaleAnnotations, Step: StepPrepare},
		{Name: CleanStaleStagePods, Step: StepPrepare},
		{Name: WaitForStaleStagePodsTerminated, Step: StepPrepare},
		{Name: CreateRegistries, Step: StepPrepare, all: IndirectImage},
		{Name: EnsureCloudSecretPropagated, Step: StepPrepare},
		{Name: CreateDirectImageMigration, Step: StepDirectImage, all: DirectImage},
		{Name: DirectImageMigrationStarted, Step: StepDirectImage, all: DirectImage},
		{Name: CreateDirectVolumeMigration, Step: StepDirectVolume, all: DirectVolume},
		{Name: DirectVolumeMigrationStarted, Step: StepDirectVolume, all: DirectVolume},
		{Name: EnsureStagePodsFromRunning, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromTemplates, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromOrphanedPVCs, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: StagePodsCreated, Step: StepStageBackup, all: HasStagePods},
		{Name: AnnotateResources, Step: StepStageBackup, all: HasStageBackup},
		{Name: RestartRestic, Step: StepStageBackup, all: HasStagePods},
		{Name: ResticRestarted, Step: StepStageBackup, all: HasStagePods},
		{Name: WaitForRegistriesReady, Step: StepStageBackup, all: IndirectImage},
		{Name: QuiesceApplications, Step: StepStageBackup, all: Quiesce},
		{Name: EnsureQuiesced, Step: StepStageBackup, all: Quiesce},
		{Name: EnsureStageBackup, Step: StepStageBackup, all: HasStageBackup},
		{Name: StageBackupCreated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageBackupReplicated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageRestore, Step: StepStageRestore, all: HasStageBackup},
		{Name: StageRestoreCreated, Step: StepStageRestore, all: HasStageBackup},
		{Name: WaitForDirectImageMigrationToComplete, Step: StepDirectImage, all: DirectImage},
		{Name: WaitForDirectVolumeMigrationToComplete, Step: StepDirectVolume, all: DirectVolume},
		{Name: DeleteRegistries, Step: StepFinal},
		{Name: EnsureStagePodsDeleted, Step: StepFinal, all: HasStagePods},
		{Name: EnsureStagePodsTerminated, Step: StepFinal, all: HasStagePods},
		{Name: EnsureAnnotationsDeleted, Step: StepFinal, all: HasStageBackup},
		{Name: Completed, Step: StepFinal},
	},
}

var FinalItinerary = Itinerary{
	Name: "Final",
	Phases: []Phase{
		{Name: Created, Step: StepPrepare},
		{Name: Started, Step: StepPrepare},
		{Name: StartRefresh, Step: StepPrepare},
		{Name: WaitForRefresh, Step: StepPrepare},
		{Name: CleanStaleAnnotations, Step: StepPrepare},
		{Name: CleanStaleStagePods, Step: StepPrepare},
		{Name: WaitForStaleStagePodsTerminated, Step: StepPrepare},
		{Name: CreateRegistries, Step: StepPrepare, all: IndirectImage},
		{Name: EnsureCloudSecretPropagated, Step: StepPrepare},
		{Name: WaitForRegistriesReady, Step: StepPrepare, all: IndirectImage},
		{Name: PreBackupHooks, Step: StepBackup},
		{Name: CreateDirectImageMigration, Step: StepBackup, all: DirectImage},
		{Name: EnsureInitialBackup, Step: StepBackup},
		{Name: InitialBackupCreated, Step: StepBackup},
		{Name: EnsureStagePodsFromRunning, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromTemplates, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromOrphanedPVCs, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: StagePodsCreated, Step: StepStageBackup, all: HasStagePods},
		{Name: AnnotateResources, Step: StepStageBackup, all: HasStageBackup},
		{Name: RestartRestic, Step: StepStageBackup, all: HasStagePods},
		{Name: ResticRestarted, Step: StepStageBackup, all: HasStagePods},
		{Name: QuiesceApplications, Step: StepStageBackup, all: Quiesce},
		{Name: EnsureQuiesced, Step: StepStageBackup, all: Quiesce},
		{Name: CreateDirectVolumeMigration, Step: StepDirectVolume, all: DirectVolume},
		{Name: DirectVolumeMigrationStarted, Step: StepDirectVolume, all: DirectVolume},
		{Name: EnsureStageBackup, Step: StepStageBackup, all: HasStageBackup},
		{Name: StageBackupCreated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageBackupReplicated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageRestore, Step: StepStageRestore, all: HasStageBackup},
		{Name: StageRestoreCreated, Step: StepStageRestore, all: HasStageBackup},
		{Name: EnsureStagePodsDeleted, Step: StepStageRestore, all: HasStagePods},
		{Name: EnsureStagePodsTerminated, Step: StepStageRestore, all: HasStagePods},
		{Name: WaitForDirectImageMigrationToComplete, Step: StepDirectImage, all: DirectImage},
		{Name: WaitForDirectVolumeMigrationToComplete, Step: StepDirectVolume, all: DirectVolume},
		{Name: EnsureAnnotationsDeleted, Step: StepRestore, all: HasStageBackup},
		{Name: EnsureInitialBackupReplicated, Step: StepRestore},
		{Name: PostBackupHooks, Step: StepRestore},
		{Name: PreRestoreHooks, Step: StepRestore},
		{Name: EnsureFinalRestore, Step: StepRestore},
		{Name: FinalRestoreCreated, Step: StepRestore},
		{Name: PostRestoreHooks, Step: StepRestore},
		{Name: DeleteRegistries, Step: StepFinal},
		{Name: Verification, Step: StepFinal, all: HasVerify},
		{Name: Completed, Step: StepFinal},
	},
}

var CancelItinerary = Itinerary{
	Name: "Cancel",
	Phases: []Phase{
		{Name: Canceling, Step: StepFinal},
		{Name: DeleteBackups, Step: StepFinal},
		{Name: DeleteRestores, Step: StepFinal},
		{Name: DeleteRegistries, Step: StepFinal},
		{Name: EnsureStagePodsDeleted, Step: StepFinal, all: HasStagePods},
		{Name: EnsureAnnotationsDeleted, Step: StepFinal, all: HasStageBackup},
		{Name: Canceled, Step: StepFinal},
		{Name: Completed, Step: StepFinal},
	},
}

var FailedItinerary = Itinerary{
	Name: "Failed",
	Phases: []Phase{
		{Name: MigrationFailed, Step: StepFinal},
		{Name: DeleteRegistries, Step: StepFinal},
		{Name: EnsureAnnotationsDeleted, Step: StepFinal, all: HasStageBackup},
		{Name: Completed, Step: StepFinal},
	},
}

var RollbackItinerary = Itinerary{
	Name: "Rollback",
	Phases: []Phase{
		{Name: Rollback, Step: StepFinal},
		{Name: DeleteBackups, Step: StepFinal},
		{Name: DeleteRestores, Step: StepFinal},
		{Name: DeleteRegistries, Step: StepFinal},
		{Name: EnsureStagePodsDeleted, Step: StepFinal},
		{Name: EnsureAnnotationsDeleted, Step: StepFinal, all: HasStageBackup},
		{Name: DeleteMigrated, Step: StepFinal},
		{Name: EnsureMigratedDeleted, Step: StepFinal},
		{Name: UnQuiesceApplications, Step: StepFinal},
		{Name: Completed, Step: StepFinal},
	},
}

// Phase defines phase in the migration
type Phase struct {
	// A phase name.
	Name string
	// High level Step this phase belongs to
	Step string
	// Step included when ALL flags evaluate true.
	all uint16
	// Step included when ANY flag evaluates true.
	any uint16
}

// Get a progress report.
// Returns: phase, n, total.
func (r Itinerary) progressReport(phaseName string) (string, int, int) {
	n := 0
	total := len(r.Phases)
	for i, phase := range r.Phases {
		if phase.Name == phaseName {
			n = i + 1
			break
		}
	}

	return phaseName, n, total
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
	Step            string
}

// Run the task.
// Each call will:
//   1. Run the current phase.
//   2. Update the phase to the next phase.
//   3. Set the Requeue (as appropriate).
//   4. Return.
func (t *Task) Run() error {
	t.Requeue = FastReQ
	t.Log.Info("[RUN]", "stage", t.stage(), "phase", t.Phase)

	err := t.init()
	if err != nil {
		return err
	}

	defer t.updatePipeline(t.Step)

	// Run the current phase.
	switch t.Phase {
	case Created, Started, Rollback:
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StartRefresh:
		t.Requeue = PollReQ
		started, err := t.startRefresh()
		if err != nil {
			return liberr.Wrap(err)
		}
		if started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		}

	case WaitForRefresh:
		t.Requeue = PollReQ
		refreshed := t.waitForRefresh()
		if refreshed {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		}
	case CreateRegistries:
		t.Requeue = PollReQ
		nEnsured, err := t.ensureMigRegistries()
		if err != nil {
			return liberr.Wrap(err)
		}
		if nEnsured == 2 {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case WaitForRegistriesReady:
		t.Requeue = PollReQ
		// First registry health check happens here
		// After this, registry health is continuously checked in validation.go
		nEnsured, message, err := ensureRegistryHealth(t.Client, t.Owner)
		if err != nil {
			return liberr.Wrap(err)
		}
		if nEnsured == 2 && message == "" {
			setMigRegistryHealthyCondition(t.Owner)
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case DeleteRegistries:
		t.Requeue = PollReQ
		err := t.deleteImageRegistryResources()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateDirectImageMigration:
		// Create the DirectImageMigration CR
		err := t.createDirectImageMigration()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForDirectImageMigrationToComplete:
		// Get the DirectImageMigration CR
		dim, err := t.getDirectImageMigration()
		if err != nil {
			return liberr.Wrap(err)
		}
		if dim == nil {
			return errors.New("DirectImageMigration not found")
		}

		completed, reasons := dim.HasCompleted()
		t.Log.Info("is migrations", "name", dim.Name, "completed", completed, "phase", dim.Status.Phase)

		if completed {
			if len(reasons) > 0 {
				t.fail(MigrationFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		}
		t.Requeue = PollReQ
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
			t.Requeue = PollReQ
		}
	case EnsureInitialBackup:
		_, err := t.ensureInitialBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
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
			t.Requeue = PollReQ
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
		t.Requeue = PollReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromTemplates:
		err := t.ensureStagePodsFromTemplates()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = PollReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromOrphanedPVCs:
		err := t.ensureStagePodsFromOrphanedPVCs()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = PollReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StagePodsCreated:
		report, err := t.ensureSourceStagePodsStarted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if report.failed {
			t.fail(SourceStagePodsFailed, report.reasons)
			break
		}
		if report.started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
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
	case CreateDirectVolumeMigration:
		err := t.createDirectVolumeMigration()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DirectVolumeMigrationStarted:
		dvm, err := t.getDirectVolumeMigration()
		if err != nil {
			return liberr.Wrap(err)
		}
		// Make sure it exists
		if dvm == nil {
			return errors.New("direct volume migration not found")
		}
		// FIXME: currently a placefiller
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForDirectVolumeMigrationToComplete:
		dvm, err := t.getDirectVolumeMigration()
		if err != nil {
			return liberr.Wrap(err)
		}
		// if no dvm, continue to next task
		if dvm == nil {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		}
		// Check if DVM is complete and report progress
		completed, reasons, progress := t.hasDirectVolumeMigrationCompleted(dvm)
		if completed {
			if len(reasons) > 0 {
				t.fail(DirectVolumeMigrationFailed, reasons)
			} else {
				t.setProgress(progress)
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.setProgress(progress)
			t.Requeue = PollReQ
		}
	case EnsureStageBackup:
		_, err := t.ensureStageBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = PollReQ
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
			t.Requeue = PollReQ
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
			t.Requeue = PollReQ
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
			t.Requeue = PollReQ
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
			t.Requeue = PollReQ
		}
	case EnsureStageRestore:
		_, err := t.ensureStageRestore()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = PollReQ
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
			t.Requeue = PollReQ
		}
	case EnsureStagePodsDeleted, CleanStaleStagePods:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsTerminated, WaitForStaleStagePodsTerminated:
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
	case EnsureAnnotationsDeleted, CleanStaleAnnotations:
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
			t.Requeue = PollReQ
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
		t.Requeue = PollReQ
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
			t.Requeue = PollReQ
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
			t.Requeue = PollReQ
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
			Message:  "The migration is being canceled.",
			Durable:  true,
		})
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}

	case MigrationFailed:
		t.Phase = Completed
		t.Step = StepFinal
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
			Message:  "The migration has been canceled.",
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
		t.Phase = t.Itinerary.Phases[0].Name
	}

	t.Step = t.Itinerary.GetStepForPhase(t.Phase)

	t.initPipeline()

	hasImageStreams, err := t.hasImageStreams()
	if err != nil {
		return err
	}

	anyPVs, _ := t.hasPVs()
	if t.stage() && (!anyPVs && !hasImageStreams) {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StageNoOp,
			Status:   True,
			Category: migapi.Warn,
			Message:  "Stage migration was run without any PVs or ImageStreams in source cluster. No Velero operations were initiated.",
			Durable:  true,
		})
	}
	return nil

}

func (t *Task) initPipeline() {
	for _, phase := range FinalItinerary.Phases {
		t.Owner.Status.AddStep(&migapi.Step{
			Name:    phase.Step,
			Message: "Not started",
		})
	}
	currentStep := t.Owner.Status.FindStep(t.Step)
	if currentStep != nil {
		currentStep.MarkStarted()
		currentStep.Phase = t.Phase
		if desc, found := PhaseDescriptions[t.Phase]; found {
			currentStep.Message = desc
		} else {
			currentStep.Message = ""
		}
	}
}

func (t *Task) updatePipeline(prevStep string) {
	oldStep := t.Owner.Status.FindStep(prevStep)
	currentStep := t.Owner.Status.FindStep(t.Step)
	if oldStep != nil && oldStep != currentStep {
		oldStep.MarkCompleted()
		if t.failed() {
			oldStep.Failed = true
		}
	}
	if currentStep != nil {
		currentStep.MarkStarted()
		currentStep.Phase = t.Phase
		if desc, found := PhaseDescriptions[t.Phase]; found {
			currentStep.Message = desc
		} else {
			currentStep.Message = ""
		}
		if t.Phase == Completed {
			currentStep.MarkCompleted()
		}
	}
	t.Owner.Status.ReflectPipeline()
}

func (t *Task) setProgress(progress []string) {
	currentStep := t.Owner.Status.FindStep(t.Step)
	if currentStep != nil {
		currentStep.Progress = progress
	}
}

// Advance the task to the next phase.
func (t *Task) next() error {
	current := -1
	for i, phase := range t.Itinerary.Phases {
		if phase.Name != t.Phase {
			continue
		}
		current = i
		break
	}
	if current == -1 {
		t.Phase = Completed
		t.Step = StepFinal
		return nil
	}
	for n := current + 1; n < len(t.Itinerary.Phases); n++ {
		next := t.Itinerary.Phases[n]
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
		t.Phase = next.Name
		t.Step = next.Step
		return nil
	}
	t.Phase = Completed
	t.Step = StepFinal
	return nil
}

// Evaluate `all` flags.
func (t *Task) allFlags(phase Phase) (bool, error) {
	anyPVs, moveSnapshotPVs := t.hasPVs()
	if phase.all&HasPVs != 0 && !anyPVs {
		return false, nil
	}
	if phase.all&HasStagePods != 0 && !t.Owner.Status.HasCondition(StagePodsCreated) {
		return false, nil
	}
	if phase.all&Quiesce != 0 && !t.quiesce() {
		return false, nil
	}
	if phase.all&HasVerify != 0 && !t.hasVerify() {
		return false, nil
	}
	hasImageStream, err := t.hasImageStreams()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if phase.all&HasISs != 0 && hasImageStream {
		return false, nil
	}
	if phase.all&DirectImage != 0 && !t.directImageMigration() {
		return false, nil
	}
	if phase.all&IndirectImage != 0 && !t.indirectImageMigration() {
		return false, nil
	}
	if phase.all&DirectVolume != 0 && !t.directVolumeMigration() {
		return false, nil
	}
	if phase.all&IndirectVolume != 0 && !t.indirectVolumeMigration() {
		return false, nil
	}
	if phase.all&HasStageBackup != 0 && !t.hasStageBackup(hasImageStream, anyPVs, moveSnapshotPVs) {
		return false, nil
	}

	return true, nil
}

// Evaluate `any` flags.
func (t *Task) anyFlags(phase Phase) (bool, error) {
	anyPVs, moveSnapshotPVs := t.hasPVs()
	if phase.any&HasPVs != 0 && anyPVs {
		return true, nil
	}
	if phase.any&HasStagePods != 0 && t.Owner.Status.HasCondition(StagePodsCreated) {
		return true, nil
	}
	if phase.any&Quiesce != 0 && t.quiesce() {
		return true, nil
	}
	if phase.any&HasVerify != 0 && t.hasVerify() {
		return true, nil
	}
	hasImageStream, err := t.hasImageStreams()
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if phase.any&HasISs != 0 && hasImageStream {
		return true, nil
	}
	if phase.any&DirectImage != 0 && t.directImageMigration() {
		return true, nil
	}
	if phase.any&IndirectImage != 0 && t.indirectImageMigration() {
		return true, nil
	}
	if phase.any&DirectVolume != 0 && t.directVolumeMigration() {
		return true, nil
	}
	if phase.any&IndirectVolume != 0 && t.indirectVolumeMigration() {
		return true, nil
	}
	if phase.any&HasStageBackup != 0 && t.hasStageBackup(hasImageStream, anyPVs, moveSnapshotPVs) {
		return true, nil
	}
	return phase.any == uint16(0), nil
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
		Message:  "The migration has failed.  See: Errors.",
		Durable:  true,
	})
	t.Phase = nextPhase
	t.Step = StepFinal
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
// First return value is PVs overall, and second is limited to Move or snapshot copy PVs
func (t *Task) hasPVs() (bool, bool) {
	var anyPVs bool
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action == migapi.PvMoveAction ||
			pv.Selection.Action == migapi.PvCopyAction && pv.Selection.CopyMethod == migapi.PvSnapshotCopyMethod {
			return true, true
		}
		if pv.Selection.Action != migapi.PvSkipAction {
			anyPVs = true
		}
	}
	return anyPVs, false
}

// Get whether the associated plan has PVs to be directly migrated
func (t *Task) hasDirectVolumes() bool {
	if t.PlanResources.MigPlan.Spec.IndirectVolumeMigration {
		return false
	}
	pvcList := t.getDirectVolumeClaimList()
	if pvcList != nil {
		return true
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

// Returns true if the IndirectImageMigration override on the plan is set (plan is configured not to do direct migration)
func (t *Task) indirectImageMigration() bool {
	return t.PlanResources.MigPlan.Spec.IndirectImageMigration
}

// Returns true if the IndirectImageMigration override on the plan is not set (plan is configured to do direct migration)
func (t *Task) directImageMigration() bool {
	return !t.indirectImageMigration()
}

// Returns true if the IndirectVolumeMigration override on the plan is set (plan is configured not to do direct migration)
func (t *Task) indirectVolumeMigration() bool {
	return t.PlanResources.MigPlan.Spec.IndirectVolumeMigration
}

// Returns true if the IndirectVolumeMigration override on the plan is not set (plan is configured to do direct migration)
// There must exist a set of direct volumes for this to return true
func (t *Task) directVolumeMigration() bool {
	return !t.indirectVolumeMigration() && t.hasDirectVolumes()
}

// Returns true if the migration requires a stage backup
func (t *Task) hasStageBackup(hasIS, anyPVs, moveSnapshotPVs bool) bool {
	return hasIS && t.indirectImageMigration() || anyPVs && t.indirectVolumeMigration() || moveSnapshotPVs
}

// Get both source and destination clusters.
func (t *Task) getBothClusters() []*migapi.MigCluster {
	return []*migapi.MigCluster{
		t.PlanResources.SrcMigCluster,
		t.PlanResources.DestMigCluster}
}

// Get both source and destination clients.
func (t *Task) getBothClients() ([]compat.Client, error) {
	list := []compat.Client{}
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
func (t *Task) getBothClientsWithNamespaces() ([]compat.Client, [][]string, error) {
	clientList, err := t.getBothClients()
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}
	namespaceList := [][]string{t.sourceNamespaces(), t.destinationNamespaces()}

	return clientList, namespaceList, nil
}

// GetStepForPhase returns which high level step current phase belongs to
func (r *Itinerary) GetStepForPhase(phaseName string) string {
	for _, phase := range r.Phases {
		if phaseName == phase.Name {
			return phase.Step
		}
	}
	return ""
}
