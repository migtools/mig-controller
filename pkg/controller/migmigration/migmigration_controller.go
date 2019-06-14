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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"time"

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

var log = logf.Log.WithName("migration")

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
		},
		&PlanPredicate{})
	if err != nil {
		return err
	}

	// Indexes
	indexer := mgr.GetFieldIndexer()

	// Plan
	err = indexer.IndexField(
		&migapi.MigMigration{},
		migapi.PlanIndexField,
		func(rawObj runtime.Object) []string {
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

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigMigration{}

// ReconcileMigMigration reconciles a MigMigration object
type ReconcileMigMigration struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile performs Migrations based on the data in MigMigration
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migmigrations/status,verbs=get;update;patch
func (r *ReconcileMigMigration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log = logf.Log.WithName(names.SimpleNameGenerator.GenerateName("migration|"))

	// Retrieve the MigMigration being reconciled
	migration := &migapi.MigMigration{}
	err := r.Get(context.TODO(), request.NamespacedName, migration)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.deleted()
		}
		return reconcile.Result{}, err
	}

	// Completed.
	if migration.IsCompleted() {
		return reconcile.Result{}, nil
	}

	// Re-queue (after) in seconds.
	requeueAfter := 0 // not re-queued.

	// Begin staging conditions.
	migration.Status.BeginStagingConditions()

	// Validate
	err = r.validate(migration)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Ensure that migrations run serially ordered by when created
	// and grouped with stage migrations followed by final migrations.
	// Reconcile of a migration not in the desired order will be postponed.
	if !migration.Status.HasBlockerCondition() {
		requeueAfter, err = r.postpone(migration)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Migrate
	if !migration.Status.HasBlockerCondition() {
		requeueAfter, err = r.migrate(migration)
		if err != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			} else {
				return reconcile.Result{}, err
			}
		}
	}

	// Ready
	migration.Status.SetReady(
		!migration.Status.HasBlockerCondition(),
		ReadyMessage)

	// End staging conditions.
	migration.Status.EndStagingConditions()

	// Apply changes.
	migration.Touch()
	err = r.Update(context.TODO(), migration)
	if err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	// Requeue
	if requeueAfter > 0 {
		delay := time.Second * time.Duration(requeueAfter)
		return reconcile.Result{RequeueAfter: delay}, nil
	}

	return reconcile.Result{}, nil
}

// Determine if a migration should be postponed.
// Migrations run serially ordered by created timestamp and grouped
// with stage migrations followed by final migrations. A migration is
// postponed when not in the desired order.
// When postponed:
//   - Returns: a requeueAfter in seconds, else 0 (not postponed).
//   - Sets the `Postponed` condition.
func (r *ReconcileMigMigration) postpone(migration *migapi.MigMigration) (int, error) {
	plan, err := migration.GetPlan(r)
	if err != nil {
		return 0, err
	}
	migrations, err := plan.ListMigrations(r)
	if err != nil {
		return 0, err
	}
	// Pending migrations.
	pending := []types.UID{}
	for _, m := range migrations {
		if !m.IsCompleted() {
			pending = append(pending, m.UID)
		}
	}

	// This migration is next.
	if len(pending) == 0 || pending[0] == migration.UID {
		return 0, nil
	}

	// Postpone
	requeueAfter := 10
	for position, uid := range pending {
		if uid == migration.UID {
			requeueAfter = position * 10
			break
		}
	}
	migration.Status.SetCondition(migapi.Condition{
		Type:     Postponed,
		Status:   True,
		Category: Critical,
		Message:  fmt.Sprintf(PostponedMessage, requeueAfter),
	})

	return requeueAfter, nil
}

// Migration has been deleted.
// Delete the `HasFinalMigration` condition on all other uncompleted migrations.
func (r *ReconcileMigMigration) deleted() error {
	migrations, err := migapi.ListMigrations(r)
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.IsCompleted() || !m.Status.HasCondition(HasFinalMigration) {
			continue
		}
		m.Status.DeleteCondition(HasFinalMigration)
		err := r.Update(context.TODO(), &m)
		if err != nil {
			return err
		}
	}

	return nil
}
