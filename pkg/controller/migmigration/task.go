package migmigration

import (
	"context"
	"fmt"
	"path"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/errorutil"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/opentracing/opentracing-go"
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
	CleanStaleVeleroCRs                    = "CleanStaleVeleroCRs"
	CleanStaleResticCRs                    = "CleanStaleResticCRs"
	CleanStaleStagePods                    = "CleanStaleStagePods"
	WaitForStaleStagePodsTerminated        = "WaitForStaleStagePodsTerminated"
	StartRefresh                           = "StartRefresh"
	WaitForRefresh                         = "WaitForRefresh"
	CreateRegistries                       = "CreateRegistries"
	CreateDirectImageMigration             = "CreateDirectImageMigration"
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
	RestartVelero                          = "RestartVelero"
	WaitForVeleroReady                     = "WaitForVeleroReady"
	RestartRestic                          = "RestartRestic"
	WaitForResticReady                     = "WaitForResticReady"
	QuiesceApplications                    = "QuiesceApplications"
	EnsureQuiesced                         = "EnsureQuiesced"
	UnQuiesceSrcApplications               = "UnQuiesceSrcApplications"
	UnQuiesceDestApplications              = "UnQuiesceDestApplications"
	SwapPVCReferences                      = "SwapPVCReferences"
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
	DeleteHookJobs                         = "DeleteHookJobs"
	DeleteDirectVolumeMigrationResources   = "DeleteDirectVolumeMigrationResources"
	DeleteDirectImageMigrationResources    = "DeleteDirectImageMigrationResources"
	MigrationFailed                        = "MigrationFailed"
	Canceling                              = "Canceling"
	Canceled                               = "Canceled"
	Rollback                               = "Rollback"
	Completed                              = "Completed"
)

// Flags
const (
	Quiesce             = 0x001   // Only when QuiescePods (true).
	HasStagePods        = 0x002   // Only when stage pods created.
	HasPVs              = 0x004   // Only when PVs migrated.
	HasVerify           = 0x008   // Only when the plan has enabled verification
	HasISs              = 0x010   // Only when ISs migrated
	DirectImage         = 0x020   // Only when using direct image migration
	IndirectImage       = 0x040   // Only when using indirect image migration
	DirectVolume        = 0x080   // Only when using direct volume migration
	IndirectVolume      = 0x100   // Only when using indirect volume migration
	HasStageBackup      = 0x200   // True when stage backup is needed
	EnableImage         = 0x400   // True when disable_image_migration is unset
	EnableVolume        = 0x800   // True when disable_volume is unset
	HasPreBackupHooks   = 0x1000  // True when prebackup hooks exist
	HasPostBackupHooks  = 0x2000  // True when postbackup hooks exist
	HasPreRestoreHooks  = 0x4000  // True when postbackup hooks exist
	HasPostRestoreHooks = 0x8000  // True when postbackup hooks exist
	StorageConversion   = 0x10000 // True when the migration is a storage conversion
)

// Migration steps
const (
	StepPrepare          = "Prepare"
	StepDirectImage      = "DirectImage"
	StepDirectVolume     = "DirectVolume"
	StepBackup           = "Backup"
	StepStageBackup      = "StageBackup"
	StepStageRestore     = "StageRestore"
	StepRestore          = "Restore"
	StepCleanup          = "Cleanup"
	StepCleanupVelero    = "CleanupVelero"
	StepCleanupHelpers   = "CleanupHelpers"
	StepCleanupMigrated  = "CleanupMigrated"
	StepCleanupUnquiesce = "CleanupUnquiesce"
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
		{Name: CleanStaleResticCRs, Step: StepPrepare},
		{Name: CleanStaleVeleroCRs, Step: StepPrepare},
		{Name: RestartVelero, Step: StepPrepare},
		{Name: CleanStaleStagePods, Step: StepPrepare},
		{Name: WaitForStaleStagePodsTerminated, Step: StepPrepare},
		{Name: CreateRegistries, Step: StepPrepare, all: IndirectImage | EnableImage | HasISs},
		{Name: CreateDirectImageMigration, Step: StepStageBackup, all: DirectImage | EnableImage},
		{Name: QuiesceApplications, Step: StepStageBackup, all: Quiesce},
		{Name: EnsureQuiesced, Step: StepStageBackup, all: Quiesce},
		{Name: CreateDirectVolumeMigration, Step: StepStageBackup, all: DirectVolume | EnableVolume},
		{Name: EnsureStagePodsFromRunning, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromTemplates, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromOrphanedPVCs, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: StagePodsCreated, Step: StepStageBackup, all: HasStagePods},
		{Name: RestartRestic, Step: StepStageBackup, all: HasStagePods},
		{Name: AnnotateResources, Step: StepStageBackup, all: HasStageBackup},
		{Name: WaitForVeleroReady, Step: StepStageBackup},
		{Name: WaitForResticReady, Step: StepStageBackup, all: HasPVs | HasStagePods},
		{Name: WaitForRegistriesReady, Step: StepStageBackup, all: IndirectImage | EnableImage | HasISs},
		{Name: EnsureCloudSecretPropagated, Step: StepStageBackup, any: HasStageBackup},
		{Name: EnsureStageBackup, Step: StepStageBackup, all: HasStageBackup},
		{Name: StageBackupCreated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageBackupReplicated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageRestore, Step: StepStageRestore, all: HasStageBackup},
		{Name: StageRestoreCreated, Step: StepStageRestore, all: HasStageBackup},
		{Name: WaitForDirectImageMigrationToComplete, Step: StepDirectImage, all: DirectImage | EnableImage},
		{Name: WaitForDirectVolumeMigrationToComplete, Step: StepDirectVolume, all: DirectVolume | EnableVolume},
		{Name: SwapPVCReferences, Step: StepCleanup, all: StorageConversion | Quiesce},
		{Name: DeleteRegistries, Step: StepCleanup},
		{Name: EnsureStagePodsDeleted, Step: StepCleanup, all: HasStagePods},
		{Name: EnsureStagePodsTerminated, Step: StepCleanup, all: HasStagePods},
		{Name: EnsureAnnotationsDeleted, Step: StepCleanup, all: HasStageBackup},
		{Name: Completed, Step: StepCleanup},
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
		{Name: CleanStaleResticCRs, Step: StepPrepare},
		{Name: CleanStaleVeleroCRs, Step: StepPrepare},
		{Name: RestartVelero, Step: StepPrepare},
		{Name: CleanStaleStagePods, Step: StepPrepare},
		{Name: WaitForStaleStagePodsTerminated, Step: StepPrepare},
		{Name: CreateRegistries, Step: StepPrepare, all: IndirectImage | EnableImage | HasISs},
		{Name: WaitForVeleroReady, Step: StepPrepare},
		{Name: WaitForRegistriesReady, Step: StepPrepare, all: IndirectImage | EnableImage | HasISs},
		{Name: EnsureCloudSecretPropagated, Step: StepPrepare},
		{Name: PreBackupHooks, Step: PreBackupHooks, all: HasPreBackupHooks},
		{Name: CreateDirectImageMigration, Step: StepBackup, all: DirectImage | EnableImage},
		{Name: EnsureInitialBackup, Step: StepBackup},
		{Name: InitialBackupCreated, Step: StepBackup},
		{Name: QuiesceApplications, Step: StepStageBackup, all: Quiesce},
		{Name: EnsureQuiesced, Step: StepStageBackup, all: Quiesce},
		{Name: EnsureStagePodsFromRunning, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromTemplates, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: EnsureStagePodsFromOrphanedPVCs, Step: StepStageBackup, all: HasPVs | IndirectVolume},
		{Name: StagePodsCreated, Step: StepStageBackup, all: HasStagePods},
		{Name: RestartRestic, Step: StepStageBackup, all: HasStagePods},
		{Name: AnnotateResources, Step: StepStageBackup, all: HasStageBackup},
		{Name: WaitForResticReady, Step: StepStageBackup, any: HasPVs | HasStagePods},
		{Name: CreateDirectVolumeMigration, Step: StepStageBackup, all: DirectVolume | EnableVolume},
		{Name: EnsureStageBackup, Step: StepStageBackup, all: HasStageBackup},
		{Name: StageBackupCreated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageBackupReplicated, Step: StepStageBackup, all: HasStageBackup},
		{Name: EnsureStageRestore, Step: StepStageRestore, all: HasStageBackup},
		{Name: StageRestoreCreated, Step: StepStageRestore, all: HasStageBackup},
		{Name: EnsureStagePodsDeleted, Step: StepStageRestore, all: HasStagePods},
		{Name: EnsureStagePodsTerminated, Step: StepStageRestore, all: HasStagePods},
		{Name: EnsureAnnotationsDeleted, Step: StepStageRestore, all: HasStageBackup},
		{Name: WaitForDirectImageMigrationToComplete, Step: StepDirectImage, all: DirectImage | EnableImage},
		{Name: WaitForDirectVolumeMigrationToComplete, Step: StepDirectVolume, all: DirectVolume | EnableVolume},
		{Name: PostBackupHooks, Step: PostBackupHooks, all: HasPostBackupHooks},
		{Name: PreRestoreHooks, Step: PreRestoreHooks, all: HasPreRestoreHooks},
		{Name: EnsureInitialBackupReplicated, Step: StepRestore},
		{Name: EnsureFinalRestore, Step: StepRestore},
		{Name: FinalRestoreCreated, Step: StepRestore},
		{Name: UnQuiesceDestApplications, Step: StepRestore},
		{Name: PostRestoreHooks, Step: PostRestoreHooks, all: HasPostRestoreHooks},
		{Name: SwapPVCReferences, Step: StepCleanup, all: StorageConversion | Quiesce},
		{Name: DeleteRegistries, Step: StepCleanup},
		{Name: Verification, Step: StepCleanup, all: HasVerify},
		{Name: Completed, Step: StepCleanup},
	},
}

var CancelItinerary = Itinerary{
	Name: "Cancel",
	Phases: []Phase{
		{Name: Canceling, Step: StepCleanupVelero},
		{Name: DeleteBackups, Step: StepCleanupVelero},
		{Name: DeleteRestores, Step: StepCleanupVelero},
		{Name: DeleteRegistries, Step: StepCleanupHelpers},
		{Name: DeleteHookJobs, Step: StepCleanupHelpers},
		{Name: DeleteDirectVolumeMigrationResources, Step: StepCleanupHelpers, all: DirectVolume},
		{Name: DeleteDirectImageMigrationResources, Step: StepCleanupHelpers, all: DirectImage},
		{Name: EnsureStagePodsDeleted, Step: StepCleanupHelpers, all: HasStagePods},
		{Name: EnsureAnnotationsDeleted, Step: StepCleanupHelpers, all: HasStageBackup},
		{Name: Canceled, Step: StepCleanup},
		{Name: Completed, Step: StepCleanup},
	},
}

var FailedItinerary = Itinerary{
	Name: "Failed",
	Phases: []Phase{
		{Name: MigrationFailed, Step: StepCleanupHelpers},
		{Name: DeleteRegistries, Step: StepCleanupHelpers},
		{Name: EnsureAnnotationsDeleted, Step: StepCleanupHelpers, all: HasStageBackup},
		{Name: Completed, Step: StepCleanup},
	},
}

var RollbackItinerary = Itinerary{
	Name: "Rollback",
	Phases: []Phase{
		{Name: Rollback, Step: StepCleanupVelero},
		{Name: DeleteBackups, Step: StepCleanupVelero},
		{Name: DeleteRestores, Step: StepCleanupVelero},
		{Name: DeleteRegistries, Step: StepCleanupHelpers},
		{Name: EnsureStagePodsDeleted, Step: StepCleanupHelpers},
		{Name: EnsureAnnotationsDeleted, Step: StepCleanupHelpers, any: HasPVs | HasISs},
		{Name: DeleteMigrated, Step: StepCleanupMigrated},
		{Name: EnsureMigratedDeleted, Step: StepCleanupMigrated},
		{Name: UnQuiesceSrcApplications, Step: StepCleanupUnquiesce},
		{Name: Completed, Step: StepCleanup},
	},
}

// Phase defines phase in the migration
type Phase struct {
	// A phase name.
	Name string
	// High level Step this phase belongs to
	Step string
	// Step included when ALL flags evaluate true.
	all uint32
	// Step included when ANY flag evaluates true.
	any uint32
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

	Tracer        opentracing.Tracer
	ReconcileSpan opentracing.Span
}

// Run the task.
// Each call will:
//   1. Run the current phase.
//   2. Update the phase to the next phase.
//   3. Set the Requeue (as appropriate).
//   4. Return.
func (t *Task) Run(ctx context.Context) error {
	// Set stage, phase, phase description, migplan name
	t.Log = t.Log.WithValues("phase", t.Phase)
	t.Requeue = FastReQ

	// Jaeger span
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, t.Tracer, "migration-phase-"+t.Phase)
		defer span.Finish()
	}

	err := t.init()
	if err != nil {
		return err
	}

	// Log "[RUN] <Phase Description>" unless we are waiting on
	// DIM or DVM (they will log their own [RUN]) with the same message.
	t.logRunHeader()

	defer t.updatePipeline()

	// Run the current phase.
	switch t.Phase {
	case Created, Started, Rollback:
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StartRefresh:
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
		refreshed := t.waitForRefresh()
		if refreshed {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case CleanStaleResticCRs:
		err := t.deleteStaleResticCRs()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CleanStaleVeleroCRs:
		err := t.deleteStaleVeleroCRs()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateRegistries:
		nEnsured, err := t.ensureMigRegistries()
		if err != nil {
			return liberr.Wrap(err)
		}
		if nEnsured == 2 {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info(fmt.Sprintf("Created [%v/2] registries, retrying.", nEnsured))
		}
	case WaitForRegistriesReady:
		// First registry health check happens here
		// After this, registry health is continuously checked in validation.go
		nEnsured, message, err := ensureRegistryHealth(t.Client, t.Owner)
		if err != nil {
			if err.Error() == "ImagePullBackOff" {
				t.fail(WaitForRegistriesReady, []string{message})
			} else {
				return liberr.Wrap(err)
			}
		}
		if nEnsured == 2 && message == "" {
			setMigRegistryHealthyCondition(t.Owner)
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info(fmt.Sprintf("Found [%v/2] registries in healthy state. Waiting.", nEnsured))
			t.Requeue = PollReQ
		}
	case DeleteRegistries:
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

		completed, reasons, progress := dim.HasCompleted()
		t.setProgress(progress)
		t.Log.Info("Waiting for ImageStream migrations to complete",
			"directImageMigration", path.Join(dim.Namespace, dim.Name),
			"directImageMigrationPhase", dim.Status.Phase,
			"completed", completed)

		if completed {
			if len(reasons) > 0 {
				t.setDirectImageMigrationWarning(dim)
				t.Log.Info("DirectImageMigration completed with warnings.",
					"directImageMigration", path.Join(dim.Namespace, dim.Name),
					"warnings", reasons)
				// Once supported, add reasons to Status.Warnings for the Step
			}
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
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
			t.Log.Info(fmt.Sprintf("Cloud secret has propagated to Velero Pod "+
				"on [%v/2] clusters. Waiting.", count))
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
			t.Log.Info(fmt.Sprintf("PreBackupHooks are still running. Waiting."))
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
			t.setInitialBackupPartialFailureWarning(backup)
			if len(reasons) > 0 {
				t.fail(InitialBackupFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			backupProgress := ""
			if backup.Status.Progress != nil {
				backupProgress = fmt.Sprintf("%v/%v",
					backup.Status.Progress.ItemsBackedUp,
					backup.Status.Progress.TotalItems)
			}
			t.Log.Info("Initial Velero Backup is incomplete. Waiting.",
				"backup", path.Join(backup.Namespace, backup.Name),
				"backupPhase", backup.Status.Phase,
				"backupProgress", backupProgress,
				"backupWarnings", backup.Status.Warnings,
				"backupErrors", backup.Status.Errors)
			t.Requeue = PollReQ
		}
	case AnnotateResources:
		finished, err := t.annotateStageResources()
		if err != nil {
			return liberr.Wrap(err)
		}
		if finished {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info("Annotated/labeled source cluster resources this reconcile, "+
				"requeuing and continuing.",
				"annotationsPerReconcile", AnnotationsPerReconcile)
		}
	case EnsureStagePodsFromRunning:
		err := t.ensureStagePodsFromRunning()
		if err != nil {
			t.Log.Error(err, "Error creating stage pods on source cluster from running pods")
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromTemplates:
		err := t.ensureStagePodsFromTemplates()
		if err != nil {
			t.Log.Error(err, "Error creating stage pods on source cluster from templates")
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureStagePodsFromOrphanedPVCs:
		err := t.ensureStagePodsFromOrphanedPVCs()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case StagePodsCreated:
		report, err := t.ensureSourceStagePodsStarted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if report.failed {
			t.Log.Info("Migration failed due to a problem with source cluster stage pods")
			t.fail(SourceStagePodsFailed, report.reasons)
			break
		}
		if report.started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info("Waiting for Stage Pods to be ready on source cluster")
			t.Requeue = PollReQ
		}
		t.setProgress(report.progress)
	case RestartRestic:
		err := t.restartResticPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case RestartVelero:
		err := t.restartVeleroPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForResticReady:
		started, err := t.haveResticPodsStarted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info("Restic is unready on the source or target cluster. Waiting.")
			t.Requeue = PollReQ
		}
	case WaitForVeleroReady:
		started, err := t.haveVeleroPodsStarted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if started {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info("Velero Pod(s) are unready on the source or target cluster. Waiting.")
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
			t.Log.Info("Quiescing on source cluster is incomplete. " +
				"Pods are not yet terminated, waiting.")
			t.Requeue = PollReQ
		}
	case UnQuiesceSrcApplications:
		err := t.unQuiesceSrcApplications()
		if err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case SwapPVCReferences:
		reasons, err := t.swapPVCReferences()
		if err != nil {
			return liberr.Wrap(err)
		}
		if len(reasons) > 0 {
			t.fail(MigrationFailed, reasons)
		} else {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		}
	case UnQuiesceDestApplications:
		err := t.unQuiesceDestApplications()
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
			break
		}
		// Check if DVM is complete and report progress
		completed, reasons, progress := t.hasDirectVolumeMigrationCompleted(dvm)
		PhaseDescriptions[t.Phase] = dvm.Status.PhaseDescription
		t.setProgress(progress)
		if completed {
			step := t.Owner.Status.FindStep(t.Step)
			step.MarkCompleted()
			if len(reasons) > 0 {
				t.setDirectVolumeMigrationFailureWarning(dvm)
			}
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
			criticalWarning, err := t.getWarningForDVM(dvm)
			if err != nil {
				return liberr.Wrap(err)
			}
			if criticalWarning != nil {
				t.Owner.Status.SetCondition(*criticalWarning)
				return nil
			}
			t.Owner.Status.DeleteCondition(DirectVolumeMigrationBlocked)
		}
	case EnsureStageBackup:
		_, err := t.ensureStageBackup()
		if err != nil {
			return liberr.Wrap(err)
		}
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
			t.setStageBackupPartialFailureWarning(backup)
			if len(reasons) > 0 {
				t.Log.Info("Migration FAILED due to Stage Velero Backup failure on source cluster.",
					"backup", path.Join(backup.Namespace, backup.Name),
					"failureReasons", reasons)
				t.fail(StageBackupFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			backupProgress := ""
			if backup.Status.Progress != nil {
				backupProgress = fmt.Sprintf("%v/%v",
					backup.Status.Progress.ItemsBackedUp,
					backup.Status.Progress.TotalItems)
			}
			t.Log.Info("Stage Backup on source cluster is "+
				"incomplete. Waiting.",
				"backup", path.Join(backup.Namespace, backup.Name),
				"backupPhase", backup.Status.Phase,
				"backupProgress", backupProgress,
				"backupWarnings", backup.Status.Warnings,
				"backupErrors", backup.Status.Errors)
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
			t.Log.Info("Stage Velero Backup has not yet "+
				"been replicated to target cluster by Velero. Waiting",
				"backup", path.Join(backup.Namespace, backup.Name))
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
			t.Log.Info("PostBackupHook(s) are incomplete. Waiting.")
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
			t.Log.Info("PreRestoreHooks(s) are incomplete. Waiting.")
			t.Requeue = PollReQ
		}
	case EnsureStageRestore:
		_, err := t.ensureStageRestore()
		if err != nil {
			return liberr.Wrap(err)
		}
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
			t.setStageRestorePartialFailureWarning(restore)
			if len(reasons) > 0 {
				t.fail(StageRestoreFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.Log.Info("Stage Velero Restore on target cluster "+
				"is incomplete. Waiting. ",
				"restore", path.Join(restore.Namespace, restore.Name),
				"restorePhase", restore.Status.Phase,
				"restoreWarnings", restore.Status.Warnings,
				"restoreErrors", restore.Status.Errors)
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
			t.Log.Info("Stage Pods have not finished terminating. Waiting")
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
			t.Log.Info("Initial Velero Backup has not yet "+
				"been replicated to target cluster by Velero. Waiting",
				"backup", path.Join(backup.Namespace, backup.Name))
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
			t.setFinalRestorePartialFailureWarning(restore)
			if len(reasons) > 0 {
				t.fail(FinalRestoreFailed, reasons)
			} else {
				if err = t.next(); err != nil {
					return liberr.Wrap(err)
				}
			}
		} else {
			t.Log.Info("Final Velero Restore on target "+
				"cluster is incomplete. Waiting.",
				"restore", path.Join(restore.Namespace, restore.Name),
				"restorePhase", restore.Status.Phase,
				"restoreWarnings", restore.Status.Warnings,
				"restoreErrors", restore.Status.Errors)
			t.Requeue = PollReQ
		}
	case PostRestoreHooks:
		status, err := t.runHooks(migapi.PostRestoreHookPhase)
		if err != nil {
			t.Log.Error(err, "Error getting final Velero Restore from target cluster.")
			t.fail(PostRestoreHooksFailed, []string{err.Error()})
			return liberr.Wrap(err)
		}
		if status {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info("PostRestoreHooks are incomplete. Waiting")
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
			t.Log.Info("Verification is incomplete. Some Pods that existed on " +
				"the source cluster have not yet been recreated and verified healthy " +
				"on target cluster. Waiting")
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
		if err := t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case MigrationFailed:
		t.Phase = Completed
		t.Step = StepCleanup
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
			t.Log.Info("Found resources associated with MigPlan on target cluster " +
				"that have not finished deleting. Waiting.")
			t.Requeue = PollReQ
		}
	case DeleteBackups:
		if err := t.deleteCorrelatedBackups(); err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DeleteRestores:
		if err := t.deleteCorrelatedRestores(); err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DeleteHookJobs:
		// Stops all the jobs for the hooks by killing the jobs and corresponding pods
		status, err := t.stopHookJobs()
		if err != nil {
			return liberr.Wrap(err)
		}
		if status {
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		}
	case DeleteDirectVolumeMigrationResources:
		// Delete all DVM Resources created on the destination cluster
		if err := t.deleteDirectVolumeMigrationResources(); err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DeleteDirectImageMigrationResources:
		// Delete all DIM Resources created on the destination cluster
		if err := t.deleteDirectImageMigrationResources(); err != nil {
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
	t.Log.V(4).Info("Running task init")
	t.Requeue = FastReQ
	if t.failed() {
		t.Itinerary = FailedItinerary
	} else if t.canceled() {
		t.Itinerary = CancelItinerary
	} else if t.rollback() {
		t.Itinerary = RollbackItinerary
	} else if t.stage() || t.migrateState() {
		t.Itinerary = StageItinerary
	} else {
		t.Itinerary = FinalItinerary
	}
	if t.Owner.Status.Itinerary != t.Itinerary.Name {
		t.Phase = t.Itinerary.Phases[0].Name
	}

	t.Step = t.Itinerary.GetStepForPhase(t.Phase)

	err := t.initPipeline(t.Owner.Status.Itinerary)
	if err != nil {
		return err
	}

	if t.stage() && !t.Owner.Status.HasCondition(StageNoOp) {
		hasImageStreams, err := t.hasImageStreams()
		if err != nil {
			return err
		}

		anyPVs, _ := t.hasPVs()
		if !anyPVs && !hasImageStreams {
			t.Owner.Status.SetCondition(migapi.Condition{
				Type:     StageNoOp,
				Status:   True,
				Category: migapi.Warn,
				Message:  "Stage migration was run without any PVs or ImageStreams in source cluster. No Velero operations were initiated.",
				Durable:  true,
			})
		}
	}
	return nil

}

func (t *Task) initPipeline(prevItinerary string) error {
	if t.Itinerary.Name != prevItinerary {
		for _, phase := range t.Itinerary.Phases {
			currentStep := t.Owner.Status.FindStep(phase.Step)
			if currentStep != nil {
				continue
			}
			allFlags, err := t.allFlags(phase)
			if err != nil {
				return liberr.Wrap(err)
			}
			if !allFlags {
				continue
			}
			anyFlags, err := t.anyFlags(phase)
			if err != nil {
				return liberr.Wrap(err)
			}
			if !anyFlags {
				continue
			}
			t.Owner.Status.AddStep(&migapi.Step{
				Name:    phase.Step,
				Message: "Not started",
			})
		}
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
	return nil
}

func (t *Task) updatePipeline() {
	t.Log.V(4).Info("Updating pipeline view of progress")
	currentStep := t.Owner.Status.FindStep(t.Step)
	for _, step := range t.Owner.Status.Pipeline {
		if currentStep != step && step.MarkedStarted() {
			step.MarkCompleted()
		}
	}
	// mark steps skipped
	for _, step := range t.Owner.Status.Pipeline {
		if step == currentStep {
			break
		} else if !step.MarkedStarted() {
			step.Skipped = true
		}
	}
	if currentStep != nil {
		currentStep.MarkStarted()
		currentStep.Phase = t.Phase
		if currentStep.Name == StepDirectVolume {
			return
		}
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
	// Write time taken to complete phase
	t.Owner.Status.StageCondition(migapi.Running)
	cond := t.Owner.Status.FindCondition(migapi.Running)
	if cond != nil {
		elapsed := time.Since(cond.LastTransitionTime.Time)
		t.Log.Info("Phase completed", "phaseElapsed", elapsed)
	}

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
		t.Step = StepCleanup
		return nil
	}
	for n := current + 1; n < len(t.Itinerary.Phases); n++ {
		next := t.Itinerary.Phases[n]
		flag, err := t.allFlags(next)
		if err != nil {
			return liberr.Wrap(err)
		}
		if !flag {
			t.Log.Info("Skipped phase due to flag evaluation.",
				"skippedPhase", next.Name)
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
	t.Step = StepCleanup
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
	if phase.all&StorageConversion != 0 {
		isStorageConversion, err := t.isStorageConversionMigration()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if !isStorageConversion {
			return false, nil
		}
	}
	if phase.all&HasVerify != 0 && !t.hasVerify() {
		return false, nil
	}
	if phase.all&HasISs != 0 {
		hasImageStream, err := t.hasImageStreams()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if !hasImageStream {
			return false, nil
		}
	}
	if phase.all&DirectImage != 0 && !t.directImageMigration() {
		return false, nil
	}
	if phase.all&IndirectImage != 0 && !t.indirectImageMigration() {
		return false, nil
	}
	if phase.all&EnableImage != 0 && t.PlanResources.MigPlan.IsImageMigrationDisabled() {
		return false, nil
	}
	if phase.all&EnableImage != 0 && t.Owner.Spec.MigrateState {
		return false, nil
	}
	if phase.all&DirectVolume != 0 && !t.directVolumeMigration() {
		return false, nil
	}
	if phase.all&IndirectVolume != 0 && !t.indirectVolumeMigration() {
		return false, nil
	}
	if phase.all&EnableVolume != 0 && t.PlanResources.MigPlan.IsVolumeMigrationDisabled() {
		return false, nil
	}
	if phase.all&HasStageBackup != 0 {
		hasImageStream, err := t.hasImageStreams()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		isStorageConversion, err := t.isStorageConversionMigration()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if isStorageConversion || !t.hasStageBackup(hasImageStream, anyPVs, moveSnapshotPVs) {
			return false, nil
		}
	}

	if phase.all&HasPreBackupHooks != 0 && !t.hasPreBackupHooks() {
		return false, nil
	}

	if phase.all&HasPostBackupHooks != 0 && !t.hasPostBackupHooks() {
		return false, nil
	}

	if phase.all&HasPreRestoreHooks != 0 && !t.hasPreRestoreHooks() {
		return false, nil
	}

	if phase.all&HasPostRestoreHooks != 0 && !t.hasPostRestoreHooks() {
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
	if phase.any&StorageConversion != 0 {
		isStorageConversion, err := t.isStorageConversionMigration()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if isStorageConversion {
			return true, nil
		}
	}
	if phase.any&HasVerify != 0 && t.hasVerify() {
		return true, nil
	}
	if phase.any&HasISs != 0 {
		hasImageStream, err := t.hasImageStreams()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if hasImageStream {
			return true, nil
		}
	}
	if phase.any&DirectImage != 0 && t.directImageMigration() {
		return true, nil
	}
	if phase.any&IndirectImage != 0 && t.indirectImageMigration() {
		return true, nil
	}
	if phase.any&EnableImage != 0 && !t.PlanResources.MigPlan.IsImageMigrationDisabled() && !t.migrateState() {
		return true, nil
	}
	if phase.any&DirectVolume != 0 && t.directVolumeMigration() {
		return true, nil
	}
	if phase.any&IndirectVolume != 0 && t.indirectVolumeMigration() {
		return true, nil
	}
	if phase.any&EnableVolume != 0 && !t.PlanResources.MigPlan.IsVolumeMigrationDisabled() {
		return true, nil
	}
	if phase.any&HasStageBackup != 0 {
		hasImageStream, err := t.hasImageStreams()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		isStorageConversion, err := t.isStorageConversionMigration()
		if err != nil {
			return false, liberr.Wrap(err)
		}
		if !isStorageConversion && t.hasStageBackup(hasImageStream, anyPVs, moveSnapshotPVs) {
			return true, nil
		}
	}
	return phase.any == uint32(0), nil
}

// Phase fail.
func (t *Task) fail(nextPhase string, reasons []string) {
	t.addErrors(reasons)
	t.Owner.AddErrors(t.Errors)
	t.Log.Info("Marking migration as FAILED. See Status.Errors",
		"migrationErrors", t.Owner.Status.Errors)
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     migapi.Failed,
		Status:   True,
		Reason:   t.Phase,
		Category: Advisory,
		Message:  "The migration has failed.  See: Errors.",
		Durable:  true,
	})
	t.failCurrentStep()
	t.Phase = nextPhase
	t.Step = StepCleanup
}

// Marks current step failed
func (t *Task) failCurrentStep() {
	currentStep := t.Owner.Status.FindStep(t.Step)
	if currentStep != nil {
		currentStep.Failed = true
	}
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	for _, e := range errors {
		t.Errors = append(t.Errors, e)
	}
}

// Migration UID.
func (t *Task) UID() string {
	return string(t.Owner.UID)
}

// Get whether the migration has failed
func (t *Task) failed() bool {
	return t.Owner.HasErrors() || t.Owner.Status.HasCondition(migapi.Failed)
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

// Get whether the migration is state transfer
func (t *Task) migrateState() bool {
	return t.Owner.Spec.MigrateState
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

// isStorageConversionMigration tells whether the migratoin is for storage conversion
func (t *Task) isStorageConversionMigration() (bool, error) {
	isIntraCluster, err := t.PlanResources.MigPlan.IsIntraCluster(t.Client)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	if isIntraCluster && t.migrateState() {
		for srcNs, destNs := range t.PlanResources.MigPlan.GetNamespaceMapping() {
			if srcNs != destNs {
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
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

// Get the persistent volumes included in the plan which are included in the
// stage backup/restore process
// This function will only return PVs that are being copied via restic or
// snapshot and any PVs selected for move.
func (t *Task) getStagePVs() migapi.PersistentVolumes {
	directVolumesEnabled := !t.PlanResources.MigPlan.Spec.IndirectVolumeMigration
	volumes := []migapi.PV{}
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		// If the pv is skipped or if its a filesystem copy with DVM enabled then
		// don't include it in a stage PV
		if pv.Selection.Action == migapi.PvSkipAction ||
			(directVolumesEnabled && pv.Selection.Action == migapi.PvCopyAction &&
				pv.Selection.CopyMethod == migapi.PvFilesystemCopyMethod) {
			continue
		}
		volumes = append(volumes, pv)
	}
	pvList := t.PlanResources.MigPlan.Spec.PersistentVolumes.DeepCopy()
	pvList.List = volumes
	return *pvList
}

// Get the persistentVolumeClaims / action mapping included in the plan which are not skipped.
func (t *Task) getPVCs() map[k8sclient.ObjectKey]migapi.PV {
	claims := map[k8sclient.ObjectKey]migapi.PV{}
	for _, pv := range t.getStagePVs().List {
		claimKey := k8sclient.ObjectKey{
			Name:      pv.PVC.GetSourceName(),
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
			pv.Selection.Action == migapi.PvCopyAction &&
				pv.Selection.CopyMethod == migapi.PvSnapshotCopyMethod {
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
		err := client.List(context.Background(), &imageStreamList, k8sclient.InNamespace(ns))
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

// Emits an INFO level warning message (no stack trace) letting the
// user know an error was encountered with a description of the phase
// where available. Stack trace will be printed shortly after this.
// This is meant to help contextualize the stack trace for the user.
func (t *Task) logErrorForPhase(phaseName string, err error) {
	t.Log.Info("Exited Phase with error.",
		"phase", phaseName,
		"phaseDescription", t.getPhaseDescription(phaseName),
		"error", errorutil.Unwrap(err).Error())
}

// Get the extended phase description for a phase.
func (t *Task) getPhaseDescription(phaseName string) string {
	// Log the extended description of current phase
	if phaseDescription, found := PhaseDescriptions[t.Phase]; found {
		return phaseDescription
	}
	t.Log.Info("Missing phase description for phase: " + phaseName)
	// If no description available, just return phase name.
	return phaseName
}

// Log the "[RUN] <Phase description>" phase kickoff string unless
// DVM or DIM is already logging a duplicate phase description.
// This is meant to cut down on log noise when two controllers
// are waiting on the same thing.
func (t *Task) logRunHeader() {
	if t.Phase != WaitForDirectVolumeMigrationToComplete &&
		t.Phase != WaitForDirectImageMigrationToComplete {
		_, n, total := t.Itinerary.progressReport(t.Phase)
		t.Log.Info(fmt.Sprintf("[RUN] (Step %v/%v) %v", n, total, t.getPhaseDescription(t.Phase)))
	}
}

func (t *Task) hasPreBackupHooks() bool {
	var anyPreBackupHooks bool

	for i := range t.PlanResources.MigPlan.Spec.Hooks {
		if t.PlanResources.MigPlan.Spec.Hooks[i].Phase == migapi.PreBackupHookPhase {
			anyPreBackupHooks = true
		}
	}
	return anyPreBackupHooks
}

func (t *Task) hasPostBackupHooks() bool {
	var anyPostBackupHooks bool

	for i := range t.PlanResources.MigPlan.Spec.Hooks {
		if t.PlanResources.MigPlan.Spec.Hooks[i].Phase == migapi.PostBackupHookPhase {
			anyPostBackupHooks = true
		}
	}
	return anyPostBackupHooks
}

func (t *Task) hasPreRestoreHooks() bool {
	var anyPreRestoreHooks bool

	for i := range t.PlanResources.MigPlan.Spec.Hooks {
		if t.PlanResources.MigPlan.Spec.Hooks[i].Phase == migapi.PreRestoreHookPhase {
			anyPreRestoreHooks = true
		}
	}
	return anyPreRestoreHooks
}
func (t *Task) hasPostRestoreHooks() bool {
	var anyPostRestoreHooks bool

	for i := range t.PlanResources.MigPlan.Spec.Hooks {
		if t.PlanResources.MigPlan.Spec.Hooks[i].Phase == migapi.PostRestoreHookPhase {
			anyPostRestoreHooks = true
		}
	}
	return anyPostRestoreHooks
}
