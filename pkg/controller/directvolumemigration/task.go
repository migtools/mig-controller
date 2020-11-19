package directvolumemigration

import (
	"crypto/rsa"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
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
	CreateDestinationNamespaces  = "CreateDestinationNamespaces"
	DestinationNamespacesCreated = "DestinationNamespacesCreated"
	CreateDestinationPVCs        = "CreateDestinationPVCs"
	DestinationPVCsCreated       = "DestinationPVCsCreated"
	CreateStunnelConfig          = "CreateStunnelConfig"
	CreateRsyncConfig            = "CreateRsyncConfig"
	CreateRsyncRoute             = "CreateRsyncRoute"
	// Do not initiate Rsync process unless we know stunnel is up and running
	// and in a healthy state

	// Distinguish rsync transfer pod step from rsync client pod
	// i.e. Create transfer pod, ensure it is healthy, then start client pod
	CreateStunnelPodOnSource        = "CreateStunnelPodOnSource"
	EnsureStunnelPodOnSourceHealthy = "EnsureStunnelPodOnSourceHealthy"
	//Next two phases are on destination
	CreateRsyncTransferPods         = "CreateRsyncTransferPods"
	WaitForRsyncTransferPodsRunning = "WaitForRsyncTransferPodsRunning"
	CreateStunnelClientPods         = "CreateStunnelClientPods"
	WaitForStunnelClientPodsRunning = "WaitForStunnelClientPodsRunning"
	CreatePVProgressCRs             = "CreatePVProgressCRs"
	CreateRsyncClientPods           = "CreateRsyncClientPods"
	WaitForRsyncClientPodsCompleted = "WaitForRsyncClientPodsCompleted"
	Verification                    = "Verification"
	DeleteRsyncResources            = "DeleteRsyncResources"
	EnsureRsyncPodsTerminated       = "EnsureRsyncPodsTerminated"
	Completed                       = "Completed"
	MigrationFailed                 = "MigrationFailed"
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
	// phaseDescription is a human readable description of the phase
	phaseDescription string
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

var VolumeMigration = Itinerary{
	Name: "VolumeMigration",
	Steps: []Step{
		{phase: Created, phaseDescription: "The DVM controller just saw DVM CR being created"},
		{phase: Started, phaseDescription: "The DVM controller is starting to work on the DVM CR"},
		{phase: Prepare, phaseDescription: "The DVM controller prepares the environment for volume migration, currently it is a no-op"},
		{phase: CreateDestinationNamespaces, phaseDescription: "Creating destination namespaces, no-op if they exist"},
		{phase: DestinationNamespacesCreated, phaseDescription: "Checking if the destination namespace is created, currently it is a no-op"},
		{phase: CreateDestinationPVCs, phaseDescription: "Creating PVCs to be migrated on the destination namespace, no-op if they exist"},
		{phase: DestinationPVCsCreated, phaseDescription: "Check if all the PVCs that were created are Bound, currently a no-op"},
		{phase: CreateRsyncRoute, phaseDescription: "Creating one route per namespace for rsync on the destination cluster"},
		{phase: CreateRsyncConfig, phaseDescription: "Creating configmap and secrets on both source and destination clusters for rsync configuration"},
		{phase: CreateStunnelConfig, phaseDescription: "Creating configmap and secrets for stunnel certs used to connect to rsync on both source and destionation cluster"},
		{phase: CreatePVProgressCRs, phaseDescription: "Creating direct volume migration progress CR to get progress percent and transfer rate"},
		{phase: CreateRsyncTransferPods, phaseDescription: "Creating rsync daemon pods on the destination cluster"},
		{phase: WaitForRsyncTransferPodsRunning, phaseDescription: "Wait for the rsync daemon pod to be running"},
		{phase: CreateStunnelClientPods, phaseDescription: "Create stunnel client pods on the source cluster"},
		{phase: WaitForStunnelClientPodsRunning, phaseDescription: "Waiting for the stunnel client pods to be running"},
		{phase: CreateRsyncClientPods, phaseDescription: "Create Rsync client pods"},
		{phase: WaitForRsyncClientPodsCompleted, phaseDescription: "Wait for the rsync client pods to be completed"},
		{phase: DeleteRsyncResources, phaseDescription: "Delete all the resources created while running this migration"},
		{phase: Completed, phaseDescription: "The migration has completed, phase check status.Itinerary to determined if it failed"},
	},
}

var FailedItinerary = Itinerary{
	Name: "VolumeMigrationFailed",
	Steps: []Step{
		{phase: MigrationFailed, phaseDescription: "The migration attempt failed, please see errors for more details"},
		{phase: Completed, phaseDescription: "The migration has completed, phase check status.Itinerary to determined if it failed"},
	},
}

// A task that provides the complete migration workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A DirectVolumeMigration resource.
// Phase - The task phase.
// Requeue - The requeueAfter duration. 0 indicates no requeue.
// Itinerary - The phase itinerary.
// Errors - Migration errors.
// Failed - Task phase has failed.
type Task struct {
	Log              logr.Logger
	Client           k8sclient.Client
	Owner            *migapi.DirectVolumeMigration
	SSHKeys          *sshKeys
	RsyncRoutes      map[string]string
	Phase            string
	PhaseDescription string
	Requeue          time.Duration
	Itinerary        Itinerary
	Errors           []string
}

type sshKeys struct {
	PublicKey  *rsa.PublicKey
	PrivateKey *rsa.PrivateKey
}

func (t *Task) init() error {
	t.RsyncRoutes = make(map[string]string)
	t.Requeue = FastReQ
	if t.failed() {
		t.Itinerary = FailedItinerary
	} else {
		t.Itinerary = VolumeMigration
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
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateDestinationNamespaces:
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
		// TODO: bad func name
		err := t.getDestinationNamespaces()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateDestinationPVCs:
		// Create the PVCs on the destination
		err := t.createDestinationPVCs()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DestinationPVCsCreated:
		// Get the PVCs on the destination and confirm they are bound
		err := t.getDestinationPVCs()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateRsyncRoute:
		err := t.createRsyncTransferRoute()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateRsyncConfig:
		err := t.createRsyncConfig()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateStunnelConfig:
		err := t.createStunnelConfig()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateRsyncTransferPods:
		err := t.createRsyncTransferPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForRsyncTransferPodsRunning:
		running, err := t.areRsyncTransferPodsRunning()
		if err != nil {
			return liberr.Wrap(err)
		}
		if running {
			t.Requeue = NoReQ
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case CreateStunnelClientPods:
		err := t.createStunnelClientPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForStunnelClientPodsRunning:
		running, err := t.areStunnelClientPodsRunning()
		if err != nil {
			return liberr.Wrap(err)
		}
		if running {
			t.Requeue = NoReQ
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case CreatePVProgressCRs:
		err := t.createPVProgressCR()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateRsyncClientPods:
		err := t.createRsyncClientPods()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForRsyncClientPodsCompleted:
		completed, failed, err := t.haveRsyncClientPodsCompletedOrFailed()
		if err != nil {
			return liberr.Wrap(err)
		}
		if failed {
			t.fail(MigrationFailed, []string{"One or more pods are in error state"})
		}
		if completed {
			t.Requeue = NoReQ
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
		}
	case DeleteRsyncResources:
		err := t.deleteRsyncResources()
		if err != nil {
			return liberr.Wrap(err)
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
		t.PhaseDescription = t.Itinerary.Steps[len(t.Itinerary.Steps)-1].phaseDescription
		return nil
	}
	for n := current + 1; n < len(t.Itinerary.Steps); n++ {
		next := t.Itinerary.Steps[n]
		t.Phase = next.phase
		t.PhaseDescription = next.phaseDescription
		return nil
	}
	t.Phase = Completed
	t.PhaseDescription = t.Itinerary.Steps[len(t.Itinerary.Steps)-1].phaseDescription
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

// Get client for source cluster
func (t *Task) getSourceClient() (compat.Client, error) {
	cluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Get client for destination cluster
func (t *Task) getDestinationClient() (compat.Client, error) {
	cluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	}
	return client, nil
}
