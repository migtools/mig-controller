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

package directvolumemigration

import (
	"context"

	"github.com/konveyor/controller/pkg/logging"
	migrationv1alpha1 "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	//corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("direct")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DirectVolumeMigration Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDirectVolumeMigration{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("directvolumemigration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DirectVolumeMigration
	err = c.Watch(&source.Kind{Type: &migrationv1alpha1.DirectVolumeMigration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by DirectVolumeMigration - change this for objects you create
	/*err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &migrationv1alpha1.DirectVolumeMigration{},
	})
	if err != nil {
		return err
	}*/

	return nil
}

var _ reconcile.Reconciler = &ReconcileDirectVolumeMigration{}

// ReconcileDirectVolumeMigration reconciles a DirectVolumeMigration object
type ReconcileDirectVolumeMigration struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DirectVolumeMigration object and makes changes based on the state read
// and what is in the DirectVolumeMigration.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directvolumemigrations,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directvolumemigrations/status,verbs=get;update;patch
func (r *ReconcileDirectVolumeMigration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the DirectVolumeMigration instance
	direct := &migrationv1alpha1.DirectVolumeMigration{}
	err := r.Get(context.TODO(), request.NamespacedName, direct)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Check if completed
	if direct.Status.Phase == Completed {
		return reconcile.Result{}, nil
	}

	// Begin staging conditions
	direct.Status.BeginStagingConditions()

	// Validation
	err = r.validate(direct)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	if !direct.Status.HasBlockerCondition() {
		_, err = r.migrate(direct)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Set to ready
	direct.Status.SetReady(
		direct.Status.Phase != Completed &&
			!direct.Status.HasBlockerCondition(),
		ReadyMessage)

	// End staging conditions
	direct.Status.EndStagingConditions()

	// Apply changes
	direct.MarkReconciled()
	err = r.Update(context.TODO(), direct)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{}, nil
}
