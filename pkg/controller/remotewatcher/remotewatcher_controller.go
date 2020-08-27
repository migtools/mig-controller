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

package remotewatcher

import (
	"github.com/konveyor/controller/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var log = logging.WithName("remote-watch")

// Add creates a new RemoteWatcher Controller with a forwardChannel
func Add(mgr manager.Manager, forwardChannel chan event.GenericEvent, fowardEvent event.GenericEvent) error {
	return add(mgr, newReconciler(mgr, forwardChannel, fowardEvent))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(
	mgr manager.Manager,
	forwardChannel chan event.GenericEvent,
	forwardEvent event.GenericEvent) reconcile.Reconciler {
	return &ReconcileRemoteWatcher{
		Client:         mgr.GetClient(),
		scheme:         mgr.GetScheme(),
		ForwardChannel: forwardChannel,
		ForwardEvent:   forwardEvent,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// c, err := controller.New("remotewatcher-controller", mgr, controller.Options{Reconciler: r})
	// if err != nil {
	// 	return err
	// }

	return nil
}

var _ reconcile.Reconciler = &ReconcileRemoteWatcher{}

// ReconcileRemoteWatcher reconciles a RemoteWatcher object
type ReconcileRemoteWatcher struct {
	client.Client
	scheme *runtime.Scheme
	// channel to forward GenericEvents to
	ForwardChannel chan event.GenericEvent
	// Event to forward when this controller gets event
	ForwardEvent event.GenericEvent
}

// Reconcile reads that state of the cluster for a RemoteWatcher object and makes changes
func (r *ReconcileRemoteWatcher) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Reset()
	// Forward a known Event back to the parent controller
	r.ForwardChannel <- r.ForwardEvent
	return reconcile.Result{}, nil
}
