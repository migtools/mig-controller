package migplan

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	gvk "github.com/fusor/mig-controller/pkg/gvk"
)

func (r ReconcileMigPlan) compareGVK(plan *migapi.MigPlan) error {
	// No spec chage this time
	if plan.HasReconciled() {
		return nil
	}

	gvkCompare, err := r.newGVKCompare(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	unsupportedMapping, err := gvkCompare.Compare()
	if err != nil {
		log.Trace(err)
		return err
	}

	reportGVK(plan, unsupportedMapping)

	return nil
}

func (r ReconcileMigPlan) newGVKCompare(plan *migapi.MigPlan) (*gvk.GVKCompare, error) {
	gvkCompare := &gvk.GVKCompare{
		Plan: plan,
	}

	err := gvkCompare.PrepareSourceDiscovery(r)
	if err != nil {
		return nil, err
	}

	err = gvkCompare.PrepareDestinationDiscovery(r)
	if err != nil {
		return nil, err
	}

	err = gvkCompare.PrepareSourceClient(r)
	if err != nil {
		return nil, err
	}

	return gvkCompare, nil
}

func reportGVK(plan *migapi.MigPlan, unsupportedMapping map[string][]schema.GroupVersionResource) {
	plan.Status.UnsupportedNamespaces = []migapi.UnsupportedNamespace{}

	for namespace, unsupportedGVRs := range unsupportedMapping {
		unsupportedResources := []string{}
		for _, res := range unsupportedGVRs {
			unsupportedResources = append(unsupportedResources, res.String())
		}

		unsupportedNamespace := migapi.UnsupportedNamespace{
			Name:                 namespace,
			UnsupportedResources: unsupportedResources,
		}
		plan.Status.UnsupportedNamespaces = append(plan.Status.UnsupportedNamespaces, unsupportedNamespace)
	}

	if len(plan.Status.UnsupportedNamespaces) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NotSupported,
			Status:   True,
			Category: Warn,
			Message:  NsNotSupported,
		})
	}
}
