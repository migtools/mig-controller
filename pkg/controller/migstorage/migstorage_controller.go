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

package migstorage

import (
	"context"
	"github.com/fusor/mig-controller/pkg/logging"
	"k8s.io/client-go/tools/record"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("storage")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MigStorage Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigStorage{
		Client:   mgr.GetClient(),
		recorder: mgr.GetRecorder("storage-controller"),
		scheme:   mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("migstorage-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to MigStorage
	err = c.Watch(
		&source.Kind{Type: &migapi.MigStorage{}},
		&handler.EnqueueRequestForObject{},
		&StoragePredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to Secrets referenced by MigStorage.
	err = c.Watch(
		&source.Kind{Type: &kapi.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigStorage{})
				}),
		})
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigStorage{}

// ReconcileMigStorage reconciles a MigStorage object
type ReconcileMigStorage struct {
	client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migstorages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migstorages/status,verbs=get;update;patch
func (r *ReconcileMigStorage) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error
	log.Reset()

	// Fetch the MigStorage instance
	storage := &migapi.MigStorage{}
	err = r.Get(context.TODO(), request.NamespacedName, storage)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Report reconcile error.
	defer func() {
		if err == nil || errors.IsConflict(err) {
			return
		}
		r.recorder.Eventf(
			storage,
			kapi.EventTypeWarning,
			"ReconcileFailed",
			"Reconcile failed: [%s].",
			err)
		storage.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), storage)
		if err != nil {
			log.Trace(err)
			return
		}
	}()

	// Begin staging conditions.
	storage.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(storage)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Ready
	storage.Status.SetReady(
		!storage.Status.HasBlockerCondition(),
		ReadyMessage)

	// End staging conditions.
	storage.Status.EndStagingConditions()

	// Apply changes.
	storage.Touch()
	err = r.Update(context.TODO(), storage)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{}, nil
}
