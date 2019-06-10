package migmigration

import (
	"context"

	appsv1 "github.com/openshift/api/apps/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

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
