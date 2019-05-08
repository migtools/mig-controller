package v1alpha1

import (
	"context"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Convenience functions for managing the object model.
///

// List MigPlans
// Returns and empty list when none found.
func ListPlans(client k8sclient.Client, ns string) ([]MigPlan, error) {
	list := MigPlanList{}
	options := k8sclient.InNamespace(ns)
	err := client.List(context.TODO(), options, &list)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// List MigCluster
// Returns and empty list when none found.
func ListClusters(client k8sclient.Client, ns string) ([]MigCluster, error) {
	list := MigClusterList{}
	options := k8sclient.InNamespace(ns)
	err := client.List(context.TODO(), options, &list)
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

// Get a referenced MigAssetCollection.
// Returns `nil` when the reference cannot be resolved.
func GetAssetCollection(client k8sclient.Client, ref *kapi.ObjectReference) (*MigAssetCollection, error) {
	if ref == nil {
		return nil, nil
	}
	object := MigAssetCollection{}
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
