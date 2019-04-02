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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var rwmInstance *RemoteWatchMap
var mu sync.RWMutex

// RemoteWatchMap provides a concurrent-safe map between MigCluster nsNames and RemoteWatchClusters
type RemoteWatchMap struct {
	nsNameToRemoteCluster map[types.NamespacedName]*RemoteWatchCluster
}

// RemoteWatchCluster tracks Remote Managers and Event Forward Channels
type RemoteWatchCluster struct {
	ForwardChannel chan event.GenericEvent
	RemoteManager  manager.Manager
	//  TODO - setup stop channel for manager so that manager will stop when we close the event channel from parent
}

func init() {
	rwmInstance := &RemoteWatchMap{}
	rwmInstance.nsNameToRemoteCluster = make(map[types.NamespacedName]*RemoteWatchCluster)
}

// Get a RemoteWatchCluster associated with a MigCluster resource nsName, return nil if not found
func Get(key types.NamespacedName) *RemoteWatchCluster {
	mu.RLock()
	defer mu.RUnlock()
	rwm, ok := rwmInstance.nsNameToRemoteCluster[key]
	if !ok {
		return nil
	}
	return rwm
}

// Set a RemoteWatchCluster associated with a MigCluster resource nsName
func Set(key types.NamespacedName, value *RemoteWatchCluster) {
	mu.Lock()
	defer mu.Unlock()
	rwmInstance.nsNameToRemoteCluster[key] = value
}

// Delete a RemoteWatchCluster associated with a MigCluster resource nsName
func Delete(key types.NamespacedName) {
	mu.Lock()
	defer mu.Unlock()
	delete(rwmInstance.nsNameToRemoteCluster, key)
}
