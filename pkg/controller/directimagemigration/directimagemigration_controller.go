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

package directimagemigration

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

var log = logging.WithName("directimage")

// Add creates a new DirectImageMigration Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDirectImageMigration{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetRecorder("directimagemigration_controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("directimagemigration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DirectImageMigration
	err = c.Watch(&source.Kind{Type: &migapi.DirectImageMigration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to MigClusters referenced by DirectImageMigrations
	err = c.Watch(
		&source.Kind{Type: &migapi.MigCluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.DirectImageMigration{})
				}),
		},
		//		&ClusterPredicate{}
	)
	if err != nil {
		return err
	}

	// Watch for changes to DirectImageStreamMigrations
	err = c.Watch(&source.Kind{Type: &migapi.DirectImageStreamMigration{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &migapi.DirectImageMigration{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDirectImageMigration{}

// ReconcileDirectImageMigration reconciles a DirectImageMigration object
type ReconcileDirectImageMigration struct {
	client.Client
	record.EventRecorder
	scheme *runtime.Scheme
	tracer opentracing.Tracer
}

// Reconcile reads that state of the cluster for a DirectImageMigration object and makes changes based on the state read
// and what is in the DirectImageMigration.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directimagemigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=directimagemigrations/status,verbs=get;update;patch
func (r *ReconcileDirectImageMigration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the DirectImageMigration instance
	log.Reset()
	log.SetValues("DirectImageMigration", request.Name)
	imageMigration := &migapi.DirectImageMigration{}
	err := r.Get(context.TODO(), request.NamespacedName, imageMigration)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: true}, err
	}

	// Set up jaeger tracing
	reconcileSpan := r.initTracer(imageMigration)
	if reconcileSpan != nil {
		defer reconcileSpan.Finish()
	}

	// Completed.
	if imageMigration.Status.Phase == Completed {
		return reconcile.Result{}, nil
	}

	// Begin staging conditions
	imageMigration.Status.BeginStagingConditions()

	// Validation
	err = r.validate(imageMigration)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Default to PollReQ, can be overridden by r.migrate phase-specific ReQ interval
	requeueAfter := time.Duration(PollReQ)

	if !imageMigration.Status.HasBlockerCondition() {
		requeueAfter, err = r.migrate(imageMigration, reconcileSpan)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Set to ready
	imageMigration.Status.SetReady(
		imageMigration.Status.Phase != Completed &&
			!imageMigration.Status.HasBlockerCondition(),
		"Image migration is ready")

	// End staging conditions
	imageMigration.Status.EndStagingConditions()

	// Apply changes
	imageMigration.MarkReconciled()
	err = r.Update(context.TODO(), imageMigration)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Requeue
	if requeueAfter > 0 {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	return reconcile.Result{}, nil
}
