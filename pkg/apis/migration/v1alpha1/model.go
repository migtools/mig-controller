package v1alpha1

import (
	"context"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"strconv"

	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

// List `Running` MigPlans
// Returns and empty list when none found.
func ListRunningPlans(client k8sclient.Client) ([]MigPlan, error) {
	list := MigPlanList{}
	options := k8sclient.ListOptions{
		LabelSelector: k8sLabels.SelectorFromSet(map[string]string{
			"migplan-running" : "true",
		}),
	}
	err := client.List(context.TODO(), &options, &list)
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

// List MigHook
// Returns and empty list when none found.
func ListHook(client k8sclient.Client) ([]MigHook, error) {
	list := MigHookList{}
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
