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

package migmigration

import (
	"context"
	"fmt"

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

var log = logf.Log.WithName("controller")

const logPrefix = "mMigration"

// TODO: don't hard-code veleroNs
const veleroNs = "velero"

// Add creates a new MigMigration Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigMigration{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("migmigration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MigMigration
	err = c.Watch(
		&source.Kind{Type: &migapi.MigMigration{}},
		&handler.EnqueueRequestForObject{},
		&MigrationPredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to MigPlans referenced by MigMigrations
	err = c.Watch(
		&source.Kind{Type: &migapi.MigPlan{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigMigration{})
				}),
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigMigration{}

// ReconcileMigMigration reconciles a MigMigration object
type ReconcileMigMigration struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile performs Migrations based on the data in MigMigration
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations/status,verbs=get;update;patch
func (r *ReconcileMigMigration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info(fmt.Sprintf("[%s] RECONCILE [%s/%s]", logPrefix, request.Namespace, request.Name))

	// Retrieve the MigMigration being reconciled
	migMigration := &migapi.MigMigration{}
	err := r.Get(context.TODO(), request.NamespacedName, migMigration)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	// Validate
	_, err = r.validate(migMigration)
	if err != nil {
		return reconcile.Result{}, err // requeue
	}

	var rres *reconcileResources

	// Perform prechecks and gather resources needed for reconcile
	rres, err = r.initReconcile(migMigration)
	if rres == nil {
		return reconcile.Result{}, err
	}

	// Mark the MigMigration as started once prechecks are passed
	rres, err = r.startMigMigration(migMigration, rres)
	if rres == nil {
		return reconcile.Result{}, err
	}

	// Ensure source cluster has a Velero Backup
	rres, err = r.ensureSourceClusterBackup(migMigration, rres)
	if rres == nil {
		return reconcile.Result{}, err
	}

	// Ensure destination cluster has a Velero Backup + Restore
	rres, err = r.ensureDestinationClusterRestore(migMigration, rres)
	if rres == nil {
		return reconcile.Result{}, err
	}

	// Mark MigMigration as complete if Velero Restore has completed
	rres, err = r.finishMigMigration(migMigration, rres)
	if rres == nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
