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
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	vrunner "github.com/fusor/mig-controller/pkg/velerorunner"
)

var migrateAnnotationKey = "openshift.io/migrate-copy-phase"
var migrateAnnotationValue = "final"

func (r *ReconcileMigMigration) migrate(migration *migapi.MigMigration) (bool, error) {
	if migration.IsCompleted() {
		return false, nil
	}

	// Ready
	plan, err := migration.GetPlan(r)
	if err != nil {
		return false, err
	}
	if !plan.Status.IsReady() {
		log.Info("Plan not ready.", "name", migration.Name)
		return false, err
	}

	// Started
	if !migration.Status.MigrationRunning {
		log.Info("Migration started.", "name", migration.Name)
		migration.MarkAsRunning()
		err = r.Update(context.TODO(), migration)
		if err != nil {
			return false, err
		}
	}

	// Build annotations
	annotations := make(map[string]string)
	annotations[migrateAnnotationKey] = migrateAnnotationValue

	// Run
	planResources, err := plan.GetRefResources(r)
	if err != nil {
		return false, err
	}
	task := vrunner.Task{
		Log:           log,
		Client:        r,
		Owner:         migration,
		PlanResources: planResources,
		Phase:         migration.Status.TaskPhase,
		Annotations:   annotations,
	}
	err = task.Run()
	if err != nil {
		return false, err
	}
	migration.Status.TaskPhase = task.Phase
	err = r.Update(context.TODO(), migration)
	if err != nil {
		return false, err
	}
	switch task.Phase {
	case vrunner.Completed:
		log.Info("Migration completed.", "name", migration.Name)
		migration.MarkAsCompleted()
		err = r.Update(context.TODO(), migration)
		if err != nil {
			return false, err
		}
	case vrunner.WaitOnResticRestart:
		return true, nil
	}

	return false, nil
}
