package migmigration

import (
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
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
	RunningMessage             = "Step: %d/%d"
	FailedMessage              = "The migration has failed.  See: Errors."
	SucceededMessage           = "The migration has completed successfully."
	UnhealthyNamespacesMessage = "'%s' cluster has unhealthy namespaces. See status.namespaces for details."
	ResticErrorsMessage        = "There were errors found in %d Restic volume restores. See restore `%s` for details"
	ResticVerifyErrorsMessage  = "There were verify errors found in %d Restic volume restores. See restore `%s` for details"
	StageNoOpMessage           = "Stage migration was run without any PVs or ImageStreams in source cluster. No Velero operations were initiated."
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
	if hasCondition {
		migration.Status.SetCondition(migapi.Condition{
			Type:     HasFinalMigration,
			Status:   True,
			Category: Critical,
			Message:  HasFinalMigrationMessage,
		})
	}

	return nil
}
