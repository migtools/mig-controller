package migdirect

import (
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Requeue
var FastReQ = time.Duration(time.Millisecond * 100)
var PollReQ = time.Duration(time.Second * 3)
var NoReQ = time.Duration(0)

// Phases
const (
	Created                      = ""
	Started                      = "Started"
	Prepare                      = "Prepare"
	EnsureDestinationNamespaces  = "EnsureDestinationNamespaces"
	DestinationNamespacesCreated = "DestinationNamespacesCreated"
	EnsureDestinationPVCs        = "EnsureDestinationPVCs"
	DestinationPVCsCreated       = "DestinationPVCsCreated"
	EnsureStunnelCerts           = "EnsureStunnelCerts"
	StunnelCertsCreated          = "StunnelCertsCreated"
	EnsureRsyncSecret            = "EnsureRsyncSecret"
	RsyncSecretCreated           = "RsyncSecretCreated"
	EnsureRsyncPods              = "EnsureRsyncPods"
	RsyncPodsCreated             = "RsyncPodsCreated"
	Verification                 = "Verification"
	EnsureRsyncPodsDeleted       = "EnsureRsyncPodsDeleted"
	EnsureRsyncPodsTerminated    = "EnsureRsyncPodsTerminated"
	DeleteStunnelCerts           = "DeleteStunnelCerts"
	DeleteRsyncSecret            = "DeleteRsyncSecret"
	Completed                    = "Completed"
	MigrationFailed              = "MigrationFailed"
)

// Flags
// TODO: are there any phases to skip?
/*const (
	Quiesce      = 0x01 // Only when QuiescePods (true).
)*/

// Step
type Step struct {
	// A phase name.
	phase string
	// Step included when ALL flags evaluate true.
	all uint8
	// Step included when ANY flag evaluates true.
	any uint8
}

type Itinerary struct {
	Name  string
	Steps []Step
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

var PVCItinerary = Itinerary{
	Name: "PVC",
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: EnsureDestinationNamespaces},
		{phase: DestinationNamespacesCreated},
		{phase: EnsureDestinationPVCs},
		{phase: DestinationPVCsCreated},
		{phase: Completed},
	},
}

var FailedItinerary = Itinerary{
	Name: "Failed",
	Steps: []Step{
		{phase: MigrationFailed},
		{phase: Completed},
	},
}

// A task that provides the complete migration workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A MigDirect resource.
// Phase - The task phase.
// Requeue - The requeueAfter duration. 0 indicates no requeue.
// Itinerary - The phase itinerary.
// Errors - Migration errors.
// Failed - Task phase has failed.
type Task struct {
	Log       logr.Logger
	Client    k8sclient.Client
	Owner     *migapi.MigDirect
	Phase     string
	Requeue   time.Duration
	Itinerary Itinerary
	Errors    []string
}

func (t *Task) init() error {
	t.Requeue = FastReQ
	if t.failed() {
		t.Itinerary = FailedItinerary
	} else {
		t.Itinerary = PVCItinerary
	}
	return nil
}

func (t *Task) Run() error {
	t.Log.Info("[RUN]", "phase", t.Phase)

	err := t.init()
	if err != nil {
		return err
	}

	// Run the current phase.
	switch t.Phase {
	case Created, Started:
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case Prepare:
	case EnsureDestinationNamespaces:
		// Create all of the namespaces the migrated PVCs are in are created on the
		// destination
		err := t.ensureDestinationNamespaces()
		if err != nil {
			liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DestinationNamespacesCreated:
		// Ensure the namespaces are created
		err := t.getDestinationNamespaces()
		if err != nil {
			liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case EnsureDestinationPVCs:
		// Create the PVCs on the destination
		err := t.ensureDestinationPVCs()
		if err != nil {
			liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DestinationPVCsCreated:
		// Get the PVCs on the destination and confirm they are bound
		err := t.getDestinationPVCs()
		if err != nil {
			liberr.Wrap(err)
		}
		t.Requeue = NoReQ
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
		t.Phase = next.phase
		return nil
	}
	t.Phase = Completed
	return nil
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

// Get whether the migration has failed
func (t *Task) failed() bool {
	return t.Owner.HasErrors() || t.Owner.Status.HasCondition(Failed)
}
