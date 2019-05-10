package migplan

import (
	"context"
	"fmt"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"strings"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
)

// Types
const (
	InvalidSourceClusterRef      = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef = "InvalidDestinationClusterRef"
	InvalidStorageRef            = "InvalidStorageRef"
	InvalidAssetCollectionRef    = "InvalidAssetCollectionRef"
	SourceClusterNotReady        = "SourceClusterNotReady"
	DestinationClusterNotReady   = "DestinationClusterNotReady"
	StorageNotReady              = "StorageNotReady"
	AssetCollectionNotReady      = "AssetCollectionNotReady"
	AssetNamespacesNotFound      = "AssetNamespacesNotFound"
	InvalidDestinationCluster    = "InvalidDestinationCluster"
	EnsureStorageFailed          = "EnsureStorageFailed"
)

// Reasons
const (
	NotSet      = "NotSet"
	NotFound    = "NotFound"
	NotDistinct = "NotDistinct"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
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
	AssetNamespaceNotFoundMessage       = "The following asset `namespaces` [%s] not found on the source cluster."
	InvalidDestinationClusterMessage    = "The `srcMigClusterRef` and `dstMigClusterRef` cannot be the same."
	EnsureStorageFailedMessage          = "Failed to create/validate backup and volume snapshot storage."
)

// Validate the plan resource.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validate(plan *migapi.MigPlan) (int, error) {
	totalSet := 0
	var err error
	nSet := 0

	// Source cluster
	nSet, err = r.validateSourceCluster(plan)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Destination cluster
	nSet, err = r.validateDestinationCluster(plan)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Storage
	nSet, err = r.validateStorage(plan)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// AssetCollection
	nSet, err = r.validateAssetCollection(plan)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Apply changes.
	plan.Status.DeleteUnstagedConditions()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

// Validate the referenced storage.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateStorage(plan *migapi.MigPlan) (int, error) {
	ref := plan.Spec.MigStorageRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidStorageRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidStorageRefMessage,
		})
		return 1, nil
	}

	storage, err := migapi.GetStorage(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if storage == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidStorageRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidStorageRefMessage,
		})
		return 1, nil
	}

	// NotReady
	if !storage.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:    StorageNotReady,
			Status:  True,
			Message: StorageNotReadyMessage,
		})
		return 1, nil
	}

	return 0, nil
}

// Validate the referenced assetCollection.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateAssetCollection(plan *migapi.MigPlan) (int, error) {
	ref := plan.Spec.MigAssetCollectionRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidAssetCollectionRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidAssetCollectionRefMessage,
		})
		return 1, nil
	}

	assetCollection, err := migapi.GetAssetCollection(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if assetCollection == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidAssetCollectionRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidAssetCollectionRefMessage,
		})
		return 1, nil
	}

	// NotReady
	if !assetCollection.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:    AssetCollectionNotReady,
			Status:  True,
			Message: AssetCollectionNotReadyMessage,
		})
		return 1, nil
	}

	// Namespaces
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return 0, err
	}
	if cluster == nil {
		return 0, nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return 0, err
	}
	notFound := make([]string, 0)
	ns := kapi.Namespace{}
	for _, name := range assetCollection.Spec.Namespaces {
		key := types.NamespacedName{Name: name}
		err := client.Get(context.TODO(), key, &ns)
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			notFound = append(notFound, name)
		} else {
			return 0, err
		}
	}
	if len(notFound) > 0 {
		message := fmt.Sprintf(AssetNamespaceNotFoundMessage, strings.Join(notFound, ", "))
		plan.Status.SetCondition(migapi.Condition{
			Type:    AssetNamespacesNotFound,
			Status:  True,
			Reason:  NotFound,
			Message: message,
		})
		return 1, nil
	}

	return 0, nil
}

// Validate the referenced source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateSourceCluster(plan *migapi.MigPlan) (int, error) {
	ref := plan.Spec.SrcMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidSourceClusterRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidSourceClusterRefMessage,
		})
		return 1, nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidSourceClusterRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidSourceClusterRefMessage,
		})
		return 1, nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:    SourceClusterNotReady,
			Status:  True,
			Message: SourceClusterNotReadyMessage,
		})
		return 1, nil
	}

	return 0, nil
}

// Validate the referenced source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateDestinationCluster(plan *migapi.MigPlan) (int, error) {
	ref := plan.Spec.DestMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidDestinationClusterRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidDestinationClusterRefMessage,
		})
		return 1, nil
	}

	// NotDistinct
	if reflect.DeepEqual(ref, plan.Spec.SrcMigClusterRef) {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidDestinationCluster,
			Status:  True,
			Reason:  NotDistinct,
			Message: InvalidDestinationClusterMessage,
		})
		return 1, nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    InvalidDestinationClusterRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidDestinationClusterRefMessage,
		})
		return 1, nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:    DestinationClusterNotReady,
			Status:  True,
			Message: DestinationClusterNotReadyMessage,
		})
		return 1, nil
	}

	return 0, nil
}
