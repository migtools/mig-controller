package migmigration

import (
	"context"

	appsv1 "github.com/openshift/api/apps/v1"
	coreappsv1 "k8s.io/api/apps/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// All quiesce functionality should be put here
func (t *Task) quiesceApplications() error {
	// If this is stage don't quiesce anything
	if t.stage() {
		return nil
	}
	err := t.scaleDownDCs()
	if err != nil {
		return err
	}
	err = t.scaleDownDeployments()
	return err
}

// Scales down Deployment Configs on source cluster
func (t *Task) scaleDownDCs() error {
	client, err := t.getSourceClient()
	if err != nil {
		return err
	}
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := appsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err = client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return err
		}
		for _, dc := range list.Items {
			dc.Spec.Replicas = 0
			err = client.Update(context.TODO(), &dc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Scales down all Deployments
func (t *Task) scaleDownDeployments() error {
	client, err := t.getSourceClient()
	if err != nil {
		return err
	}
	zeroReplica := int32(0)
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := coreappsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err = client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			return err
		}
		for _, dep := range list.Items {
			dep.Spec.Replicas = &zeroReplica
			err = client.Update(context.TODO(), &dep)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
