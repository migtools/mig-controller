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
	migref "github.com/konveyor/mig-controller/pkg/reference"
	corev1 "k8s.io/api/core/v1"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
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
	Running                            = "Running"
	Succeeded                          = "Succeeded"
	SucceededWithWarnings              = "SucceededWithWarnings"
	Failed                             = "Failed"
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
func (r ReconcileMigMigration) validate(migration *migapi.MigMigration) error {
	// Plan
	plan, err := r.validatePlan(migration)
	if err != nil {
		err = liberr.Wrap(err)
	}

	// Final migration.
	err = r.validateFinalMigration(plan, migration)
	if err != nil {
		err = liberr.Wrap(err)
	}

	// Validate registries running.
	err = r.validateRegistriesRunning(migration)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return nil
}

// Validate the referenced plan.
func (r ReconcileMigMigration) validatePlan(migration *migapi.MigMigration) (*migapi.MigPlan, error) {
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
	}

	return plan, nil
}

// Validate (other) final migrations associated with the plan.
// An error condition is added when:
//   When validating `stage` migrations:
//     A final migration has started or has completed.
//   When validating `final` migrations:
//     A final migratoin has successfully completed.
func (r ReconcileMigMigration) validateFinalMigration(plan *migapi.MigPlan, migration *migapi.MigMigration) error {
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
		if m.Spec.Rollback && m.Status.HasCondition(Succeeded) {
			hasCondition = false
			break
		}

		// Stage
		if migration.Spec.Stage {
			if m.Status.HasAnyCondition(Running, Succeeded, Failed) {
				hasCondition = true
				break
			}
		}
		// Final
		if !migration.Spec.Stage {
			if m.Status.HasCondition(Succeeded) {
				hasCondition = true
				break
			}
		}
	}
	// Allow perform Rollback on finished Plan.
	if hasCondition && !migration.Spec.Rollback {
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

func (r ReconcileMigMigration) validateRegistriesRunning(migration *migapi.MigMigration) error {
	// Run validation for registry health if registries have been created or are unhealthy.
	// Validation starts running after phase 'WaitForRegistriesReady' sets 'RegistriesHealthy'.
	if migration.Status.HasAnyCondition(RegistriesHealthy, RegistriesUnhealthy) {
		nEnsured, message, err := ensureRegistryHealth(r.Client, migration)
		if err != nil {
			return liberr.Wrap(err)
		}
		if nEnsured != 2 {
			migration.Status.DeleteCondition(RegistriesHealthy)
			migration.Status.SetCondition(migapi.Condition{
				Type:     RegistriesUnhealthy,
				Status:   True,
				Category: migapi.Critical,
				Message:  message,
				Durable:  true,
			})
		} else if nEnsured == 2 {
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

func isRegistryPodUnHealthy(registryPods corev1.PodList) (bool, corev1.Pod, string) {
	unHealthyPod := corev1.Pod{}
	for _, registryPod := range registryPods.Items {
		for _, containerStatus := range registryPod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				return true, registryPod, containerStatus.State.Waiting.Reason
			}
		}
	}
	return false, unHealthyPod, "Ready"
}

func getRegistryPods(plan *migapi.MigPlan, registryClient compat.Client) (corev1.PodList, error) {

	registryPodList := corev1.PodList{}
	err := registryClient.List(context.TODO(), &k8sclient.ListOptions{
		LabelSelector: k8sLabels.SelectorFromSet(map[string]string{
			migapi.MigrationRegistryLabel: True,
			"migplan":                     string(plan.UID),
		}),
	}, &registryPodList)

	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	return registryPodList, nil
}
