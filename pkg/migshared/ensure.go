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

package migshared

import (
	vrunner "github.com/fusor/mig-controller/pkg/velerorunner"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureBackup ensures that a source cluster Velero Backup exists
func EnsureBackup(c k8sclient.Client, backupNsName types.NamespacedName, rres *ReconcileResources, isStageBackup bool, logPrefix string) (*ReconcileResources, error) {
	// Build controller-runtime client for srcMigCluster
	srcClusterK8sClient, err := rres.SrcMigCluster.GetClient(c)
	if err != nil {
		log.Error(err, "[%s] Failed to GET srcClusterK8sClient", logPrefix)
		return nil, nil // don't requeue
	}

	assetNamespaces := rres.MigAssets.Spec.Namespaces
	vBackupNew := vrunner.BuildVeleroBackup(backupNsName, assetNamespaces, isStageBackup)
	srcBackup, err := vrunner.RunBackup(srcClusterK8sClient, vBackupNew, backupNsName, logPrefix)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil // don't requeue
		}
		return nil, err // requeue
	}
	if srcBackup == nil {
		return nil, nil // don't requeue
	}
	rres.SrcBackup = srcBackup
	return rres, nil
}

// EnsureRestore ensures that a destination cluster Velero Backup + Restore exists
func EnsureRestore(c k8sclient.Client, backupNsName types.NamespacedName, restoreNsName types.NamespacedName, rres *ReconcileResources, logPrefix string) (*ReconcileResources, error) {
	// Build controller-runtime client for destMigCluster
	destClusterK8sClient, err := rres.DestMigCluster.GetClient(c)
	if err != nil {
		log.Error(err, "[%s] Failed to GET destClusterK8sClient", logPrefix)
		return nil, nil // don't requeue
	}

	// Create Velero Restore on destMigCluster pointing at Velero Backup unique name
	vRestoreNew := vrunner.BuildVeleroRestore(restoreNsName, backupNsName.Name)
	destRestore, err := vrunner.RunRestore(destClusterK8sClient, vRestoreNew, restoreNsName, backupNsName, logPrefix)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil // don't requeue
		}
		return nil, err // requeue
	}
	if destRestore == nil {
		return nil, nil // don't requeue
	}

	rres.DestRestore = destRestore
	return rres, nil
}
