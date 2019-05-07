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
	"fmt"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	vrunner "github.com/fusor/mig-controller/pkg/velerorunner"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileMigStage) precheck(migStage *migapi.MigStage) bool {
	// Return false if MigStage is already complete
	if migStage.Status.StageCompleted == true {
		return false
	}
	// Return false if MigStage isn't ready
	if !migStage.Status.IsReady() {
		return false
	}
	// Return true is everything looks ready to run
	return true
}

func (r *ReconcileMigStage) getResources(migStage *migapi.MigStage) (*migapi.PlanRefResources, error) {
	// Build ReconcileResources for MigStage containing data needed for rest of reconcile process
	migPlan, err := migStage.GetPlan(r.Client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET MigPlan referenced by MigStage [%s/%s]",
			logPrefix, migStage.Namespace, migStage.Name))
		return nil, err
	}

	rres, err := migPlan.GetRefResources(r.Client, logPrefix)
	return rres, nil // continue
}

func (r *ReconcileMigStage) startMigStage(migStage *migapi.MigStage, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	// If all references are marked as ready, run MarkAsRunning() to set this Migration into "Running" state
	changed := migStage.MarkAsRunning()
	if changed {
		err := r.Update(context.TODO(), migStage)
		if err != nil {
			log.Info("[%s] Failed to mark MigStage [%s/%s] as running", logPrefix, migStage.Namespace, migStage.Name)
			return nil, err // requeue
		}
		log.Info(fmt.Sprintf("[%s] STARTED MigStage [%s/%s]", logPrefix, migStage.Namespace, migStage.Name))
	}
	return rres, nil // continue
}

func getSrcBackupName(migStage *migapi.MigStage) types.NamespacedName {
	var backupNsName types.NamespacedName
	if migStage.Status.SrcBackupRef == nil {
		backupNsName = types.NamespacedName{
			Name:      migStage.Name + "-",
			Namespace: veleroNs,
		}
	} else {
		backupNsName = types.NamespacedName{
			Name:      migStage.Status.SrcBackupRef.Name,
			Namespace: migStage.Status.SrcBackupRef.Namespace,
		}
	}
	return backupNsName
}

// Create Velero Backup on srcCluster looking at namespaces in MigAssetCollection
func (r *ReconcileMigStage) ensureSrcBackup(migStage *migapi.MigStage, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	// Determine appropriate backupNsName to use in ensureBackup
	backupNsName := getSrcBackupName(migStage)

	// Ensure that a backup exists with backupNsName
	rres, err := vrunner.EnsureBackup(r.Client, backupNsName, rres, true, logPrefix)
	if err != nil {
		return nil, err // requeue
	}

	// Update MigStage with reference to Velero Backup
	migStage.Status.SrcBackupRef = &kapi.ObjectReference{Name: rres.SrcBackup.Name, Namespace: rres.SrcBackup.Namespace}
	err = r.Update(context.TODO(), migStage)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to UPDATE MigStage with Velero Backup reference", logPrefix))
		return nil, err // requeue
	}

	return rres, nil // continue
}

func getDestRestoreName(migStage *migapi.MigStage) types.NamespacedName {
	var restoreNsName types.NamespacedName
	if migStage.Status.DestRestoreRef == nil {
		restoreNsName = types.NamespacedName{
			Name:      migStage.Name + "-",
			Namespace: veleroNs,
		}
	} else {
		restoreNsName = types.NamespacedName{
			Name:      migStage.Status.DestRestoreRef.Name,
			Namespace: migStage.Status.DestRestoreRef.Namespace,
		}
	}
	return restoreNsName
}

// Create Velero Restore on destMigCluster pointing at Velero Backup unique name
func (r *ReconcileMigStage) ensureDestRestore(migStage *migapi.MigStage, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	backupNsName := getSrcBackupName(migStage)
	restoreNsName := getDestRestoreName(migStage)

	rres, err := vrunner.EnsureRestore(r.Client, backupNsName, restoreNsName, rres, logPrefix)
	if err != nil {
		return nil, err // requeue
	}
	if rres == nil {
		return nil, nil // don't requeue
	}

	// Update MigStage with reference to Velero Retore
	migStage.Status.DestRestoreRef = &kapi.ObjectReference{Name: rres.DestRestore.Name, Namespace: rres.DestRestore.Namespace}
	err = r.Update(context.TODO(), migStage)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to UPDATE MigStage with Velero Restore reference", logPrefix))
		return nil, err // requeue
	}

	return rres, nil // continue
}

func (r *ReconcileMigStage) finishMigStage(migStage *migapi.MigStage, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	if rres.DestRestore.Status.Phase == velerov1.RestorePhaseCompleted {
		changed := migStage.MarkAsCompleted()
		if changed {
			err := r.Update(context.TODO(), migStage)
			if err != nil {
				log.Info("[%s] Failed to mark MigStage [%s/%s] as completed", logPrefix, migStage.Namespace, migStage.Name)
				return nil, err // requeue
			}
			log.Info(fmt.Sprintf("[%s] FINISHED MigStage [%s/%s]", logPrefix, migStage.Namespace, migStage.Name))
		}
	}
	return rres, nil // continue
}
