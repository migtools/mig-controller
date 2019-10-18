package migmigration

import (
	"time"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Requeue
var FastReQ = time.Duration(time.Millisecond * 100)
var PollReQ = time.Duration(time.Second * 3)
var NoReQ = time.Duration(0)

// Phases
const (
	Created                       = ""
	Started                       = "Started"
	Prepare                       = "Prepare"
	EnsureCloudSecretPropagated   = "EnsureCloudSecretPropagated"
	EnsureInitialBackup           = "EnsureInitialBackup"
	InitialBackupCreated          = "InitialBackupCreated"
	InitialBackupFailed           = "InitialBackupFailed"
	AnnotateResources             = "AnnotateResources"
	EnsureStagePods               = "EnsureStagePods"
	StagePodsCreated              = "StagePodsCreated"
	RestartRestic                 = "RestartRestic"
	ResticRestarted               = "ResticRestarted"
	QuiesceApplications           = "QuiesceApplications"
	EnsureQuiesced                = "EnsureQuiesced"
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
	Verification                  = "Verification"
	EnsureStagePodsDeleted        = "EnsureStagePodsDeleted"
	EnsureAnnotationsDeleted      = "EnsureAnnotationsDeleted"
	Completed                     = "Completed"
)

// Flags
const (
	Quiesce      = 0x01 // Only when QuiescePods (true).
	HasStagePods = 0x02 // Only when stage pods created.
	HasPVs       = 0x04 // Only when PVs migrated.
	HasVerify    = 0x08 // Only when the plan has enabled verification
)

type Itinerary []Step

var StageItinerary = Itinerary{
	{phase: Created},
	{phase: Started},
	{phase: Prepare},
	{phase: EnsureCloudSecretPropagated},
	{phase: AnnotateResources, flags: HasPVs},
	{phase: EnsureStagePods, flags: HasPVs},
	{phase: StagePodsCreated, flags: HasStagePods},
	{phase: RestartRestic, flags: HasStagePods},
	{phase: ResticRestarted, flags: HasStagePods},
	{phase: QuiesceApplications, flags: Quiesce},
	{phase: EnsureQuiesced, flags: Quiesce},
	{phase: EnsureStageBackup, flags: HasPVs},
	{phase: StageBackupCreated, flags: HasPVs},
	{phase: EnsureStageBackupReplicated, flags: HasPVs},
	{phase: EnsureStageRestore, flags: HasPVs},
	{phase: StageRestoreCreated, flags: HasPVs},
	{phase: EnsureStagePodsDeleted, flags: HasStagePods},
	{phase: EnsureAnnotationsDeleted, flags: HasPVs},
	{phase: Completed},
}

var FinalItinerary = Itinerary{
	{phase: Created},
	{phase: Started},
	{phase: Prepare},
	{phase: EnsureCloudSecretPropagated},
	{phase: EnsureInitialBackup},
	{phase: InitialBackupCreated},
	{phase: AnnotateResources, flags: HasPVs},
	{phase: EnsureStagePods, flags: HasPVs},
	{phase: StagePodsCreated, flags: HasStagePods},
	{phase: RestartRestic, flags: HasStagePods},
	{phase: ResticRestarted, flags: HasStagePods},
	{phase: QuiesceApplications, flags: Quiesce},
	{phase: EnsureQuiesced, flags: Quiesce},
	{phase: EnsureStageBackup, flags: HasPVs},
	{phase: StageBackupCreated, flags: HasPVs},
	{phase: EnsureStageBackupReplicated, flags: HasPVs},
	{phase: EnsureStageRestore, flags: HasPVs},
	{phase: StageRestoreCreated, flags: HasPVs},
	{phase: EnsureStagePodsDeleted, flags: HasStagePods},
	{phase: EnsureAnnotationsDeleted, flags: HasPVs},
	{phase: EnsureInitialBackupReplicated},
	{phase: EnsureFinalRestore},
	{phase: FinalRestoreCreated},
	{phase: Verification, flags: HasVerify},
	{phase: Completed},
}

// Step
// phase - Phase name.
// flags - Used to qualify the step.
type Step struct {
	phase string
	flags uint8
}

// Get a progress report.
// Returns: phase, n, total.
func (r Itinerary) progressReport(phase string) (string, int, int) {
	n := 0
	total := len(r)
	for i, step := range r {
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
	BackupResources []string
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

	t.init()

	// Run the current phase.
	switch t.Phase {
	case Created, Started:
		t.next()
	case Prepare:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = t.deleteAnnotations()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.next()
	case EnsureCloudSecretPropagated:
		count := 0
		for _, cluster := range t.getBothClusters() {
			propagated, err := t.veleroPodCredSecretPropagated(cluster)
			if err != nil {
				log.Trace(err)
				return err
			}
			if propagated {
				count++
			} else {
				break
			}
		}
		if count == 2 {
			t.next()
		} else {
			t.Requeue = PollReQ
		}
	case EnsureInitialBackup:
		_, err := t.ensureInitialBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Requeue = NoReQ
		t.next()
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
				t.failed(InitialBackupFailed, reasons)
			} else {
				t.next()
			}
		} else {
			t.Requeue = NoReQ
		}
	case AnnotateResources:
		err := t.annotateStageResources()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.next()
	case EnsureStagePods:
		_, err := t.ensureStagePodsCreated()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.next()
	case StagePodsCreated:
		started, err := t.ensureStagePodsStarted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if started {
			t.next()
		} else {
			t.Requeue = NoReQ
		}
	case RestartRestic:
		err := t.restartResticPods()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Requeue = PollReQ
		t.next()
	case ResticRestarted:
		started, err := t.haveResticPodsStarted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if started {
			t.next()
		} else {
			t.Requeue = PollReQ
		}
	case QuiesceApplications:
		err := t.quiesceApplications()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.next()
	case EnsureQuiesced:
		quiesced, err := t.ensureQuiescedPodsTerminated()
		if err != nil {
			log.Trace(err)
			return err
		}
		if quiesced {
			t.next()
		} else {
			t.Requeue = PollReQ
		}
	case EnsureStageBackup:
		_, err := t.ensureStageBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.Requeue = 0
		t.next()
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
				t.failed(StageBackupFailed, reasons)
			} else {
				t.next()
			}
		} else {
			t.Requeue = NoReQ
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
			t.next()
		} else {
			t.Requeue = NoReQ
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
		t.Requeue = NoReQ
		t.next()
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
				t.failed(StageRestoreFailed, reasons)
			} else {
				t.next()
			}
		} else {
			t.Requeue = NoReQ
		}
	case EnsureStagePodsDeleted:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		t.next()
	case EnsureAnnotationsDeleted:
		if !t.keepAnnotations() {
			err := t.deleteAnnotations()
			if err != nil {
				log.Trace(err)
				return err
			}
		}
		t.next()
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
			t.next()
		} else {
			t.Requeue = NoReQ
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
		t.Requeue = NoReQ
		t.next()
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
				t.failed(FinalRestoreFailed, reasons)
			} else {
				t.next()
			}
		} else {
			t.Requeue = NoReQ
		}
	case Verification:
		completed, err := t.VerificationCompleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if completed {
			t.next()
		} else {
			t.Requeue = PollReQ
		}
	case StageBackupFailed, StageRestoreFailed:
		err := t.ensureStagePodsDeleted()
		if err != nil {
			log.Trace(err)
			return err
		}
		if !t.keepAnnotations() {
			err = t.deleteAnnotations()
			if err != nil {
				log.Trace(err)
				return err
			}
		}
		t.Requeue = NoReQ
		t.next()
	case InitialBackupFailed, FinalRestoreFailed:
		t.next()
	case Completed:
	}

	if t.Phase == Completed {
		t.Requeue = NoReQ
		t.Log.Info("[COMPLETED]")
	}

	return nil
}

// Initialize.
func (t *Task) init() {
	t.Requeue = FastReQ
	if t.stage() {
		t.Itinerary = StageItinerary
	} else {
		t.Itinerary = FinalItinerary
	}
}

// Advance the task to the next phase.
func (t *Task) next() {
	current := -1
	for i, step := range t.Itinerary {
		if step.phase != t.Phase {
			continue
		}
		current = i
		break
	}
	if current == -1 {
		t.Phase = Completed
		return
	}
	for n := current + 1; n < len(t.Itinerary); n++ {
		next := t.Itinerary[n]
		if next.flags&HasPVs != 0 && !t.hasPVs() {
			continue
		}
		if next.flags&HasStagePods != 0 && !t.Owner.Status.HasCondition(StagePodsCreated) {
			continue
		}
		if next.flags&Quiesce != 0 && !t.quiesce() {
			continue
		}
		if next.flags&HasVerify != 0 && !t.hasVerify() {
			continue
		}
		t.Phase = next.phase
		return
	}

	t.Phase = Completed
}

// Phase failed.
func (t *Task) failed(nextPhase string, reasons []string) {
	t.addErrors(reasons)
	t.Phase = nextPhase
	t.Owner.AddErrors(t.Errors)
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     Failed,
		Status:   True,
		Reason:   t.Phase,
		Category: Advisory,
		Message:  FailedMessage,
		Durable:  true,
	})
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
			log.Trace(err)
			return nil, err
		}
		list = append(list, client)
	}

	return list, nil
}

// Get both source and destination clients with associated namespaces.
func (t *Task) getBothClientsWithNamespaces() ([]k8sclient.Client, [][]string, error) {
	clientList, err := t.getBothClients()
	if err != nil {
		log.Trace(err)
		return nil, nil, err
	}
	namespaceList := [][]string{t.sourceNamespaces(), t.destinationNamespaces()}

	return clientList, namespaceList, nil
}
