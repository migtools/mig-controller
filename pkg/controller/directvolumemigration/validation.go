package directvolumemigration

import (
	"context"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	kapi "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// Types
const (
	InvalidSourceClusterRef         = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef    = "InvalidDestinationClusterRef"
	InvalidSourceCluster            = "InvalidSourceCluster"
	InvalidDestinationCluster       = "InvalidDestinationCluster"
	InvalidPVCs                     = "InvalidPVCs"
	SourceClusterNotReady           = "SourceClusterNotReady"
	DestinationClusterNotReady      = "DestinationClusterNotReady"
	PVCsNotFoundOnSourceCluster     = "PodsNotFoundOnSourceCluster"
	StunnelClientPodsPending        = "StunnelClientPodsPending"
	RsyncTransferPodsPending        = "RsyncTransferPodsPending"
	RsyncRouteNotAdmitted           = "RsyncRouteNotAdmitted"
	Running                         = "Running"
	Failed                          = "Failed"
	RsyncClientPodsPending          = "RsyncClientPodsPending"
	Succeeded                       = "Succeeded"
	SourceToDestinationNetworkError = "SourceToDestinationNetworkError"
	FailedCreatingRsyncPods         = "FailedCreatingRsyncPods"
	FailedDeletingRsyncPods         = "FailedDeletingRsyncPods"
	RsyncServerPodsRunningAsNonRoot = "RsyncServerPodsRunningAsNonRoot"
)

// Reasons
const (
	NotFound           = "NotFound"
	NotSet             = "NotSet"
	NotDistinct        = "NotDistinct"
	NotReady           = "NotReady"
	RsyncTimeout       = "RsyncTimedOut"
	RsyncNoRouteToHost = "RsyncNoRouteToHost"
)

// Messages
const (
	ReadyMessage                              = "Direct migration is ready"
	RunningMessage                            = "Step: %d/%d"
	InvalidSourceClusterReferenceMessage      = "The source cluster reference is invalid"
	InvalidDestinationClusterReferenceMessage = "The destination cluster reference is invalid"
	InvalidSourceClusterMessage               = "The source cluster is invalid"
	InvalidDestinationClusterMessage          = "The destination cluster is invalid"
	InvalidPVCsMessage                        = "The set of persistent volume claims is invalid"
	SourceClusterNotReadyMessage              = "The source cluster is not ready"
	DestinationClusterNotReadyMessage         = "The destination cluster is not ready"
	PVCsNotFoundOnSourceClusterMessage        = "The set of pvcs were not found on source cluster"
	SucceededMessage                          = "The migration has succeeded"
	FailedMessage                             = "The migration has failed.  See: Errors."
)

// Categories
const (
	Critical = migapi.Critical
	Advisory = migapi.Advisory
	Warn     = migapi.Warn
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Validate the direct resource
func (r ReconcileDirectVolumeMigration) validate(ctx context.Context, direct *migapi.DirectVolumeMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validate")
		defer span.Finish()
	}
	err := r.validateSrcCluster(ctx, direct)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateDestCluster(ctx, direct)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validatePVCs(ctx, direct)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r ReconcileDirectVolumeMigration) validateSrcCluster(ctx context.Context, direct *migapi.DirectVolumeMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateSrcCluster")
		defer span.Finish()
	}

	ref := direct.Spec.SrcMigClusterRef

	// Not Set
	if !migref.RefSet(ref) {
		direct.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidSourceClusterReferenceMessage,
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Not found
	if cluster == nil {
		direct.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidSourceClusterReferenceMessage,
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		direct.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNotReady,
			Status:   True,
			Reason:   NotReady,
			Category: Critical,
			Message:  SourceClusterNotReadyMessage,
		})
	}
	return nil
}

func (r ReconcileDirectVolumeMigration) validateDestCluster(ctx context.Context, direct *migapi.DirectVolumeMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateDestCluster")
		defer span.Finish()
	}

	ref := direct.Spec.DestMigClusterRef

	if !migref.RefSet(ref) {
		direct.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidDestinationClusterReferenceMessage,
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Not found
	if cluster == nil {
		direct.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidDestinationClusterReferenceMessage,
		})
		return nil
	}

	// Not ready
	if !cluster.Status.IsReady() {
		direct.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNotReady,
			Status:   True,
			Reason:   NotReady,
			Category: Critical,
			Message:  DestinationClusterNotReadyMessage,
		})
	}
	return nil
}

// TODO: Validate that storage class mappings have valid storage class selections
// Leaving as TODO because this is technically already validated from the
// migplan, so not necessary from directvolumemigration controller to be fair
func (r ReconcileDirectVolumeMigration) validateStorageClassMappings(direct *migapi.DirectVolumeMigration) error {
	return nil
}

func (r ReconcileDirectVolumeMigration) validatePVCs(ctx context.Context, direct *migapi.DirectVolumeMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePVCs")
		defer span.Finish()
	}

	allPVCs := direct.Spec.PersistentVolumeClaims

	// Check if PVCs were set
	if allPVCs == nil {
		direct.Status.SetCondition(migapi.Condition{
			Type:     InvalidPVCs,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidPVCsMessage,
		})
		return nil
	}
	// Get source cluster client
	cluster, err := direct.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Check if these PVCs actually exist on the source
	// cluster
	notFound := make([]string, 0)
	for _, specPVC := range allPVCs {
		// Check if pvc actually exists and is bound on source cluster
		// TODO: Check if PVC is actually attached. We should
		// assume all apps are quiesced
		pvc := kapi.PersistentVolumeClaim{}
		key := types.NamespacedName{Name: specPVC.Name, Namespace: specPVC.Namespace}
		err = client.Get(context.TODO(), key, &pvc)
		if err == nil {
			continue
		}
		if k8serror.IsNotFound(err) {
			notFound = append(notFound, specPVC.Name)
		} else {
			return liberr.Wrap(err)
		}
	}
	if len(notFound) > 0 {
		direct.Status.SetCondition(migapi.Condition{
			Type:     PVCsNotFoundOnSourceCluster,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  PVCsNotFoundOnSourceClusterMessage,
			Items:    notFound,
		})
		return nil
	}
	return nil
}
