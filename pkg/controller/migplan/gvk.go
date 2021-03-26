package migplan

import (
	"context"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/gvk"
	"github.com/opentracing/opentracing-go"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func (r ReconcileMigPlan) compareGVK(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "compareGVK")
		defer span.Finish()
	}

	// No spec chage this time
	if plan.HasReconciled() || !clustersReady(plan) {
		plan.Status.StageCondition(GVKsIncompatible)
		return nil
	}

	gvkCompare, err := r.newGVKCompare(plan)
	if err != nil {
		err = liberr.Wrap(err)
	}

	incompatibleMapping, err := gvkCompare.Compare()
	if err != nil {
		err = liberr.Wrap(err)
	}

	reportGVK(plan, incompatibleMapping)

	return nil
}

func (r ReconcileMigPlan) newGVKCompare(plan *migapi.MigPlan) (*gvk.Compare, error) {
	srcCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	dstCluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	srcClient, err := srcCluster.GetClient(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	dstClient, err := dstCluster.GetClient(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	dynamicClient, err := dynamic.NewForConfig(srcClient.RestConfig())
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return &gvk.Compare{
		Plan:                  plan,
		SrcClient:             dynamicClient,
		DstDiscovery:          dstClient,
		SrcDiscovery:          srcClient,
		CohabitatingResources: gvk.NewCohabitatingResources(),
	}, nil
}

func reportGVK(plan *migapi.MigPlan, incompatibleMapping map[string][]schema.GroupVersionResource) {
	incompatibleNamespaces := []migapi.IncompatibleNamespace{}

	for namespace, incompatibleGVKs := range incompatibleMapping {
		incompatibleResources := []migapi.IncompatibleGVK{}
		for _, res := range incompatibleGVKs {
			incompatibleResources = append(incompatibleResources, migapi.FromGVR(res))
		}

		incompatibleNamespace := migapi.IncompatibleNamespace{
			Name: namespace,
			GVKs: incompatibleResources,
		}
		incompatibleNamespaces = append(incompatibleNamespaces, incompatibleNamespace)
	}

	if len(incompatibleNamespaces) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     GVKsIncompatible,
			Status:   True,
			Category: Warn,
			Message:  "Some namespaces contain GVKs incompatible with destination cluster. See: `incompatibleNamespaces` for details.",
		})
	}

	plan.Status.Incompatible = migapi.Incompatible{
		Namespaces: incompatibleNamespaces,
	}
}

// Check if any blocker condition appeared on the migPlan after cluster validation phase
func clustersReady(plan *migapi.MigPlan) bool {
	clustersNotReadyConditions := []string{
		InvalidDestinationClusterRef,
		InvalidDestinationCluster,
		InvalidDestinationClusterRef,
		InvalidSourceClusterRef,
		DestinationClusterNotReady,
		SourceClusterNotReady,
	}
	return !plan.Status.HasAnyCondition(clustersNotReadyConditions...)
}
