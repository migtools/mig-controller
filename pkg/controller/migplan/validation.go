package migplan

import (
	"context"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// Types
const (
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

// Statuses
const (
	True  = "True"
	False = "False"
)

// Messages
const (
	ReadyMessage                        = "The migration plan is ready."
	InvalidSourceClusterRefMessage      = "The `srcMigClusterRef` must reference a `migcluster`."
	InvalidDestinationClusterRefMessage = "The `dstMigClusterRef` must reference a `migcluster`."
	InvalidStorageRefMessage            = "The `migStorageRef` must reference a `migstorage`."
	InvalidAssetCollectionRefMessage    = "The `migAssetCollectionRef` must reference a `migassetcollection`."
	SourceClusterNotReadyMessage        = "The referenced `srcMigClusterRef` does not have a `Ready` condition."
	DestinationClusterNotReadyMessage   = "The referenced `dstMigClusterRef` does not have a `Ready` condition."
	StorageNotReadyMessage              = "The referenced `migStorageRef` does not have a `Ready` condition."
	AssetCollectionNotReadyMessage      = "The referenced `migAssetCollectionRef` does not have a `Ready` condition."
)

// Validate the plan resource.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validate(plan *migapi.MigPlan) (error, int) {
	totalSet := 0
	var err error
	nSet := 0

	// Source cluster
	err, nSet = r.validateSourceCluster(plan)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Destination cluster
	err, nSet = r.validateDestinationCluster(plan)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Storage
	err, nSet = r.validateStorage(plan)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// AssetCollection
	err, nSet = r.validateAssetCollection(plan)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Apply changes.
	err = r.Update(context.TODO(), plan)
	if err != nil {
		return err, 0
	}

	return err, totalSet
}

// Validate the referenced storage.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateStorage(plan *migapi.MigPlan) (error, int) {
	ref := plan.Spec.MigStorageRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidStorageRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidStorageRefMessage,
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
			Type:    InvalidStorageRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidStorageRefMessage,
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
			Type:    StorageNotReady,
			Status:  True,
			Message: StorageNotReadyMessage,
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

// Validate the referenced assetCollection.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateAssetCollection(plan *migapi.MigPlan) (error, int) {
	ref := plan.Spec.MigAssetCollectionRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidAssetCollectionRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidAssetCollectionRefMessage,
		})
		plan.Status.DeleteCondition(AssetCollectionNotReady)
		return nil, 1
	}

	err, assetCollection := r.getAssetCollection(ref, plan.Namespace)
	if err != nil {
		return err, 0
	}

	// NotFound
	if assetCollection == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidAssetCollectionRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidAssetCollectionRefMessage,
		})
		plan.Status.DeleteCondition(AssetCollectionNotReady)
		return nil, 1
	} else {
		plan.Status.DeleteCondition(InvalidAssetCollectionRef)
	}

	// NotReady
	_, ready := assetCollection.Status.FindCondition(Ready)
	if ready == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    AssetCollectionNotReady,
			Status:  True,
			Message: AssetCollectionNotReadyMessage,
		})
		return nil, 1
	} else {
		plan.Status.DeleteCondition(AssetCollectionNotReady)
	}

	return nil, 0
}

func (r ReconcileMigPlan) getAssetCollection(ref *kapi.ObjectReference, ns string) (error, *migapi.MigAssetCollection) {
	key := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}

	assetCollection := migapi.MigAssetCollection{}
	err := r.Get(context.TODO(), key, &assetCollection)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return err, nil
		}
	}

	return nil, &assetCollection
}

// Validate the referenced source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateSourceCluster(plan *migapi.MigPlan) (error, int) {
	ref := plan.Spec.SrcMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidSourceClusterRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidSourceClusterRefMessage,
		})
		plan.Status.DeleteCondition(SourceClusterNotReady)
		return nil, 1
	}

	err, cluster := r.getCluster(ref, plan.Namespace)
	if err != nil {
		return err, 0
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidSourceClusterRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidSourceClusterRefMessage,
		})
		plan.Status.DeleteCondition(SourceClusterNotReady)
		return nil, 1
	} else {
		plan.Status.DeleteCondition(InvalidSourceClusterRef)
	}

	// NotReady
	_, ready := cluster.Status.FindCondition(Ready)
	if ready == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    SourceClusterNotReady,
			Status:  True,
			Message: SourceClusterNotReadyMessage,
		})
		return nil, 1
	} else {
		plan.Status.DeleteCondition(SourceClusterNotReady)
	}

	return nil, 0
}

// Validate the referenced source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateDestinationCluster(plan *migapi.MigPlan) (error, int) {
	ref := plan.Spec.DestMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidDestinationClusterRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidDestinationClusterRefMessage,
		})
		plan.Status.DeleteCondition(DestinationClusterNotReady)
		return nil, 1
	}

	err, cluster := r.getCluster(ref, plan.Namespace)
	if err != nil {
		return err, 0
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidDestinationClusterRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidDestinationClusterRefMessage,
		})
		plan.Status.DeleteCondition(DestinationClusterNotReady)
		return nil, 1
	} else {
		plan.Status.DeleteCondition(InvalidDestinationClusterRef)
	}

	// NotReady
	_, ready := cluster.Status.FindCondition(Ready)
	if ready == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    DestinationClusterNotReady,
			Status:  True,
			Message: DestinationClusterNotReadyMessage,
		})
		return nil, 1
	} else {
		plan.Status.DeleteCondition(DestinationClusterNotReady)
	}

	return nil, 0
}

func (r ReconcileMigPlan) getCluster(ref *kapi.ObjectReference, ns string) (error, *migapi.MigCluster) {
	key := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}

	cluster := migapi.MigCluster{}
	err := r.Get(context.TODO(), key, &cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return err, nil
		}
	}

	return nil, &cluster
}
