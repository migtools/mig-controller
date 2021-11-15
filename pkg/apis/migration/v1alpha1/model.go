package v1alpha1

import (
	"context"
	"strconv"

	liberr "github.com/konveyor/controller/pkg/error"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Convenience functions for managing the object model.
///

const (
	MigplanMigrationRunning = "migplan.migration.openshift.io/running"
	MigplanMigrationFailed  = "migplan.migration.openshift.io/failed"
)

// List `open` MigPlans
// Returns and empty list when none found.
func ListPlans(client k8sclient.Client) ([]MigPlan, error) {
	list := MigPlanList{}
	options := k8sclient.MatchingFields{ClosedIndexField: strconv.FormatBool(false)}
	inNamespace := k8sclient.InNamespace(OpenshiftMigrationNamespace)
	err := client.List(context.TODO(), &list, inNamespace, options)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// List MigPlans with labels
// Returns and empty list when none found.
func ListPlansWithLabels(client k8sclient.Client, labels map[string]string) ([]MigPlan, error) {
	list := MigPlanList{}
	inNamespace := k8sclient.InNamespace(OpenshiftMigrationNamespace)
	err := client.List(
		context.TODO(),
		&list,
		inNamespace,
		&k8sclient.ListOptions{
			LabelSelector: k8sLabels.SelectorFromSet(labels),
		},
	)
	return list.Items, err
}

// List MigCluster
// Returns and empty list when none found.
func ListClusters(client k8sclient.Client) ([]MigCluster, error) {
	list := MigClusterList{}
	inNamespace := k8sclient.InNamespace(OpenshiftMigrationNamespace)
	err := client.List(context.TODO(), &list, inNamespace)
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

// Get a referenced Migration for DVM.
// Return nil if the reference cannot be resolved.
func GetMigrationForDVM(client k8sclient.Client, owners []metav1.OwnerReference) (*MigMigration, error) {
	if len(owners) == 0 {
		return nil, nil
	}
	migrationName := owners[0].Name
	migrationObject := MigMigration{}
	err := client.Get(context.TODO(),
		k8sclient.ObjectKey{
			Name:      migrationName,
			Namespace: OpenshiftMigrationNamespace,
		}, &migrationObject)

	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, liberr.Wrap(err)
		}
	}

	return &migrationObject, nil
}

// List MigStorage
// Returns and empty list when none found.
func ListStorage(client k8sclient.Client) ([]MigStorage, error) {
	list := MigStorageList{}
	inNamespace := k8sclient.InNamespace(OpenshiftMigrationNamespace)
	err := client.List(context.TODO(), &list, inNamespace)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// List MigHook
// Returns and empty list when none found.
func ListHook(client k8sclient.Client) ([]MigHook, error) {
	list := MigHookList{}
	inNamespace := k8sclient.InNamespace(OpenshiftMigrationNamespace)
	err := client.List(context.TODO(), &list, inNamespace)
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

// List MigMigrations
// Returns and empty list when none found.
func ListMigrations(client k8sclient.Client) ([]MigMigration, error) {
	list := MigMigrationList{}
	inNamespace := k8sclient.InNamespace(OpenshiftMigrationNamespace)
	err := client.List(context.TODO(), &list, inNamespace)
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

// Get a referenced ImageStream.
// Returns `nil` when the reference cannot be resolved.
func GetImageStream(client k8sclient.Client, ref *kapi.ObjectReference) (*imagev1.ImageStream, error) {
	if ref == nil {
		return nil, nil
	}
	object := imagev1.ImageStream{}
	err := client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		&object)
	if err != nil {
		return nil, err
	}

	return &object, err
}
