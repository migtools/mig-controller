package migmigration

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
	InvalidPlanRef = "InvalidPlanRef"
	PlanNotReady   = "PlanNotReady"
)

// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Statuses
const (
	True  = migapi.True
	False = migapi.False
)

// Messages
const (
	ReadyMessage          = "The migration is ready."
	InvalidPlanRefMessage = "The `migPlanRef` must reference a `migplan`."
	PlanNotReadyMessage   = "The referenced `migPlanRef` does not have a `Ready` condition."
)

// Validate the plan resource.
// Returns error and the total error conditions set.
func (r ReconcileMigMigration) validate(migration *migapi.MigMigration) (error, int) {
	totalSet := 0
	var err error
	nSet := 0

	// Plan
	err, nSet = r.validatePlan(migration)
	if err != nil {
		return err, 0
	}
	totalSet += nSet

	// Ready
	migration.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	err = r.Update(context.TODO(), migration)
	if err != nil {
		return err, 0
	}

	return err, totalSet
}

// Validate the referenced plan.
// Returns error and the total error conditions set.
func (r ReconcileMigMigration) validatePlan(migration *migapi.MigMigration) (error, int) {
	ref := migration.Spec.MigPlanRef

	// NotSet
	if !migref.RefSet(ref) {
		migration.Status.SetCondition(migapi.Condition{
			Type:    InvalidPlanRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidPlanRefMessage,
		})
		migration.Status.DeleteCondition(PlanNotReady)
		return nil, 1
	}

	err, plan := r.getPlan(ref)
	if err != nil {
		return err, 0
	}

	// NotFound
	if plan == nil {
		migration.Status.SetCondition(migapi.Condition{
			Type:    InvalidPlanRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidPlanRefMessage,
		})
		migration.Status.DeleteCondition(PlanNotReady)
		return nil, 1
	} else {
		migration.Status.DeleteCondition(InvalidPlanRef)
	}

	// NotReady
	if !plan.Status.IsReady() {
		migration.Status.SetCondition(migapi.Condition{
			Type:    PlanNotReady,
			Status:  True,
			Message: PlanNotReadyMessage,
		})
		return nil, 1
	} else {
		migration.Status.DeleteCondition(PlanNotReady)
	}

	return nil, 0
}

func (r ReconcileMigMigration) getPlan(ref *kapi.ObjectReference) (error, *migapi.MigPlan) {
	key := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}

	plan := migapi.MigPlan{}
	err := r.Get(context.TODO(), key, &plan)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		} else {
			return err, nil
		}
	}

	return nil, &plan
}
