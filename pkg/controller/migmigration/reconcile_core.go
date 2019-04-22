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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileMigMigration) initReconcile(migMigration *migapi.MigMigration) (bool, error) {
	// Return if MigMigration is complete
	if migMigration.Status.MigrationCompleted == true {
		return false, nil // don't requeue
	}
	// Return if MigMigration isn't ready
	if !migMigration.Status.IsReady() {
		return false, nil //don't requeue
	}
	// Build ReconcileResources for MigMigration containing data needed for rest of reconcile process
	rres, err := r.getReconcileResources(migMigration)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil // don't requeue
		}
		return false, err // requeue
	}

	r.resources = rres
	return true, nil // continue
}

func (r *ReconcileMigMigration) startMigMigration(migMigration *migapi.MigMigration) (bool, error) {
	// If all references are marked as ready, run MarkAsRunning() to set this Migration into "Running" state
	changed := migMigration.MarkAsRunning()
	if changed {
		err := r.Update(context.TODO(), migMigration)
		if err != nil {
			log.Info("[%s] Failed to mark MigMigration [%s/%s] as running", logPrefix, migMigration.Namespace, migMigration.Name)
			return false, err // requeue
		}
		log.Info(fmt.Sprintf("[%s] STARTED MigMigration [%s/%s]", logPrefix, migMigration.Namespace, migMigration.Name))
	}
	return true, nil // continue
}

func (r *ReconcileMigMigration) ensureSourceClusterBackup(migMigration *migapi.MigMigration) (bool, error) {
	// Build controller-runtime client for srcMigCluster
	srcClusterK8sClient, err := r.resources.srcMigCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "[%s] Failed to GET srcClusterK8sClient", logPrefix)
		return false, nil // don't requeue
	}

	// Create Velero Backup on srcCluster looking at namespaces in MigAssetCollection
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
	srcBackup, err := vrunner.RunBackup(srcClusterK8sClient, backupNsName, r.resources.migAssets, logPrefix)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil // don't requeue
		}
		return false, err // requeue
	}
	if srcBackup == nil {
		return false, nil // don't requeue
	}
	r.resources.srcBackup = srcBackup

	// Update MigMigration with reference to Velero Backup
	migMigration.Status.SrcBackupRef = &kapi.ObjectReference{Name: srcBackup.Name, Namespace: srcBackup.Namespace}
	err = r.Update(context.TODO(), migMigration)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to UPDATE MigMigration with Velero Backup reference", logPrefix))
		return false, err // requeue
	}

	return true, nil // continue
}

func (r *ReconcileMigMigration) ensureDestinationClusterRestore(migMigration *migapi.MigMigration) (bool, error) {
	// Build controller-runtime client for destMigCluster
	destClusterK8sClient, err := r.resources.destMigCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "[%s] Failed to GET destClusterK8sClient", logPrefix)
		return false, nil // don't requeue
	}

	// Create Velero Restore on destMigCluster pointing at Velero Backup unique name
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

	// Reference the Velero Backup attached to the MigMigration
	backupNsName := types.NamespacedName{
		Name:      migMigration.Status.SrcBackupRef.Name,
		Namespace: migMigration.Status.SrcBackupRef.Namespace,
	}

	destRestore, err := vrunner.RunRestore(destClusterK8sClient, restoreNsName, backupNsName, logPrefix)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil // don't requeue
		}
		return false, err // requeue
	}
	if destRestore == nil {
		return false, nil // don't requeue
	}
	r.resources.destRestore = destRestore

	// Update MigMigration with reference to Velero Retore
	migMigration.Status.DestRestoreRef = &kapi.ObjectReference{Name: destRestore.Name, Namespace: destRestore.Namespace}
	err = r.Update(context.TODO(), migMigration)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to UPDATE MigMigration with Velero Restore reference", logPrefix))
		return false, err // requeue
	}

	return true, nil // continue
}

func (r *ReconcileMigMigration) finishMigMigration(migMigration *migapi.MigMigration) (bool, error) {
	if r.resources.destRestore.Status.Phase == velerov1.RestorePhaseCompleted {
		changed := migMigration.MarkAsCompleted()
		if changed {
			err := r.Update(context.TODO(), migMigration)
			if err != nil {
				log.Info("[%s] Failed to mark MigMigration [%s/%s] as completed", logPrefix, migMigration.Namespace, migMigration.Name)
				return false, err // requeue
			}
			log.Info(fmt.Sprintf("[%s] FINISHED MigMigration [%s/%s]", logPrefix, migMigration.Namespace, migMigration.Name))
		}
	}
	return true, nil // continue
}
