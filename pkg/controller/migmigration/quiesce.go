package migmigration

import (
	"context"
	appsv1 "github.com/openshift/api/apps/v1"
	coreappsv1 "k8s.io/api/apps/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// All quiesce functionality should be put here
func (t *Task) quiesceApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownDCs(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownDeployments(client)
	if err != nil {
		log.Trace(err)
	}

	return err
}

// Scales down Deployment Configs on source cluster
func (t *Task) scaleDownDCs(client k8sclient.Client) error {
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := appsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, dc := range list.Items {
			if dc.Spec.Replicas == 0 {
				continue
			}
			dc.Spec.Replicas = 0
			err = client.Update(context.TODO(), &dc)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}

// Scales down all Deployments
func (t *Task) scaleDownDeployments(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := coreappsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, dep := range list.Items {
			if dep.Spec.Replicas == &zero {
				continue
			}
			dep.Spec.Replicas = &zero
			err = client.Update(context.TODO(), &dep)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}
