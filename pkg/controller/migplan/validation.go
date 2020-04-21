package migplan

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/health"
	"github.com/konveyor/mig-controller/pkg/pods"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/konveyor/mig-controller/pkg/settings"
	kapi "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/exec"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	Suspended                                  = "Suspended"
	InvalidSourceClusterRef                    = "InvalidSourceClusterRef"
	InvalidDestinationClusterRef               = "InvalidDestinationClusterRef"
	InvalidStorageRef                          = "InvalidStorageRef"
	SourceClusterNotReady                      = "SourceClusterNotReady"
	DestinationClusterNotReady                 = "DestinationClusterNotReady"
	StorageNotReady                            = "StorageNotReady"
	NsListEmpty                                = "NamespaceListEmpty"
	InvalidDestinationCluster                  = "InvalidDestinationCluster"
	NsNotFoundOnSourceCluster                  = "NamespaceNotFoundOnSourceCluster"
	NsNotFoundOnDestinationCluster             = "NamespaceNotFoundOnDestinationCluster"
	NsLimitExceeded                            = "NamespaceLimitExceeded"
	PodLimitExceeded                           = "PodLimitExceeded"
	SourceClusterProxySecretMisconfigured      = "SourceClusterProxySecretMisconfigured"
	DestinationClusterProxySecretMisconfigured = "DestinationClusterProxySecretMisconfigured"
	PlanConflict                               = "PlanConflict"
	PvInvalidAction                            = "PvInvalidAction"
	PvNoSupportedAction                        = "PvNoSupportedAction"
	PvInvalidStorageClass                      = "PvInvalidStorageClass"
	PvInvalidAccessMode                        = "PvInvalidAccessMode"
	PvNoStorageClassSelection                  = "PvNoStorageClassSelection"
	PvWarnNoCephAvailable                      = "PvWarnNoCephAvailable"
	PvWarnAccessModeUnavailable                = "PvWarnAccessModeUnavailable"
	PvInvalidCopyMethod                        = "PvInvalidCopyMethod"
	PvNoCopyMethodSelection                    = "PvNoCopyMethodSelection"
	PvWarnCopyMethodSnapshot                   = "PvWarnCopyMethodSnapshot"
	NfsNotAccessible                           = "NfsNotAccessible"
	NfsAccessCannotBeValidated                 = "NfsAccessCannotBeValidated"
	PvLimitExceeded                            = "PvLimitExceeded"
	StorageEnsured                             = "StorageEnsured"
	RegistriesEnsured                          = "RegistriesEnsured"
	PvsDiscovered                              = "PvsDiscovered"
	Closed                                     = "Closed"
	SourcePodsNotHealthy                       = "SourcePodsNotHealthy"
	GVKsIncompatible                           = "GVKsIncompatible"
	InvalidHookRef                             = "InvalidHookRef"
	HookNotReady                               = "HookNotReady"
	InvalidHookNSName                          = "InvalidHookNSName"
	InvalidHookSAName                          = "InvalidHookSAName"
	TemplatesInvalid                           = "TemplatesInvalid"
	HookPhaseUnknown                           = "HookPhaseUnknown"
	HookPhaseDuplicate                         = "HookPhaseDuplicate"
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
	NotSet        = "NotSet"
	NotFound      = "NotFound"
	KeyNotFound   = "KeyNotFound"
	NotDistinct   = "NotDistinct"
	LimitExceeded = "LimitExceeded"
	NotDone       = "NotDone"
	Done          = "Done"
	Conflict      = "Conflict"
	NotHealthy    = "NotHealthy"
	Incorrect     = "Incorrect"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage                                      = "The migration plan is ready."
	SuspendedMessage                                  = "Limited validation; PV discovery and resource reconciliation suspended."
	InvalidSourceClusterRefMessage                    = "The `srcMigClusterRef` must reference a `migcluster`."
	InvalidDestinationClusterRefMessage               = "The `dstMigClusterRef` must reference a `migcluster`."
	InvalidStorageRefMessage                          = "The `migStorageRef` must reference a `migstorage`."
	SourceClusterNotReadyMessage                      = "The referenced `srcMigClusterRef` does not have a `Ready` condition."
	DestinationClusterNotReadyMessage                 = "The referenced `dstMigClusterRef` does not have a `Ready` condition."
	StorageNotReadyMessage                            = "The referenced `migStorageRef` does not have a `Ready` condition."
	NsListEmptyMessage                                = "The `namespaces` list may not be empty."
	InvalidDestinationClusterMessage                  = "The `srcMigClusterRef` and `dstMigClusterRef` cannot be the same."
	NsGVKsIncompatible                                = "Some namespaces contain GVKs incompatible with destination cluster. See: `incompatibleNamespaces` for details"
	NsNotFoundOnSourceClusterMessage                  = "Namespaces [] not found on the source cluster."
	NsNotFoundOnDestinationClusterMessage             = "Namespaces [] not found on the destination cluster."
	NsLimitExceededMessage                            = "Namespace limit: %d exceeded, found:%d."
	PodLimitExceededMessage                           = "Pod limit: %d exceeded, found: %d."
	SourceClusterProxySecretMisconfiguredMessage      = "Source cluster proxy secret is misconfigured"
	DestinationClusterProxySecretMisconfiguredMessage = "Destination cluster proxy secret is misconfigured"
	PlanConflictMessage                               = "The plan is in conflict with []."
	PvInvalidActionMessage                            = "PV in `persistentVolumes` [] has an unsupported `action`."
	PvNoSupportedActionMessage                        = "PV in `persistentVolumes` [] with no `SupportedActions`."
	PvInvalidStorageClassMessage                      = "PV in `persistentVolumes` [] has an unsupported `storageClass`."
	PvInvalidAccessModeMessage                        = "PV in `persistentVolumes` [] has an invalid `accessMode`."
	PvNoStorageClassSelectionMessage                  = "PV in `persistentVolumes` [] has no `Selected.StorageClass`. Make sure that the necessary static persistent volumes exist in the destination cluster."
	PvWarnNoCephAvailableMessage                      = "Ceph is not available on destination. If this is desired, please install the rook operator. The following PVs will use the default storage class instead: []"
	PvWarnAccessModeUnavailableMessage                = "AccessMode for PVC in `persistentVolumes` [] unavailable in chosen storage class"
	PvInvalidCopyMethodMessage                        = "PV in `persistentVolumes` [] has an invalid `copyMethod`."
	PvNoCopyMethodSelectionMessage                    = "PV in `persistentVolumes` [] has no `Selected.CopyMethod`."
	PvWarnCopyMethodSnapshotMessage                   = "CopyMethod for PV in `persistentVolumes` [] is set to `snapshot`. Make sure that the chosen storage class is compatible with the source volume's storage type for Snapshot support."
	PvLimitExceededMessage                            = "PV limit: %d exceeded, found: %d."
	NfsNotAccessibleMessage                           = "NFS servers [] not accessible on the destination cluster."
	NfsAccessCannotBeValidatedMessage                 = "NFS access cannot be validated on the destination cluster."
	StorageEnsuredMessage                             = "The storage resources have been created."
	RegistriesEnsuredMessage                          = "The migration registry resources have been created."
	PvsDiscoveredMessage                              = "The `persistentVolumes` list has been updated with discovered PVs."
	ClosedMessage                                     = "The migration plan is closed."
	SourcePodsNotHealthyMessage                       = "Source namespace(s) contain unhealthy pods. See: `unhealthyNamespaces` for details."
	InvalidHookRefMessage                             = "One or more referenced hooks do not exist."
	HookNotReadyMessage                               = "One or more referenced hooks are not ready."
	InvalidHookNSNameMessage                          = "The executionNamespace specified is invalid, DNS-1123 label regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?'."
	InvalidHookSANameMessage                          = "The serviceAccount specified is invalid, DNS-1123 subdomain regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'"
	HookPhaseUnknownMessage                           = "The hook phase must be one of: PreRestore, PostRestore, PreBackup, PostBackup"
	HookPhaseDuplicateMessage                         = "Only one hook may be specified per phase"
	TemplatesInvalidMessage                           = "Some of the provided templates are invalid: []."
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

	// Pod limit within each namespace.
	err = r.validatePodLimit(plan)
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

	// Required templates
	err = r.validateAdditionalTemplates(plan)
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

	// Registry proxy secret
	err = r.validateRegistryProxySecrets(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Pods
	err = r.validatePods(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Hooks
	err = r.validateHooks(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	// GVK
	err = r.compareGVK(plan)
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
	count := len(plan.Spec.Namespaces)
	if count == 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsListEmpty,
			Status:   True,
			Category: Critical,
			Message:  NsListEmptyMessage,
		})
		return nil
	}
	limit := Settings.Plan.NsLimit
	if count > limit {
		message := fmt.Sprintf(NsLimitExceededMessage, limit, count)
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsLimitExceeded,
			Status:   True,
			Reason:   LimitExceeded,
			Category: Critical,
			Message:  message,
		})
		return nil
	}

	return nil
}

// Validate the total number of running pods (limit) across namespaces.
func (r ReconcileMigPlan) validatePodLimit(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended, NsLimitExceeded) {
		plan.Status.StageCondition(PodLimitExceeded)
		return nil
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
	count := 0
	limit := Settings.Plan.PodLimit
	for _, name := range plan.GetSourceNamespaces() {
		list := kapi.PodList{}
		options := k8sclient.ListOptions{
			FieldSelector: fields.SelectorFromSet(
				fields.Set{
					"status.phase": string(kapi.PodRunning),
				}),
			Namespace: name,
		}
		err := client.List(context.TODO(), &options, &list)
		if err != nil {
			log.Trace(err)
			return err
		}
		count += len(list.Items)
	}
	if count > limit {
		message := fmt.Sprintf(PodLimitExceededMessage, limit, count)
		plan.Status.SetCondition(migapi.Condition{
			Type:     PodLimitExceeded,
			Status:   True,
			Reason:   LimitExceeded,
			Category: Warn,
			Message:  message,
		})
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

func (r ReconcileMigPlan) validateAdditionalTemplates(plan *migapi.MigPlan) error {
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	_, err = plan.ListTemplates(client)
	if err != nil {
		templates := []string{}
		for _, template := range plan.Spec.TemplateResources.Templates {
			templates = append(templates, template.Resource)
		}
		plan.Status.SetCondition(migapi.Condition{
			Type:     TemplatesInvalid,
			Status:   True,
			Reason:   Incorrect,
			Category: Critical,
			Message:  TemplatesInvalidMessage,
			Items:    templates,
		})
	}

	return nil
}

// Validate required namespaces on the source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateSourceNamespaces(plan *migapi.MigPlan) error {
	namespaces := []string{migapi.VeleroNamespace}
	if plan.Status.HasAnyCondition(Suspended, NsLimitExceeded) {
		plan.Status.StageCondition(NsNotFoundOnSourceCluster)
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
		namespaceMapping := strings.Split(name, ":")
		name = namespaceMapping[0]
		key := types.NamespacedName{Name: name}
		err := client.Get(context.TODO(), key, &ns)
		if err == nil {
			continue
		}
		if k8serror.IsNotFound(err) {
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
		if k8serror.IsNotFound(err) {
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
	if destMigCluster == nil {
		return nil
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
			Category: Warn,
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

// Validate proxy secrets. Should only exist 1 or none
func (r ReconcileMigPlan) validateRegistryProxySecrets(plan *migapi.MigPlan) error {
	err := r.validateSourceRegistryProxySecret(plan)
	if err != nil {
		return err
	}
	return r.validateDestinationRegistryProxySecret(plan)
}

// Validate registry proxy secret ensuring it has the proper keys
func (r *ReconcileMigPlan) validateRegistryProxySecret(secret *kapi.Secret) bool {
	fields := []string{
		settings.HttpProxy,
		settings.HttpsProxy,
		settings.NoProxy,
	}
	for _, key := range fields {
		if _, found := secret.Data[key]; found {
			return true
		}
	}

	return false
}

// Validate source proxy secret
func (r ReconcileMigPlan) validateSourceRegistryProxySecret(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended, InvalidSourceClusterRef, SourceClusterNotReady) {
		return nil
	}

	list := kapi.SecretList{}
	selector := k8sLabels.SelectorFromSet(map[string]string{
		"migration-proxy-config": "true",
	})

	// Source cluster proxy secret validation
	srcCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		log.Trace(err)
		return err
	}

	if srcCluster == nil {
		return nil
	}

	srcClient, err := srcCluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = srcClient.List(
		context.TODO(),
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
		&list,
	)
	if err != nil {
		log.Trace(err)
		return err
	}
	if len(list.Items) == 0 {
		// No proxy secret is valid configuration
		return nil
	}
	if len(list.Items) > 1 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterProxySecretMisconfigured,
			Status:   True,
			Reason:   Conflict,
			Category: Critical,
			Message:  SourceClusterProxySecretMisconfiguredMessage,
		})
		return nil
	}
	if !r.validateRegistryProxySecret(&list.Items[0]) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterProxySecretMisconfigured,
			Status:   True,
			Reason:   KeyNotFound,
			Category: Critical,
			Message:  SourceClusterProxySecretMisconfiguredMessage,
		})
	}
	return nil
}

// Validate destination proxy secret
func (r ReconcileMigPlan) validateDestinationRegistryProxySecret(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended, InvalidDestinationClusterRef, DestinationClusterNotReady) {
		return nil
	}

	list := kapi.SecretList{}
	selector := k8sLabels.SelectorFromSet(map[string]string{
		"migration-proxy-config": "true",
	})

	// Destination cluster proxy secret validation
	destCluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		log.Trace(err)
		return err
	}

	if destCluster == nil {
		return nil
	}

	destClient, err := destCluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = destClient.List(
		context.TODO(),
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
		&list,
	)
	if err != nil {
		log.Trace(err)
		return err
	}
	if len(list.Items) == 0 {
		// No proxy secret is valid configuration
		return nil
	}
	if len(list.Items) > 1 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterProxySecretMisconfigured,
			Status:   True,
			Reason:   Conflict,
			Category: Critical,
			Message:  DestinationClusterProxySecretMisconfiguredMessage,
		})
		return nil
	}
	if !r.validateRegistryProxySecret(&list.Items[0]) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterProxySecretMisconfigured,
			Status:   True,
			Reason:   KeyNotFound,
			Category: Critical,
			Message:  DestinationClusterProxySecretMisconfiguredMessage,
		})
	}
	return nil
}

// Validate the pods, all should be healthy befor migration
func (r ReconcileMigPlan) validatePods(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended) {
		plan.Status.StageCondition(SourcePodsNotHealthy)
		return nil
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

	unhealthyResources := migapi.UnhealthyResources{}
	for _, ns := range plan.Spec.Namespaces {
		unhealthyPods, err := health.PodsUnhealthy(client, &k8sclient.ListOptions{
			Namespace: ns,
		})
		if err != nil {
			log.Trace(err)
			return err
		}

		workload := migapi.Workload{
			Name: "Pods",
		}
		for _, unstrucredPod := range *unhealthyPods {
			pod := &kapi.Pod{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrucredPod.UnstructuredContent(), pod)
			if err != nil {
				log.Trace(err)
				return err
			}

			workload.Resources = append(workload.Resources, pod.Name)
		}

		if len(workload.Resources) != 0 {
			unhealthyNamespace := migapi.UnhealthyNamespace{
				Name:      ns,
				Workloads: []migapi.Workload{workload},
			}
			unhealthyResources.Namespaces = append(unhealthyResources.Namespaces, unhealthyNamespace)
		}
	}
	plan.Spec.UnhealthyResources = unhealthyResources

	if len(plan.Spec.UnhealthyResources.Namespaces) != 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourcePodsNotHealthy,
			Status:   True,
			Reason:   NotHealthy,
			Category: Warn,
			Message:  SourcePodsNotHealthyMessage,
		})
	}

	return nil
}

func (r ReconcileMigPlan) validateHooks(plan *migapi.MigPlan) error {
	var preBackupCount, postBackupCount, preRestoreCount, postRestoreCount int = 0, 0, 0, 0

	for _, hook := range plan.Spec.Hooks {
		migHook := migapi.MigHook{}
		err := r.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      hook.Reference.Name,
				Namespace: hook.Reference.Namespace,
			},
			&migHook)

		// NotFound
		if k8serror.IsNotFound(err) {
			plan.Status.SetCondition(migapi.Condition{
				Type:     InvalidHookRef,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  InvalidHookRefMessage,
			})
			return nil
		} else if err != nil {
			log.Trace(err)
			return err
		}

		// InvalidHookSA
		if errs := validation.IsDNS1123Subdomain(hook.ServiceAccount); len(errs) != 0 {
			plan.Status.SetCondition(migapi.Condition{
				Type:     InvalidHookSAName,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  InvalidHookSANameMessage,
			})
		}

		// InvalidHookNS
		if errs := validation.IsDNS1123Label(hook.ExecutionNamespace); len(errs) != 0 {
			plan.Status.SetCondition(migapi.Condition{
				Type:     InvalidHookNSName,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  InvalidHookNSNameMessage,
			})
		}

		// NotReady
		if !migHook.Status.IsReady() {
			plan.Status.SetCondition(migapi.Condition{
				Type:     HookNotReady,
				Status:   True,
				Category: Critical,
				Message:  HookNotReadyMessage,
			})
			return nil
		}

		switch hook.Phase {
		case migapi.PreRestoreHookPhase:
			preRestoreCount++
		case migapi.PostRestoreHookPhase:
			postRestoreCount++
		case migapi.PreBackupHookPhase:
			preBackupCount++
		case migapi.PostBackupHookPhase:
			postBackupCount++
		default:
			plan.Status.SetCondition(migapi.Condition{
				Type:     HookPhaseUnknown,
				Status:   True,
				Category: Critical,
				Message:  HookPhaseUnknown,
			})
			return nil
		}
	}

	if preRestoreCount > 1 ||
		postRestoreCount > 1 ||
		preBackupCount > 1 ||
		postBackupCount > 1 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     HookPhaseDuplicateMessage,
			Status:   True,
			Category: Critical,
			Message:  HookPhaseDuplicateMessage,
		})
		return nil
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

//
// NFS validation
//
const (
	NfsPort = "2049"
)

// NFS validation.
//   Plan - A migration plan.
//   cluster - The destination cluster.
//   client - A client for the destination.
//   restCfg - A rest configuration for the destination.
//   pod - The pod to be used to execute commands.
type NfsValidation struct {
	Plan    *migapi.MigPlan
	cluster *migapi.MigCluster
	client  k8sclient.Client
	restCfg *rest.Config
	pod     *kapi.Pod
}

// Validate the NFS servers referenced by PVs have network accessible
// on the destination cluster. PVs referencing inaccessible NFS servers
// will be updated to not support the `move` action.
func (r *NfsValidation) Run(client k8sclient.Client) error {
	if !r.Plan.Status.HasAnyCondition(PvsDiscovered) {
		return nil
	}
	if r.Plan.Status.HasAnyCondition(Suspended) {
		r.Plan.Status.StageCondition(NfsNotAccessible)
		return nil
	}
	err := r.init(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	if !r.Plan.Status.HasCondition(NfsAccessCannotBeValidated) {
		err = r.validate()
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Set the cluster, client, restCfg, pod attributes.
func (r *NfsValidation) init(client k8sclient.Client) error {
	var err error
	r.cluster, err = r.Plan.GetDestinationCluster(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	if !r.cluster.Spec.IsHostCluster {
		r.client, err = r.cluster.GetClient(client)
		if err != nil {
			log.Trace(err)
			return err
		}
		r.restCfg, err = r.cluster.BuildRestConfig(client)
		if err != nil {
			log.Trace(err)
			return err
		}
		err = r.findPod()
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Validate PVs.
func (r *NfsValidation) validate() error {
	server := map[string]bool{}
	for i := range r.Plan.Spec.PersistentVolumes.List {
		pv := &r.Plan.Spec.PersistentVolumes.List[i]
		if pv.NFS == nil {
			continue
		}
		if passed, found := server[pv.NFS.Server]; found {
			if !passed {
				r.validationFailed(pv)
			}
			continue
		}
		passed := true
		if !r.cluster.Spec.IsHostCluster {
			command := pods.PodCommand{
				Args:    []string{"/usr/bin/nc", "-zv", pv.NFS.Server, NfsPort},
				RestCfg: r.restCfg,
				Pod:     r.pod,
			}
			err := command.Run()
			if err != nil {
				_, cast := err.(exec.CodeExitError)
				if cast {
					passed = false
				} else {
					log.Trace(err)
					return err
				}
			}
		} else {
			address := fmt.Sprintf("%s:%s", pv.NFS.Server, NfsPort)
			conn, err := net.Dial("tcp", address)
			if err == nil {
				conn.Close()
			} else {
				passed = false
			}
		}
		server[pv.NFS.Server] = passed
		if !passed {
			r.validationFailed(pv)
		}
	}
	notAccessible := []string{}
	for s, passed := range server {
		if !passed {
			notAccessible = append(notAccessible, s)
		}
	}
	if len(notAccessible) > 0 {
		r.Plan.Status.SetCondition(migapi.Condition{
			Type:     NfsNotAccessible,
			Status:   True,
			Category: Warn,
			Message:  NfsNotAccessibleMessage,
			Items:    notAccessible,
		})
	}

	return nil
}

// Find a pod suitable for command execution.
func (r *NfsValidation) findPod() error {
	list := kapi.PodList{}
	options := k8sclient.MatchingLabels(map[string]string{"component": "velero"})
	options.Namespace = migapi.VeleroNamespace
	err := r.client.List(
		context.TODO(),
		options,
		&list)
	if err != nil {
		log.Trace(err)
		return err
	}
	for i := range list.Items {
		r.pod = &list.Items[i]
		command := pods.PodCommand{
			Args:    []string{"/usr/bin/nc", "-h"},
			RestCfg: r.restCfg,
			Pod:     r.pod,
		}
		err := command.Run()
		if err != nil {
			r.pod = nil
		}
	}
	if r.pod == nil {
		r.Plan.Status.SetCondition(migapi.Condition{
			Type:     NfsAccessCannotBeValidated,
			Status:   True,
			Category: Error,
			Reason:   NotFound,
			Message:  NfsAccessCannotBeValidatedMessage,
		})
		return nil
	}

	return nil
}

// Validation failed.
// The `move` action is removed.
func (r *NfsValidation) validationFailed(pv *migapi.PV) {
	kept := []string{}
	for _, action := range pv.Supported.Actions {
		if action != migapi.PvMoveAction {
			kept = append(kept, action)
		}
	}

	pv.Supported.Actions = kept
}
