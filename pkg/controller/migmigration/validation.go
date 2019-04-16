package migmigration

import (
	"context"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
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
func (r ReconcileMigMigration) validate(migration *migapi.MigMigration) (int, error) {
	totalSet := 0
	var err error
	nSet := 0

	// Plan
	nSet, err = r.validatePlan(migration)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Ready
	migration.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	err = r.Update(context.TODO(), migration)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

// Validate the referenced plan.
// Returns error and the total error conditions set.
func (r ReconcileMigMigration) validatePlan(migration *migapi.MigMigration) (int, error) {
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
		return 1, nil
	}

	plan, err := migapi.GetPlan(r, ref)
	if err != nil {
		return 0, err
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
		return 1, nil
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
		return 1, nil
	} else {
		migration.Status.DeleteCondition(PlanNotReady)
	}

	return 0, nil
}
