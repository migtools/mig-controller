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

package migstage

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("stage")

const logPrefix = "mStage"

// TODO: don't hard-code veleroNs
const veleroNs = "velero"

// Add creates a new MigStage Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigStage{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("migstage-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MigStage
	err = c.Watch(
		&source.Kind{Type: &migapi.MigStage{}},
		&handler.EnqueueRequestForObject{},
		&StagePredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to MigPlans referenced by MigStages
	err = c.Watch(
		&source.Kind{Type: &migapi.MigPlan{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigStage{})
				}),
		})
	if err != nil {
		return err
	}

	// Watch for changes to MigClusters referenced by MigStages
	err = c.Watch(
		&source.Kind{Type: &migapi.MigCluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigStage{})
				}),
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigStage{}

// ReconcileMigStage reconciles a MigStage object
type ReconcileMigStage struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MigStage object and makes changes based on the state read
// and what is in the MigStage.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migstages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migstages/status,verbs=get;update;patch
func (r *ReconcileMigStage) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("Reconcile", "request", request)

	// Fetch the MigStage instance
	migStage := &migapi.MigStage{}
	err := r.Get(context.TODO(), request.NamespacedName, migStage)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Validate
	_, err = r.validate(migStage)
	if err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	if !migStage.Status.IsReady() {
		return reconcile.Result{}, err
	}

	err = r.stage(migStage)
	if err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		} else {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
