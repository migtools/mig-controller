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
	"fmt"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// ReconcileResources holds common data needed for MigStage and MigMigration to
// reconcile. At the beginning of a reconcile, this data will be compiled by
// fetching information from each cluster involved in the migration.
type ReconcileResources struct {
	MigPlan        *migapi.MigPlan
	MigAssets      *migapi.MigAssetCollection
	SrcMigCluster  *migapi.MigCluster
	DestMigCluster *migapi.MigCluster

	SrcBackup   *velerov1.Backup
	DestRestore *velerov1.Restore
}

var log = logf.Log.WithName("controller")

// GetReconcileResources gets referenced resources from a MigPlan and loads them into a ReconcileResources struct
func GetReconcileResources(client k8sclient.Client, migPlan *migapi.MigPlan, logPrefix string) (*ReconcileResources, error) {
	resources := &ReconcileResources{}

	// MigPlan
	resources.MigPlan = migPlan

	// MigAssetCollection
	migAssets, err := migPlan.GetAssetCollection(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET MigAssetCollection referenced by MigPlan [%s/%s]",
			logPrefix, migPlan.Namespace, migPlan.Name))
		return nil, err
	}
	resources.MigAssets = migAssets

	// SrcMigCluster
	srcMigCluster, err := migPlan.GetSourceCluster(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET SrcMigCluster referenced by MigPlan [%s/%s]",
			logPrefix, migPlan.Namespace, migPlan.Name))
		return nil, err
	}
	resources.SrcMigCluster = srcMigCluster

	// DestMigCluster
	destMigCluster, err := migPlan.GetDestinationCluster(client)
	if err != nil {
		log.Info(fmt.Sprintf("[%s] Failed to GET DestMigCluster referenced by MigPlan [%s/%s]",
			logPrefix, migPlan.Namespace, migPlan.Name))
		return nil, err
	}
	resources.DestMigCluster = destMigCluster

	return resources, nil
}
