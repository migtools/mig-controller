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

package util

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller")

// Kind strings to be used in KubeResource object creation
// Use these kinds when adding to the ResourceParentsMap
const (
	KindSecret = "Secret"

	KindMigPlan    = "MigPlan"
	KindMigCluster = "MigCluster"
	KindMigAssets  = "MigAssetCollection"
	KindMigStorage = "MigStorage"

	KindClusterRegCluster = "Cluster"

	KindVeleroBackup                 = "VeleroBackup"
	KindVeleroRestore                = "VeleroRestore"
	KindVeleroBackupStorageLocation  = "VeleroBackupStorageLocation"
	KindVeleroVolumeSnapshotLocation = "VeleroVolumeSnapshotLocation"
)

var rpmInstance *ResourceParentsMap
var createChildParentsMapOnce sync.Once

// KubeResource is a simplified way to related child to parent resources
type KubeResource struct {
	Kind   string
	NsName types.NamespacedName
}

// ResourceParentsMap maps from "parents resources" to "child resources"
// Maintaining this allows event triggering with a 1:N reference relationship
type ResourceParentsMap struct {
	sync.RWMutex
	childToParentsMap map[KubeResource][]KubeResource // 1:N child:parent mapping
}

// GetResourceParentsMap must be called to get a reference to the singleton ResourceParentsMap
func GetResourceParentsMap() *ResourceParentsMap {
	createChildParentsMapOnce.Do(func() {
		rpmInstance = &ResourceParentsMap{}
		rpmInstance.childToParentsMap = make(map[KubeResource][]KubeResource)
	})
	return rpmInstance
}

// Definitely useful
//  - [DONE] Get the list of parent resources _of a particular kind_ for a particular child resource
//  - [DONE] Add a reference from a child resource to a parent resource
//  - [DONE] Delete a reference from a child resource to a parent resource
//  - [DONE] Get list of child resources for a particular parent

// Maybe useful
//  - Get the list of all child resources of a particular kind

// GetParentsOfKind ...
func (r *ResourceParentsMap) GetParentsOfKind(child KubeResource, parentKind string) []KubeResource {
	r.RLock()
	defer r.RUnlock()

	parents, ok := rpmInstance.childToParentsMap[child]
	if !ok {
		return nil
	}

	// Find parents of particular kind
	matchingParents := []KubeResource{}
	for i := range parents {
		if parents[i].Kind == parentKind {
			matchingParents = append(matchingParents, parents[i])
		}
	}

	// Will return empty slice if nothing found
	return matchingParents
}

// GetChildrenOfKind ...
func (r *ResourceParentsMap) GetChildrenOfKind(parent KubeResource, childKind string) []KubeResource {
	r.RLock()
	defer r.RUnlock()

	// First get all of the child keys of the appropriate kind
	matchKeys := []KubeResource{}
	for childKey := range rpmInstance.childToParentsMap {
		if childKey.Kind == childKind {
			matchKeys = append(matchKeys, childKey)
		}
	}

	// Next filter down to only children with the requested parent
	// childrenWithCorrectParent := []KubeResource{}
	hasCorrectParentMap := make(map[KubeResource]int)
	for i := range matchKeys {
		parents, ok := rpmInstance.childToParentsMap[matchKeys[i]]
		if !ok {
			panic("GetChildrenOfKind - expected child key not found")
		}
		// Check if desired parent is attached to child
		for j := range parents {
			if parents[j] == parent {
				hasCorrectParentMap[parents[j]] = 1
			}
		}
	}

	// Build the final list of correct parents to return
	childrenOfKind := []KubeResource{}
	for child := range hasCorrectParentMap {
		childrenOfKind = append(childrenOfKind, child)
	}

	return childrenOfKind
}

// AddChildToParent ...
func (r *ResourceParentsMap) AddChildToParent(child KubeResource, parent KubeResource) {
	r.Lock()
	defer r.Unlock()

	parents, ok := rpmInstance.childToParentsMap[child]

	// Create parent list on child if it doesn't exist
	if !ok {
		rpmInstance.childToParentsMap[child] = []KubeResource{}
		parents = rpmInstance.childToParentsMap[child]
	}

	// Don't add a duplicate parent refs to a child
	for i := range parents {
		if parents[i] == parent {
			return
		}
	}

	// Add the new parent to the child resource
	parents = append(parents, parent)
	rpmInstance.childToParentsMap[child] = parents
}

// DeleteChildFromParent ...
func (r *ResourceParentsMap) DeleteChildFromParent(child KubeResource, parent KubeResource) {
	r.Lock()
	defer r.Unlock()

	childParents, ok := rpmInstance.childToParentsMap[child]
	if !ok {
		return
	}

	for i := range childParents {
		if childParents[i] == parent {
			childParents = append(childParents[:i], childParents[i+1:]...)
			rpmInstance.childToParentsMap[child] = childParents
			return
		}
	}
}
