package migplan

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/gvk"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func (r ReconcileMigPlan) compareGVK(plan *migapi.MigPlan) error {
	// No spec chage this time
	if plan.HasReconciled() || !clustersReady(plan) {
		plan.Status.StageCondition(GVKsIncompatible)
		return nil
	}

	gvkCompare, err := r.newGVKCompare(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	incompatibleMapping, err := gvkCompare.Compare()
	if err != nil {
		log.Trace(err)
		return err
	}

	reportGVK(plan, incompatibleMapping)

	return nil
}

func (r ReconcileMigPlan) newGVKCompare(plan *migapi.MigPlan) (*gvk.Compare, error) {
	srcCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	dstCluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	src, err := srcCluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	srcClient := src.(compat.Client)
	dst, err := dstCluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	dstClient := dst.(compat.Client)
	dynamicClient, err := dynamic.NewForConfig(srcClient.Config)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	return &gvk.Compare{
		Plan:         plan,
		SrcClient:    dynamicClient,
		DstDiscovery: dstClient,
		SrcDiscovery: srcClient,
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
			Message:  NsGVKsIncompatible,
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
