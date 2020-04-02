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
	"github.com/konveyor/mig-controller/pkg/controller/remotewatcher"
	"github.com/konveyor/mig-controller/pkg/remote"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// StartRemoteWatch will configure a new RemoteWatcher manager + controller to monitor Velero
// events on a remote cluster. A GenericEvent channel will be configured to funnel events from
// the RemoteWatcher controller to the MigCluster controller.
func StartRemoteWatch(r *ReconcileMigCluster, config remote.ManagerConfig) error {
	remoteWatchMap := remote.GetWatchMap()

	mgr, err := manager.New(config.RemoteRestConfig, manager.Options{})
	if err != nil {
		log.Error(err, "[rWatch] Unable to set up remote watcher controller manager")
		return err
	}

	// Parent controller watches for events from forwardChannel.
	log.Info("[rWatch] Creating forwardChannel")
	forwardChannel := make(chan event.GenericEvent)

	log.Info("[rWatch] Starting watch on forwardChannel")
	err = r.Controller.Watch(&source.Channel{Source: forwardChannel}, &handler.EnqueueRequestForObject{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Add remoteWatcher to remote MGR
	log.Info("[rWatch] Adding controller to manager")
	forwardEvent := event.GenericEvent{
		Meta:   config.ParentMeta,
		Object: config.ParentObject,
	}
	err = remotewatcher.Add(mgr, forwardChannel, forwardEvent)
	if err != nil {
		log.Error(err, "Error adding RemoteWatcher controller to manager")
		return err
	}

	sigStopChan := make(chan struct{})
	log.Info("[rWatch] Starting manager")
	go mgr.Start(sigStopChan)

	log.Info("[rWatch] Manager started")
	// TODO: provide a way to dynamically change where events are being forwarded to (multiple controllers)
	// Create remoteWatchCluster tracking obj and attach reference to parent object so we don't create extra
	remoteWatchCluster := &remote.WatchCluster{ForwardChannel: forwardChannel, RemoteManager: mgr, StopChannel: sigStopChan}

	// MigClusters have a 1:1 association with a RemoteWatchCluster, so we will store the mapping
	// to avoid creating duplicate remote managers in the future.
	remoteWatchMap.Set(config.ParentNsName, remoteWatchCluster)

	return nil
}

// StopRemoteWatch will close a remote watch's stop channel
// and delete it from the remote watch map.
func StopRemoteWatch(nsName types.NamespacedName) {
	remoteWatchMap := remote.GetWatchMap()
	remoteWatchCluster := remoteWatchMap.Get(nsName)
	if remoteWatchCluster != nil {
		close(remoteWatchCluster.StopChannel)
		remoteWatchMap.Delete(nsName)
	}
}
