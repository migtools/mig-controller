package migplan

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	gvk "github.com/fusor/mig-controller/pkg/gvkcompare"
)

func (r ReconcileMigPlan) compareGVK(plan *migapi.MigPlan) error {
	// No spec chage this time
	if plan.HasReconciled() {
		return nil
	}

	plan.Status.InitUnsupported()

	gvkCompare, err := r.prepareGVKCompare(plan)
	if err != nil {
		log.Error(err, "Failed to prepare GVK compare")
		return err
	}

	err = gvkCompare.Compare()
	if err != nil {
		log.Error(err, "Failed to compare GVRs on both clusters")
		return err
	}

	if len(plan.Status.UnsupportedNamespaces) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NotSupported,
			Status:   True,
			Category: Warn,
			Message:  NsNotSupported,
			Items:    plan.Status.SelectUnsupported(),
		})
	}

	return nil
}

func (r ReconcileMigPlan) prepareGVKCompare(plan *migapi.MigPlan) (*gvk.GVKCompare, error) {
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
