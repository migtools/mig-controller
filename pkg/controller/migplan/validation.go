package migplan

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// Types
const (
	// Type
	Ready                        = "Ready"
	InvalidSourceClusterRef      = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef = "InvalidDestinationClusterRef"
	InvalidStorageRef            = "InvalidStorageRef"
	InvalidAssetCollectionRef    = "InvalidAssetCollectionRef"
	SourceClusterNotReady        = "SourceClusterNotReady"
	DestinationClusterNotReady   = "DestinationClusterNotReady"
	StorageNotReady              = "StorageNotReady"
	AssetCollectionNotReady      = "AssetCollectionNotReady"
)

// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Status
const (
	True  = "True"
	False = "False"
)

// Validate the plan resource.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validate(plan *migapi.MigPlan) (error, int) {
	totalSet := 0

	// Storage
	err, nSet := r.validateStorage(plan)
	totalSet += nSet

	// Apply changes.
	err = r.Update(context.TODO(), plan)
	return err, totalSet
}

// Validate the referenced storage.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateStorage(plan *migapi.MigPlan) (error, int) {
	ref := plan.Spec.MigStorageRef

	// NotSet
	if ref == nil || (ref.Name == "") {
		plan.Status.SetCondition(migapi.Condition{
			Type:   InvalidStorageRef,
			Status: True,
			Reason: NotSet,
		})
		plan.Status.DeleteCondition(StorageNotReady)
		return nil, 1
	}

	err, storage := r.getStorage(ref, plan.Namespace)
	if err != nil {
		return err, 0
	}

	// NotFound
	if storage == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:   InvalidStorageRef,
			Status: True,
			Reason: NotFound,
		})
		plan.Status.DeleteCondition(StorageNotReady)
		return nil, 1
	} else {
		plan.Status.DeleteCondition(InvalidStorageRef)
	}

	// NotReady
	_, ready := storage.Status.FindCondition(Ready)
	if ready == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:   StorageNotReady,
			Status: True,
		})
		return nil, 1
	} else {
		plan.Status.DeleteCondition(StorageNotReady)
	}

	return nil, 0
}

func (r ReconcileMigPlan) getStorage(ref *kapi.ObjectReference, ns string) (error, *migapi.MigStorage) {
	key := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	if key.Namespace == "" {
		key.Namespace = ns
	}

	storage := migapi.MigStorage{}
	err := r.Get(context.TODO(), key, &storage)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return err, nil
		}
	}

	return nil, &storage
}
