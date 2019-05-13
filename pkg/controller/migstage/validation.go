package migstage

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
func (r ReconcileMigStage) validate(stage *migapi.MigStage) (int, error) {
	totalSet := 0
	var err error
	nSet := 0

	// Plan
	nSet, err = r.validatePlan(stage)
	if err != nil {
		return 0, err
	}
	totalSet += nSet

	// Ready
	stage.Status.SetReady(totalSet == 0, ReadyMessage)

	// Apply changes.
	stage.Status.DeleteUnstagedConditions()
	err = r.Update(context.TODO(), stage)
	if err != nil {
		return 0, err
	}

	return totalSet, err
}

// Validate the referenced plan.
// Returns error and the total error conditions set.
func (r ReconcileMigStage) validatePlan(stage *migapi.MigStage) (int, error) {
	ref := stage.Spec.MigPlanRef

	// NotSet
	if !migref.RefSet(ref) {
		stage.Status.SetCondition(migapi.Condition{
			Type:    InvalidPlanRef,
			Status:  True,
			Reason:  NotSet,
			Message: InvalidPlanRefMessage,
		})
		return 1, nil
	}

	plan, err := migapi.GetPlan(r, ref)
	if err != nil {
		return 0, err
	}

	// NotFound
	if plan == nil {
		stage.Status.SetCondition(migapi.Condition{
			Type:    InvalidPlanRef,
			Status:  True,
			Reason:  NotFound,
			Message: InvalidPlanRefMessage,
		})
		return 1, nil
	}

	// NotReady
	if !plan.Status.IsReady() {
		stage.Status.SetCondition(migapi.Condition{
			Type:    PlanNotReady,
			Status:  True,
			Message: PlanNotReadyMessage,
		})
		return 1, nil
	}

	return 0, nil
}
