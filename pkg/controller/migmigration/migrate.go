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

func (r *ReconcileMigMigration) migrate(migration *migapi.MigMigration) error {
	if migration.IsCompleted() {
		return nil
	}

	// Ready
	plan, err := migration.GetPlan(r)
	if err != nil {
		return err
	}
	if !plan.Status.IsReady() {
		log.Info("Plan not ready.", "name", migration.Name)
		return err
	}

	// Started
	if !migration.Status.MigrationRunning {
		log.Info("Migration started.", "name", migration.Name)
		migration.MarkAsRunning()
		err = r.Update(context.TODO(), migration)
		if err != nil {
			return err
		}
	}

	// Run
	planResources, err := plan.GetRefResources(r, "deprecated")
	if err != nil {
		return err
	}
	task := vrunner.Task{
		Log:           log,
		Client:        r,
		Owner:         migration,
		PlanResources: planResources,
	}
	completed, err := task.Run()
	if err != nil {
		return err
	}

	// Completed
	if completed {
		log.Info("Migration completed.", "name", migration.Name)
		migration.MarkAsCompleted()
		err = r.Update(context.TODO(), migration)
		if err != nil {
			return err
		}
	}

	return nil
}
