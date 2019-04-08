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

package migcluster

import (
	"github.com/fusor/mig-controller/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterToMigClusters maps a Cluster request to MigCluster requests
func ClusterToMigClusters(a handler.MapObject) []reconcile.Request {
	// Customize these kinds for each mapFunc
	childKind := util.KindClusterRegCluster
	parentKind := util.KindMigCluster

	log.Info("[mapFn] Cluster => MigCluster")
	return util.MapChildToParents(a, childKind, parentKind)
}

// SecretToMigClusters maps a Secret request to MigCluster requests
func SecretToMigClusters(a handler.MapObject) []reconcile.Request {
	// Customize these kinds for each mapFunc
	childKind := util.KindSecret
	parentKind := util.KindMigCluster

	log.Info("[mapFn] Secret => MigCluster")
	return util.MapChildToParents(a, childKind, parentKind)
}
