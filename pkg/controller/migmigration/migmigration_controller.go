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
	"reflect"
	"time"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	"github.com/fusor/mig-controller/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	// Fetch the MigMigration migMigration
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
		return reconcile.Result{}, err
	}

	// Don't do any more work if Migration is complete
	if migMigration.Status.MigrationCompleted == true {
		return reconcile.Result{}, nil // don't requeue
	}

	// Don't continue if not ready.
	if !migMigration.Status.IsReady() {
		return reconcile.Result{}, nil
	}

	// Retrieve MigPlan from ref
	migPlanRef := migMigration.Spec.MigPlanRef
	migPlan := &migapi.MigPlan{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: migPlanRef.Name, Namespace: migPlanRef.Namespace}, migPlan)
	if err != nil {
		log.Info(fmt.Sprintf("[mMigration] Error getting MigPlan [%s/%s]", migPlanRef.Namespace, migPlanRef.Name))
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	// [TODO] Create Velero BackupStorageLocation if needed

	// [TODO] Create Velero VolumeSnapshotLocation if needed

	// Retrieve MigAssetCollection from ref
	assetsRef := migPlan.Spec.MigAssetCollectionRef
	assets := &migapi.MigAssetCollection{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: assetsRef.Name, Namespace: assetsRef.Namespace}, assets)
	if err != nil {
		log.Info(fmt.Sprintf("[mMigration] Error getting MigAssetCollection [%s/%s]", assetsRef.Namespace, assetsRef.Name))
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	// If all references are marked as ready, set StartTimestamp and mark as running
	if migMigration.Status.MigrationCompleted == false && migMigration.Status.MigrationRunning == false {
		migMigration.Status.MigrationRunning = true
		migMigration.Status.MigrationCompleted = false
		migMigration.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
		err = r.Update(context.TODO(), migMigration)
		if err != nil {
			log.Error(err, "[mMigration] Failed to UPDATE MigMigration with 'StartTimestamp'")
			return reconcile.Result{}, err // requeue
		}
		log.Info(fmt.Sprintf("[mMigration] Started MigMigration [%s/%s]", migMigration.Namespace, migMigration.Name))
	}

	// *****************************
	// Get the srcCluster MigCluster
	srcClusterRef := migPlan.Spec.SrcMigClusterRef
	srcCluster := &migapi.MigCluster{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: srcClusterRef.Name, Namespace: srcClusterRef.Namespace}, srcCluster)
	if err != nil {
		log.Info(fmt.Sprintf("[mMigration] Error getting srcCluster MigCluster [%s/%s]", srcClusterRef.Namespace, srcClusterRef.Name))
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	srcClusterK8sClient, err := srcCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "Failed to GET srcClusterK8sClient")
		return reconcile.Result{}, nil
	}

	// Create Velero Backup on srcCluster looking at namespaces in MigAssetCollection referenced by MigPlan
	backupNamespaces := assets.Spec.Namespaces
	backupUniqueName := fmt.Sprintf("%s-velero-backup", migMigration.Name)
	vBackupNew := util.BuildVeleroBackup(veleroNs, backupUniqueName, backupNamespaces)

	vBackupExisting := &velerov1.Backup{}
	err = srcClusterK8sClient.Get(context.TODO(), types.NamespacedName{Name: vBackupNew.Name, Namespace: vBackupNew.Namespace}, vBackupExisting)
	if err != nil {
		if errors.IsNotFound(err) {
			// Backup not found
			err = srcClusterK8sClient.Create(context.TODO(), vBackupNew)
			if err != nil {
				log.Error(err, "[mMigration] Failed to CREATE Velero Backup on source cluster")
				return reconcile.Result{}, nil
			}
			log.Info("[mMigration] Velero Backup CREATED successfully on source cluster")
		}
		// Error reading the 'Backup' object - requeue the request.
		log.Error(err, "[mMigration] Exit 4: Requeueing")
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(vBackupNew.Spec, vBackupExisting.Spec) {
		// Send "Create" action for Velero Backup to K8s API
		vBackupExisting.Spec = vBackupNew.Spec
		err = srcClusterK8sClient.Update(context.TODO(), vBackupExisting)
		if err != nil {
			log.Error(err, "[mMigration] Failed to UPDATE Velero Backup on src cluster")
			return reconcile.Result{}, nil
		}
		log.Info("[mMigration] Velero Backup UPDATED successfully on source cluster")
	} else {
		log.Info("[mMigration] Velero Backup EXISTS on source cluster")
	}

	// ******************************
	// Get the destCluster MigCluster
	destClusterRef := migPlan.Spec.DestMigClusterRef
	destCluster := &migapi.MigCluster{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: destClusterRef.Name, Namespace: destClusterRef.Namespace}, destCluster)
	if err != nil {
		log.Info(fmt.Sprintf("[mMigration] Error getting destCluster MigCluster [%s/%s]", destClusterRef.Name, destClusterRef.Namespace))
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	destClusterK8sClient, err := destCluster.BuildControllerRuntimeClient(r.Client)
	if err != nil {
		log.Error(err, "Failed to GET destClusterK8sClient")
		return reconcile.Result{}, nil
	}

	// Create Velero Restore on dstCluster pointing at Velero Backup unique name
	restoreUniqueName := fmt.Sprintf("%s-velero-restore", migMigration.Name)
	vRestoreNew := util.BuildVeleroRestore(veleroNs, restoreUniqueName, backupUniqueName)

	vRestoreExisting := &velerov1.Restore{}
	err = destClusterK8sClient.Get(context.TODO(), types.NamespacedName{Name: vRestoreNew.Name, Namespace: vRestoreNew.Namespace}, vRestoreExisting)
	if err != nil {
		if errors.IsNotFound(err) {
			// Restore not found, we need to check if the Backup we want to use has completed
			vBackupDestCluster := &velerov1.Backup{}
			err2 := destClusterK8sClient.Get(context.TODO(), types.NamespacedName{Name: backupUniqueName, Namespace: veleroNs}, vBackupDestCluster)
			if err2 != nil {
				if errors.IsNotFound(err2) {
					log.Info(fmt.Sprintf("[mMigration] Velero Backup doesn't yet exist on destination cluster [%s/%s], waiting...", veleroNs, backupUniqueName))
					return reconcile.Result{}, nil // don't requeue
				}
			}

			if vBackupDestCluster.Status.Phase != velerov1.BackupPhaseCompleted {
				log.Info(fmt.Sprintf("[mMigration] Velero Backup on destination cluster in unusable phase [%s] [%s/%s]", vBackupDestCluster.Status.Phase, veleroNs, backupUniqueName))
				return reconcile.Result{}, nil // don't requeue
			}

			log.Info(fmt.Sprintf("[mMigration] Found completed Backup on destination cluster [%s/%s], creating Restore on destination cluster", veleroNs, backupUniqueName))
			// Create a restore once we're certain that the required Backup exists
			err = destClusterK8sClient.Create(context.TODO(), vRestoreNew)
			if err != nil {
				log.Error(err, "[mMigration] Failed to CREATE Velero Restore on destination cluster")
				return reconcile.Result{}, nil
			}
			log.Info("[mMigration] Velero Restore CREATED successfully on destination cluster")
		}
		// Error reading the 'Backup' object - requeue the request.
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(vRestoreNew.Spec, vRestoreExisting.Spec) {
		// Send "Create" action for Velero Backup to K8s API
		vRestoreExisting.Spec = vRestoreNew.Spec
		err = destClusterK8sClient.Update(context.TODO(), vRestoreExisting)
		if err != nil {
			log.Error(err, "[mMigration] Failed to UPDATE Velero Restore")
			return reconcile.Result{}, nil
		}
		log.Info("[mMigration] Velero Restore UPDATED successfully on destination cluster")
	} else {
		log.Info("[mMigration] Velero Restore EXISTS on destination cluster")
	}

	// Monitor changes to Velero Restore state on dstCluster, wait for completion (watch MigCluster)

	// Mark MigMigration as complete
	// If all references are marked as ready, set StartTimestamp and mark as running
	if vRestoreExisting.Status.Phase == velerov1.RestorePhaseCompleted {
		if migMigration.Status.MigrationRunning == true && migMigration.Status.MigrationCompleted == false {
			migMigration.Status.MigrationRunning = false
			migMigration.Status.MigrationCompleted = true
			migMigration.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
			err = r.Update(context.TODO(), migMigration)
			if err != nil {
				log.Error(err, "[mMigration] Failed to UPDATE MigMigration with 'CompletionTimestamp'")
				return reconcile.Result{}, err // requeue
			}
			log.Info(fmt.Sprintf("[mMigration] Finished MigMigration [%s/%s]", migMigration.Namespace, migMigration.Name))
		}
	}

	return reconcile.Result{}, nil
}
