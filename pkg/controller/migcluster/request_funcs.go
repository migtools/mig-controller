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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterToMigCluster maps a Cluster request to a MigCluster request
func ClusterToMigCluster(a handler.MapObject) []reconcile.Request {
	log.Info("[mapFn] ClusterToMigCluster running...")
	// Create childCrCluster to look up parents
	childCrCluster := util.KubeResource{
		Kind: util.KindClusterRegCluster,
		NsName: types.NamespacedName{
			Name:      a.Meta.GetName(),
			Namespace: a.Meta.GetNamespace(),
		},
	}
	// Find all parent resources of this child
	rpm := util.GetResourceParentsMap()
	parents := rpm.GetParentsOfKind(childCrCluster, util.KindMigCluster)
	// Build list of requests to dispatch
	requests := []reconcile.Request{}
	for i := range parents {
		newRequest := reconcile.Request{
			NamespacedName: parents[i].NsName,
		}
		requests = append(requests, newRequest)
	}
	return requests
}

// SecretToMigCluster maps a Secret request to a MigCluster request
func SecretToMigCluster(a handler.MapObject) []reconcile.Request {
	log.Info("[mapFn] SecretToMigCluster running...")
	// Create childCrCluster to look up parents
	childSecret := util.KubeResource{
		Kind: util.KindSecret,
		NsName: types.NamespacedName{
			Name:      a.Meta.GetName(),
			Namespace: a.Meta.GetNamespace(),
		},
	}
	// Find all parent resources of this child
	rpm := util.GetResourceParentsMap()
	parents := rpm.GetParentsOfKind(childSecret, util.KindSecret)
	// Build list of requests to dispatch
	requests := []reconcile.Request{}
	for i := range parents {
		newRequest := reconcile.Request{
			NamespacedName: parents[i].NsName,
		}
		requests = append(requests, newRequest)
	}
	return requests
}
