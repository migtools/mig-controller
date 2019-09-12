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

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Backup resources.
var stagingResources = []string{
	"serviceaccount",
	"persistentvolumes",
	"persistentvolumeclaims",
	"namespaces",
	"imagestreams",
	"imagestreamtags",
	"secrets",
	"configmaps",
	"pods",
}
var excludedInitialResources = []string{
	"persistentvolumes",
	"persistentvolumeclaims",
}

// Perform the migration.
func (r *ReconcileMigMigration) migrate(migration *migapi.MigMigration) (time.Duration, error) {
	// Ready
	plan, err := migration.GetPlan(r)
	if err != nil {
		return 0, err
	}
	if !plan.Status.IsReady() {
		log.Info("Plan not ready.", "name", migration.Name)
		return 0, err
	}

	// Resources
	planResources, err := plan.GetRefResources(r)
	if err != nil {
		log.Trace(err)
		return 0, err
	}

	// Started
	if migration.Status.StartTimestamp == nil {
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
		log.Trace(err)
		return NoReQ, err
	}

	// Result
	migration.Status.Phase = task.Phase

	// Completed
	if task.Phase == Completed {
		migration.Status.DeleteCondition(Running)
		failed := task.Owner.Status.FindCondition(Failed)
		if failed == nil {
			migration.Status.SetCondition(migapi.Condition{
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
	migration.Status.SetCondition(migapi.Condition{
		Type:     Running,
		Status:   True,
		Reason:   step,
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
	annotations[ControllerGenerated] = "true"
	if migration.Spec.Stage {
		annotations[StageOrFinalAnnotation] = "stage"
	} else {
		annotations[StageOrFinalAnnotation] = "final"
	}
	return annotations
}

// Get the resources (kinds) to be included in the backup.
func (r *ReconcileMigMigration) getBackupResources(migration *migapi.MigMigration) []string {
	if migration.Spec.Stage {
		return stagingResources
	}

	return []string{}
}
