package directvolumemigration

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/opentracing/opentracing-go"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Requeue
var FastReQ = time.Duration(time.Millisecond * 100)
var PollReQ = time.Duration(time.Second * 3)
var NoReQ = time.Duration(0)

// Phases
const (
	Created                              = ""
	Started                              = "Started"
	Prepare                              = "Prepare"
	CleanStaleRsyncResources             = "CleanStaleRsyncResources"
	CreateDestinationNamespaces          = "CreateDestinationNamespaces"
	DestinationNamespacesCreated         = "DestinationNamespacesCreated"
	CreateDestinationPVCs                = "CreateDestinationPVCs"
	DestinationPVCsCreated               = "DestinationPVCsCreated"
	CreateStunnelConfig                  = "CreateStunnelConfig"
	CreateRsyncConfig                    = "CreateRsyncConfig"
	CreateRsyncRoute                     = "CreateRsyncRoute"
	EnsureRsyncRouteAdmitted             = "EnsureRsyncRouteAdmitted"
	CreateRsyncTransferPods              = "CreateRsyncTransferPods"
	WaitForRsyncTransferPodsRunning      = "WaitForRsyncTransferPodsRunning"
	CreateStunnelClientPods              = "CreateStunnelClientPods"
	WaitForStunnelClientPodsRunning      = "WaitForStunnelClientPodsRunning"
	CreatePVProgressCRs                  = "CreatePVProgressCRs"
	CreateRsyncClientPods                = "CreateRsyncClientPods"
	WaitForRsyncClientPodsCompleted      = "WaitForRsyncClientPodsCompleted"
	Verification                         = "Verification"
	DeleteRsyncResources                 = "DeleteRsyncResources"
	WaitForRsyncResourcesTerminated      = "WaitForRsyncResourcesTerminated"
	WaitForStaleRsyncResourcesTerminated = "WaitForStaleRsyncResourcesTerminated"
	Completed                            = "Completed"
	MigrationFailed                      = "MigrationFailed"
)

// labels
const (
	DirectVolumeMigration                   = "directvolumemigration"
	DirectVolumeMigrationRsyncTransfer      = "directvolumemigration-rsync-transfer"
	DirectVolumeMigrationRsyncConfig        = "directvolumemigration-rsync-config"
	DirectVolumeMigrationRsyncCreds         = "directvolumemigration-rsync-creds"
	DirectVolumeMigrationRsyncTransferSvc   = "directvolumemigration-rsync-transfer-svc"
	DirectVolumeMigrationRsyncTransferRoute = "dvm"
	DirectVolumeMigrationStunnelConfig      = "directvolumemigration-stunnel-config"
	DirectVolumeMigrationStunnelCerts       = "directvolumemigration-stunnel-certs"
	DirectVolumeMigrationRsyncPass          = "directvolumemigration-rsync-pass"
	DirectVolumeMigrationStunnelTransfer    = "directvolumemigration-stunnel-transfer"
	DirectVolumeMigrationRsync              = "rsync"
	DirectVolumeMigrationRsyncClient        = "rsync-client"
	DirectVolumeMigrationStunnel            = "stunnel"
	MigratedByPlanLabel                     = "migration.openshift.io/migrated-by-migplan"      // (migplan UID)
	MigratedByMigrationLabel                = "migration.openshift.io/migrated-by-migmigration" // (migmigration UID)
	PartOfLabel                             = "openshift-migration"
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

var VolumeMigration = Itinerary{
	Name: "VolumeMigration",
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: CleanStaleRsyncResources},
		{phase: WaitForStaleRsyncResourcesTerminated},
		{phase: CreateDestinationNamespaces},
		{phase: DestinationNamespacesCreated},
		{phase: CreateDestinationPVCs},
		{phase: DestinationPVCsCreated},
		{phase: CreateRsyncRoute},
		{phase: EnsureRsyncRouteAdmitted},
		{phase: CreateRsyncConfig},
		{phase: CreateStunnelConfig},
		{phase: CreatePVProgressCRs},
		{phase: CreateRsyncTransferPods},
		{phase: WaitForRsyncTransferPodsRunning},
		{phase: CreateStunnelClientPods},
		{phase: WaitForStunnelClientPodsRunning},
		{phase: CreateRsyncClientPods},
		{phase: WaitForRsyncClientPodsCompleted},
		{phase: DeleteRsyncResources},
		{phase: WaitForRsyncResourcesTerminated},
		{phase: Completed},
	},
}

var FailedItinerary = Itinerary{
	Name: "VolumeMigrationFailed",
	Steps: []Step{
		{phase: MigrationFailed},
		{phase: Completed},
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
	PlanResources    *migapi.PlanResources
	MigrationUID     string
	Requeue          time.Duration
	Itinerary        Itinerary
	Errors           []string

	Tracer        opentracing.Tracer
	ReconcileSpan opentracing.Span
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
	t.Log = t.Log.WithValues("Phase", t.Phase)
	// Init
	err := t.init()
	if err != nil {
		return err
	}

	// Log '[RUN] (Step 12/37) <Extended Phase Description>'
	t.logRunHeader()

	// Set up span for task.Run
	if t.ReconcileSpan != nil {
		phaseSpan := t.Tracer.StartSpan(
			"dvm-phase-"+t.Phase,
			opentracing.ChildOf(t.ReconcileSpan.Context()),
		)
		defer phaseSpan.Finish()
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
	case CleanStaleRsyncResources:
		// TODO Need to add some labels during DVM run to differentiate
		// deletion of rsync resources that are active vs stale. Using
		// one label for both is the wrong approach.
		err := t.deleteRsyncResources()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
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
	case EnsureRsyncRouteAdmitted:
		admitted, reasons, err := t.areRsyncRoutesAdmitted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if admitted {
			t.Requeue = NoReQ
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Log.Info("Some Rsync Transfer Routes have not yet been admitted. Waiting.")
			t.Requeue = PollReQ
			t.Owner.Status.StageCondition(Running)
			cond := t.Owner.Status.FindCondition(Running)
			if cond == nil {
				return fmt.Errorf("unable to find running condition")
			}
			now := time.Now().UTC()
			msg := fmt.Sprintf("Rsync Transfer Routes have failed to be admitted within 3 minutes on "+
				"destination cluster. Errors: %v", reasons)
			t.Log.Info(msg)
			if now.Sub(cond.LastTransitionTime.Time.UTC()) > 3*time.Minute {
				t.Owner.Status.SetCondition(
					migapi.Condition{
						Type:     RsyncRouteNotAdmitted,
						Status:   True,
						Reason:   migapi.NotReady,
						Category: Warn,
						Message:  msg,
					},
				)
			}
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
			conditions := t.Owner.Status.FindConditionByCategory(Warn)
			for _, c := range conditions {
				if c.Reason == RsyncTransferPodsPending {
					t.Owner.Status.DeleteCondition(RsyncTransferPodsPending)
				}
			}
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Requeue = PollReQ
			t.Owner.Status.StageCondition(Running)
			cond := t.Owner.Status.FindCondition(Running)
			if cond == nil {
				return fmt.Errorf("'Running' condition not found on DVM [%v/%v]", t.Owner.Namespace, t.Owner.Name)
			}
			now := time.Now().UTC()
			if now.Sub(cond.LastTransitionTime.Time.UTC()) > 10*time.Minute {
				msg := "Rsync Transfer Pod(s) on destination cluster have not started Running after 10 minutes."
				t.Log.Info(msg)
				t.Owner.Status.SetCondition(
					migapi.Condition{
						Type:     RsyncTransferPodsPending,
						Status:   True,
						Reason:   migapi.NotReady,
						Category: Warn,
						Message:  msg,
					},
				)
			}
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
			conditions := t.Owner.Status.FindConditionByCategory(Warn)
			for _, c := range conditions {
				if c.Reason == StunnelClientPodsPending {
					t.Owner.Status.DeleteCondition("StunnelClientPodsPending")
				}
			}
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		} else {
			t.Owner.Status.StageCondition(Running)
			cond := t.Owner.Status.FindCondition(Running)
			if cond == nil {
				return fmt.Errorf("`Running` condition missing on DVM %s/%s", t.Owner.Namespace, t.Owner.Name)
			}
			now := time.Now().UTC()
			if now.After(cond.LastTransitionTime.Time) {
				if now.Sub(cond.LastTransitionTime.Time).Round(time.Minute) > 10*time.Minute {
					t.Owner.Status.SetCondition(
						migapi.Condition{
							Type:     StunnelClientPodsPending,
							Status:   True,
							Reason:   migapi.NotReady,
							Category: Warn,
							Message:  "Stunnel Client Pod(s) on the source cluster are not ready after 10 minutes.",
							Durable:  true,
						},
					)
				}
			}
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
		if completed {
			isStunnelTimeout, err := hasAllRsyncClientPodsTimedOut(t.getPVCNamespaceMap(), t.Client, t.Owner.Name)
			if err != nil {
				return err
			}
			if isStunnelTimeout {
				t.Owner.Status.SetCondition(migapi.Condition{
					Type:     migapi.ReconcileFailed,
					Status:   True,
					Reason:   SourceToDestinationNetworkError,
					Category: migapi.Error,
					Message: "All the rsync client pods on source are timing out at 20 seconds, " +
						"please check your network configuration (like egressnetworkpolicy) that would block traffic from " +
						"source namespace to destination",
					Durable: true,
				})
				t.fail(MigrationFailed, []string{"All the source cluster Rsync Pods have timed out, look at error condition for more details"})
				t.Requeue = NoReQ
				return nil
			}
			isNoRouteToHost, err := isAllRsyncClientPodsNoRouteToHost(t.getPVCNamespaceMap(), t.Client, t.Owner.Name)
			if err != nil {
				return err
			}
			if isNoRouteToHost {
				t.Owner.Status.SetCondition(migapi.Condition{
					Type:     migapi.ReconcileFailed,
					Status:   True,
					Reason:   SourceToDestinationNetworkError,
					Category: migapi.Error,
					Message: "All Rsync client Pods on Source Cluster are failing because of \"no route to host\" error," +
						"please check your network configuration",
					Durable: true,
				})
				t.fail(MigrationFailed, []string{"All the source Rsync client Pods have timed out, look at error condition for more details"})
				t.Requeue = NoReQ
				return nil
			}
			if failed {
				t.fail(MigrationFailed, []string{"One or more Rsync client Pods are in error state"})
			}
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
	case WaitForStaleRsyncResourcesTerminated, WaitForRsyncResourcesTerminated:
		err, deleted := t.waitForRsyncResourcesDeleted()
		if err != nil {
			return liberr.Wrap(err)
		}
		if deleted {
			t.Requeue = NoReQ
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
		}
		t.Log.Info("Stale Rsync resources are still terminating. Waiting.")
		t.Requeue = PollReQ
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
	// Write time taken to complete phase
	t.Owner.Status.Conditions.StageCondition(Running)
	cond := t.Owner.Status.FindCondition(Running)
	if cond != nil {
		elapsed := time.Since(cond.LastTransitionTime.Time)
		t.Log.Info("Phase completed", "phaseElapsed", elapsed)
	}

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
		t.PhaseDescription = phaseDescriptions[t.Phase]
		return nil
	}
	for n := current + 1; n < len(t.Itinerary.Steps); n++ {
		next := t.Itinerary.Steps[n]
		t.Phase = next.phase
		t.PhaseDescription = phaseDescriptions[t.Phase]
		return nil
	}
	t.Phase = Completed
	t.PhaseDescription = phaseDescriptions[t.Phase]
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

// Get DVM labels for the migration
func (t *Task) buildDVMLabels() map[string]string {
	dvmLabels := make(map[string]string)

	dvmLabels["app"] = DirectVolumeMigrationRsyncTransfer
	dvmLabels["owner"] = DirectVolumeMigration
	dvmLabels["app.kubernetes.io/part-of"] = PartOfLabel

	return dvmLabels
}

// Get the extended phase description for a phase.
func (t *Task) getPhaseDescription(phaseName string) string {
	// Log the extended description of current phase
	if phaseDescription, found := phaseDescriptions[t.Phase]; found {
		return phaseDescription
	}
	t.Log.Info("Missing phase description for phase: " + phaseName)
	// If no description available, just return phase name.
	return phaseName
}

// Log the "[RUN] (Step 12/37) <Phase description>" phase kickoff string
// This is meant to cut down on log noise when two controllers
// are waiting on the same thing.
func (t *Task) logRunHeader() {
	_, n, total := t.Itinerary.progressReport(t.Phase)
	t.Log.Info(fmt.Sprintf("[RUN] (Step %v/%v) %v", n, total, t.getPhaseDescription(t.Phase)))
}
