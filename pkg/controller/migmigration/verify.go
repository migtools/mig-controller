package migmigration

import (
	"context"
	"fmt"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

var podManagingResources = [...]schema.GroupVersionKind{
	schema.GroupVersionKind{
		Group: "apps",
		Kind:  "statefulset",
	},
	schema.GroupVersionKind{
		Group: "apps",
		Kind:  "deployment",
	},
	schema.GroupVersionKind{
		Group: "",
		Kind:  "replicationcontroller",
	},
	schema.GroupVersionKind{
		Group: "extensions",
		Kind:  "replicaset",
	},
	schema.GroupVersionKind{
		Group: "apps.openshift.io",
		Kind:  "deploymentconfig",
	},
}

// MigrationCompleted will determine when the verification should be stopped
func (t *Task) MigrationCompleted() (bool, error) {
	plan := t.PlanResources.MigPlan
	client, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}

	// Check existing pods health
	err = t.verifyHealth(client)
	if err != nil {
		return false, err
	}

	// Scan namespaces for resources to wait
	for _, namespace := range plan.Spec.Namespaces {
		options := k8sclient.InNamespace(namespace)

		finished, err := t.namespacePodsRecreated(client, options)
		if err != nil {
			return false, err
		}
		if !finished {
			// Stop when unhealthy resources were confirmed
			return !t.Owner.IsHealthy(), nil
		}
	}

	return true, nil
}

func (t *Task) verifyHealth(client k8sclient.Client) error {
	t.Owner.Status.UnhealthyResources = migapi.UnhealthyResources{}
	if err := t.verifyAppState(client); err != nil {
		return err
	}

	t.reportMigrationHealth()

	return nil
}

func (t *Task) verifyAppState(client k8sclient.Client) error {
	// Scan namespaces
	for _, namespace := range t.PlanResources.MigPlan.Spec.Namespaces {
		options := k8sclient.InNamespace(namespace)

		unhealthyWorkloads, err := t.verifyInNamespaceHealth(client, options)
		if err != nil {
			return err
		}

		destinationNamespaces := &t.Owner.Status.UnhealthyResources.Namespaces
		t.Owner.Status.UnhealthyResources.AddUnhealthyResources(
			client,
			destinationNamespaces,
			namespace,
			unhealthyWorkloads)
	}

	return nil
}

func (t *Task) verifyInNamespaceHealth(client k8sclient.Client, options *k8sclient.ListOptions) (*[]unstructured.Unstructured, error) {

	pods, err := t.Owner.Status.UnhealthyResources.VerifyPods(client, options)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

func (t *Task) getUnhealthyNamespaces(clusterNamespaces *[]migapi.UnhealthyNamespace) (unhealthyNamespaceNames []string) {
	for _, namespace := range *clusterNamespaces {
		unhealthyNamespaceNames = append(unhealthyNamespaceNames, namespace.Name)
	}
	return
}

func (t *Task) reportMigrationHealth() {
	namespaceNames := t.getUnhealthyNamespaces(&t.Owner.Status.UnhealthyResources.Namespaces)
	if len(namespaceNames) != 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     migapi.UnhealthyDestinationNamespaces,
			Status:   True,
			Category: migapi.Warn,
			Reason:   migapi.ErrorsDetected,
			Message:  migapi.UnhealthyDestination,
			Items:    namespaceNames,
		})
	}
}

func (t *Task) namespacePodsRecreated(client k8sclient.Client, options *k8sclient.ListOptions) (bool, error) {
	finished, err := t.VerifyDaemonSets(client, options)
	if err != nil || !finished {
		return finished, err
	}

	return t.VerifyOwnerResources(client, options)
}

// VerifyOwnerResources does verification for ReplicaSets, Deployments, Statefulsets and DeploymentConfigs.
func (t *Task) VerifyOwnerResources(client k8sclient.Client, options *k8sclient.ListOptions) (bool, error) {
	podManagersList := unstructured.UnstructuredList{}

	for _, gvk := range podManagingResources {
		podManagersList.SetGroupVersionKind(gvk)
		err := client.List(context.TODO(), options, &podManagersList)
		if err != nil {
			return false, err
		}

		for _, podManager := range podManagersList.Items {
			expectedReplicas, found, err := unstructured.NestedFieldNoCopy(podManager.Object, "spec", "replicas")
			if err != nil {
				return false, err
			}
			if !found {
				err = fmt.Errorf("Replicas field was not found in %s kind", podManager.GetKind())
				return false, err
			}
			replicas, ok := expectedReplicas.(int64)
			if !ok {
				err = fmt.Errorf("Can't convert Replicas field to int64 for %s kind", podManager.GetKind())
				return false, err
			}
			if replicas == 0 {
				continue
			}

			readyReplicas, foundReady, err := unstructured.NestedFieldNoCopy(podManager.Object, "status", "readyReplicas")
			if err != nil {
				return false, err
			}

			if !foundReady || expectedReplicas != readyReplicas {
				return false, nil
			}
		}
	}

	return true, nil
}

// VerifyDaemonSets checks the state of DaemonSets
func (t *Task) VerifyDaemonSets(client k8sclient.Client, options *k8sclient.ListOptions) (bool, error) {
	daemonSetList := v1beta1.DaemonSetList{}

	err := client.List(context.TODO(), options, &daemonSetList)
	if err != nil {
		return false, err
	}

	for _, ds := range daemonSetList.Items {
		if ds.Status.DesiredNumberScheduled != ds.Status.NumberReady {
			return false, nil
		}
	}

	return true, nil
}
