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
	"github.com/fusor/mig-controller/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MigPlanToMigMigration ...
func MigPlanToMigMigration(a handler.MapObject) []reconcile.Request {
	childKind := util.KindMigPlan
	parentKind := util.KindMigMigration

	return util.MapChildToParents(a, childKind, parentKind)
}

// MigClusterToMigMigration ...
func MigClusterToMigMigration(a handler.MapObject) []reconcile.Request {
	childKind := util.KindMigCluster
	parentKind := util.KindMigMigration

	return util.MapChildToParents(a, childKind, parentKind)
}

// MigStageToMigMigration ...
func MigStageToMigMigration(a handler.MapObject) []reconcile.Request {
	childKind := util.KindMigStage
	parentKind := util.KindMigMigration

	return util.MapChildToParents(a, childKind, parentKind)
}
