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

package migcluster

import (
	"context"
	"strconv"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/remote"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// StartRemoteWatch will configure a new RemoteWatcher manager that provides a cached client
func StartRemoteWatch(r *ReconcileMigCluster, config remote.ManagerConfig) error {
	remoteWatchMap := remote.GetWatchMap()

	mgr, err := manager.New(config.RemoteRestConfig, manager.Options{Scheme: config.Scheme, MetricsBindAddress: "0"})
	if err != nil {
		return liberr.Wrap(err)
	}

	// sigStopChan := make(chan struct{})
	ctx := context.TODO()
	ctx, stopFunc := context.WithCancel(ctx)
	log.Info("Remote cache: Starting manager for MigCluster",
		"migCluster", config.ParentNsName)
	go mgr.Start(ctx)

	// Indexes
	indexer := mgr.GetFieldIndexer()

	// Plan
	err = indexer.IndexField(
		context.TODO(),
		&migapi.MigPlan{},
		migapi.ClosedIndexField,
		func(rawObj k8sclient.Object) []string {
			p, cast := rawObj.(*migapi.MigPlan)
			if !cast {
				return nil
			}
			return []string{
				strconv.FormatBool(p.Spec.Closed),
			}
		})
	if err != nil {
		stopFunc()
		return err
	}
	// Pod
	err = indexer.IndexField(
		context.TODO(),
		&kapi.Pod{},
		"status.phase",
		func(rawObj k8sclient.Object) []string {
			p, cast := rawObj.(*kapi.Pod)
			if !cast {
				return nil
			}
			return []string{
				string(p.Status.Phase),
			}
		})
	if err != nil {
		stopFunc()
		return err
	}
	// Event
	err = indexer.IndexField(
		context.TODO(),
		&kapi.Event{},
		"involvedObject.uid",
		func(rawObj k8sclient.Object) []string {
			e, cast := rawObj.(*kapi.Event)
			if !cast {
				return nil
			}
			return []string{
				string(e.InvolvedObject.UID),
			}
		})
	if err != nil {
		stopFunc()
		return err
	}

	log.Info("Remote cache: Manager started for MigCluster",
		"migCluster", config.ParentNsName)

	// Create remoteWatchCluster tracking obj and attach reference to parent object so we don't create extra
	remoteWatchCluster := &remote.WatchCluster{RemoteManager: mgr, StopFunc: stopFunc}

	// MigClusters have a 1:1 association with a RemoteWatchCluster, so we will store the mapping
	// to avoid creating duplicate remote managers in the future.
	remoteWatchMap.Set(config.ParentNsName, remoteWatchCluster)

	return nil
}

// IsRemoteWatchConsistent checks whether remote watch in-memory is consistent with given new config
func IsRemoteWatchConsistent(nsName types.NamespacedName, config *rest.Config) bool {
	inMemoryWatch := remote.GetWatchMap().Get(types.NamespacedName{
		Name:      nsName.Name,
		Namespace: nsName.Namespace,
	})
	if inMemoryWatch == nil {
		return false
	}
	inMemoryConfig := inMemoryWatch.RemoteManager.GetConfig()
	return migapi.AreRestConfigsEqual(inMemoryConfig, config)
}

// StopRemoteWatch will run the remote manager stopFunc
// and delete it from the remote watch map.
func StopRemoteWatch(nsName types.NamespacedName) {
	remoteWatchMap := remote.GetWatchMap()
	remoteWatchCluster := remoteWatchMap.Get(nsName)
	if remoteWatchCluster != nil {
		// close(remoteWatchCluster.StopChannel)
		remoteWatchCluster.StopFunc()
		remoteWatchMap.Delete(nsName)
	}
}
