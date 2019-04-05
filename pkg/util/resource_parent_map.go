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
)

var rpmInstance *ResourceParentsMap
var createResourceParentMapOnce sync.Once

// ResourceParent holds information about a Parent resource which references a Child resource.
type ResourceParent struct {
	ParentKind   string
	ParentNsName types.NamespacedName
}

// ResourceParentsMap maps from Mig K8s kinds to child objects to parents of those child objects.
// Meant to be used so that controllers for parent resources can enqueue events from child resources
// on the relevant parent resource.
type ResourceParentsMap struct {
	sync.RWMutex
	kindToChildrenAndParentsMap map[string]map[types.NamespacedName][]ResourceParent
}

// GetResourceParentsMap must be called to get a reference to the singleton ResourceParentsMap
func GetResourceParentsMap() *ResourceParentsMap {
	createResourceParentMapOnce.Do(func() {
		rpmInstance = &ResourceParentsMap{}
		rpmInstance.kindToChildrenAndParentsMap = make(map[string]map[types.NamespacedName][]ResourceParent)
	})
	return rpmInstance
}

// GetResourceParents ...
func (r *ResourceParentsMap) GetResourceParents(childKind string, childNsName types.NamespacedName) []ResourceParent {
	r.RLock()
	defer r.RUnlock()
	childKindResources, ok := rpmInstance.kindToChildrenAndParentsMap[childKind]
	if !ok {
		return nil
	}
	resourceParents, ok := childKindResources[childNsName]
	if !ok {
		return nil
	}
	return resourceParents
}

// SetResourceParents ...
func (r *ResourceParentsMap) SetResourceParents(childKind string, childNsName types.NamespacedName, parents []ResourceParent) {
	r.Lock()
	defer r.Unlock()
	rpmInstance.kindToChildrenAndParentsMap[childKind][childNsName] = parents
}

// DeleteResourceParents ...
func (r *ResourceParentsMap) DeleteResourceParents(childKind string, childNsName types.NamespacedName) {
	r.Lock()
	defer r.Unlock()
	delete(rpmInstance.kindToChildrenAndParentsMap[childKind], childNsName)
}

// AddResourceParent ...
func (r *ResourceParentsMap) AddResourceParent(childKind string, childNsName types.NamespacedName, parent ResourceParent) {
	r.Lock()
	defer r.Unlock()

	resourceParents := rpmInstance.kindToChildrenAndParentsMap[childKind][childNsName]
	resourceParents = append(resourceParents, parent)
}

// DeleteResourceParent ...
func (r *ResourceParentsMap) DeleteResourceParent(childKind string, childNsName types.NamespacedName, parent ResourceParent) {
	r.Lock()
	defer r.Unlock()

	childKindResources, ok := rpmInstance.kindToChildrenAndParentsMap[childKind]
	if !ok {
		return
	}
	resourceParents, ok := childKindResources[childNsName]
	if !ok {
		return
	}

	for i := range resourceParents {
		if resourceParents[i] == parent {
			resourceParents = append(resourceParents[:i], resourceParents[i+1:]...)
			return
		}
	}
}
