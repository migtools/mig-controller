package directvolumemigration

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/opentracing/opentracing-go"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	virtv1 "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Requeue
var FastReQ = time.Duration(time.Millisecond * 100)
var PollReQ = time.Duration(time.Second * 3)
var NoReQ = time.Duration(0)

// Phases
const (
	Created                                     = ""
	Started                                     = "Started"
	Prepare                                     = "Prepare"
	CleanStaleRsyncResources                    = "CleanStaleRsyncResources"
	CreateDestinationNamespaces                 = "CreateDestinationNamespaces"
	DestinationNamespacesCreated                = "DestinationNamespacesCreated"
	CreateDestinationPVCs                       = "CreateDestinationPVCs"
	CreateStunnelConfig                         = "CreateStunnelConfig"
	CreateRsyncConfig                           = "CreateRsyncConfig"
	CreateRsyncRoute                            = "CreateRsyncRoute"
	EnsureRsyncRouteAdmitted                    = "EnsureRsyncRouteAdmitted"
	CreateRsyncTransferPods                     = "CreateRsyncTransferPods"
	WaitForRsyncTransferPodsRunning             = "WaitForRsyncTransferPodsRunning"
	CreatePVProgressCRs                         = "CreatePVProgressCRs"
	RunRsyncOperations                          = "RunRsyncOperations"
	DeleteRsyncResources                        = "DeleteRsyncResources"
	WaitForRsyncResourcesTerminated             = "WaitForRsyncResourcesTerminated"
	WaitForStaleRsyncResourcesTerminated        = "WaitForStaleRsyncResourcesTerminated"
	Completed                                   = "Completed"
	MigrationFailed                             = "MigrationFailed"
	VerifyVMs                                   = "VerifyVMs"
	DeleteStaleVirtualMachineInstanceMigrations = "DeleteStaleVirtualMachineInstanceMigrations"
)

// labels
const (
	DirectVolumeMigration                        = "directvolumemigration"
	DirectVolumeMigrationRsyncTransfer           = "directvolumemigration-rsync-transfer"
	DirectVolumeMigrationRsyncTransferBlock      = "directvolumemigration-rsync-transfer-block"
	DirectVolumeMigrationRsyncConfig             = "directvolumemigration-rsync-config"
	DirectVolumeMigrationRsyncCreds              = "directvolumemigration-rsync-creds"
	DirectVolumeMigrationRsyncTransferSvc        = "directvolumemigration-rsync-transfer-svc"
	DirectVolumeMigrationRsyncTransferSvcBlock   = "directvolumemigration-rsync-transfer-svc-block"
	DirectVolumeMigrationRsyncTransferRoute      = "dvm"
	DirectVolumeMigrationRsyncTransferRouteBlock = "dvm-block"
	DirectVolumeMigrationStunnelConfig           = "crane2-stunnel-config"
	DirectVolumeMigrationStunnelCerts            = "crane2-stunnel-secret"
	DirectVolumeMigrationRsyncPass               = "directvolumemigration-rsync-pass"
	DirectVolumeMigrationStunnelTransfer         = "directvolumemigration-stunnel-transfer"
	DirectVolumeMigrationRsync                   = "rsync"
	DirectVolumeMigrationRsyncClient             = "rsync-client"
	DirectVolumeMigrationStunnel                 = "stunnel"
	MigratedByDirectVolumeMigration              = "migration.openshift.io/migrated-by-directvolumemigration" // (dvm UID)
	MigrationSourceFor                           = "migration.openshift.io/source-for-directvolumemigration"  // (dvm UID)
)

// Itinerary names
const (
	VolumeMigrationItinerary         = "VolumeMigration"
	VolumeMigrationFailedItinerary   = "VolumeMigrationFailed"
	VolumeMigrationRollbackItinerary = "VolumeMigrationRollback"
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
	Name: VolumeMigrationItinerary,
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: CleanStaleRsyncResources},
		{phase: WaitForStaleRsyncResourcesTerminated},
		{phase: VerifyVMs},
		{phase: CreateDestinationNamespaces},
		{phase: DestinationNamespacesCreated},
		{phase: CreateDestinationPVCs},
		{phase: DeleteStaleVirtualMachineInstanceMigrations},
		{phase: CreateRsyncRoute},
		{phase: EnsureRsyncRouteAdmitted},
		{phase: CreateRsyncConfig},
		{phase: CreateStunnelConfig},
		{phase: CreatePVProgressCRs},
		{phase: CreateRsyncTransferPods},
		{phase: WaitForRsyncTransferPodsRunning},
		{phase: RunRsyncOperations},
		{phase: DeleteRsyncResources},
		{phase: WaitForRsyncResourcesTerminated},
		{phase: Completed},
	},
}

var FailedItinerary = Itinerary{
	Name: VolumeMigrationFailedItinerary,
	Steps: []Step{
		{phase: MigrationFailed},
		{phase: Completed},
	},
}

var RollbackItinerary = Itinerary{
	Name: VolumeMigrationRollbackItinerary,
	Steps: []Step{
		{phase: Created},
		{phase: Started},
		{phase: Prepare},
		{phase: CleanStaleRsyncResources},
		{phase: DeleteStaleVirtualMachineInstanceMigrations},
		{phase: WaitForStaleRsyncResourcesTerminated},
		{phase: RunRsyncOperations},
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
	Log                          logr.Logger
	Client                       k8sclient.Client
	PrometheusAPI                prometheusv1.API
	sourceClient                 compat.Client
	destinationClient            compat.Client
	restConfig                   *rest.Config
	Owner                        *migapi.DirectVolumeMigration
	SSHKeys                      *sshKeys
	EndpointType                 migapi.EndpointType
	RsyncRoutes                  map[string]string
	Phase                        string
	PhaseDescription             string
	PlanResources                *migapi.PlanResources
	Requeue                      time.Duration
	Itinerary                    Itinerary
	Errors                       []string
	SparseFileMap                sparseFilePVCMap
	SourceLimitRangeMapping      limitRangeMap
	DestinationLimitRangeMapping limitRangeMap
	VirtualMachineMappings       VirtualMachineMappings
	PromQuery                    func(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error)
	Tracer                       opentracing.Tracer
	ReconcileSpan                opentracing.Span
}

type limitRangeMap map[string]corev1.LimitRange

type sshKeys struct {
	PublicKey  *rsa.PublicKey
	PrivateKey *rsa.PrivateKey
}

type VirtualMachineMappings struct {
	volumeVMIMMap        map[string]*virtv1.VirtualMachineInstanceMigration
	volumeVMNameMap      map[string]string
	runningVMVolumeNames []string
}

func (t *Task) init() error {
	t.RsyncRoutes = make(map[string]string)
	t.Requeue = FastReQ
	if t.failed() {
		t.Itinerary = FailedItinerary
	} else if t.Owner.IsRollback() {
		t.Itinerary = RollbackItinerary
	} else {
		t.Itinerary = VolumeMigration
	}
	if t.Itinerary.Name != t.Owner.Status.Itinerary {
		t.Phase = t.Itinerary.Steps[0].phase
	}
	// Initialize the source and destination clients
	_, err := t.getSourceClient()
	if err != nil {
		return err
	}
	_, err = t.getDestinationClient()
	return err
}

func (t *Task) populateVMMappings(namespace string) error {
	t.VirtualMachineMappings = VirtualMachineMappings{}
	volumeVMIMMap, err := t.getVolumeVMIMInNamespaces([]string{namespace})
	if err != nil {
		return err
	}
	volumeVMMap, err := getRunningVmVolumeMap(t.sourceClient, namespace)
	if err != nil {
		return err
	}
	volumeNames, err := t.getRunningVMVolumes([]string{namespace})
	if err != nil {
		return err
	}
	t.VirtualMachineMappings.volumeVMIMMap = volumeVMIMMap
	t.VirtualMachineMappings.volumeVMNameMap = volumeVMMap
	t.VirtualMachineMappings.runningVMVolumeNames = volumeNames
	return nil
}

func (t *Task) Run(ctx context.Context) error {
	t.Log = t.Log.WithValues("phase", t.Phase)
	// Init
	err := t.init()
	if err != nil {
		return err
	}

	// Log '[RUN] (Step 12/37) <Extended Phase Description>'
	t.logRunHeader()

	// Set up span for task.Run
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, t.Tracer, "dvm-phase-"+t.Phase)
		defer span.Finish()
	}

	// Run the current phase.
	switch t.Phase {
	case Created, Started, Prepare:
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
	case VerifyVMs:
		err := t.verifyVMs()
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
			return liberr.Wrap(err)
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
	case CreateRsyncRoute:
		err := t.ensureRsyncEndpoints()
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
		err := t.ensureStunnelTransport()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case CreateRsyncTransferPods:
		err := t.ensureRsyncTransferServer()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForRsyncTransferPodsRunning:
		running, nonRunningPods, err := t.areRsyncTransferPodsRunning()
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
			if now.Sub(cond.LastTransitionTime.Time.UTC()) > 3*time.Minute {
				// ["ns1/pod1", "ns2/pod2"]
				nonRunningPodStrings := []string{}
				for _, nonRunningPod := range nonRunningPods {
					if nonRunningPod != nil {
						nonRunningPodStrings = append(nonRunningPodStrings,
							fmt.Sprintf("oc describe pod %s -n %s", nonRunningPod.Name, nonRunningPod.Namespace))
					}
				}

				msg := fmt.Sprintf("Rsync Transfer Pod(s) on destination cluster have not started Running within 3 minutes. "+
					"Run these command(s) to check Pod warning events: [%s]",
					strings.Join(nonRunningPodStrings, ", "))

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
	case RunRsyncOperations:
		allCompleted, anyFailed, failureReasons, err := t.runRsyncOperations()
		if err != nil {
			return liberr.Wrap(FatalPlanError, err.Error())
		}
		t.Requeue = PollReQ
		if allCompleted {
			t.Requeue = NoReQ
			if anyFailed {
				t.fail(MigrationFailed, Critical, failureReasons)
				return nil
			}
			if err = t.next(); err != nil {
				return liberr.Wrap(err)
			}
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
	case DeleteRsyncResources:
		err := t.deleteRsyncResources()
		if err != nil {
			return liberr.Wrap(err)
		}
		t.Requeue = NoReQ
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case DeleteStaleVirtualMachineInstanceMigrations:
		if err := t.deleteStaleVirtualMachineInstanceMigrations(); err != nil {
			return liberr.Wrap(err)
		}
		if err = t.next(); err != nil {
			return liberr.Wrap(err)
		}
	case WaitForStaleRsyncResourcesTerminated, WaitForRsyncResourcesTerminated:
		deleted, err := t.waitForRsyncResourcesDeleted()
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
func (t *Task) fail(nextPhase, category string, reasons []string) {
	t.addErrors(reasons)
	t.Owner.AddErrors(t.Errors)
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     Failed,
		Status:   True,
		Reason:   t.Phase,
		Category: category,
		Message:  FailedMessage,
		Durable:  true,
	})
	if category == Critical {
		t.Phase = MigrationFailed
	} else {
		t.Phase = nextPhase
	}
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	t.Errors = append(t.Errors, errors...)
}

// Get whether the migration has failed
func (t *Task) failed() bool {
	return t.Owner.HasErrors() || t.Owner.Status.HasCondition(Failed)
}

// Get client for source cluster
func (t *Task) getSourceClient() (compat.Client, error) {
	if t.sourceClient != nil {
		return t.sourceClient, nil
	}
	cluster, err := t.Owner.GetSourceCluster(t.Client)
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	}
	t.sourceClient = client
	return client, nil
}

// Get client for destination cluster
func (t *Task) getDestinationClient() (compat.Client, error) {
	if t.destinationClient != nil {
		return t.destinationClient, nil
	}
	if t.Owner == nil {
		return nil, fmt.Errorf("owner is nil")
	}
	cluster, err := t.Owner.GetDestinationCluster(t.Client)
	if err != nil {
		return nil, err
	}
	client, err := cluster.GetClient(t.Client)
	if err != nil {
		return nil, err
	}
	t.destinationClient = client
	return client, nil
}

// Get DVM labels for the migration
func (t *Task) buildDVMLabels() map[string]string {

	dvmLabels := t.Owner.GetCorrelationLabels()
	dvmLabels["app"] = DirectVolumeMigrationRsyncTransfer
	dvmLabels["owner"] = DirectVolumeMigration
	dvmLabels[migapi.PartOfLabel] = migapi.Application
	// Label resources for rollback targeting
	if t.PlanResources != nil {
		if t.PlanResources.MigPlan != nil {
			dvmLabels[migapi.MigPlanLabel] = string(t.PlanResources.MigPlan.UID)
		}
	}

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
