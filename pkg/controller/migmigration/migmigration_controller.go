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
	vrunner "github.com/fusor/mig-controller/pkg/velerorunner"

	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
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

const logPrefix = "mMigration"

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

// Reconcile performs Migrations based on the data in MigMigration
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations/status,verbs=get;update;patch
func (r *ReconcileMigMigration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info(fmt.Sprintf("[mMigration] RECONCILE [%s/%s]", request.Namespace, request.Name))

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
	_, err = r.validate(migMigration)
	if err != nil {
		return reconcile.Result{}, err // requeue
	}
	// Return if MigMigration is complete
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
	changed := migMigration.MarkAsRunning()
	if changed {
		err = r.Update(context.TODO(), migMigration)
		if err != nil {
			log.Info("[mMigration] Failed to mark MigMigration [%s/%s] as running", migMigration.Namespace, migMigration.Name)
			return reconcile.Result{}, err // requeue
		}
		log.Info(fmt.Sprintf("[mMigration] STARTED MigMigration [%s/%s]", migMigration.Namespace, migMigration.Name))
	}
	// ******************************
	// Build controller-runtime client for srcMigCluster
	srcClusterK8sClient, err := rres.srcMigCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "Failed to GET srcClusterK8sClient")
		return reconcile.Result{}, nil // don't requeue
	}

	// Create Velero Backup on srcCluster looking at namespaces in MigAssetCollection
	var backupNsName types.NamespacedName
	if migMigration.Status.SrcBackupRef == nil {
		backupNsName = types.NamespacedName{
			Name:      migMigration.Name + "-",
			Namespace: veleroNs,
		}
	} else {
		backupNsName = types.NamespacedName{
			Name:      migMigration.Status.SrcBackupRef.Name,
			Namespace: migMigration.Status.SrcBackupRef.Namespace,
		}
	}
	srcBackup, err := vrunner.RunBackup(srcClusterK8sClient, backupNsName, rres.migAssets, logPrefix)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}
	if srcBackup == nil {
		return reconcile.Result{}, nil // don't requeue
	}

	// Update MigMigration with reference to Velero Backup
	migMigration.Status.SrcBackupRef = &kapi.ObjectReference{Name: srcBackup.Name, Namespace: srcBackup.Namespace}
	err = r.Update(context.TODO(), migMigration)
	if err != nil {
		log.Info(fmt.Sprintf("[mMigration] Failed to UPDATE MigMigration with Velero Backup reference"))
		return reconcile.Result{}, err // requeue
	}

	// ******************************
	// Build controller-runtime client for destMigCluster
	destClusterK8sClient, err := rres.destMigCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "[mMigration] Failed to GET destClusterK8sClient")
		return reconcile.Result{}, nil // don't requeue
	}

	// Create Velero Restore on destMigCluster pointing at Velero Backup unique name
	var restoreNsName types.NamespacedName
	if migMigration.Status.DestRestoreRef == nil {
		restoreNsName = types.NamespacedName{
			Name:      migMigration.Name + "-",
			Namespace: veleroNs,
		}
	} else {
		restoreNsName = types.NamespacedName{
			Name:      migMigration.Status.DestRestoreRef.Name,
			Namespace: migMigration.Status.DestRestoreRef.Namespace,
		}
	}
	destRestore, err := vrunner.RunRestore(destClusterK8sClient, restoreNsName, backupNsName, logPrefix)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}
	if destRestore == nil {
		return reconcile.Result{}, nil // don't requeue
	}

	// Update MigMigration with reference to Velero Retore
	migMigration.Status.DestRestoreRef = &kapi.ObjectReference{Name: destRestore.Name, Namespace: destRestore.Namespace}
	err = r.Update(context.TODO(), migMigration)
	if err != nil {
		log.Info(fmt.Sprintf("[mMigration] Failed to UPDATE MigMigration with Velero Restore reference"))
		return reconcile.Result{}, err // requeue
	}

	// Mark MigMigration as complete if Velero Restore has completed
	if destRestore.Status.Phase == velerov1.RestorePhaseCompleted {
		changed = migMigration.MarkAsCompleted()
		if changed {
			err = r.Update(context.TODO(), migMigration)
			if err != nil {
				log.Info("[mMigration] Failed to mark MigMigration [%s/%s] as completed", migMigration.Namespace, migMigration.Name)
				return reconcile.Result{}, err // requeue
			}
			log.Info(fmt.Sprintf("[mMigration] FINISHED MigMigration [%s/%s]", migMigration.Namespace, migMigration.Name))
		}
	}

	return reconcile.Result{}, nil
}
