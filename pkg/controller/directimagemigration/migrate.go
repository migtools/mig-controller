/*
Copyright 2020 Red Hat Inc.

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

package directimagemigration

import (
	"context"
	"fmt"
	"time"

	"github.com/konveyor/mig-controller/pkg/errorutil"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *ReconcileDirectImageMigration) migrate(ctx context.Context, imageMigration *migapi.DirectImageMigration) (time.Duration, error) {
	// Started
	if imageMigration.Status.StartTimestamp == nil {
		log.Info("Marking DirectImageMigration as started.")
		imageMigration.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}

	// Run
	task := Task{
		Log:    log,
		Client: r,
		Owner:  imageMigration,
		Phase:  imageMigration.Status.Phase,

		Tracer: r.tracer,
	}
	err := task.Run(ctx)
	if err != nil {
		if errors.IsConflict(errorutil.Unwrap(err)) {
			log.V(4).Info("Conflict error during task.Run, requeueing.")
			return FastReQ, nil
		}
		log.Info("Phase execution failed.",
			"phase", task.Phase,
			"phaseDescription", task.getPhaseDescription(task.Phase),
			"error", errorutil.Unwrap(err).Error())
		log.Trace(err)
		task.fail(MigrationFailed, []string{err.Error()})
		return task.Requeue, nil
	}

	// Result
	imageMigration.Status.Phase = task.Phase
	imageMigration.Status.Itinerary = task.Itinerary.Name

	// Completed
	if task.Phase == Completed {
		imageMigration.Status.DeleteCondition(migapi.Running)
		failed := task.Owner.Status.FindCondition(migapi.Failed)
		if failed == nil {
			imageMigration.Status.SetCondition(migapi.Condition{
				Type:     migapi.Succeeded,
				Status:   migapi.True,
				Reason:   task.Phase,
				Category: migapi.Advisory,
				Message:  "The migration has succeeded.",
				Durable:  true,
			})
		}
		return NoReQ, nil
	}

	// Running
	step, n, total := task.Itinerary.progressReport(task.Phase)
	message := fmt.Sprintf("Step: %d/%d", n, total)
	imageMigration.Status.SetCondition(migapi.Condition{
		Type:     migapi.Running,
		Status:   migapi.True,
		Reason:   step,
		Category: migapi.Advisory,
		Message:  message,
	})

	return task.Requeue, nil
}
