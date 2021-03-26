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

package migmigration

import (
	"context"
	"fmt"
	"time"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/cache"
	"github.com/konveyor/mig-controller/pkg/errorutil"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("migration")

// Add creates a new MigMigration Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigMigration{
		Client:           mgr.GetClient(),
		scheme:           mgr.GetScheme(),
		EventRecorder:    mgr.GetEventRecorderFor("migmigration_controller"),
		uidGenerationMap: cache.CreateUIDToGenerationMap(),
	}
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
		handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			return migref.GetRequests(a, migapi.MigMigration{})
		}),
		&PlanPredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to DirectImageMigrations
	err = c.Watch(&source.Kind{Type: &migapi.DirectImageMigration{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &migapi.MigMigration{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to DirectImageMigrations
	err = c.Watch(&source.Kind{Type: &migapi.DirectVolumeMigration{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &migapi.MigMigration{},
	})
	if err != nil {
		return err
	}

	// Indexes
	indexer := mgr.GetFieldIndexer()

	// Plan
	err = indexer.IndexField(
		context.Background(),
		&migapi.MigMigration{},
		migapi.PlanIndexField,
		func(rawObj client.Object) []string {
			m, cast := rawObj.(*migapi.MigMigration)
			if !cast {
				return nil
			}
			ref := m.Spec.MigPlanRef
			if !migref.RefSet(ref) {
				return nil
			}
			return []string{
				fmt.Sprintf("%s/%s", ref.Namespace, ref.Name),
			}
		})
	if err != nil {
		return err
	}

	// Event
	err = indexer.IndexField(
		context.TODO(),
		&kapi.Event{},
		"involvedObject.uid",
		func(rawObj k8sclient.Object) []string {
			e, cast := rawObj.(*kapi.Event)
			if !cast {
				return nil
			}
			return []string{
				string(e.InvolvedObject.UID),
			}
		})
	if err != nil {
		return err
	}

	// Gather migration metrics every 10 seconds
	recordMetrics(mgr.GetClient())

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigMigration{}

// ReconcileMigMigration reconciles a MigMigration object
type ReconcileMigMigration struct {
	client.Client
	record.EventRecorder

	scheme           *runtime.Scheme
	tracer           opentracing.Tracer
	uidGenerationMap *cache.UIDToGenerationMap
}

// Reconcile performs Migrations based on the data in MigMigration
func (r *ReconcileMigMigration) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var err error
	// Set values.
	log = logging.WithName("migration", "migMigration", request.Name)

	// Retrieve the MigMigration being reconciled
	migration := &migapi.MigMigration{}
	err = r.Get(context.TODO(), request.NamespacedName, migration)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.deleted()
			return reconcile.Result{Requeue: false}, nil
		}
		log.Info("Error getting migmigration for reconcile, requeueing.")
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Check if cache is still catching up
	if r.uidGenerationMap.IsCacheStale(migration.UID, migration.Generation) {
		return reconcile.Result{Requeue: true}, nil
	}

	// Get jaeger spans for migration and reconcile, add to ctx
	_, reconcileSpan := r.initTracer(migration)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	// Ensure debugging labels are present on migmigration
	err = r.ensureDebugLabels(migration)
	if err != nil {
		log.Info("Error setting debug labels, requeueing.")
		log.Trace(err)
		return reconcile.Result{Requeue: true}, err
	}

	// Report reconcile error.
	defer func() {
		// Only log critical conditions.
		critConditions := migration.Status.Conditions.FindConditionByCategory(Critical)
		if len(critConditions) > 0 {
			log.Info("CR", "critical_conditions", critConditions)
		}
		migration.Status.Conditions.RecordEvents(migration, r.EventRecorder)
		if err == nil || errors.IsConflict(errorutil.Unwrap(err)) {
			return
		}
		migration.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), migration)
		if err != nil {
			log.Error(err, "Error updating resource on reconcile exit.")
			log.Trace(err)
			return
		}
	}()

	// Completed.
	if migration.Status.Phase == Completed {
		return reconcile.Result{Requeue: false}, nil
	}

	// Owner Reference
	err = r.setOwnerReference(migration)
	if err != nil {
		log.Info("Failed to set owner references, requeuing.")
		log.Trace(err)
		return reconcile.Result{Requeue: true}, err
	}

	// Begin staging conditions.
	migration.Status.BeginStagingConditions()

	// Validate
	err = r.validate(ctx, migration)
	if err != nil {
		log.Info("Validation failed, requeueing")
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Default to PollReQ, can be overridden by r.postpone() or r.migrate()
	requeueAfter := time.Duration(PollReQ)

	// Ensure that migrations run serially ordered by when created
	// and grouped with stage migrations followed by final migrations.
	// Reconcile of a migration not in the desired order will be postponed.
	if !migration.Status.HasBlockerCondition() {
		requeueAfter, err = r.postpone(migration)
		if err != nil {
			log.Info("Failed to check if postpone required, requeueing")
			log.Trace(err)
			return reconcile.Result{Requeue: true}, err
		}
	}

	// Migrate
	if !migration.Status.HasBlockerCondition() {
		requeueAfter, err = r.migrate(ctx, migration)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Ready
	migration.Status.SetReady(
		migration.Status.Phase != Completed &&
			!migration.Status.HasBlockerCondition(),
		"The migration is ready.")

	// End staging conditions.
	migration.Status.EndStagingConditions()

	// Apply changes.
	migration.MarkReconciled()
	err = r.Update(context.TODO(), migration)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Record reconciled generation
	r.uidGenerationMap.RecordReconciledGeneration(migration.UID, migration.Generation)

	// Requeue
	if requeueAfter > 0 {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	return reconcile.Result{Requeue: false}, nil
}

// Determine if a migration should be postponed.
// Migrations run serially ordered by created timestamp and grouped
// with stage migrations followed by final migrations. A migration is
// postponed when not in the desired order.
// When postponed:
//   - Returns: a requeueAfter as time.Duration, else 0 (not postponed).
//   - Sets the `Postponed` condition.
func (r *ReconcileMigMigration) postpone(migration *migapi.MigMigration) (time.Duration, error) {
	plan, err := migration.GetPlan(r)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	migrations, err := plan.ListMigrations(r)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	// Pending migrations.
	pending := []types.UID{}
	for _, m := range migrations {
		if m.Status.Phase != Completed {
			pending = append(pending, m.UID)
		}
	}

	// This migration is next.
	if len(pending) == 0 || pending[0] == migration.UID {
		return 0, nil
	}

	// Postpone
	requeueAfter := time.Second * 10
	for position, uid := range pending {
		if uid == migration.UID {
			requeueAfter = time.Second * time.Duration(position*10)
			break
		}
	}
	migration.Status.SetCondition(migapi.Condition{
		Type:     Postponed,
		Status:   True,
		Category: Critical,
		Message:  fmt.Sprintf("Postponed %d seconds to ensure migrations run serially and in order.", requeueAfter/time.Second),
	})

	return requeueAfter, nil
}

// Migration has been deleted.
// Delete the `HasFinalMigration` condition on all other uncompleted migrations.
func (r *ReconcileMigMigration) deleted() error {
	migrations, err := migapi.ListMigrations(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, m := range migrations {
		if m.Status.Phase == Completed || !m.Status.HasCondition(HasFinalMigration) {
			continue
		}
		m.Status.DeleteCondition(HasFinalMigration)
		err := r.Update(context.TODO(), &m)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Set the owner reference is set to the plan.
func (r *ReconcileMigMigration) setOwnerReference(migration *migapi.MigMigration) error {
	plan, err := migapi.GetPlan(r, migration.Spec.MigPlanRef)
	if err != nil {
		return liberr.Wrap(err)
	}
	if plan == nil {
		return nil
	}
	for i := range migration.OwnerReferences {
		ref := &migration.OwnerReferences[i]
		if ref.Kind == plan.Kind {
			ref.APIVersion = plan.APIVersion
			ref.Name = plan.Name
			ref.UID = plan.UID
			return nil
		}
	}
	migration.OwnerReferences = append(
		migration.OwnerReferences,
		v1.OwnerReference{
			APIVersion: plan.APIVersion,
			Kind:       plan.Kind,
			Name:       plan.Name,
			UID:        plan.UID,
		})

	return nil
}

// Ensures that the labels required to assist debugging are present on migmigration
func (r *ReconcileMigMigration) ensureDebugLabels(migration *migapi.MigMigration) error {
	if migration.Spec.MigPlanRef == nil {
		return nil
	}
	if migration.Labels == nil {
		migration.Labels = make(map[string]string)
	} else {
		if value, exists := migration.Labels[migapi.MigPlanDebugLabel]; exists {
			if value == migration.Spec.MigPlanRef.Name {
				return nil
			}
		}
	}
	migration.Labels[migapi.MigPlanDebugLabel] = migration.Spec.MigPlanRef.Name
	err := r.Update(context.TODO(), migration)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}
