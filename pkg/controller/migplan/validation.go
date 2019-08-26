package migplan

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
)

// Types
const (
	Suspended                      = "Suspended"
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
	PlanConflict                   = "PlanConflict"
	PvInvalidAction                = "PvInvalidAction"
	PvNoSupportedAction            = "PvNoSupportedAction"
	PvInvalidStorageClass          = "PvInvalidStorageClass"
	PvInvalidAccessMode            = "PvInvalidAccessMode"
	PvNoStorageClassSelection      = "PvNoStorageClassSelection"
	PvWarnNoCephAvailable          = "PvWarnNoCephAvailable"
	PvWarnAccessModeUnavailable    = "PvWarnAccessModeUnavailable"
	PvInvalidCopyMethod            = "PvInvalidCopyMethod"
	PvNoCopyMethodSelection        = "PvNoCopyMethodSelection"
	PvWarnCopyMethodSnapshot       = "PvWarnCopyMethodSnapshot"
	StorageEnsured                 = "StorageEnsured"
	RegistriesEnsured              = "RegistriesEnsured"
	PvsDiscovered                  = "PvsDiscovered"
	Closed                         = "Closed"
)

// Categories
const (
	Advisory = migapi.Advisory
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
	Conflict    = "Conflict"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage                          = "The migration plan is ready."
	SuspendedMessage                      = "Limited validation; PV discovery and resource reconciliation suspended."
	InvalidSourceClusterRefMessage        = "The `srcMigClusterRef` must reference a `migcluster`."
	InvalidDestinationClusterRefMessage   = "The `dstMigClusterRef` must reference a `migcluster`."
	InvalidStorageRefMessage              = "The `migStorageRef` must reference a `migstorage`."
	SourceClusterNotReadyMessage          = "The referenced `srcMigClusterRef` does not have a `Ready` condition."
	DestinationClusterNotReadyMessage     = "The referenced `dstMigClusterRef` does not have a `Ready` condition."
	StorageNotReadyMessage                = "The referenced `migStorageRef` does not have a `Ready` condition."
	NsListEmptyMessage                    = "The `namespaces` list may not be empty."
	InvalidDestinationClusterMessage      = "The `srcMigClusterRef` and `dstMigClusterRef` cannot be the same."
	NsNotFoundOnSourceClusterMessage      = "Namespaces [] not found on the source cluster."
	NsNotFoundOnDestinationClusterMessage = "Namespaces [] not found on the destination cluster."
	PlanConflictMessage                   = "The plan is in conflict with []."
	PvInvalidActionMessage                = "PV in `persistentVolumes` [] has an unsupported `action`."
	PvNoSupportedActionMessage            = "PV in `persistentVolumes` [] with no `SupportedActions`."
	PvInvalidStorageClassMessage          = "PV in `persistentVolumes` [] has an unsupported `storageClass`."
	PvInvalidAccessModeMessage            = "PV in `persistentVolumes` [] has an invalid `accessMode`."
	PvNoStorageClassSelectionMessage      = "PV in `persistentVolumes` [] has no `Selected.StorageClass`."
	PvWarnNoCephAvailableMessage          = "Ceph is not available on destination. If this is desired, please install the rook operator. The following PVs will use the default storage class instead: []"
	PvWarnAccessModeUnavailableMessage    = "AccessMode for PVC in `persistentVolumes` [] unavailable in chosen storage class"
	PvInvalidCopyMethodMessage            = "PV in `persistentVolumes` [] has an invalid `copyMethod`."
	PvNoCopyMethodSelectionMessage        = "PV in `persistentVolumes` [] has no `Selected.CopyMethod`."
	PvWarnCopyMethodSnapshotMessage       = "CopyMethod for PV in `persistentVolumes` [] is set to `snapshot`. Make sure that the chosen storage class is compatible with the source volume's storage type for Snapshot support."
	StorageEnsuredMessage                 = "The storage resources have been created."
	RegistriesEnsuredMessage              = "The migration registry resources have been created."
	PvsDiscoveredMessage                  = "The `persistentVolumes` list has been updated with discovered PVs."
	ClosedMessage                         = "The migration plan is closed."
)

// Valid AccessMode values
var validAccessModes = []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany}

// Validate the plan resource.
func (r ReconcileMigPlan) validate(plan *migapi.MigPlan) error {
	// Source cluster
	err := r.validateSourceCluster(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Destination cluster
	err = r.validateDestinationCluster(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Storage
	err = r.validateStorage(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Migrated namespaces.
	err = r.validateNamespaces(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Required namespaces.
	err = r.validateRequiredNamespaces(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Conflict
	err = r.validateConflict(plan)
	if err != nil {
		log.Trace(err)
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
		log.Trace(err)
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
		log.Trace(err)
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
		log.Trace(err)
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
	if plan.Status.HasAnyCondition(Suspended) {
		return nil
	}
	err := r.validateSourceNamespaces(plan)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = r.validateDestinationNamespaces(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Validate required namespaces on the source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateSourceNamespaces(plan *migapi.MigPlan) error {
	namespaces := []string{migapi.VeleroNamespace}
	if plan.Status.HasAnyCondition(Suspended) {
		return nil
	}
	for _, ns := range plan.Spec.Namespaces {
		namespaces = append(namespaces, ns)
	}
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		log.Trace(err)
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
			log.Trace(err)
			return err
		}
	}
	if len(notFound) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnSourceCluster,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  NsNotFoundOnSourceClusterMessage,
			Items:    notFound,
		})
		return nil
	}

	return nil
}

// Validate required namespaces on the destination cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateDestinationNamespaces(plan *migapi.MigPlan) error {
	namespaces := []string{migapi.VeleroNamespace}
	if plan.Status.HasAnyCondition(Suspended) {
		return nil
	}
	cluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		log.Trace(err)
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
			log.Trace(err)
			return err
		}
	}
	if len(notFound) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnDestinationCluster,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  NsNotFoundOnDestinationClusterMessage,
			Items:    notFound,
		})
		return nil
	}

	return nil
}

// Validate the plan does not conflict with another plan.
func (r ReconcileMigPlan) validateConflict(plan *migapi.MigPlan) error {
	plans, err := migapi.ListPlans(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	list := []string{}
	for _, p := range plans {
		if plan.UID == p.UID {
			continue
		}
		if plan.HasConflict(&p) {
			list = append(list, p.Name)
		}
	}
	if len(list) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PlanConflict,
			Status:   True,
			Reason:   Conflict,
			Category: Error,
			Message:  PlanConflictMessage,
			Items:    list,
		})
	}

	return nil
}

// Validate PV actions.
func (r ReconcileMigPlan) validatePvSelections(plan *migapi.MigPlan) error {
	invalidAction := make([]string, 0)
	unsupported := make([]string, 0)

	storageClasses := map[string][]kapi.PersistentVolumeAccessMode{}
	missingStorageClass := make([]string, 0)
	invalidStorageClass := make([]string, 0)
	invalidAccessMode := make([]string, 0)
	unavailableAccessMode := make([]string, 0)
	missingCopyMethod := make([]string, 0)
	invalidCopyMethod := make([]string, 0)
	warnCopyMethodSnapshot := make([]string, 0)

	if plan.Status.HasAnyCondition(Suspended) {
		return nil
	}
	destMigCluster, err := plan.GetDestinationCluster(r.Client)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, storageClass := range destMigCluster.Spec.StorageClasses {
		storageClasses[storageClass.Name] = storageClass.AccessModes
	}

	for _, pv := range plan.Spec.PersistentVolumes.List {
		actions := map[string]bool{}
		if len(pv.Supported.Actions) > 0 {
			for _, a := range pv.Supported.Actions {
				actions[a] = true
			}
		} else {
			unsupported = append(unsupported, pv.Name)
			actions[""] = true
		}
		_, found := actions[pv.Selection.Action]
		if !found {
			invalidAction = append(invalidAction, pv.Name)
			continue
		}
		// Don't report StorageClass, AccessMode, CopyMethod errors if Action != 'copy'
		if pv.Selection.Action != migapi.PvCopyAction {
			continue
		}
		if pv.Selection.StorageClass == "" {
			missingStorageClass = append(missingStorageClass, pv.Name)
		} else {
			storageClassAccessModes, found := storageClasses[pv.Selection.StorageClass]
			if found {
				if pv.Selection.AccessMode != "" {
					if !containsAccessMode(validAccessModes, pv.Selection.AccessMode) {
						invalidAccessMode = append(invalidAccessMode, pv.Name)
					} else if !containsAccessMode(storageClassAccessModes, pv.Selection.AccessMode) {
						unavailableAccessMode = append(unavailableAccessMode, pv.Name)
					}
				} else {
					foundMode := false
					for _, accessMode := range pv.PVC.AccessModes {
						if containsAccessMode(storageClassAccessModes, accessMode) {
							foundMode = true
						}
					}
					if !foundMode {
						unavailableAccessMode = append(unavailableAccessMode, pv.Name)
					}
				}
			} else {
				invalidStorageClass = append(invalidStorageClass, pv.Name)
			}
		}
		if pv.Selection.CopyMethod == "" {
			missingCopyMethod = append(missingCopyMethod, pv.Name)
		} else {
			copyMethods := map[string]bool{}
			for _, m := range pv.Supported.CopyMethods {
				copyMethods[m] = true
			}
			_, found := copyMethods[pv.Selection.CopyMethod]
			if !found {
				invalidCopyMethod = append(invalidCopyMethod, pv.Name)
			} else if pv.Selection.CopyMethod == migapi.PvSnapshotCopyMethod {
				// Warn if Snapshot is selected
				warnCopyMethodSnapshot = append(warnCopyMethodSnapshot, pv.Name)
			}
		}

	}
	if len(invalidAction) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidAction,
			Status:   True,
			Reason:   NotDone,
			Category: Error,
			Message:  PvInvalidActionMessage,
			Items:    invalidAction,
		})
	}
	if len(unsupported) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoSupportedAction,
			Status:   True,
			Category: Warn,
			Message:  PvNoSupportedActionMessage,
			Items:    unsupported,
		})
	}
	if len(invalidStorageClass) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidStorageClass,
			Status:   True,
			Reason:   NotDone,
			Category: Error,
			Message:  PvInvalidStorageClassMessage,
			Items:    invalidStorageClass,
		})
	}
	if len(invalidAccessMode) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidAccessMode,
			Status:   True,
			Category: Error,
			Message:  PvInvalidAccessModeMessage,
			Items:    invalidAccessMode,
		})
	}
	if len(missingStorageClass) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoStorageClassSelection,
			Status:   True,
			Category: Error,
			Message:  PvNoStorageClassSelectionMessage,
			Items:    missingStorageClass,
		})
	}
	if len(unavailableAccessMode) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvWarnAccessModeUnavailable,
			Status:   True,
			Category: Warn,
			Message:  PvWarnAccessModeUnavailableMessage,
			Items:    unavailableAccessMode,
		})
	}
	if len(missingCopyMethod) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoCopyMethodSelection,
			Status:   True,
			Category: Error,
			Message:  PvNoCopyMethodSelectionMessage,
			Items:    missingCopyMethod,
		})
	}
	if len(invalidCopyMethod) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidCopyMethod,
			Status:   True,
			Category: Error,
			Message:  PvInvalidCopyMethodMessage,
			Items:    invalidCopyMethod,
		})
	}
	if len(warnCopyMethodSnapshot) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvWarnCopyMethodSnapshot,
			Status:   True,
			Category: Warn,
			Message:  PvWarnCopyMethodSnapshotMessage,
			Items:    warnCopyMethodSnapshot,
		})
	}

	return nil
}

func containsAccessMode(modeList []kapi.PersistentVolumeAccessMode, accessMode kapi.PersistentVolumeAccessMode) bool {
	for _, mode := range modeList {
		if mode == accessMode {
			return true
		}
	}
	return false
}
