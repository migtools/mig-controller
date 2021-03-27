package migmigration

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	migevent "github.com/konveyor/mig-controller/pkg/event"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	UnhealthyNamespaces                = "UnhealthyNamespaces"
	InvalidPlanRef                     = "InvalidPlanRef"
	PlanNotReady                       = "PlanNotReady"
	PlanClosed                         = "PlanClosed"
	HasFinalMigration                  = "HasFinalMigration"
	Postponed                          = "Postponed"
	SucceededWithWarnings              = "SucceededWithWarnings"
	ResticErrors                       = "ResticErrors"
	ResticVerifyErrors                 = "ResticVerifyErrors"
	VeleroInitialBackupPartiallyFailed = "VeleroInitialBackupPartiallyFailed"
	VeleroStageBackupPartiallyFailed   = "VeleroStageBackupPartiallyFailed"
	VeleroStageRestorePartiallyFailed  = "VeleroStageRestorePartiallyFailed"
	VeleroFinalRestorePartiallyFailed  = "VeleroFinalRestorePartiallyFailed"
	DirectImageMigrationFailed         = "DirectImageMigrationFailed"
	StageNoOp                          = "StageNoOp"
	RegistriesHealthy                  = "RegistriesHealthy"
	RegistriesUnhealthy                = "RegistriesUnhealthy"
	StaleSrcVeleroCRsDeleted           = "StaleSrcVeleroCRsDeleted"
	StaleDestVeleroCRsDeleted          = "StaleDestVeleroCRsDeleted"
	StaleResticCRsDeleted              = "StaleResticCRsDeleted"
	DirectVolumeMigrationBlocked       = "DirectVolumeMigrationBlocked"
)

// Categories
const (
	Critical = migapi.Critical
	Advisory = migapi.Advisory
)

// Reasons
const (
	NotSet         = "NotSet"
	NotFound       = "NotFound"
	Cancel         = "Cancel"
	ErrorsDetected = "ErrorsDetected"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Validate the plan resource.
// Returns error and the total error conditions set.
func (r ReconcileMigMigration) validate(ctx context.Context, migration *migapi.MigMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		var span opentracing.Span
		span, ctx = opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validate")
		defer span.Finish()
	}

	// Plan
	plan, err := r.validatePlan(ctx, migration)
	if err != nil {
		log.V(4).Error(err, "Validation check for attached plan failed")
		err = liberr.Wrap(err)
	}

	// Final migration.
	err = r.validateFinalMigration(ctx, plan, migration)
	if err != nil {
		log.V(4).Error(err, "Validation check for existing final migration failed")
		err = liberr.Wrap(err)
	}

	// Validate registries running.
	err = r.validateRegistriesRunning(ctx, migration)
	if err != nil {
		log.V(4).Error(err, "Validation of running registries failed")
		err = liberr.Wrap(err)
	}

	return nil
}

// Validate the referenced plan.
func (r ReconcileMigMigration) validatePlan(ctx context.Context, migration *migapi.MigMigration) (*migapi.MigPlan, error) {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validatePlan")
		defer span.Finish()
	}

	log.V(4).Info("Validating plan spec references")
	ref := migration.Spec.MigPlanRef

	// NotSet
	if !migref.RefSet(ref) {
		migration.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  fmt.Sprintf("The `migPlanRef` must reference a valid `migplan`."),
		})
		log.V(4).Info("The `migPlanRef` does not reference a valid `migplan`.")
		return nil, nil
	}

	plan, err := migapi.GetPlan(r, ref)
	if err != nil {
		return nil, err
	}

	// NotFound
	if plan == nil {
		migration.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message: fmt.Sprintf("The `migPlanRef` must reference a valid `migplan`, subject: %s.",
				path.Join(migration.Spec.MigPlanRef.Namespace, migration.Spec.MigPlanRef.Name)),
		})
		log.V(4).Info("The `migPlanRef` does not reference a valid `migplan`.")
		return plan, nil
	}

	// NotReady
	if !plan.Status.IsReady() {
		migration.Status.SetCondition(migapi.Condition{
			Type:     PlanNotReady,
			Status:   True,
			Category: Critical,
			Message: fmt.Sprintf("The referenced `migPlanRef` does not have a `Ready` condition, subject: %s.",
				path.Join(migration.Spec.MigPlanRef.Namespace, migration.Spec.MigPlanRef.Name)),
		})
		log.V(4).Info("The referenced `migPlanRef` does not have a `Ready` condition")
	}

	// Closed
	if plan.Spec.Closed {
		migration.Status.SetCondition(migapi.Condition{
			Type:     PlanClosed,
			Status:   True,
			Category: Critical,
			Message: fmt.Sprintf("The associated migration plan is closed, subject: %s.",
				path.Join(migration.Spec.MigPlanRef.Namespace, migration.Spec.MigPlanRef.Name)),
		})
		log.V(4).Info("The associated migration plan is closed")
	}

	return plan, nil
}

// Validate (other) final migrations associated with the plan.
// An error condition is added when:
//   When validating `stage` migrations:
//     A final migration has started or has completed.
//   When validating `final` migrations:
//     A final migratoin has successfully completed.
func (r ReconcileMigMigration) validateFinalMigration(ctx context.Context, plan *migapi.MigPlan,
	migration *migapi.MigMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateFinalMigration")
		defer span.Finish()
	}
	if plan == nil {
		return nil
	}
	migrations, err := plan.ListMigrations(r)
	if err != nil {
		return nil
	}

	// Sort migrations by timestamp, newest first.
	sort.Slice(migrations, func(i, j int) bool {
		ts1 := migrations[i].CreationTimestamp
		ts2 := migrations[j].CreationTimestamp
		return ts1.Time.After(ts2.Time)
	})

	hasCondition := false
	for _, m := range migrations {
		// Ignore self, stage migrations, canceled migrations
		if m.UID == migration.UID || m.Spec.Stage || m.Spec.Canceled {
			continue
		}

		// If newest migration is a successful rollback, allow further migrations
		if m.Spec.Rollback && m.Status.HasCondition(migapi.Succeeded) {
			hasCondition = false
			break
		}

		// Stage
		if migration.Spec.Stage {
			if m.Status.HasAnyCondition(migapi.Running, migapi.Succeeded, migapi.Failed) {
				hasCondition = true
				break
			}
		}
		// Final
		if !migration.Spec.Stage {
			if m.Status.HasCondition(migapi.Succeeded) {
				hasCondition = true
				break
			}
		}
	}

	// Allow perform Rollback on finished Plan.
	if hasCondition && !migration.Spec.Rollback {
		log.V(4).Info("The associated MigPlan already has a final migration")
		migration.Status.SetCondition(migapi.Condition{
			Type:     HasFinalMigration,
			Status:   True,
			Category: Critical,
			Message: fmt.Sprintf("The associated MigPlan already has a final migration, subject: %s.",
				path.Join(migration.Spec.MigPlanRef.Namespace, migration.Spec.MigPlanRef.Name)),
		})
	}

	return nil
}

func (r ReconcileMigMigration) validateRegistriesRunning(ctx context.Context, migration *migapi.MigMigration) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "validateRegistriesRunning")
		defer span.Finish()
	}

	// Run validation for registry health if registries have been created or are unhealthy.
	// Validation starts running after phase 'WaitForRegistriesReady' sets 'RegistriesHealthy'.
	if migration.Status.HasAnyCondition(RegistriesHealthy, RegistriesUnhealthy) {
		nEnsured, message, err := ensureRegistryHealth(r.Client, migration)
		if err != nil {
			return liberr.Wrap(err)
		}
		if nEnsured != 2 {
			log.Info(fmt.Sprintf("Found %v/2 registries in healthy condition. Registries are unhealthy.", nEnsured), "message", message)
			migration.Status.DeleteCondition(RegistriesHealthy)
			migration.Status.SetCondition(migapi.Condition{
				Type:     RegistriesUnhealthy,
				Status:   True,
				Category: migapi.Critical,
				Message:  message,
				Durable:  true,
			})
		} else if nEnsured == 2 {
			log.Info("Found 2/2 registries in healthy condition.", "message", message)
			migration.Status.DeleteCondition(RegistriesUnhealthy)
			setMigRegistryHealthyCondition(migration)
		}
	}
	return nil
}

func setMigRegistryHealthyCondition(migration *migapi.MigMigration) {
	migration.Status.SetCondition(migapi.Condition{
		Type:     RegistriesHealthy,
		Status:   True,
		Category: migapi.Required,
		Message:  "The migration registries are healthy.",
		Durable:  true,
	})
}

// Validate that migration registries on both source and dest clusters are healthy
func ensureRegistryHealth(c k8sclient.Client, migration *migapi.MigMigration) (int, string, error) {
	log.Info("Checking registry health")

	nEnsured := 0
	unHealthyPod := corev1.Pod{}
	unHealthyClusterName := ""
	reason := ""

	plan, err := migration.GetPlan(c)
	if err != nil {
		return 0, "", liberr.Wrap(err)
	}
	srcCluster, err := plan.GetSourceCluster(c)
	if err != nil {
		return 0, "", liberr.Wrap(err)
	}
	destCluster, err := plan.GetDestinationCluster(c)
	if err != nil {
		return 0, "", liberr.Wrap(err)
	}

	clusters := []*migapi.MigCluster{srcCluster, destCluster}
	for _, cluster := range clusters {

		if !cluster.Status.IsReady() {
			continue
		}

		client, err := cluster.GetClient(c)
		if err != nil {
			return nEnsured, "", liberr.Wrap(err)
		}

		registryPods, err := getRegistryPods(plan, client)
		if err != nil {
			log.Trace(err)
			return nEnsured, "", liberr.Wrap(err)
		}

		for _, pod := range registryPods.Items {
			// Logs abnormal events for Registry Pods if any are found
			migevent.LogAbnormalEventsForResource(
				client, log,
				"Found abnormal event for Registry Pod",
				types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name},
				pod.UID, "Pod")
		}

		registryPodCount := len(registryPods.Items)
		if registryPodCount < 1 {
			unHealthyClusterName = cluster.ObjectMeta.Name
			message := fmt.Sprintf("Migration Registry Pod is missing from cluster %s", unHealthyClusterName)
			return nEnsured, message, nil
		}

		registryStatusUnhealthy, podObj, state := isRegistryPodUnHealthy(registryPods)
		if !registryStatusUnhealthy {
			nEnsured++
		} else {
			unHealthyPod = podObj
			unHealthyClusterName = cluster.ObjectMeta.Name
			reason = state
		}
	}

	if nEnsured != 2 {
		message := fmt.Sprintf("Migration Registry Pod %s/%s is in unhealthy state on cluster %s, the Pod is in %s state",
			unHealthyPod.Namespace, unHealthyPod.Name, unHealthyClusterName, reason)
		if reason == "ImagePullBackOff" {
			return nEnsured, message, errors.New(reason)
		}
		return nEnsured, message, nil
	}

	return nEnsured, "", nil
}

// Checking the health of registry pod, return health status, pod and container status of the pod.
func isRegistryPodUnHealthy(registryPods corev1.PodList) (bool, corev1.Pod, string) {
	unHealthyPod := corev1.Pod{}
	for _, registryPod := range registryPods.Items {
		for _, containerStatus := range registryPod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				if containerStatus.State.Waiting != nil {
					return true, registryPod, containerStatus.State.Waiting.Reason
				} else if containerStatus.State.Terminated != nil {
					return true, registryPod, "Terminating"
				} else {
					return true, registryPod, "Running"
				}
			}
		}
	}
	return false, unHealthyPod, "Ready"
}

func getRegistryPods(plan *migapi.MigPlan, registryClient compat.Client) (corev1.PodList, error) {

	registryPodList := corev1.PodList{}
	err := registryClient.List(
		context.TODO(),
		&registryPodList,
		k8sclient.MatchingLabels(map[string]string{
			migapi.MigrationRegistryLabel: True,
			"migplan":                     string(plan.UID),
		}),
	)

	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	return registryPodList, nil
}
