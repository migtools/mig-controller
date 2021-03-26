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

package mighook

import (
	"context"

	"github.com/konveyor/mig-controller/pkg/cache"
	"github.com/konveyor/mig-controller/pkg/errorutil"
	"github.com/opentracing/opentracing-go"

	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
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

var log = logging.WithName("hook")

// Add creates a new MigHook Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigHook{
		Client:           mgr.GetClient(),
		scheme:           mgr.GetScheme(),
		EventRecorder:    mgr.GetEventRecorderFor("mighook_controller"),
		uidGenerationMap: cache.CreateUIDToGenerationMap(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("mighook-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MigHook
	err = c.Watch(
		&source.Kind{Type: &migapi.MigHook{}},
		&handler.EnqueueRequestForObject{},
		&HookPredicate{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigHook{}

// ReconcileMigHook reconciles a MigHook object
type ReconcileMigHook struct {
	client.Client
	record.EventRecorder

	scheme           *runtime.Scheme
	tracer           opentracing.Tracer
	uidGenerationMap *cache.UIDToGenerationMap
}

func (r *ReconcileMigHook) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var err error
	log = logging.WithName("hook", "migHook", request.Name)

	// Fetch the MigHook instance
	hook := &migapi.MigHook{}
	err = r.Get(context.TODO(), request.NamespacedName, hook)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Check if cache is still catching up
	if r.uidGenerationMap.IsCacheStale(hook.UID, hook.Generation) {
		return reconcile.Result{Requeue: true}, nil
	}

	// Get jaeger span for reconcile, add to ctx
	reconcileSpan := r.initTracer(hook)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	// Report reconcile error.
	defer func() {
		log.Info("CR", "conditions", hook.Status.Conditions)
		hook.Status.Conditions.RecordEvents(hook, r.EventRecorder)
		if err == nil || errors.IsConflict(errorutil.Unwrap(err)) {
			return
		}
		hook.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), hook)
		if err != nil {
			log.Trace(err)
			return
		}
	}()

	// Begin staging conditions.
	hook.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(ctx, hook)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Ready
	hook.Status.SetReady(
		!hook.Status.HasBlockerCondition(),
		"The hook is ready.")

	// End staging conditions.
	hook.Status.EndStagingConditions()

	// Apply changes.
	hook.MarkReconciled()
	err = r.Update(context.TODO(), hook)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Record reconciled generation
	r.uidGenerationMap.RecordReconciledGeneration(hook.UID, hook.Generation)

	// Done
	return reconcile.Result{Requeue: false}, nil
}
