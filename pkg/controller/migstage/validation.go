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

// Categories
const (
	Critical = migapi.Critical
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
func (r ReconcileMigStage) validate(stage *migapi.MigStage) error {
	stage.Status.BeginStagingConditions()

	// Plan
	err := r.validatePlan(stage)
	if err != nil {
		return err
	}

	// Ready
	stage.Status.SetReady(
		!stage.Status.HasBlockerCondition(),
		ReadyMessage)

	// Apply changes.
	stage.Status.EndStagingConditions()
	err = r.Update(context.TODO(), stage)
	if err != nil {
		return err
	}

	return nil
}

// Validate the referenced plan.
func (r ReconcileMigStage) validatePlan(stage *migapi.MigStage) error {
	ref := stage.Spec.MigPlanRef

	// NotSet
	if !migref.RefSet(ref) {
		stage.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotSet,
			Category: Critical,
			Message:  InvalidPlanRefMessage,
		})
		return nil
	}

	plan, err := migapi.GetPlan(r, ref)
	if err != nil {
		return err
	}

	// NotFound
	if plan == nil {
		stage.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotFound,
			Category: Critical,
			Message:  InvalidPlanRefMessage,
		})
		return nil
	}

	// NotReady
	if !plan.Status.IsReady() {
		stage.Status.SetCondition(migapi.Condition{
			Type:     PlanNotReady,
			Status:   True,
			Category: Critical,
			Message:  PlanNotReadyMessage,
		})
		return nil
	}

	return nil
}
