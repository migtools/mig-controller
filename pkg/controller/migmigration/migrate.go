/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package migmigration

import (
	"fmt"
	"time"

	"github.com/konveyor/mig-controller/pkg/errorutil"

	mapset "github.com/deckarep/golang-set"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Perform the migration.
func (r *ReconcileMigMigration) migrate(migration *migapi.MigMigration) (time.Duration, error) {
	// Ready
	plan, err := migration.GetPlan(r)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	if !plan.Status.IsReady() {
		log.Info("Plan not ready. Migration can't run unless Plan is ready.", "name", migration.Name)
		return 0, liberr.Wrap(err)
	}

	// Resources
	planResources, err := plan.GetRefResources(r)
	if err != nil {
		return 0, liberr.Wrap(err)
	}

	// Started
	if migration.Status.StartTimestamp == nil {
		log.Info("Marking migration as started.")
		migration.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}

	// Run
	task := Task{
		Log:             log,
		Client:          r,
		Owner:           migration,
		PlanResources:   planResources,
		Phase:           migration.Status.Phase,
		Annotations:     r.getAnnotations(migration),
		BackupResources: r.getBackupResources(migration),
	}
	err = task.Run()
	if err != nil {
		if errors.IsConflict(errorutil.Unwrap(err)) {
			log.V(4).Info("Conflict error during task.Run, requeueing.")
			return FastReQ, nil
		}
		log.Trace(err)
		task.fail(MigrationFailed, []string{err.Error()})
		return task.Requeue, nil
	}

	// Result
	migration.Status.Phase = task.Phase
	migration.Status.Itinerary = task.Itinerary.Name

	// Completed
	if task.Phase == Completed {
		migration.Status.DeleteCondition(Running)
		failed := task.Owner.Status.FindCondition(Failed)
		warnings := task.Owner.Status.FindConditionByCategory(migapi.Warn)
		if failed == nil && len(warnings) == 0 {
			migration.Status.SetCondition(migapi.Condition{
				Type:     Succeeded,
				Status:   True,
				Reason:   task.Phase,
				Category: Advisory,
				Message:  "The migration has completed successfully.",
				Durable:  true,
			})
		}
		if failed == nil && len(warnings) > 0 {
			migration.Status.SetCondition(migapi.Condition{
				Type:     SucceededWithWarnings,
				Status:   True,
				Reason:   task.Phase,
				Category: Advisory,
				Message:  "The migration has completed with warnings, please look at `Warn` conditions.",
				Durable:  true,
			})
		}
		return NoReQ, nil
	}

	phase, n, total := task.Itinerary.progressReport(task.Phase)
	message := fmt.Sprintf("Step: %d/%d", n, total)
	migration.Status.SetCondition(migapi.Condition{
		Type:     Running,
		Status:   True,
		Reason:   phase,
		Category: Advisory,
		Message:  message,
	})

	return task.Requeue, nil
}

// Get annotations.
// TODO: Revisit this. We are hardcoding this for now until 2 things occur.
// 1. We are properly setting this annotation from user input to the UI
// 2. We fix the plugin to operate migration specific behavior on the
// migrateAnnnotationKey
func (r *ReconcileMigMigration) getAnnotations(migration *migapi.MigMigration) map[string]string {
	annotations := make(map[string]string)
	if migration.Spec.Stage {
		annotations[StageOrFinalMigrationAnnotation] = StageMigration
	} else {
		annotations[StageOrFinalMigrationAnnotation] = FinalMigration
	}
	return annotations
}

// Get the resources (kinds) to be included in the backup.
func (r *ReconcileMigMigration) getBackupResources(migration *migapi.MigMigration) mapset.Set {
	if migration.Spec.Stage {
		return settings.IncludedStageResources
	}
	return settings.ExcludedStageResources
}
