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
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var rwmInstance *RemoteWatchMap
var createRemoteWatchMapOnce sync.Once

// RemoteWatchCluster tracks Remote Managers and Event Forward Channels
type RemoteWatchCluster struct {
	ForwardChannel chan event.GenericEvent
	RemoteManager  manager.Manager
	//  TODO - setup stop channel for manager so that manager will stop when we close the event channel from parent
}

// RemoteWatchMap provides a map between MigCluster nsNames and RemoteWatchClusters
type RemoteWatchMap struct {
	sync.RWMutex
	nsNameToRemoteCluster map[types.NamespacedName]*RemoteWatchCluster
}

// GetRemoteWatchMap returns the shared RemoteWatchMap instance
func GetRemoteWatchMap() *RemoteWatchMap {
	createRemoteWatchMapOnce.Do(func() {
		rwmInstance = &RemoteWatchMap{}
		rwmInstance.nsNameToRemoteCluster = make(map[types.NamespacedName]*RemoteWatchCluster)
	})
	return rwmInstance
}

// GetRWC gets the RemoteWatchCluster associated with a MigCluster resource nsName, return nil if not found
func (r *RemoteWatchMap) GetRWC(key types.NamespacedName) *RemoteWatchCluster {
	r.RLock()
	defer r.RUnlock()
	rwc, ok := rwmInstance.nsNameToRemoteCluster[key]
	if !ok {
		return nil
	}
	return rwc
}

// SetRWC sets the RemoteWatchCluster associated with a MigCluster resource nsName
func (r *RemoteWatchMap) SetRWC(key types.NamespacedName, value *RemoteWatchCluster) {
	r.Lock()
	defer r.Unlock()
	rwmInstance.nsNameToRemoteCluster[key] = value
}

// DeleteRWC deletes the RemoteWatchCluster associated with a MigCluster resource nsName
func (r *RemoteWatchMap) DeleteRWC(key types.NamespacedName) {
	r.Lock()
	defer r.Unlock()
	delete(rwmInstance.nsNameToRemoteCluster, key)
}
