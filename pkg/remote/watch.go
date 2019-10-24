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

package remote

import (
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var rwmInstance *WatchMap
var createRemoteWatchMapOnce sync.Once

// ManagerConfig specifies config options for setting up a RemoteWatch Manager
type ManagerConfig struct {
	// rest.Config for remote cluster to watch Velero events on
	RemoteRestConfig *rest.Config
	// nsname used in mapping MigCluster resources to RemoteWatchManagers
	ParentNsName types.NamespacedName
	// MigMigration v1.Object and runtime.Object needed for remote cluster to properly forward events
	ParentMeta   v1.Object
	ParentObject runtime.Object
}

// TODO: add support for forwarding events to multiple channels so that MigStage and
// MigMigration controllers can also be notified of Velero events on remote clusters.

// WatchCluster tracks Remote Managers and Event Forward Channels
type WatchCluster struct {
	ForwardChannel chan event.GenericEvent
	RemoteManager  manager.Manager
	StopChannel    chan<- struct{}
}

// WatchMap provides a map between MigCluster nsNames and RemoteWatchClusters
type WatchMap struct {
	mutex                 sync.RWMutex
	nsNameToRemoteCluster map[types.NamespacedName]*WatchCluster
}

// GetWatchMap returns the shared RemoteWatchMap instance
func GetWatchMap() *WatchMap {
	createRemoteWatchMapOnce.Do(func() {
		rwmInstance = &WatchMap{}
		rwmInstance.nsNameToRemoteCluster = make(map[types.NamespacedName]*WatchCluster)
	})
	return rwmInstance
}

// Get the RemoteWatchCluster associated with a MigCluster resource nsName, return nil if not found
func (r *WatchMap) Get(key types.NamespacedName) *WatchCluster {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	rwc, ok := rwmInstance.nsNameToRemoteCluster[key]
	if !ok {
		return nil
	}
	return rwc
}

// Set the RemoteWatchCluster associated with a MigCluster resource nsName
func (r *WatchMap) Set(key types.NamespacedName, value *WatchCluster) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	rwmInstance.nsNameToRemoteCluster[key] = value
}

// Delete the RemoteWatchCluster associated with a MigCluster resource nsName
func (r *WatchMap) Delete(key types.NamespacedName) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(rwmInstance.nsNameToRemoteCluster, key)
}
