package migplan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"sort"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/controller/migcluster"
	"github.com/konveyor/mig-controller/pkg/health"
	"github.com/konveyor/mig-controller/pkg/pods"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/konveyor/mig-controller/pkg/settings"
	"github.com/opentracing/opentracing-go"
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
	ClusterVersionMismatch                     = "ClusterVersionMismatch"
	SourceClusterNoRegistryPath                = "SourceClusterNoRegistryPath"
	DestinationClusterNoRegistryPath           = "DestinationClusterNoRegistryPath"
	StorageNotReady                            = "StorageNotReady"
	NsListEmpty                                = "NamespaceListEmpty"
	InvalidDestinationCluster                  = "InvalidDestinationCluster"
	NsNotFoundOnSourceCluster                  = "NamespaceNotFoundOnSourceCluster"
	NsNotFoundOnDestinationCluster             = "NamespaceNotFoundOnDestinationCluster"
	NsLimitExceeded                            = "NamespaceLimitExceeded"
	NsLengthExceeded                           = "NamespaceLengthExceeded"
	NsHaveNodeSelectors                        = "NamespacesHaveNodeSelectors"
	DuplicateNsOnSourceCluster                 = "DuplicateNamespaceOnSourceCluster"
	DuplicateNsOnDestinationCluster            = "DuplicateNamespaceOnDestinationCluster"
	PodLimitExceeded                           = "PodLimitExceeded"
	SourceClusterProxySecretMisconfigured      = "SourceClusterProxySecretMisconfigured"
	DestinationClusterProxySecretMisconfigured = "DestinationClusterProxySecretMisconfigured"
	PlanConflict                               = "PlanConflict"
	PvNameConflict                             = "PvNameConflict"
	PvInvalidAction                            = "PvInvalidAction"
	PvNoSupportedAction                        = "PvNoSupportedAction"
	PvInvalidStorageClass                      = "PvInvalidStorageClass"
	PvInvalidAccessMode                        = "PvInvalidAccessMode"
	PvNoStorageClassSelection                  = "PvNoStorageClassSelection"
	PvWarnNoCephAvailable                      = "PvWarnNoCephAvailable"
	PvWarnAccessModeUnavailable                = "PvWarnAccessModeUnavailable"
	PvInvalidCopyMethod                        = "PvInvalidCopyMethod"
	PvCapacityAdjustmentRequired               = "PvCapacityAdjustmentRequired"
	PvUsageAnalysisFailed                      = "PvUsageAnalysisFailed"
	PvNoCopyMethodSelection                    = "PvNoCopyMethodSelection"
	PvWarnCopyMethodSnapshot                   = "PvWarnCopyMethodSnapshot"
	NfsNotAccessible                           = "NfsNotAccessible"
	NfsAccessCannotBeValidated                 = "NfsAccessCannotBeValidated"
	PvLimitExceeded                            = "PvLimitExceeded"
	StorageEnsured                             = "StorageEnsured"
	RegistriesEnsured                          = "RegistriesEnsured"
	RegistriesHealthy                          = "RegistriesHealthy"
	PvsDiscovered                              = "PvsDiscovered"
	Closed                                     = "Closed"
	SourcePodsNotHealthy                       = "SourcePodsNotHealthy"
	GVKsIncompatible                           = "GVKsIncompatible"
	InvalidHookRef                             = "InvalidHookRef"
	InvalidResourceList                        = "InvalidResourceList"
	HookNotReady                               = "HookNotReady"
	InvalidHookNSName                          = "InvalidHookNSName"
	InvalidHookSAName                          = "InvalidHookSAName"
	HookPhaseUnknown                           = "HookPhaseUnknown"
	HookPhaseDuplicate                         = "HookPhaseDuplicate"
	IntraClusterMigration                      = "IntraClusterMigration"
	MigrationTypeIdentified                    = "MigrationTypeIdentified"
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
	NotSet                 = "NotSet"
	NotFound               = "NotFound"
	KeyNotFound            = "KeyNotFound"
	NotDistinct            = "NotDistinct"
	LimitExceeded          = "LimitExceeded"
	LengthExceeded         = "LengthExceeded"
	NotDone                = "NotDone"
	Done                   = "Done"
	Conflict               = "Conflict"
	NotHealthy             = "NotHealthy"
	NodeSelectorsDetected  = "NodeSelectorsDetected"
	DuplicateNs            = "DuplicateNamespaces"
	ConflictingNamespaces  = "ConflictingNamespaces"
	ConflictingPermissions = "ConflictingPermissions"
	StorageConversionPlan  = "StorageConversionPlan"
	StateMigrationPlan     = "StateMigrationPlan"
	NamespaceMigrationPlan = "NamespaceMigrationPlan"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// OpenShift NS annotations
const (
	openShiftMCSAnnotation       = "openshift.io/sa.scc.mcs"
	openShiftSuppGroupAnnotation = "openshift.io/sa.scc.supplemental-groups"
	openShiftUIDRangeAnnotation  = "openshift.io/sa.scc.uid-range"
)

// Valid AccessMode values
var validAccessModes = []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany}

// Validate the plan resource.
func (r ReconcileMigPlan) validate(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validate")
		defer span.Finish()
	}
	// Source cluster
	err := r.validateSourceCluster(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Destination cluster
	err = r.validateDestinationCluster(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// validates possible migration options available for this plan
	err = r.validatePossibleMigrationTypes(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Storage
	err = r.validateStorage(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Migrated namespaces.
	err = r.validateNamespaces(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Validates pod properties (e.g. limit of number of active pods, presence of node-selectors)
	// within each namespace.
	err = r.validatePodProperties(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Required namespaces.
	err = r.validateRequiredNamespaces(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Conflict
	err = r.validateConflict(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Registry proxy secret
	err = r.validateRegistryProxySecrets(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Validate health of Pods
	err = r.validatePodHealth(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Hooks
	err = r.validateHooks(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Included Resources
	err = r.validateIncludedResources(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// GVK
	err = r.compareGVK(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Versions
	err = r.validateOperatorVersions(ctx, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// validateIncludedResources checks spec.IncludedResources field of the plan for presence of
// invalid resource Group/Kinds, raises critical condition if one or more invalid resources found
func (r ReconcileMigPlan) validateIncludedResources(ctx context.Context, plan *migapi.MigPlan) error {
	srcCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcCluster == nil || !srcCluster.Status.IsReady() {
		return nil
	}
	srcClient, err := srcCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	_, invalidResources, err := plan.GetIncludedResourcesList(srcClient)
	if err != nil {
		log.Error(err,
			"error creating list of included resources",
			"failedResources", strings.Join(invalidResources, ","))
	}
	if len(invalidResources) > 0 {
		plan.Status.SetCondition(
			migapi.Condition{
				Category: migapi.Critical,
				Status:   True,
				Type:     InvalidResourceList,
				Message:  "Resources [] specified in spec.includedResourceList not found. Resource Group & Kind must be valid. See controller logs for more details.",
				Items:    invalidResources,
			},
		)
	}
	return nil
}

// validatePossibleMigrationTypes looks at various migplan fields and determines what kind of migration is possible
// based on the type of the migration, validates user selections specific to each type
func (r ReconcileMigPlan) validatePossibleMigrationTypes(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePossibleMigrationTypes")
		defer span.Finish()
	}
	isIntraCluster, err := plan.IsIntraCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	migrations, err := plan.ListMigrations(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	// find out whether any of the source namespaces are mapped to destination namespaces
	mappedNamespaces := 0
	for srcNs, destNs := range plan.GetNamespaceMapping() {
		if srcNs != destNs {
			mappedNamespaces += 1
		}
	}
	// try to infer migration type based on existing migrations created for the plan
	if len(migrations) > 0 {
		sort.Slice(
			migrations, func(i, j int) bool {
				a := migrations[i].ObjectMeta.CreationTimestamp
				b := migrations[j].ObjectMeta.CreationTimestamp
				return b.Before(&a)
			})
		successfulRollbackExists := migrations[0].Spec.Rollback &&
			migrations[0].Status.HasAnyCondition(migapi.Succeeded)
		// if the last migration was a successful rollback, the plan is reset and new types of migrations are possible
		if successfulRollbackExists {
			plan.Status.DeleteCondition(MigrationTypeIdentified)
		} else {
			for _, migration := range migrations {
				if migration.Spec.MigrateState {
					// if plan is intra-cluster and all the source namespaces are mapped to themselves
					// the migration has to be used for storage conversion
					if isIntraCluster && mappedNamespaces == 0 {
						plan.Status.SetCondition(migapi.Condition{
							Type:     MigrationTypeIdentified,
							Status:   True,
							Reason:   StorageConversionPlan,
							Category: migapi.Advisory,
							Message:  "The migration plan was previously used for Storage Conversion. It can only be used for further Storage Conversions. Other migrations will be possible only after a successful rollback is performed.",
							Durable:  true,
						})
						return nil
					} else {
						plan.Status.SetCondition(migapi.Condition{
							Type:     MigrationTypeIdentified,
							Status:   True,
							Reason:   StateMigrationPlan,
							Category: migapi.Advisory,
							Message:  "The migration plan was previously used for State Migrations. This plan can only be used for further State Migrations. Other migrations are possible only after a successful rollback is performed.",
							Durable:  true,
						})
						return nil
					}
				}
			}
			plan.Status.SetCondition(migapi.Condition{
				Type:     MigrationTypeIdentified,
				Status:   True,
				Reason:   NamespaceMigrationPlan,
				Category: migapi.Advisory,
				Message:  "This migration plan was previously used for migrating namespaces. This plan can only be used for further Stage/Final Migration. Other migrations are possible only after a successful rollback is performed.",
				Durable:  true,
			})
		}
	}
	// when there are no migrations for the plan, use migration plan information
	// and user inputs to determine possible types of migrations
	// if the plan is intra-cluster, normal migrations are not possible
	// plan can only be used either for state migrations or storage conversions
	if isIntraCluster {
		// if some namespaces are mapped and some are not, then we cannot determine the possible migration types
		if mappedNamespaces > 0 && mappedNamespaces < len(plan.GetSourceNamespaces()) {
			plan.Status.SetCondition(migapi.Condition{
				Type:     IntraClusterMigration,
				Status:   True,
				Reason:   ConflictingNamespaces,
				Category: Critical,
				Message:  "This is an intra-cluster migration plan. Either all or none of the source namespaces should be mapped to distinct destination namespaces.",
			})
			return nil
		}
		// if all source namespaces are mapped to themselves, the plan can only be used
		// for storage conversion
		if mappedNamespaces == 0 {
			plan.Status.SetCondition(migapi.Condition{
				Type:     MigrationTypeIdentified,
				Status:   True,
				Reason:   StorageConversionPlan,
				Category: migapi.Advisory,
				Message:  "This is an intra-cluster migration plan and none of the source namespaces are mapped to different destination namespaces. This plan can only be used for Storage Conversion.",
			})
			return nil
		}
		// if all source namespaces are mapped to different destination namespaces, the
		// plan can only be used for state migrations
		if mappedNamespaces == len(plan.GetSourceNamespaces()) {
			plan.Status.SetCondition(migapi.Condition{
				Type:     MigrationTypeIdentified,
				Status:   True,
				Reason:   StateMigrationPlan,
				Category: migapi.Advisory,
				Message:  "This is an intra-cluster migration plan and all of the source namespaces are mapped to different destination namespaces. This plan can only be used for State Migration.",
			})
			return nil
		}
	} else {
		// if any of the PVC names are mapped to different destination PVCs, the plan can only be used for State Migrations
		for _, pv := range plan.Spec.PersistentVolumes.List {
			if pv.PVC.GetSourceName() != pv.PVC.GetTargetName() {
				plan.Status.SetCondition(migapi.Condition{
					Type:     MigrationTypeIdentified,
					Status:   True,
					Reason:   StateMigrationPlan,
					Category: migapi.Advisory,
					Message:  "One or more source PVCs are mapped to different destination PVCs. This plan can only be used for State Migration.",
				})
				return nil
			}
		}
	}
	return nil
}

// getPotentialFilePermissionConflictNamespaces iterates over source and destination namespaces and checks whether
// the destination namespaces already exist. If they do, further checks whether the source and destination ns'es
// have the same UID, Supp Groups and SELinux labels set. Returns all namespaces which have conflict in that info
func (r ReconcileMigPlan) getPotentialFilePermissionConflictNamespaces(plan *migapi.MigPlan) ([]string, error) {
	srcCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if srcCluster == nil || !srcCluster.Status.IsReady() {
		return nil, nil
	}
	srcClient, err := srcCluster.GetClient(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	destCluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if destCluster == nil {
		return nil, nil
	}
	destClient, err := destCluster.GetClient(r)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	erroredNs := []string{}
	potentiallyProblematicNs := []string{}
	for srcNs, destNs := range plan.GetNamespaceMapping() {
		srcNsDef := &kapi.Namespace{}
		err = srcClient.Get(context.TODO(), types.NamespacedName{Name: srcNs}, srcNsDef)
		if err != nil {
			erroredNs = append(erroredNs, destNs)
			continue
		}
		destNsDef := &kapi.Namespace{}
		err = destClient.Get(context.TODO(), types.NamespacedName{Name: destNs}, destNsDef)
		if err != nil {
			if !k8serror.IsNotFound(err) {
				erroredNs = append(erroredNs, destNs)
			}
			continue
		}
		if !areNamespaceAnnotationValuesEqual(openShiftUIDRangeAnnotation, srcNsDef, destNsDef) ||
			!areNamespaceAnnotationValuesEqual(openShiftSuppGroupAnnotation, srcNsDef, destNsDef) ||
			!areNamespaceAnnotationValuesEqual(openShiftMCSAnnotation, srcNsDef, destNsDef) {
			potentiallyProblematicNs = append(potentiallyProblematicNs, destNs)
		}
	}
	if len(erroredNs) > 0 {
		log.V(4).Error(
			errors.New("failed to gather UID/GID/Selinux Labels information"),
			fmt.Sprintf("namespaces [%s]", strings.Join(erroredNs, ",")))
	}
	return potentiallyProblematicNs, nil
}

func areNamespaceAnnotationValuesEqual(annotation string, srcNs *kapi.Namespace, destNs *kapi.Namespace) bool {
	isEqual := false
	sourceVal, srcExists := srcNs.Annotations[annotation]
	destVal, destExists := destNs.Annotations[annotation]
	if srcExists && destExists && (sourceVal == destVal) {
		isEqual = true
	}
	if !srcExists && !destExists {
		isEqual = true
	}
	return isEqual
}

// Validate the referenced storage.
func (r ReconcileMigPlan) validateStorage(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateStorage")
		defer span.Finish()
	}
	ref := plan.Spec.MigStorageRef

	// NotSet
	if !isStorageConversionPlan(plan) && !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidStorageRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `spec.migStorageRef` must reference a valid `migstorage`.",
		})
		return nil
	}

	storage, err := migapi.GetStorage(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// NotFound
	if !isStorageConversionPlan(plan) && storage == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidStorageRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message: fmt.Sprintf("The `spec.migStorageRef` must reference a valid `migstorage`, subject: %s.",
				path.Join(plan.Spec.MigStorageRef.Namespace, plan.Spec.MigStorageRef.Name)),
		})
		return nil
	}

	// NotReady
	if !isStorageConversionPlan(plan) && !storage.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     StorageNotReady,
			Status:   True,
			Category: Critical,
			Message: fmt.Sprintf("The referenced `migStorageRef` does not have a `Ready` condition, subject: %s.",
				path.Join(plan.Spec.MigStorageRef.Namespace, plan.Spec.MigStorageRef.Name)),
		})
	}

	return nil
}

// Validate the referenced assetCollection.
func (r ReconcileMigPlan) validateNamespaces(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateNamespaces")
		defer span.Finish()
	}

	count := len(plan.Spec.Namespaces)
	if count == 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsListEmpty,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "The `namespaces` list may not be empty.",
		})
		return nil
	}
	limit := Settings.Plan.NsLimit
	if count > limit {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsLimitExceeded,
			Status:   True,
			Reason:   LimitExceeded,
			Category: Critical,
			Message:  fmt.Sprintf("Namespace limit: %d exceeded, found:%d.", limit, count),
		})
		return nil
	}
	namespaces := r.validateNamespaceLengthForDVM(plan)
	if len(namespaces) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsLengthExceeded,
			Status:   True,
			Reason:   LengthExceeded,
			Category: Warn,
			Message:  "Namespaces [] exceed 59 characters and no destination cluster route subdomain was configured. Direct Volume Migration may fail if you do not set `cluster_subdomain` value on the `MigrationController` CR.",
			Items:    namespaces,
		})
		return nil
	}
	filePermissionIssues, err := r.getPotentialFilePermissionConflictNamespaces(plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	if len(filePermissionIssues) > 0 {
		plan.Status.SetCondition(
			migapi.Condition{
				Type:     IntraClusterMigration,
				Status:   True,
				Reason:   ConflictingPermissions,
				Category: Warn,
				Message:  "Destination namespaces [] already exist in the target cluster with different values of UID/Supplemental Groups/SELinux Labels. Migrating PV data into these namespaces may result in file permission issues. Either delete the destination namespaces or map to different namespaces to avoid file permission issues.",
				Items:    filePermissionIssues,
			},
		)
	}
	return nil
}

func (r ReconcileMigPlan) validateNamespaceLengthForDVM(plan *migapi.MigPlan) []string {
	items := []string{}
	// If validation for destination cluster ref failed, we can't check this
	if plan.Status.HasAnyCondition(
		InvalidDestinationClusterRef, InvalidDestinationCluster) {
		return items
	}
	// This is not relevant if the plan is not running DVM
	// This validation is also not needed if the user has supplied the route
	// subdomain for the destination cluster
	cluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		return items
	}
	subdomain, _ := cluster.GetClusterSubdomain(r)
	if plan.Spec.IndirectVolumeMigration || subdomain != "" {
		return items
	}
	for _, ns := range plan.GetDestinationNamespaces() {
		// If length of namespace is 60+ characters, route creation will fail as
		// the route generator will attempt to create a route with:
		// dvm-<namespace> and this cannot exceed 63 characters
		if len(ns) > 59 {
			items = append(items, ns)
		}
	}
	return items
}

// Validates the following properties of pods across namespaces:
// 1. Validate the total number of running pods (limit) across namespaces.
// 2. Whether any pods have node-selectors or node names associated with it. If so, a warning is raised to indicate the
// list of namespaces associated with that pod.
func (r ReconcileMigPlan) validatePodProperties(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePodProperties")
		defer span.Finish()
	}
	if plan.Status.HasAnyCondition(Suspended, NsLimitExceeded) {
		plan.Status.StageCondition(PodLimitExceeded)
		return nil
	}
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	count := 0
	limit := Settings.Plan.PodLimit

	nsWithNodeSelectors := make([]string, 0)
	for _, name := range plan.GetSourceNamespaces() {
		list := kapi.PodList{}
		options := k8sclient.ListOptions{
			FieldSelector: fields.SelectorFromSet(
				fields.Set{
					"status.phase": string(kapi.PodRunning),
				}),
			Namespace: name,
		}
		err := client.List(context.TODO(), &list, &options)
		if err != nil {
			return liberr.Wrap(err)
		}
		count += len(list.Items)
		if r.hasCustomNodeSelectors(list.Items) {
			nsWithNodeSelectors = append(nsWithNodeSelectors, name)
		}
	}
	if count > limit {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PodLimitExceeded,
			Status:   True,
			Reason:   LimitExceeded,
			Category: Warn,
			Message:  fmt.Sprintf("Pod limit: %d exceeded, found: %d.", limit, count),
		})
	}

	if len(nsWithNodeSelectors) > 0 {
		msgFormat := "Found Pods with non-default `Spec.NodeSelector` set in namespaces: [%s]. " +
			"This field will be cleared on Pods restored into the target cluster."
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsHaveNodeSelectors,
			Status:   True,
			Reason:   NodeSelectorsDetected,
			Category: Warn,
			Durable:  true,
			Message: fmt.Sprintf(msgFormat,
				strings.Join(nsWithNodeSelectors, ", ")),
		})
	} else {
		plan.Status.DeleteCondition(NsHaveNodeSelectors)
	}

	return nil
}

// Checks the list of Pods for any non-default nodeselectors.
// Returns true if custom nodeselectors found on any Pod.
func (r ReconcileMigPlan) hasCustomNodeSelectors(podList []kapi.Pod) bool {
	// Known default node selector values. Ignore these if we spot them on Pods
	defaultNodeSelectors := []string{
		"node-role.kubernetes.io/compute",
	}
	for _, pod := range podList {
		if pod.Spec.NodeSelector == nil {
			continue
		}
		for nodeSelector := range pod.Spec.NodeSelector {
			for _, defaultSelector := range defaultNodeSelectors {
				// Return true if node selector on Pod is not one of the defaults
				if nodeSelector != defaultSelector {
					return true
				}
			}
		}
	}
	return false
}

// Validate the referenced source cluster.
func (r ReconcileMigPlan) validateSourceCluster(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateSourceCluster")
		defer span.Finish()
	}
	ref := plan.Spec.SrcMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `srcMigClusterRef` must reference a valid `migcluster`.",
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidSourceClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message: fmt.Sprintf("The `srcMigClusterRef` must reference a valid `migcluster`, subject: %s.",
				path.Join(plan.Spec.SrcMigClusterRef.Namespace, plan.Spec.SrcMigClusterRef.Name)),
		})
		return nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNotReady,
			Status:   True,
			Category: Critical,
			Message: fmt.Sprintf("The referenced `srcMigClusterRef` does not have a `Ready` condition, subject: %s",
				path.Join(plan.Spec.SrcMigClusterRef.Namespace, plan.Spec.SrcMigClusterRef.Name)),
		})
		return nil
	}

	// No Registry Path
	registryPath, err := cluster.GetRegistryPath(r)
	if !plan.Spec.IndirectImageMigration && (err != nil || registryPath == "") {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterNoRegistryPath,
			Status:   True,
			Category: Critical,
			Reason:   NotSet,
			Message: fmt.Sprintf("Direct image migration is selected and the source cluster %s is missing a configured Registry Path",
				path.Join(ref.Namespace, ref.Name)),
		})
		return nil
	}

	return nil
}

// Validate the referenced source cluster.
func (r ReconcileMigPlan) validateDestinationCluster(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateDestinationCluster")
		defer span.Finish()
	}
	ref := plan.Spec.DestMigClusterRef

	// NotSet
	if !migref.RefSet(ref) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  "The `dstMigClusterRef` must reference a valid `migcluster`.",
		})
		return nil
	}

	cluster, err := migapi.GetCluster(r, ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	// NotFound
	if cluster == nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:     InvalidDestinationClusterRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message: fmt.Sprintf("The `dstMigClusterRef` must reference a valid `migcluster`, subject: %s.",
				path.Join(plan.Spec.DestMigClusterRef.Namespace, plan.Spec.DestMigClusterRef.Name)),
		})
		return nil
	}

	// NotReady
	if !cluster.Status.IsReady() {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNotReady,
			Status:   True,
			Category: Critical,
			Message: fmt.Sprintf("The referenced `dstMigClusterRef` does not have a `Ready` condition, subject: %s",
				path.Join(plan.Spec.DestMigClusterRef.Namespace, plan.Spec.DestMigClusterRef.Name)),
		})
		return nil
	}

	// No Registry Path
	registryPath, err := cluster.GetRegistryPath(r)
	if !plan.Spec.IndirectImageMigration && (err != nil || registryPath == "") {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterNoRegistryPath,
			Status:   True,
			Category: Critical,
			Reason:   NotSet,
			Message: fmt.Sprintf("Direct image migration is selected and the destination cluster %s is missing a configured Registry Path",
				path.Join(ref.Namespace, ref.Name)),
		})
		return nil
	}

	return nil
}

func (r ReconcileMigPlan) validateOperatorVersions(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateOperatorVersions")
		defer span.Finish()
	}

	if plan.Status.HasAnyCondition(
		InvalidDestinationClusterRef, InvalidDestinationCluster,
		InvalidSourceClusterRef, InvalidSourceClusterRef) {
		return nil
	}
	destRef := plan.Spec.DestMigClusterRef
	srcRef := plan.Spec.SrcMigClusterRef

	srcCluster, err := migapi.GetCluster(r, srcRef)
	if err != nil {
		return liberr.Wrap(err)
	}

	destCluster, err := migapi.GetCluster(r, destRef)
	if err != nil {
		return liberr.Wrap(err)
	}
	srcHasMismatch := srcCluster.Status.HasAnyCondition(
		migcluster.OperatorVersionMismatch,
		migcluster.ClusterOperatorVersionNotFound)
	destHasMismatch := destCluster.Status.HasAnyCondition(
		migcluster.OperatorVersionMismatch,
		migcluster.ClusterOperatorVersionNotFound)
	if srcHasMismatch || destHasMismatch {
		plan.Status.SetCondition(migapi.Condition{
			Type:     ClusterVersionMismatch,
			Status:   True,
			Category: Warn,
			Reason:   Conflict,
			Message:  fmt.Sprintf("Cluster operator versions do not match. Source, destination, and host clusters must all have the same MTC operator version."),
		})
		return nil
	}

	return nil
}

// Validate required namespaces.
func (r ReconcileMigPlan) validateRequiredNamespaces(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateRequiredNamespaces")
		defer span.Finish()
	}
	err := r.validateSourceNamespaces(plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateDestinationNamespaces(plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Validate required namespaces on the source cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateSourceNamespaces(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended, NsLimitExceeded) {
		plan.Status.StageCondition(NsNotFoundOnSourceCluster)
		return nil
	}
	namespaces := plan.GetSourceNamespaces()
	duplicates := listDuplicateNamespaces(namespaces)
	if len(duplicates) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DuplicateNsOnSourceCluster,
			Status:   True,
			Reason:   DuplicateNs,
			Category: Critical,
			Message:  "Duplicate source cluster namespaces [] in migplan.",
			Items:    duplicates,
		})
		return nil
	}
	namespaces = append(namespaces, migapi.VeleroNamespace)
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
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
			return liberr.Wrap(err)
		}
	}
	if len(notFound) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnSourceCluster,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Namespaces [] not found on the source cluster.",
			Items:    notFound,
		})
		return nil
	}

	return nil
}

// Validate required namespaces on the destination cluster.
// Returns error and the total error conditions set.
func (r ReconcileMigPlan) validateDestinationNamespaces(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended) {
		return nil
	}

	namespaces := plan.GetDestinationNamespaces()
	duplicates := listDuplicateNamespaces(namespaces)
	if len(duplicates) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DuplicateNsOnDestinationCluster,
			Status:   True,
			Reason:   DuplicateNs,
			Category: Critical,
			Message:  "Duplicate destination cluster namespaces [] in migplan.",
			Items:    duplicates,
		})
		return nil
	}
	longNamespaceNames := []string{}
	for _, ns := range namespaces {
		if len(ns) > 63 {
			longNamespaceNames = append(longNamespaceNames, ns)
		}
	}
	if len(longNamespaceNames) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsLengthExceeded,
			Status:   True,
			Reason:   LengthExceeded,
			Category: Critical,
			Message:  fmt.Sprintf("Destination Namespaces [] exceed 63 characters and are invalid."),
			Items:    longNamespaceNames,
		})
		return nil
	}

	namespaces = []string{migapi.VeleroNamespace}
	cluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}
	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
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
			return liberr.Wrap(err)
		}
	}
	if len(notFound) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     NsNotFoundOnDestinationCluster,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  "Namespaces [] not found on the destination cluster.",
			Items:    notFound,
		})
		return nil
	}

	return nil
}

func listDuplicateNamespaces(nsList []string) []string {
	found := make(map[string]bool)
	duplicates := []string{}
	for _, name := range nsList {
		if _, value := found[name]; !value {
			found[name] = true
		} else {
			duplicates = append(duplicates, name)
		}
	}
	return duplicates
}

// Validate the plan does not conflict with another plan.
func (r ReconcileMigPlan) validateConflict(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateConflict")
		defer span.Finish()
	}
	plans, err := migapi.ListPlans(r)
	if err != nil {
		return liberr.Wrap(err)
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
			Message:  "The migration plan is in conflict with [].",
			Items:    list,
		})
	}

	return nil
}

// Validate PV actions.
func (r ReconcileMigPlan) validatePvSelections(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePvSelections")
		defer span.Finish()
	}
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
	invalidPVCNameMappings := make([]string, 0)

	if plan.Status.HasAnyCondition(Suspended) {
		return nil
	}
	for _, storageClass := range plan.Status.DestStorageClasses {
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

		// if the plan is for Storage Conversion, also ensure that the pvc names are mapped correctly
		if isStorageConversionPlan(plan) {
			if pv.PVC.GetSourceName() == pv.PVC.GetTargetName() {
				invalidPVCNameMappings = append(invalidPVCNameMappings, pv.PVC.Name)
			}
		}
	}
	if len(invalidAction) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidAction,
			Status:   True,
			Reason:   NotDone,
			Category: Error,
			Message:  "PV in `persistentVolumes` [] has an unsupported `action`.",
			Items:    invalidAction,
		})
	}
	if len(unsupported) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoSupportedAction,
			Status:   True,
			Category: Warn,
			Message:  "PV in `persistentVolumes` [] with no `SupportedActions`.",
			Items:    unsupported,
		})
	}
	if len(invalidStorageClass) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidStorageClass,
			Status:   True,
			Reason:   NotDone,
			Category: Error,
			Message:  "PV in `persistentVolumes` [] has an unsupported `storageClass`.",
			Items:    invalidStorageClass,
		})
	}
	if len(invalidAccessMode) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidAccessMode,
			Status:   True,
			Category: Error,
			Message:  "PV in `persistentVolumes` [] has an invalid `accessMode`.",
			Items:    invalidAccessMode,
		})
	}
	if len(missingStorageClass) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoStorageClassSelection,
			Status:   True,
			Reason:   NotFound,
			Category: Warn,
			Message: "PV in `persistentVolumes` [] has no `Selected.StorageClass`. Make sure that the necessary static" +
				" persistent volumes exist in the destination cluster.",
			Items: missingStorageClass,
		})
	}
	if len(unavailableAccessMode) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvWarnAccessModeUnavailable,
			Status:   True,
			Category: Warn,
			Message:  "AccessMode for PVC in `persistentVolumes` [] unavailable in chosen storage class.",
			Items:    unavailableAccessMode,
		})
	}
	if len(missingCopyMethod) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNoCopyMethodSelection,
			Status:   True,
			Reason:   NotFound,
			Category: Error,
			Message:  "PV in `persistentVolumes` [] has no `Selected.CopyMethod`.",
			Items:    missingCopyMethod,
		})
	}
	if len(invalidCopyMethod) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvInvalidCopyMethod,
			Status:   True,
			Category: Error,
			Message:  "PV in `persistentVolumes` [] has an invalid `copyMethod`.",
			Items:    invalidCopyMethod,
		})
	}
	if len(warnCopyMethodSnapshot) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvWarnCopyMethodSnapshot,
			Status:   True,
			Category: Warn,
			Message: "CopyMethod for PV in `persistentVolumes` [] is set to `snapshot`. Make sure that the chosen " +
				"storage class is compatible with the source volume's storage type for Snapshot support.",
			Items: warnCopyMethodSnapshot,
		})
	}
	if len(invalidPVCNameMappings) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvNameConflict,
			Status:   True,
			Category: Error,
			Message:  "This is a storage migration plan and source PVCs [] are not mapped to distinct destination PVCs. This either indicates a problem in user input or controller failing to automatically add a prefix to destination PVC name. Please map the PVC names correctly.",
			Items:    invalidPVCNameMappings,
		})
	}

	return nil
}

// Validate proxy secrets. Should only exist 1 or none
func (r ReconcileMigPlan) validateRegistryProxySecrets(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateRegistryProxySecrets")
		defer span.Finish()
	}
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
		return liberr.Wrap(err)
	}

	if srcCluster == nil {
		return nil
	}

	srcClient, err := srcCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = srcClient.List(
		context.TODO(),
		&list,
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
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
			Message:  "Source cluster proxy secret is misconfigured, more than one instances found.",
		})
		return nil
	}
	if !r.validateRegistryProxySecret(&list.Items[0]) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourceClusterProxySecretMisconfigured,
			Status:   True,
			Reason:   KeyNotFound,
			Category: Critical,
			Message:  "Source cluster proxy secret is misconfigured, secret has invalid keys.",
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
		return liberr.Wrap(err)
	}

	if destCluster == nil {
		return nil
	}

	destClient, err := destCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = destClient.List(
		context.TODO(),
		&list,
		&k8sclient.ListOptions{
			Namespace:     migapi.VeleroNamespace,
			LabelSelector: selector,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
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
			Message:  "Destination cluster proxy secret is misconfigured, more than one instances found.",
		})
		return nil
	}
	if !r.validateRegistryProxySecret(&list.Items[0]) {
		plan.Status.SetCondition(migapi.Condition{
			Type:     DestinationClusterProxySecretMisconfigured,
			Status:   True,
			Reason:   KeyNotFound,
			Category: Critical,
			Message:  "Destination cluster proxy secret is misconfigured, secret has invalid keys.",
		})
	}
	return nil
}

// Validate the pods to verify if they are healthy before migration
func (r ReconcileMigPlan) validatePodHealth(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePodHealth")
		defer span.Finish()
	}

	if plan.Status.HasAnyCondition(Suspended) {
		plan.Status.StageCondition(SourcePodsNotHealthy)
		return nil
	}

	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	if cluster == nil || !cluster.Status.IsReady() {
		return nil
	}

	client, err := cluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	unhealthyResources := migapi.UnhealthyResources{}
	for _, ns := range plan.Spec.Namespaces {
		unhealthyPods, err := health.PodsUnhealthy(client, &k8sclient.ListOptions{
			Namespace: ns,
		})
		if err != nil {
			return liberr.Wrap(err)
		}

		workload := migapi.Workload{
			Name: "Pods",
		}
		for _, unstrucredPod := range *unhealthyPods {
			pod := &kapi.Pod{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrucredPod.UnstructuredContent(), pod)
			if err != nil {
				return liberr.Wrap(err)
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
	plan.Status.UnhealthyResources = unhealthyResources

	if len(plan.Status.UnhealthyResources.Namespaces) != 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     SourcePodsNotHealthy,
			Status:   True,
			Reason:   NotHealthy,
			Category: Warn,
			Message:  "Source namespace(s) contain unhealthy pods. See: `Status.namespaces` for details.",
		})
	}

	return nil
}

func (r ReconcileMigPlan) validateHooks(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateHooks")
		defer span.Finish()
	}

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
				Message:  "One or more referenced hooks do not exist.",
			})
			return nil
		} else if err != nil {
			return liberr.Wrap(err)
		}

		// InvalidHookSA
		if errs := validation.IsDNS1123Subdomain(hook.ServiceAccount); len(errs) != 0 {
			plan.Status.SetCondition(migapi.Condition{
				Type:     InvalidHookSAName,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message: "The serviceAccount specified is invalid, DNS-1123 subdomain regex used for validation" +
					" is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'",
			})
		}

		// InvalidHookNS
		if errs := validation.IsDNS1123Label(hook.ExecutionNamespace); len(errs) != 0 {
			plan.Status.SetCondition(migapi.Condition{
				Type:     InvalidHookNSName,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message: "The executionNamespace specified is invalid, DNS-1123 label regex used for validation" +
					" is '[a-z0-9]([-a-z0-9]*[a-z0-9])?'.",
			})
		}

		// NotReady
		if !migHook.Status.IsReady() {
			plan.Status.SetCondition(migapi.Condition{
				Type:     HookNotReady,
				Status:   True,
				Category: Critical,
				Message:  "One or more referenced hooks are not ready.",
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
				Message:  "One or more referenced hooks are in an unknown phase.",
			})
			return nil
		}
	}

	if preRestoreCount > 1 ||
		postRestoreCount > 1 ||
		preBackupCount > 1 ||
		postBackupCount > 1 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     HookPhaseDuplicate,
			Status:   True,
			Category: Critical,
			Message:  "Only one hook may be specified per phase.",
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
		return liberr.Wrap(err)
	}
	if !r.Plan.Status.HasCondition(NfsAccessCannotBeValidated) {
		err = r.validate()
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Set the cluster, client, restCfg, pod attributes.
func (r *NfsValidation) init(client k8sclient.Client) error {
	var err error
	r.cluster, err = r.Plan.GetDestinationCluster(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if !r.cluster.Spec.IsHostCluster {
		r.client, err = r.cluster.GetClient(client)
		if err != nil {
			return liberr.Wrap(err)
		}
		r.restCfg, err = r.cluster.BuildRestConfig(client)
		if err != nil {
			return liberr.Wrap(err)
		}
		err = r.findPod()
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Validate PVs.
func (r *NfsValidation) validate() error {
	server := map[string]bool{}
	for i := range r.Plan.Spec.PersistentVolumes.List {
		pv := &r.Plan.Spec.PersistentVolumes.List[i]
		// Skip non-NFS PVs and PVs which data should be copied
		if pv.NFS == nil || pv.Selection.Action == "copy" {
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
					return liberr.Wrap(err)
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
			Message:  "NFS servers [] not accessible on the destination cluster.",
			Items:    notAccessible,
		})
	}

	return nil
}

// Find a pod suitable for command execution.
func (r *NfsValidation) findPod() error {
	podList, err := pods.FindVeleroPods(r.client)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, pod := range podList {
		r.pod = &pod
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
			Message:  "NFS access cannot be validated on the destination cluster.",
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
