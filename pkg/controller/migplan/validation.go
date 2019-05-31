package migplan

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
)

// Types
const (
	InvalidSourceClusterRef        = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef   = "InvalidDestinationClusterRef"
	InvalidStorageRef              = "InvalidStorageRef"
	SourceClusterNotReady          = "SourceClusterNotReady"
	DestinationClusterNotReady     = "DestinationClusterNotReady"
	StorageNotReady                = "StorageNotReady"
	NsListEmpty                    = "NamespaceListEmpty"
	InvalidDestinationCluster      = "InvalidDestinationCluster"
	NsNotFoundOnSourceCluster      = "NamespaceNotFoundOnSourceCluster"
	NsNotFoundOnDestinationCluster = "NamespaceNotFoundOnDestinationCluster"
	PvInvalidAction                = "PvInvalidAction"
	PvNoSupportedAction            = "PvNoSupportedAction"
	StorageEnsured                 = "StorageEnsured"
	RegistriesEnsured              = "RegistriesEnsured"
	PvsDiscovered                  = "PvsDiscovered"
)

// Categories
const (
	Critical = migapi.Critical
	Error    = migapi.Error
	Warn     = migapi.Warn
)

// Reasons
const (
	NotSet      = "NotSet"
	NotFound    = "NotFound"
	NotDistinct = "NotDistinct"
	NotDone     = "NotDone"
	Done        = "Done"
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
	SourceClusterNotReadyMessage          = "The referenced `srcMigClusterRef` does not have a `Ready` condition."
	DestinationClusterNotReadyMessage     = "The referenced `dstMigClusterRef` does not have a `Ready` condition."
	StorageNotReadyMessage                = "The referenced `migStorageRef` does not have a `Ready` condition."
	NsListEmptyMessage                    = "The `namespaces` list may not be empty."
	InvalidDestinationClusterMessage      = "The `srcMigClusterRef` and `dstMigClusterRef` cannot be the same."
	NsNotFoundOnSourceClusterMessage      = "Namespaces [%s] not found on the source cluster."
	NsNotFoundOnDestinationClusterMessage = "Namespaces [%s] not found on the destination cluster."
	PvInvalidActionMessage                = "PV in `persistentVolumes` [%s] has an unsupported `action`."
	PvNoSupportedActionMessage            = "PV in `persistentVolumes` [%s] with no `SupportedActions`."
	StorageEnsuredMessage                 = "The storage resources have been created."
	RegistriesEnsuredMessage              = "The migration registry resources have been created."
	PvsDiscoveredMessage                  = "The `persistentVolumes` list has been updated with discovered PVs."
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

	// Migrated namespaces.
	err = r.validateNamespaces(plan)
	if err != nil {
		return err
	}

	// Required namespaces.
	err = r.validateRequiredNamespaces(plan)
	if err != nil {
		return err
	}

	// PV list.
	err = r.validatePvAction(plan)
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
			Category: Critical,
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
			Category: Critical,
			Message:  InvalidStorageRefMessage,
		})
		return nil
	}

	// NotReady
	if !storage.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     StorageNotReady,
			Status:   True,
			Category: Critical,
			Message:  StorageNotReadyMessage,
		})
		return nil
	}

	return nil
}

// Validate the referenced assetCollection.
func (r ReconcileMigPlan) validateNamespaces(plan *migapi.MigPlan) error {
	if len(plan.Spec.Namespaces) == 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsListEmpty,
			Status:   True,
			Category: Critical,
			Message:  NsListEmptyMessage,
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
			Category: Critical,
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
			Category: Critical,
			Message:  InvalidSourceClusterRefMessage,
		})
		return nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNotReady,
			Status:   True,
			Category: Critical,
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
			Category: Critical,
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
			Category: Critical,
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
			Category: Critical,
			Message:  InvalidDestinationClusterRefMessage,
		})
		return nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNotReady,
			Status:   True,
			Category: Critical,
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
	namespaces := []string{migapi.VeleroNamespace}
	for _, ns := range plan.Spec.Namespaces {
		namespaces = append(namespaces, ns)
	}
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
			Category: Critical,
			Message:  message,
		})
		return nil
	}

	return nil
}

// Validate required namespaces on the destination cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateDestinationNamespaces(plan *migapi.MigPlan) error {
	namespaces := []string{migapi.VeleroNamespace}
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
			Category: Critical,
			Message:  message,
		})
		return nil
	}

	return nil
}

// Validate PV actions.
func (r ReconcileMigPlan) validatePvAction(plan *migapi.MigPlan) error {
	invalid := make([]string, 0)
	unsupported := make([]string, 0)
	for _, pv := range plan.Spec.PersistentVolumes.List {
		actions := map[string]bool{}
		if len(pv.SupportedActions) > 0 {
			for _, a := range pv.SupportedActions {
				actions[a] = true
			}
		} else {
			unsupported = append(unsupported, pv.Name)
			actions[""] = true
		}
		_, found := actions[pv.Action]
		if !found {
			invalid = append(invalid, pv.Name)
		}
	}
	if len(invalid) > 0 {
		message := fmt.Sprintf(
			PvInvalidActionMessage,
			strings.Join(invalid, ", "))
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidAction,
			Status:   True,
			Reason:   NotDone,
			Category: Error,
			Message:  message,
		})
	}
	if len(unsupported) > 0 {
		message := fmt.Sprintf(
			PvNoSupportedActionMessage,
			strings.Join(unsupported, ", "))
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoSupportedAction,
			Status:   True,
			Category: Warn,
			Message:  message,
		})
		return nil
	}

	return nil
}

// The collection contains a PV discovery blocker condition.
func (r ReconcileMigPlan) hasPvDiscoveryBlocker(plan *migapi.MigPlan) bool {
	return plan.Status.HasCondition(
		InvalidSourceClusterRef,
		SourceClusterNotReady,
		NsListEmpty,
		NsNotFoundOnSourceCluster,
	)
}
