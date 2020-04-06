package v1alpha1

import (
	"context"
	"strconv"

	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Convenience functions for managing the object model.
///

// List `open` MigPlans
// Returns and empty list when none found.
func ListPlans(client k8sclient.Client) ([]MigPlan, error) {
	list := MigPlanList{}
	options := k8sclient.MatchingField(ClosedIndexField, strconv.FormatBool(false))
	err := client.List(context.TODO(), options, &list)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// List MigCluster
// Returns and empty list when none found.
func ListClusters(client k8sclient.Client) ([]MigCluster, error) {
	list := MigClusterList{}
	err := client.List(context.TODO(), nil, &list)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// Get a referenced MigPlan.
// Returns `nil` when the reference cannot be resolved.
func GetPlan(client k8sclient.Client, ref *kapi.ObjectReference) (*MigPlan, error) {
	if ref == nil {
		return nil, nil
	}
	object := MigPlan{}
	err := client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		&object)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &object, err
}

// Get a referenced MigCluster.
// Returns `nil` when the reference cannot be resolved.
func GetCluster(client k8sclient.Client, ref *kapi.ObjectReference) (*MigCluster, error) {
	if ref == nil {
		return nil, nil
	}
	object := MigCluster{}
	err := client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		&object)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &object, err
}

// Get a referenced MigStorage.
// Returns `nil` when the reference cannot be resolved.
func GetStorage(client k8sclient.Client, ref *kapi.ObjectReference) (*MigStorage, error) {
	if ref == nil {
		return nil, nil
	}
	object := MigStorage{}
	err := client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		&object)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &object, err
}

// List MigStorage
// Returns and empty list when none found.
func ListStorage(client k8sclient.Client) ([]MigStorage, error) {
	list := MigStorageList{}
	err := client.List(context.TODO(), nil, &list)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// List MigMigrations
// Returns and empty list when none found.
func ListMigrations(client k8sclient.Client) ([]MigMigration, error) {
	list := MigMigrationList{}
	err := client.List(context.TODO(), nil, &list)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// Get a referenced Secret.
// Returns `nil` when the reference cannot be resolved.
func GetSecret(client k8sclient.Client, ref *kapi.ObjectReference) (*kapi.Secret, error) {
	if ref == nil {
		return nil, nil
	}
	object := kapi.Secret{}
	err := client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		&object)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &object, err
}

// DeleteMigrated - dispatches delete requests for migrated resources
func DeleteMigrated(config *rest.Config, uid string) error {
	GVRs, err := getGVRs(config)
	if err != nil {
		return err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	listOptions := k8sclient.MatchingLabels(map[string]string{
		MigratedByLabel: uid,
	}).AsListOptions()
	foreground := metav1.DeletePropagationForeground
	deleteOptions := (&k8sclient.DeleteOptions{
		PropagationPolicy: &foreground,
	}).AsDeleteOptions()

	for _, gvr := range GVRs {
		list, err := client.Resource(gvr).List(*listOptions)
		if err != nil {
			return err
		}
		for _, r := range list.Items {
			err = client.Resource(gvr).Namespace(r.GetNamespace()).Delete(r.GetName(), deleteOptions)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// WaitForMigratedDeletion - returns true if all of the migrated resources were deleted
func WaitForMigratedDeletion(config *rest.Config, uid string) (bool, error) {
	GVRs, err := getGVRs(config)
	if err != nil {
		return false, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return false, err
	}
	listOptions := k8sclient.MatchingLabels(map[string]string{
		MigratedByLabel: uid,
	}).AsListOptions()
	for _, gvr := range GVRs {
		// Count resource occurences
		list, err := client.Resource(gvr).List(*listOptions)
		if err != nil {
			return false, err
		}
		if len(list.Items) > 0 {
			return false, nil
		}
	}

	return true, nil
}

func getGVRs(config *rest.Config) ([]schema.GroupVersionResource, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	resourceList, err := CollectResources(discoveryClient)
	if err != nil {
		return nil, err
	}
	GVRs, err := ConvertToGVRList(resourceList)
	if err != nil {
		return nil, err
	}

	return GVRs, nil
}
