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
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

// reconcileResources holds the data needed for MigMigration to reconcile.
// At the beginning of a reconcile, this data will be compiled by fetching
// information from each cluster involved in the migration.
type reconcileResources struct {
	migPlan        *migapi.MigPlan
	migAssets      *migapi.MigAssetCollection
	srcMigCluster  *migapi.MigCluster
	destMigCluster *migapi.MigCluster
	migStage       *migapi.MigStage
}

// getReconcileResources puts together a struct with all resources needed to perform a MigMigration reconcile
func (r *ReconcileMigMigration) getReconcileResources(migMigration *migapi.MigMigration) (*reconcileResources, error) {
	resources := &reconcileResources{}

	// MigPlan
	migPlan, err := migMigration.GetMigPlan(r.Client)
	if err != nil {
		return nil, err
	}
	resources.migPlan = migPlan

	// MigAssetCollection
	migAssets, err := migPlan.GetMigAssetCollection(r.Client)
	if err != nil {
		return nil, err
	}
	resources.migAssets = migAssets

	// SrcMigCluster
	srcMigCluster, err := migPlan.GetSrcMigCluster(r.Client)
	if err != nil {
		return nil, err
	}
	resources.srcMigCluster = srcMigCluster

	// DestMigCluster
	destMigCluster, err := migPlan.GetDestMigCluster(r.Client)
	if err != nil {
		return nil, err
	}
	resources.destMigCluster = destMigCluster

	return resources, nil
}
