/*
Copyright 2020 Red Hat Inc.

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

package directimagestreammigration

import (
	"context"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("directimagestream")

// Add creates a new DirectImageStreamMigration Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDirectImageStreamMigration{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetEventRecorderFor("directimagestreammigration_controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("directimagestreammigration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DirectImageStreamMigration
	err = c.Watch(&source.Kind{Type: &migapi.DirectImageStreamMigration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to MigClusters referenced by DirectImageMigrations
	err = c.Watch(
		&source.Kind{Type: &migapi.MigCluster{}},
		handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			return migref.GetRequests(a, migapi.DirectImageStreamMigration{})
		}),
	)
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDirectImageStreamMigration{}

// ReconcileDirectImageStreamMigration reconciles a DirectImageStreamMigration object
type ReconcileDirectImageStreamMigration struct {
	client.Client
	record.EventRecorder
	scheme *runtime.Scheme
	tracer opentracing.Tracer
}

// Reconcile reads that state of the cluster for a DirectImageStreamMigration object and makes changes based on the state read
// and what is in the DirectImageStreamMigration.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directimagestreammigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directimagestreammigrations/status,verbs=get;update;patch
func (r *ReconcileDirectImageStreamMigration) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log = logging.WithName("directimagestream", "dism", request.Name)
	// Fetch the DirectImageStreamMigration instance
	imageStreamMigration := &migapi.DirectImageStreamMigration{}
	err := r.Get(context.TODO(), request.NamespacedName, imageStreamMigration)
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
	migration, err := imageStreamMigration.GetMigrationForDISM(r)
	if migration != nil {
		log.Real = log.WithValues("migMigration", migration.Name)
	}

	// Set up jaeger tracing, add to ctx
	reconcileSpan, err := r.initTracer(*imageStreamMigration)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	// Completed.
	if imageStreamMigration.Status.Phase == Completed {
		return reconcile.Result{Requeue: false}, nil
	}

	// Begin staging conditions
	imageStreamMigration.Status.BeginStagingConditions()

	// Validation
	err = r.validate(ctx, imageStreamMigration)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Default to PollReQ, can be overridden by r.migrate phase-specific ReQ interval
	requeueAfter := time.Duration(PollReQ)

	if !imageStreamMigration.Status.HasBlockerCondition() {
		requeueAfter, err = r.migrate(ctx, imageStreamMigration)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Set to ready
	imageStreamMigration.Status.SetReady(
		imageStreamMigration.Status.Phase != Completed &&
			!imageStreamMigration.Status.HasBlockerCondition(),
		"ImageStream migration is ready")

	// End staging conditions
	imageStreamMigration.Status.EndStagingConditions()

	// Apply changes
	imageStreamMigration.MarkReconciled()
	err = r.Update(context.TODO(), imageStreamMigration)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Requeue
	if requeueAfter > 0 {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	return reconcile.Result{Requeue: false}, nil
}
