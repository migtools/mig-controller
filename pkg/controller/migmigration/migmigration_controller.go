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

	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

// reconcileResources holds the data needed for MigMigration to reconcile.
// At the beginning of a reconcile, this data will be compiled by fetching
// information from each cluster involved in the migration.
type reconcileResources struct {
	migPlan        *migapi.MigPlan
	migAssets      *migapi.MigAssetCollection
	srcMigCluster  *migapi.MigCluster
	destMigCluster *migapi.MigCluster
	migStage       *migapi.MigStage

	backup  *velerov1.Backup
	restore *velerov1.Restore
}

// Reconcile performs Migrations based on the data in MigMigration
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations/status,verbs=get;update;patch
func (r *ReconcileMigMigration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info(fmt.Sprintf("[mMigration] RECONCILE [%s/%s]", request.Namespace, request.Name))
	// TODO: instead of individually getting each piece of the data model, see if it's
	// possible to write a function that will compile one struct with all of the info
	// we need to perform a Migration operation in one place.

	// Hardcode Velero namespace for now
	veleroNs := "velero"

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
	err, _ = r.validate(migMigration)
	if err != nil {
		return reconcile.Result{}, err // requeue
	}

	// Return if Migration is complete
	if migMigration.Status.MigrationCompleted == true {
		return reconcile.Result{}, nil // don't requeue
	}
	// Return if MigMigration isn't ready
	if !migMigration.Status.IsReady() {
		return reconcile.Result{}, nil //don't requeue
	}

	// Build ReconcileResources for MigMigration containing data needed for rest of reconcile process
	rres, err := r.getReconcileResources(migMigration)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	// If all references are marked as ready, run MarkAsRunning() to set this Migration into "Running" state
	err = migMigration.MarkAsRunning(r.Client)
	if err != nil {
		return reconcile.Result{}, err // requeue
	}

	// ******************************
	// Build controller-runtime client for srcMigCluster
	srcClusterK8sClient, err := rres.srcMigCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "Failed to GET srcClusterK8sClient")
		return reconcile.Result{}, nil
	}

	// Create Velero Backup on srcCluster looking at namespaces in MigAssetCollection referenced by MigPlan
	backupUniqueName := fmt.Sprintf("%s-velero-backup", migMigration.Name)
	backupNsName := types.NamespacedName{Name: backupUniqueName, Namespace: veleroNs}
	srcBackup, err := migMigration.RunBackup(srcClusterK8sClient, backupNsName, rres.migAssets)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}
	if srcBackup == nil {
		return reconcile.Result{}, nil // don't requeue
	}
	rres.backup = srcBackup

	// ******************************
	// Build controller-runtime client for destMigCluster
	destClusterK8sClient, err := rres.destMigCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "Failed to GET destClusterK8sClient")
		return reconcile.Result{}, nil
	}

	// Create Velero Restore on destMigCluster pointing at Velero Backup unique name
	restoreUniqueName := fmt.Sprintf("%s-velero-restore", migMigration.Name)
	restoreNsName := types.NamespacedName{Name: restoreUniqueName, Namespace: veleroNs}
	destRestore, err := migMigration.RunRestore(destClusterK8sClient, restoreNsName, backupNsName)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}
	if destRestore == nil {
		return reconcile.Result{}, nil // don't requeue
	}
	rres.restore = destRestore

	// Mark MigMigration as complete if Velero Restore has completed
	if rres.restore.Status.Phase == velerov1.RestorePhaseCompleted {
		err = migMigration.MarkAsCompleted(r.Client)
		if err != nil {
			return reconcile.Result{}, err // requeue
		}
	}

	return reconcile.Result{}, nil
}

// getReconcileResources gets all of the information needed to perform a reconcile
func (r *ReconcileMigMigration) getReconcileResources(migMigration *migapi.MigMigration) (*reconcileResources, error) {
	resources := &reconcileResources{}

	// MigPlan
	migPlan, err := migMigration.GetMigPlan(r.Client)
	if err != nil {
		return nil, err
	}
	resources.migPlan = migPlan

	// MigAssetCollection
	migAssets, err := migPlan.GetMigAssetCollection(r.Client)
	if err != nil {
		return nil, err
	}
	resources.migAssets = migAssets

	// SrcMigCluster
	srcMigCluster, err := migPlan.GetSrcMigCluster(r.Client)
	if err != nil {
		return nil, err
	}
	resources.srcMigCluster = srcMigCluster

	// DestMigCluster
	destMigCluster, err := migPlan.GetDestMigCluster(r.Client)
	if err != nil {
		return nil, err
	}
	resources.destMigCluster = destMigCluster

	// TODO - MigStage
	// TODO - Backup
	// TODO - Restore

	return resources, nil
}
