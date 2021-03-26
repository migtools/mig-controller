/*
Copyright 2021 Red Hat Inc.

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

package cache

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// Functions in this file solve the problem of duplicate reconciles when the cached
// client hasn't received the latest resource and feeds a stale resource to controller.
// We will reject reconciling a resource if its generation is older than one we've
// successfully reconciled.

// UIDToGenerationMap keeps track of highest reconciled generation of a particular resource UID
type UIDToGenerationMap struct {
	mutex                   sync.RWMutex
	resourceUIDtoGeneration map[types.UID]int64
}

// CreateUIDToGenerationMap creates a new UID => generation map
func CreateUIDToGenerationMap() *UIDToGenerationMap {
	uidGenMap := UIDToGenerationMap{}
	uidGenMap.resourceUIDtoGeneration = make(map[types.UID]int64)
	return &uidGenMap
}

// getGenerationForUID returns the latest successfully reconciled resource generation
// returns -1 if not found.
func (u *UIDToGenerationMap) getGenerationForUID(resourceUID types.UID) int64 {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	generation, ok := u.resourceUIDtoGeneration[resourceUID]
	if !ok {
		return -1
	}
	return generation
}

// setGenerationForUID returns the latest successfully reconciled resource generation
func (u *UIDToGenerationMap) setGenerationForUID(resourceUID types.UID, generation int64) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.resourceUIDtoGeneration[resourceUID] = generation
}

// IsCacheStale checks if the cached client is providing a resource we've successfully reconciled.
func (u *UIDToGenerationMap) IsCacheStale(resourceUID types.UID, generation int64) bool {
	reconciledGeneration := u.getGenerationForUID(resourceUID)
	// Resource should be reconciled if we've never seen it
	if generation == -1 {
		return false
	}
	// Resource is stale if we've reconciled a newer generation
	if generation < reconciledGeneration {
		return true
	}
	return false
}

// RecordReconciledGeneration records that a resource UID generation was pushed back to the APIserver
func (u *UIDToGenerationMap) RecordReconciledGeneration(resourceUID types.UID, generation int64) {
	u.setGenerationForUID(resourceUID, generation)
}
