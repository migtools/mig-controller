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
func (r ReconcileMigMigration) validate(migration *migapi.MigMigration) error {
	migration.Status.BeginStagingConditions()

	// Plan
	err := r.validatePlan(migration)
	if err != nil {
		return err
	}

	// Ready
	migration.Status.SetReady(
		!migration.Status.HasBlockerCondition(),
		ReadyMessage)

	// Apply changes.
	migration.Status.EndStagingConditions()
	err = r.Update(context.TODO(), migration)
	if err != nil {
		return err
	}

	return nil
}

// Validate the referenced plan.
// Returns error and the total error conditions set.
func (r ReconcileMigMigration) validatePlan(migration *migapi.MigMigration) error {
	ref := migration.Spec.MigPlanRef

	// NotSet
	if !migref.RefSet(ref) {
		migration.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotSet,
			Category: migapi.Error,
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
		migration.Status.SetCondition(migapi.Condition{
			Type:     InvalidPlanRef,
			Status:   True,
			Reason:   NotFound,
			Category: migapi.Error,
			Message:  InvalidPlanRefMessage,
		})
		return nil
	}

	// NotReady
	if !plan.Status.IsReady() {
		migration.Status.SetCondition(migapi.Condition{
			Type:     PlanNotReady,
			Status:   True,
			Category: migapi.Error,
			Message:  PlanNotReadyMessage,
		})
		return nil
	}

	return nil
}
