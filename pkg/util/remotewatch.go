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
	"os"

	"github.com/fusor/mig-controller/pkg/controller/remotewatcher"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// RemoteManagerConfig specifies config options for setting up a RemoteWatch Manager
type RemoteManagerConfig struct {
	// rest.Config for remote cluster to watch Velero events on
	remoteRestConfig *rest.Config
	// nsname used in mapping MigCluster resources to RemoteWatchManagers
	parentNsName types.NamespacedName
	// MigMigration object containing v1.Object and runtime.Object needed for remote cluster to properly forward events
	parentResource *migrationv1alpha1.MigMigration
}

// TODO: add support for forwarding events to multiple channels so that MigStage and
// MigMigration controllers can also be notified of Velero events on remote clusters.

// StartRemoteWatch will configure a new RemoteWatcher manager + controller to monitor Velero
// events on a remote cluster. A GenericEvent channel will be configured to funnel events from
// the RemoteWatcher controller to the MigCluster controller.
func StartRemoteWatch(r *ReconcileMigrationAIO, config RemoteManagerConfig, watchKey string) error {

	mgr, err := manager.New(config.remoteRestConfig, manager.Options{})
	if err != nil {
		log.Error(err, "<RemoteWatcher> unable to set up remote watcher controller manager")
		os.Exit(1)
	}

	log.Info("<RemoteWatcher> Adding Velero to scheme...")
	if err := velerov1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add Velero APIs to scheme")
		os.Exit(1)
	}

	// Parent controller watches for events from forwardChannel.
	log.Info("<RemoteWatcher> Creating forwardChannel...")
	forwardChannel := make(chan event.GenericEvent)

	log.Info("<RemoteWatcher> Starting watch on forwardChannel...")
	err = r.Controller.Watch(&source.Channel{Source: forwardChannel}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Add remoteWatcher to remote MGR
	log.Info("<RemoteWatcher> Adding controller to manager...")
	forwardEvent := event.GenericEvent{
		Meta:   config.parentResource.GetObjectMeta(),
		Object: config.parentResource,
	}
	err = remotewatcher.Add(mgr, forwardChannel, forwardEvent)
	if err != nil {
		log.Info("<RemoteWatcher> Error adding remotewatcher")
		return err
	}

	// TODO: keep track of sigStopChan so that we can stop the manager at a later time
	sigStopChan := make(chan struct{})
	log.Info("<RemoteWatcher> Starting manager...")
	go mgr.Start(sigStopChan)

	log.Info("<RemoteWatcher> Manager started!")
	// TODO: provide a way to dynamically change where events are being forwarded to (multiple controllers)
	// Create remoteWatchCluster tracking obj and attach reference to parent object so we don't create extra
	remoteWatchCluster := &RemoteWatchCluster{ForwardChannel: forwardChannel, RemoteManager: mgr}

	// TODO: since MigClusters will have a 1:1 association with a RemoteWatchCluster, should be able to switch
	// back to using config.parentNsName instead of watchKey

	// Temporarily tracking remoteWatchClusters using watchKey instead of parentNsName to faciliate POC design
	SetRWC(types.NamespacedName{Name: watchKey, Namespace: watchKey}, remoteWatchCluster)
	// SetRWC(config.parentNsName, remoteWatchCluster)

	log.Info("<RemoteWatcher> Added mapping from nsName to remoteWatchCluster")

	return nil
}
