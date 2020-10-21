package migmigration

import (
	"context"
	"fmt"

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
	UnhealthyNamespaces = "UnhealthyNamespaces"
	InvalidPlanRef      = "InvalidPlanRef"
	PlanNotReady        = "PlanNotReady"
	PlanClosed          = "PlanClosed"
	HasFinalMigration   = "HasFinalMigration"
	Postponed           = "Postponed"
	Running             = "Running"
	Succeeded           = "Succeeded"
	Failed              = "Failed"
	ResticErrors        = "ResticErrors"
	ResticVerifyErrors  = "ResticVerifyErrors"
	StageNoOp           = "StageNoOp"
	RegistriesHealthy   = "RegistriesHealthy"
	RegistriesUnhealthy = "RegistriesUnhealthy"
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

// Messages
const (
	ReadyMessage               = "The migration is ready."
	InvalidPlanRefMessage      = "The `migPlanRef` must reference a `migplan`."
	PlanNotReadyMessage        = "The referenced `migPlanRef` does not have a `Ready` condition."
	PlanClosedMessage          = "The associated migration plan is closed."
	HasFinalMigrationMessage   = "The associated MigPlan already has a final migration."
	PostponedMessage           = "Postponed %d seconds to ensure migrations run serially and in order."
	CanceledMessage            = "The migration has been canceled."
	CancelInProgressMessage    = "The migration is being canceled."
	FailedMessage              = "The migration has failed.  See: Errors."
	SucceededMessage           = "The migration has completed successfully."
	UnhealthyNamespacesMessage = "'%s' cluster has unhealthy namespaces. See status.namespaces for details."
	ResticErrorsMessage        = "There were errors found in %d Restic volume restores. See restore `%s` for details"
	ResticVerifyErrorsMessage  = "There were verify errors found in %d Restic volume restores. See restore `%s` for details"
	StageNoOpMessage           = "Stage migration was run without any PVs or ImageStreams in source cluster. No Velero operations were initiated."
	RegistriesHealthyMessage   = "The migration registries are healthy."
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
			Message:  InvalidPlanRefMessage,
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
			Message:  InvalidPlanRefMessage,
		})
		return plan, nil
	}

	// NotReady
	if !plan.Status.IsReady() {
		migration.Status.SetCondition(migapi.Condition{
			Type:     PlanNotReady,
			Status:   True,
			Category: Critical,
			Message:  PlanNotReadyMessage,
		})
	}

	// Closed
	if plan.Spec.Closed {
		migration.Status.SetCondition(migapi.Condition{
			Type:     PlanClosed,
			Status:   True,
			Category: Critical,
			Message:  PlanClosedMessage,
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
	hasCondition := false
	for _, m := range migrations {
		// ignore canceled
		if m.UID == migration.UID || m.Spec.Stage || m.Spec.Canceled {
			continue
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
			Message:  HasFinalMigrationMessage,
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
		Message:  RegistriesHealthyMessage,
		Durable:  true,
	})
}

// Validate that migration registries on both source and dest clusters are healthy
func ensureRegistryHealth(c k8sclient.Client, migration *migapi.MigMigration) (int, string, error) {

	nEnsured := 0
	unHealthyPod := corev1.Pod{}
	unHealthyClusterName := ""

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

		registryStatusUnhealthy, podObj := isRegistryPodUnHealthy(registryPods)

		if !registryStatusUnhealthy {
			nEnsured++
		} else {
			unHealthyPod = podObj
			unHealthyClusterName = cluster.ObjectMeta.Name
		}
	}

	if nEnsured != 2 {
		message := fmt.Sprintf("Migration Registry Pod %s/%s is in unhealthy state on cluster %s",
			unHealthyPod.Namespace, unHealthyPod.Name, unHealthyClusterName)
		return nEnsured, message, nil
	}

	return nEnsured, "", nil
}

func isRegistryPodUnHealthy(registryPods corev1.PodList) (bool, corev1.Pod) {
	unHealthyPod := corev1.Pod{}
	for _, registryPod := range registryPods.Items {
		if !registryPod.Status.ContainerStatuses[0].Ready {
			return true, registryPod
		}
	}
	return false, unHealthyPod
}

func getRegistryPods(plan *migapi.MigPlan, registryClient compat.Client) (corev1.PodList, error) {

	registryPodList := corev1.PodList{}
	err := registryClient.List(context.TODO(), &k8sclient.ListOptions{
		LabelSelector: k8sLabels.SelectorFromSet(map[string]string{
			"migplan": string(plan.UID),
		}),
	}, &registryPodList)

	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	return registryPodList, nil
}
