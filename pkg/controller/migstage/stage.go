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
var stageAnnotationKey = "openshift.io/migrate-copy-phase"
var pvAnnotationKey = "openshift.io/migrate-type"
var stageAnnotationValue = "stage"

func (r *ReconcileMigStage) stage(stageMigration *migapi.MigStage) (bool, error) {
	if stageMigration.IsCompleted() {
		return false, nil
	}

	// Ready
	plan, err := stageMigration.GetPlan(r)
	if err != nil {
		return false, err
	}
	if !plan.Status.IsReady() {
		log.Info("Plan not ready.", "name", stageMigration.Name)
		return false, err
	}

	// Started
	if !stageMigration.Status.StageRunning {
		log.Info("Stage started.", "name", stageMigration.Name)
		stageMigration.MarkAsRunning()
		err = r.Update(context.TODO(), stageMigration)
		if err != nil {
			return false, err
		}
	}

	// Build annotations
	annotations := make(map[string]string)
	annotations[stageAnnotationKey] = stageAnnotationValue
	// TODO: Revisit this. We are hardcoding this for now until 2 things occur.
	// 1. We are properly setting this annotation from user input to the UI
	// 2. We fix the plugin to operate migration specific behavior on the
	// migrateAnnnotationKey
	annotations[pvAnnotationKey] = "swing"

	// Run
	planResources, err := plan.GetRefResources(r)
	if err != nil {
		return false, err
	}
	task := vrunner.Task{
		Log:             log,
		Client:          r,
		Owner:           stageMigration,
		PlanResources:   planResources,
		Phase:           stageMigration.Status.TaskPhase,
		BackupResources: stageResources,
		Annotations:     annotations,
	}
	err = task.Run()
	if err != nil {
		return false, err
	}
	stageMigration.Status.TaskPhase = task.Phase
	err = r.Update(context.TODO(), stageMigration)
	if err != nil {
		return false, err
	}
	switch task.Phase {
	case vrunner.Completed:
		log.Info("Stage completed.", "name", stageMigration.Name)
		stageMigration.MarkAsCompleted()
		err = r.Update(context.TODO(), stageMigration)
		if err != nil {
			return false, err
		}
	case vrunner.WaitOnResticRestart:
		return true, nil
	}

	return false, nil
}
