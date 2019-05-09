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

package migstage

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	vrunner "github.com/fusor/mig-controller/pkg/velerorunner"
)

var stageResources = []string{"pods", "persistentvolumes", "persistentvolumeclaims", "imagestreams", "imagestreamtags"}

func (r *ReconcileMigStage) stage(stageMigration *migapi.MigStage) error {
	if stageMigration.IsCompleted() {
		return nil
	}

	// Ready
	plan, err := stageMigration.GetPlan(r)
	if err != nil {
		return err
	}
	if !plan.Status.IsReady() {
		log.Info("Plan not ready.", "name", stageMigration.Name)
		return err
	}

	// Started
	if !stageMigration.Status.StageRunning {
		log.Info("Stage started.", "name", stageMigration.Name)
		stageMigration.MarkAsRunning()
		err = r.Update(context.TODO(), stageMigration)
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
		Log:             log,
		Client:          r,
		Owner:           stageMigration,
		PlanResources:   planResources,
		BackupResources: stageResources,
	}
	completed, err := task.Run()
	if err != nil {
		return err
	}

	// Completed
	if completed {
		log.Info("Stage completed.", "name", stageMigration.Name)
		stageMigration.MarkAsCompleted()
		err = r.Update(context.TODO(), stageMigration)
		if err != nil {
			return err
		}
	}

	return nil
}
