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

	"github.com/fusor/mig-controller/pkg/controller/remotewatcher"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	migrationv1alpha1 "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
)

var rwmInstance *RemoteWatchMap
var createRemoteWatchMapOnce sync.Once

// RemoteManagerConfig specifies config options for setting up a RemoteWatch Manager
type RemoteManagerConfig struct {
	// rest.Config for remote cluster to watch Velero events on
	RemoteRestConfig *rest.Config
	// nsname used in mapping MigCluster resources to RemoteWatchManagers
	ParentNsName types.NamespacedName
	// MigMigration object containing v1.Object and runtime.Object needed for remote cluster to properly forward events
	ParentResource *migrationv1alpha1.MigCluster
}

// TODO: add support for forwarding events to multiple channels so that MigStage and
// MigMigration controllers can also be notified of Velero events on remote clusters.

// StartRemoteWatch will configure a new RemoteWatcher manager + controller to monitor Velero
// events on a remote cluster. A GenericEvent channel will be configured to funnel events from
// the RemoteWatcher controller to the MigCluster controller.
func StartRemoteWatch(r *ReconcileMigCluster, config RemoteManagerConfig) error {
	rwm := GetRemoteWatchMap()

	mgr, err := manager.New(config.RemoteRestConfig, manager.Options{})
	if err != nil {
		log.Error(err, "[rWatch] unable to set up remote watcher controller manager")
		return err
	}

	log.Info("[rWatch] Adding Velero to scheme...")
	if err := velerov1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add Velero APIs to scheme")
		return err
	}

	// Parent controller watches for events from forwardChannel.
	log.Info("[rWatch] Creating forwardChannel...")
	forwardChannel := make(chan event.GenericEvent)

	log.Info("[rWatch] Starting watch on forwardChannel...")
	err = r.Controller.Watch(&source.Channel{Source: forwardChannel}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Add remoteWatcher to remote MGR
	log.Info("[rWatch] Adding controller to manager...")
	forwardEvent := event.GenericEvent{
		Meta:   config.ParentResource.GetObjectMeta(),
		Object: config.ParentResource,
	}
	err = remotewatcher.Add(mgr, forwardChannel, forwardEvent)
	if err != nil {
		log.Info("[rWatch] Error adding RemoteWatcher controller to manager")
		return err
	}

	// TODO: keep track of sigStopChan so that we can stop the manager at a later time
	sigStopChan := make(chan struct{})
	log.Info("[rWatch] Starting manager...")
	go mgr.Start(sigStopChan)

	log.Info("[rWatch] Manager started!")
	// TODO: provide a way to dynamically change where events are being forwarded to (multiple controllers)
	// Create remoteWatchCluster tracking obj and attach reference to parent object so we don't create extra
	rwc := &RemoteWatchCluster{ForwardChannel: forwardChannel, RemoteManager: mgr}

	// MigClusters have a 1:1 association with a RemoteWatchCluster, so we will store the mapping
	// to avoid creating duplicate remote managers in the future.
	rwm.SetRWC(config.ParentNsName, rwc)

	return nil
}

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
