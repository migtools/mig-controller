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
	"time"

	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/opentracing/opentracing-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("directvolume")

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
	err = c.Watch(&source.Kind{Type: &migapi.DirectVolumeMigration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &migapi.DirectVolumeMigrationProgress{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &migapi.DirectVolumeMigration{},
		})
	if err != nil {
		return err
	}

	// TODO: Modify this to watch the proper list of resources

	return nil
}

var _ reconcile.Reconciler = &ReconcileDirectVolumeMigration{}

// ReconcileDirectVolumeMigration reconciles a DirectVolumeMigration object
type ReconcileDirectVolumeMigration struct {
	client.Client
	scheme *runtime.Scheme
	tracer opentracing.Tracer
}

// Reconcile reads that state of the cluster for a DirectVolumeMigration object and makes changes based on the state read
// and what is in the DirectVolumeMigration.Spec
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directvolumemigrations,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directvolumemigrations/status,verbs=get;update;patch
func (r *ReconcileDirectVolumeMigration) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	// Set values
	log = logging.WithName("directvolume", "dvm", request.Name)

	// Fetch the DirectVolumeMigration instance
	direct := &migapi.DirectVolumeMigration{}
	err := r.Get(context.TODO(), request.NamespacedName, direct)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{Requeue: false}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: true}, err
	}

	// Set MigMigration name key on logger
	migration, err := direct.GetMigrationForDVM(r)
	if migration != nil {
		log.Real = log.WithValues("migMigration", migration.Name)
	}

	// Set up jaeger tracing, add to ctx
	reconcileSpan := r.initTracer(direct)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	// Check if completed
	if direct.Status.Phase == Completed {
		return reconcile.Result{Requeue: false}, nil
	}

	// Begin staging conditions
	direct.Status.BeginStagingConditions()

	// Validation
	err = r.validate(ctx, direct)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Default to PollReQ, can be overridden by r.migrate phase-specific ReQ interval
	requeueAfter := time.Duration(PollReQ)

	if !direct.Status.HasBlockerCondition() {
		requeueAfter, err = r.migrate(ctx, direct)
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

	// Requeue
	if requeueAfter > 0 {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	// Done
	return reconcile.Result{Requeue: false}, nil
}
