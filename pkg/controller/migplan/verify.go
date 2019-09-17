package migplan

import (
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	UnhealthySourceNamespaces      = "UnhealthySourceNamespaces"
	UnhealthyDestinationNamespaces = "UnhealthyDestinationNamespaces"
)

// Reasons
const (
	ErrorsDetected = "ErrorsDetected"
)

// Messages
const (
	UnhealthyDestination = "The destination cluster has unhealthy migrated namespaces []. See status.destination for details."
	UnhealthySource      = "The source cluster has unhealthy namespaces []. See status.source for details."
)

func (r ReconcileMigPlan) handleVerify(plan *migapi.MigPlan) error {
	if plan.GetVerify() {
		plan.Status.UnhealthyResources = migapi.UnhealthyResources{}
		if err := r.verifyDestinationAppState(plan); err != nil {
			return err
		}

		if err := r.verifySourceAppState(plan); err != nil {
			return err
		}

		r.reportHealth(plan)
		plan.UnsetVerify()
	}

	return nil
}

func (r ReconcileMigPlan) verifySourceAppState(plan *migapi.MigPlan) error {
	// Get srcMigCluster
	srcMigCluster, err := plan.GetSourceCluster(r.Client)
	if err != nil {
		return err
	}

	srcClient, err := srcMigCluster.GetClient(r)
	if err != nil {
		return err
	}

	// Scan namespaces
	for _, namespace := range plan.Spec.Namespaces {
		options := k8sclient.InNamespace(namespace)

		unhealthyWorkloads, err := r.verifyInNamespaceHealth(srcClient, options, plan)
		if err != nil {
			return err
		}

		sourceNamespaces := &plan.Status.Source
		plan.Status.UnhealthyResources.AddUnhealthyResources(
			srcClient,
			sourceNamespaces,
			namespace,
			unhealthyWorkloads)
	}

	return nil
}

func (r ReconcileMigPlan) verifyDestinationAppState(plan *migapi.MigPlan) error {
	// Get destMigCluster
	destMigCluster, err := plan.GetDestinationCluster(r.Client)
	if err != nil {
		return err
	}

	dstClient, err := destMigCluster.GetClient(r.Client)
	if err != nil {
		return err
	}

	// Scan namespaces
	for _, namespace := range plan.Spec.Namespaces {
		options := k8sclient.InNamespace(namespace)

		unhealthyWorkloads, err := r.verifyInNamespaceHealth(dstClient, options, plan)
		if err != nil {
			return err
		}

		destinationNamespaces := &plan.Status.Destination
		plan.Status.UnhealthyResources.AddUnhealthyResources(
			dstClient,
			destinationNamespaces,
			namespace,
			unhealthyWorkloads)
	}

	return nil
}

func (r ReconcileMigPlan) verifyInNamespaceHealth(client k8sclient.Client, options *k8sclient.ListOptions, plan *migapi.MigPlan) (*[]unstructured.Unstructured, error) {

	pods, err := plan.Status.UnhealthyResources.VerifyPods(client, options)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

func (r ReconcileMigPlan) getUnhealthyNamespaces(clusterNamespaces *[]migapi.UnhealthyNamespace) (unhealthyNamespaceNames []string) {
	for _, namespace := range *clusterNamespaces {
		unhealthyNamespaceNames = append(unhealthyNamespaceNames, namespace.UnhealthyNamespace)
	}
	return
}

func (r ReconcileMigPlan) reportHealth(plan *migapi.MigPlan) {
	namespaceNames := r.getUnhealthyNamespaces(&plan.Status.UnhealthyResources.Source)
	if len(namespaceNames) != 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     UnhealthySourceNamespaces,
			Status:   True,
			Category: Warn,
			Message:  UnhealthySource,
			Items:    namespaceNames,
		})
	}

	namespaceNames = r.getUnhealthyNamespaces(&plan.Status.UnhealthyResources.Destination)
	if len(namespaceNames) != 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     UnhealthyDestinationNamespaces,
			Status:   True,
			Category: Warn,
			Message:  UnhealthyDestination,
			Items:    namespaceNames,
		})
	}
}
