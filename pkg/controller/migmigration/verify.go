package migmigration

import (
	"context"
	"fmt"

	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	health "github.com/konveyor/mig-controller/pkg/health"
)

// VerificationCompleted will determine when the verification should be stopped
func (t *Task) VerificationCompleted() (bool, error) {
	dstCluster := t.PlanResources.DestMigCluster
	if !dstCluster.Status.IsReady() {
		return false, nil
	}

	client, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}

	// Collect and update info about unhealthy pods
	err = t.updateResourcesHealth(client)
	if err != nil {
		return false, err
	}

	// Check that all pods are recreated
	finished, err := t.podsRecreated(client)
	if err != nil {
		return false, err
	}
	if !finished {
		// Stop when unhealthy resources were confirmed
		return !t.Owner.IsHealthy(), nil
	}

	return true, nil
}

func (t *Task) updateResourcesHealth(client k8sclient.Client) error {
	t.Owner.Status.UnhealthyResources = migapi.UnhealthyResources{}
	if err := t.getAppState(client); err != nil {
		return err
	}

	t.reportHealthCondition()

	return nil
}

func (t *Task) getAppState(client k8sclient.Client) error {
	// Scan namespaces
	for _, namespace := range t.PlanResources.MigPlan.Spec.Namespaces {
		options := k8sclient.InNamespace(namespace)

		unhealthyPods, err := health.PodsUnhealthy(client, options)
		if err != nil {
			return err
		}

		destinationNamespaces := &t.Owner.Status.UnhealthyResources.Namespaces
		t.Owner.Status.UnhealthyResources.AddResources(
			client,
			destinationNamespaces,
			namespace,
			unhealthyPods)
	}

	return nil
}

func (t *Task) podsRecreated(client k8sclient.Client) (bool, error) {
	targetNamespaces := t.PlanResources.MigPlan.Spec.Namespaces
	// Scan namespaces for resources to wait
	for _, namespace := range targetNamespaces {
		options := k8sclient.InNamespace(namespace)

		finished, err := health.DaemonSetsRecreated(client, options)
		if err != nil || !finished {
			return finished, err
		}

		finished, err = health.PodManagersRecreated(client, options)
		if err != nil || !finished {
			return finished, err
		}
	}

	return true, nil
}

func (t *Task) reportHealthCondition() {
	destination := t.PlanResources.DestMigCluster.GetName()
	if !t.Owner.IsHealthy() {
		t.Log.Info("Verification discovered unhealthy resources")
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     UnhealthyNamespaces,
			Status:   True,
			Category: migapi.Warn,
			Reason:   ErrorsDetected,
			Message:  fmt.Sprintf(UnhealthyNamespacesMessage, destination),
		})
	}
}

// Trigger a refresh on MigPlan resource
func (t *Task) refreshPlan() error {
	plan := t.PlanResources.MigPlan
	plan.Spec.Refresh = true
	err := t.Client.Update(context.TODO(), plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// Verify plan finished with refresh before migrating
func (t *Task) ensureRefreshed() bool {
	refreshed := false

	if !t.PlanResources.MigPlan.Spec.Refresh {
		refreshed = true
	}

	return refreshed
}
