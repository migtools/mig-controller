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
	"fmt"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	vrunner "github.com/fusor/mig-controller/pkg/velerorunner"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileMigMigration) precheck(migMigration *migapi.MigMigration) bool {
	// Return false if MigMigration is already complete
	if migMigration.Status.MigrationCompleted == true {
		return false
	}
	// Return false if MigMigration isn't ready
	if !migMigration.Status.IsReady() {
		return false
	}
	// Return true is everything looks ready to run
	return true
}

func (r *ReconcileMigMigration) getResources(migMigration *migapi.MigMigration) (*migapi.PlanRefResources, error) {
	// Build ReconcileResources for MigMigration containing data needed for rest of reconcile process
	migPlan, err := migMigration.GetPlan(r.Client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET MigPlan referenced by MigMigration [%s/%s]",
			logPrefix, migMigration.Namespace, migMigration.Name))
		return nil, err
	}

	rres, err := migPlan.GetRefResources(r.Client, logPrefix)
	return rres, nil // continue
}

func (r *ReconcileMigMigration) startMigMigration(migMigration *migapi.MigMigration, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	// If all references are marked as ready, run MarkAsRunning() to set this Migration into "Running" state
	changed := migMigration.MarkAsRunning()
	if changed {
		err := r.Update(context.TODO(), migMigration)
		if err != nil {
			log.Info("[%s] Failed to mark MigMigration [%s/%s] as running", logPrefix, migMigration.Namespace, migMigration.Name)
			return nil, err // requeue
		}
		log.Info(fmt.Sprintf("[%s] STARTED MigMigration [%s/%s]", logPrefix, migMigration.Namespace, migMigration.Name))
	}
	return rres, nil // continue
}

func getSrcBackupName(migMigration *migapi.MigMigration) types.NamespacedName {
	var backupNsName types.NamespacedName
	if migMigration.Status.SrcBackupRef == nil {
		backupNsName = types.NamespacedName{
			Name:      migMigration.Name + "-",
			Namespace: veleroNs,
		}
	} else {
		backupNsName = types.NamespacedName{
			Name:      migMigration.Status.SrcBackupRef.Name,
			Namespace: migMigration.Status.SrcBackupRef.Namespace,
		}
	}
	return backupNsName
}

// Create Velero Backup on srcCluster looking at namespaces in MigAssetCollection
func (r *ReconcileMigMigration) ensureSrcBackup(migMigration *migapi.MigMigration, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	// Determine appropriate backupNsName to use in ensureBackup
	backupNsName := getSrcBackupName(migMigration)

	// Ensure that a backup exists with backupNsName
	rres, err := vrunner.EnsureBackup(r.Client, backupNsName, rres, false, logPrefix)
	if err != nil {
		return nil, err // requeue
	}

	// Update MigMigration with reference to Velero Backup
	migMigration.Status.SrcBackupRef = &kapi.ObjectReference{Name: rres.SrcBackup.Name, Namespace: rres.SrcBackup.Namespace}
	err = r.Update(context.TODO(), migMigration)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to UPDATE MigMigration with Velero Backup reference", logPrefix))
		return nil, err // requeue
	}

	return rres, nil // continue
}

func getDestRestoreName(migMigration *migapi.MigMigration) types.NamespacedName {
	var restoreNsName types.NamespacedName
	if migMigration.Status.DestRestoreRef == nil {
		restoreNsName = types.NamespacedName{
			Name:      migMigration.Name + "-",
			Namespace: veleroNs,
		}
	} else {
		restoreNsName = types.NamespacedName{
			Name:      migMigration.Status.DestRestoreRef.Name,
			Namespace: migMigration.Status.DestRestoreRef.Namespace,
		}
	}
	return restoreNsName
}

// Create Velero Restore on destMigCluster pointing at Velero Backup unique name
func (r *ReconcileMigMigration) ensureDestRestore(migMigration *migapi.MigMigration, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	backupNsName := getSrcBackupName(migMigration)
	restoreNsName := getDestRestoreName(migMigration)

	rres, err := vrunner.EnsureRestore(r.Client, backupNsName, restoreNsName, rres, logPrefix)
	if err != nil {
		return nil, err // requeue
	}
	if rres == nil {
		return nil, nil // don't requeue
	}

	// Update MigMigration with reference to Velero Retore
	migMigration.Status.DestRestoreRef = &kapi.ObjectReference{Name: rres.DestRestore.Name, Namespace: rres.DestRestore.Namespace}
	err = r.Update(context.TODO(), migMigration)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to UPDATE MigMigration with Velero Restore reference", logPrefix))
		return nil, err // requeue
	}

	return rres, nil // continue
}

func (r *ReconcileMigMigration) finishMigMigration(migMigration *migapi.MigMigration, rres *migapi.PlanRefResources) (*migapi.PlanRefResources, error) {
	if rres.DestRestore.Status.Phase == velerov1.RestorePhaseCompleted {
		changed := migMigration.MarkAsCompleted()
		if changed {
			err := r.Update(context.TODO(), migMigration)
			if err != nil {
				log.Info("[%s] Failed to mark MigMigration [%s/%s] as completed", logPrefix, migMigration.Namespace, migMigration.Name)
				return nil, err // requeue
			}
			log.Info(fmt.Sprintf("[%s] FINISHED MigMigration [%s/%s]", logPrefix, migMigration.Namespace, migMigration.Name))
		}
	}
	return rres, nil // continue
}
