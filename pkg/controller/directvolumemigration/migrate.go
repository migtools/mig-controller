package directvolumemigration

import (
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/mig-controller/pkg/errorutil"
	"time"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *ReconcileDirectVolumeMigration) migrate(direct *migapi.DirectVolumeMigration) (time.Duration, error) {

	migration, planResources, err := r.getDVMMigrationAndPlanResources(direct)
	if err != nil {
		return 0, liberr.Wrap(err)
	}

	// Started
	if direct.Status.StartTimestamp == nil {
		direct.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}

	// Run
	task := Task{
		Log:              log,
		Client:           r,
		Owner:            direct,
		Phase:            direct.Status.Phase,
		PhaseDescription: direct.Status.PhaseDescription,
		PlanResources:    planResources,
		MigrationUID:     string(migration.UID),
	}
	err = task.Run()
	if err != nil {
		if errors.IsConflict(errorutil.Unwrap(err)) {
			return FastReQ, nil
		}
		log.Trace(err)
		task.fail(MigrationFailed, []string{err.Error()})
		return task.Requeue, nil
	}

	// Result
	direct.Status.PhaseDescription = task.PhaseDescription
	direct.Status.Phase = task.Phase
	direct.Status.Itinerary = task.Itinerary.Name

	// Completed
	if task.Phase == Completed {
		direct.Status.DeleteCondition(Running)
		failed := task.Owner.Status.FindCondition(Failed)
		if failed == nil {
			direct.Status.SetCondition(migapi.Condition{
				Type:     Succeeded,
				Status:   True,
				Reason:   task.Phase,
				Category: Advisory,
				Message:  SucceededMessage,
				Durable:  true,
			})
		}
		return NoReQ, nil
	}

	// Running
	step, n, total := task.Itinerary.progressReport(task.Phase)
	message := fmt.Sprintf(RunningMessage, n, total)
	direct.Status.SetCondition(migapi.Condition{
		Type:     Running,
		Status:   True,
		Reason:   step,
		Category: Advisory,
		Message:  message,
	})

	return task.Requeue, nil
}

// fetches DVM Migration object and Migplan resources if DVM has an owner reference
func (r *ReconcileDirectVolumeMigration) getDVMMigrationAndPlanResources(direct *migapi.DirectVolumeMigration) (*migapi.MigMigration, *migapi.PlanResources, error) {

	if len(direct.OwnerReferences) > 0 {

		migration := &migapi.MigMigration{}
		planResources := &migapi.PlanResources{}

		// Ready
		migration, err := direct.GetMigrationForDVM(r)
		if err != nil {
			return migration, planResources, liberr.Wrap(err)
		}

		if migration == nil {
			log.Info("Migration not found for DVM", "name", direct.Name)
			return migration, planResources, liberr.Wrap(err)
		}

		plan, err := migration.GetPlan(r)
		if err != nil {
			return migration, planResources, liberr.Wrap(err)
		}
		if !plan.Status.IsReady() {
			log.Info("Plan not ready.", "name", migration.Name)
			return migration, planResources, liberr.Wrap(err)
		}

		// Resources
		planResources, err = plan.GetRefResources(r)
		if err != nil {
			return migration, planResources, liberr.Wrap(err)
		}
		return migration, planResources, nil
	}
	return &migapi.MigMigration{}, &migapi.PlanResources{}, nil
}
