package migplan

import (
	"context"
	"fmt"
	"github.com/fusor/mig-controller/pkg/velerorunner"
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
	InvalidSourceClusterRef        = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef   = "InvalidDestinationClusterRef"
	InvalidStorageRef              = "InvalidStorageRef"
	InvalidAssetCollectionRef      = "InvalidAssetCollectionRef"
	SourceClusterNotReady          = "SourceClusterNotReady"
	DestinationClusterNotReady     = "DestinationClusterNotReady"
	StorageNotReady                = "StorageNotReady"
	AssetCollectionNotReady        = "AssetCollectionNotReady"
	AssetNamespaceNotFound         = "AssetNamespaceNotFound"
	InvalidDestinationCluster      = "InvalidDestinationCluster"
	NsNotFoundOnSourceCluster      = "NamespaceNotFoundOnSourceCluster"
	NsNotFoundOnDestinationCluster = "NamespaceNotFoundOnDestinationCluster"
	StorageEnsured                 = "StorageEnsured"
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
	ReadyMessage                          = "The migration plan is ready."
	InvalidSourceClusterRefMessage        = "The `srcMigClusterRef` must reference a `migcluster`."
	InvalidDestinationClusterRefMessage   = "The `dstMigClusterRef` must reference a `migcluster`."
	InvalidStorageRefMessage              = "The `migStorageRef` must reference a `migstorage`."
	InvalidAssetCollectionRefMessage      = "The `migAssetCollectionRef` must reference a `migassetcollection`."
	SourceClusterNotReadyMessage          = "The referenced `srcMigClusterRef` does not have a `Ready` condition."
	DestinationClusterNotReadyMessage     = "The referenced `dstMigClusterRef` does not have a `Ready` condition."
	StorageNotReadyMessage                = "The referenced `migStorageRef` does not have a `Ready` condition."
	AssetCollectionNotReadyMessage        = "The referenced `migAssetCollectionRef` does not have a `Ready` condition."
	AssetNamespaceNotFoundMessage         = "The following asset `namespaces` [%s] not found on the source cluster."
	InvalidDestinationClusterMessage      = "The `srcMigClusterRef` and `dstMigClusterRef` cannot be the same."
	NsNotFoundOnSourceClusterMessage      = "Namespaces [%s] not found on the source cluster."
	NsNotFoundOnDestinationClusterMessage = "Namespaces [%s] not found on the destination cluster."
	StorageEnsuredMessage                 = "The storage resources have been created."
)

// Validate the plan resource.
func (r ReconcileMigPlan) validate(plan *migapi.MigPlan) error {
	plan.Status.BeginStagingConditions()

	// Source cluster
	err := r.validateSourceCluster(plan)
	if err != nil {
		return err
	}

	// Destination cluster
	err = r.validateDestinationCluster(plan)
	if err != nil {
		return err
	}

	// Storage
	err = r.validateStorage(plan)
	if err != nil {
		return err
	}

	// AssetCollection
	err = r.validateAssetCollection(plan)
	if err != nil {
		return err
	}

	// Required namespaces.
	err = r.validateRequiredNamespaces(plan)
	if err != nil {
		return err
	}

	// Apply changes.
	plan.Status.EndStagingConditions()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		return err
	}

	return nil
}

// Validate the referenced storage.
func (r ReconcileMigPlan) validateStorage(plan *migapi.MigPlan) error {
	ref := plan.Spec.MigStorageRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidStorageRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
			Message:  InvalidStorageRefMessage,
		})
		return nil
	}

	storage, err := migapi.GetStorage(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if storage == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidStorageRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidStorageRefMessage,
		})
		return nil
	}

	// NotReady
	if !storage.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     StorageNotReady,
			Status:   True,
			Category: migapi.Error,
			Message:  StorageNotReadyMessage,
		})
		return nil
	}

	return nil
}

// Validate the referenced assetCollection.
func (r ReconcileMigPlan) validateAssetCollection(plan *migapi.MigPlan) error {
	ref := plan.Spec.MigAssetCollectionRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidAssetCollectionRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
			Message:  InvalidAssetCollectionRefMessage,
		})
		return nil
	}

	assetCollection, err := migapi.GetAssetCollection(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if assetCollection == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidAssetCollectionRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidAssetCollectionRefMessage,
		})
		return nil
	}

	// NotReady
	if !assetCollection.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     AssetCollectionNotReady,
			Status:   True,
			Category: migapi.Error,
			Message:  AssetCollectionNotReadyMessage,
		})
		return nil
	}

	// Namespaces
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return err
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return err
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
			return err
		}
	}
	if len(notFound) > 0 {
		message := fmt.Sprintf(AssetNamespaceNotFoundMessage, strings.Join(notFound, ", "))
		plan.Status.SetCondition(migapi.Condition{
			Type:     AssetNamespaceNotFound,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  message,
		})
		return nil
	}

	return nil
}

// Validate the referenced source cluster.
func (r ReconcileMigPlan) validateSourceCluster(plan *migapi.MigPlan) error {
	ref := plan.Spec.SrcMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
			Message:  InvalidSourceClusterRefMessage,
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidSourceClusterRefMessage,
		})
		return nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNotReady,
			Status:   True,
			Category: migapi.Error,
			Message:  SourceClusterNotReadyMessage,
		})
		return nil
	}

	return nil
}

// Validate the referenced source cluster.
func (r ReconcileMigPlan) validateDestinationCluster(plan *migapi.MigPlan) error {
	ref := plan.Spec.DestMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
			Message:  InvalidDestinationClusterRefMessage,
		})
		return nil
	}

	// NotDistinct
	if reflect.DeepEqual(ref, plan.Spec.SrcMigClusterRef) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationCluster,
			Status:   True,
			Reason:   NotDistinct,
			Category: migapi.Error,
			Message:  InvalidDestinationClusterMessage,
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidDestinationClusterRefMessage,
		})
		return nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNotReady,
			Status:   True,
			Category: migapi.Error,
			Message:  DestinationClusterNotReadyMessage,
		})
		return nil
	}

	return nil
}

// Validate required namespaces.
func (r ReconcileMigPlan) validateRequiredNamespaces(plan *migapi.MigPlan) error {
	// Source
	err := r.validateSourceNamespaces(plan)
	if err != nil {
		return err
	}

	// Destination
	err = r.validateDestinationNamespaces(plan)
	if err != nil {
		return err
	}

	return nil
}

// Validate required namespaces on the source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateSourceNamespaces(plan *migapi.MigPlan) error {
	namespaces := []string{velerorunner.VeleroNamespace}
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return err
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return err
	}
	ns := kapi.Namespace{}
	notFound := make([]string, 0)
	for _, name := range namespaces {
		key := types.NamespacedName{Name: name}
		err := client.Get(context.TODO(), key, &ns)
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			notFound = append(notFound, name)
		} else {
			return err
		}
	}
	if len(notFound) > 0 {
		message := fmt.Sprintf(NsNotFoundOnSourceClusterMessage, strings.Join(notFound, ", "))
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnSourceCluster,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  message,
		})
		return nil
	}

	return nil
}

// Validate required namespaces on the destination cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateDestinationNamespaces(plan *migapi.MigPlan) error {
	namespaces := []string{velerorunner.VeleroNamespace}
	cluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		return err
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return err
	}
	ns := kapi.Namespace{}
	notFound := make([]string, 0)
	for _, name := range namespaces {
		key := types.NamespacedName{Name: name}
		err := client.Get(context.TODO(), key, &ns)
		if err == nil {
			continue
		}
		if errors.IsNotFound(err) {
			notFound = append(notFound, name)
		} else {
			return err
		}
	}
	if len(notFound) > 0 {
		message := fmt.Sprintf(NsNotFoundOnDestinationClusterMessage, strings.Join(notFound, ", "))
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnDestinationCluster,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  message,
		})
		return nil
	}

	return nil
}
