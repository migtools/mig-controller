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

package migplan

import (
	"context"
	"strconv"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migctl "github.com/fusor/mig-controller/pkg/controller/migmigration"
	"github.com/fusor/mig-controller/pkg/logging"
	migref "github.com/fusor/mig-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("plan")

// Add creates a new MigPlan Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigPlan{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("migplan-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to MigPlan
	err = c.Watch(&source.Kind{
		Type: &migapi.MigPlan{}},
		&handler.EnqueueRequestForObject{},
		&PlanPredicate{},
	)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to MigClusters referenced by MigPlans
	err = c.Watch(
		&source.Kind{Type: &migapi.MigCluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigPlan{})
				}),
		},
		&ClusterPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to MigStorage referenced by MigPlans
	err = c.Watch(
		&source.Kind{Type: &migapi.MigStorage{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigPlan{})
				}),
		},
		&StoragePredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to MigMigrations.
	err = c.Watch(
		&source.Kind{Type: &migapi.MigMigration{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(MigrationRequests),
		},
		&MigrationPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Indexes
	indexer := mgr.GetFieldIndexer()

	// Plan
	err = indexer.IndexField(
		&migapi.MigPlan{},
		migapi.ClosedIndexField,
		func(rawObj runtime.Object) []string {
			p, cast := rawObj.(*migapi.MigPlan)
			if !cast {
				return nil
			}
			return []string{
				strconv.FormatBool(p.Spec.Closed),
			}
		})
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigPlan{}

// ReconcileMigPlan reconciles a MigPlan object
type ReconcileMigPlan struct {
	client.Client
	scheme *runtime.Scheme
}

// Automatically generate RBAC rules
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.openshift.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=namespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=persistentvolumes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=persistentvolumeclaims/status,verbs=get;update;patch
func (r *ReconcileMigPlan) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error
	log.Reset()

	// Fetch the MigPlan instance
	plan := &migapi.MigPlan{}
	err = r.Get(context.TODO(), request.NamespacedName, plan)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Trace(err)
		return reconcile.Result{}, err
	}

	// Plan deleted.
	if plan.DeletionTimestamp != nil {
		retry := false
		err := r.planDeleted(plan)
		if err != nil {
			log.Trace(err)
			retry = r.retryFinalizer(plan)
		}
		return reconcile.Result{Requeue: retry}, nil
	} else {
		added := plan.EnsureFinalizer()
		if added {
			err = r.Update(context.TODO(), plan)
			if err != nil {
				log.Trace(err)
				return reconcile.Result{Requeue: true}, nil
			}
		}
	}

	// Report reconcile error.
	defer func() {
		if err == nil || errors.IsConflict(err) {
			return
		}
		plan.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), plan)
		if err != nil {
			log.Trace(err)
			return
		}
	}()

	// Plan closed.
	closed, err := r.handleClosed(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}
	if closed {
		return reconcile.Result{}, nil
	}

	// Begin staging conditions.
	plan.Status.BeginStagingConditions()

	// Plan Suspended
	err = r.planSuspended(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Validations.
	err = r.validate(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// PV discovery
	err = r.updatePvs(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Validate PV actions.
	err = r.validatePvSelections(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Storage
	err = r.ensureStorage(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Migration Registry
	err = r.ensureMigRegistries(plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Ready
	plan.Status.SetReady(
		plan.Status.HasCondition(StorageEnsured, PvsDiscovered, RegistriesEnsured) &&
			!plan.Status.HasBlockerCondition(),
		ReadyMessage)

	// End staging conditions.
	plan.Status.EndStagingConditions()

	// Apply changes.
	plan.Touch()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{}, nil
}

// The plan has been deleted.
// Delete all `remote` resources created by the plan
// on all clusters.
func (r *ReconcileMigPlan) planDeleted(plan *migapi.MigPlan) error {
	var err error
	migrations, err := plan.ListMigrations(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, migration := range migrations {
		err = r.Delete(context.TODO(), migration)
		if err != nil {
			log.Trace(err)
			return err
		}
	}
	clusters, err := migapi.ListClusters(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, cluster := range clusters {
		err = cluster.DeleteResources(r, plan.GetCorrelationLabels())
		if err != nil {
			log.Trace(err)
			return err
		}
	}
	plan.Touch()
	plan.DeleteFinalizer()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Get whether the finalizer may retry.
func (r *ReconcileMigPlan) retryFinalizer(plan *migapi.MigPlan) bool {
	retries := 3
	key := "retry-finalizer"
	if plan.Annotations == nil {
		plan.Annotations = map[string]string{}
	}
	n := 0
	if v, found := plan.Annotations[key]; found {
		n, _ = strconv.Atoi(v)
	} else {
		n = retries
	}
	if n > 0 {
		n--
		plan.Annotations[key] = strconv.Itoa(n)
	} else {
		plan.DeleteFinalizer()
	}
	err := r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
	}

	return n > 0
}

// Detect that a plan is been closed and ensure all its referenced
// resources have been cleaned up.
func (r *ReconcileMigPlan) handleClosed(plan *migapi.MigPlan) (bool, error) {
	closed := plan.Spec.Closed
	if !closed || plan.Status.HasCondition(Closed) {
		return closed, nil
	}

	plan.Touch()
	plan.Status.SetReady(false, ReadyMessage)
	err := r.Update(context.TODO(), plan)
	if err != nil {
		return closed, err
	}

	err = r.ensureClosed(plan)
	return closed, err
}

// Ensure that resources managed by the plan have been cleaned up.
func (r *ReconcileMigPlan) ensureClosed(plan *migapi.MigPlan) error {
	clusters, err := migapi.ListClusters(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, cluster := range clusters {
		err = cluster.DeleteResources(r, plan.GetCorrelationLabels())
		if err != nil {
			log.Trace(err)
			return err
		}
	}
	plan.Status.DeleteCondition(StorageEnsured, RegistriesEnsured, Suspended)
	plan.Status.SetCondition(migapi.Condition{
		Type:     Closed,
		Status:   True,
		Category: Advisory,
		Message:  ClosedMessage,
	})
	// Apply changes.
	plan.Touch()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Determine whether the plan is `suspended`.
// A plan is considered`suspended` when a migration is running or the final migration has
// completed successfully. While suspended, reconcile is limited to basic validation
// and PV discovery and ensuring resources is not performed.
func (r *ReconcileMigPlan) planSuspended(plan *migapi.MigPlan) error {
	suspended := false
	migrations, err := plan.ListMigrations(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, m := range migrations {
		if m.Status.HasCondition(migctl.Running) {
			suspended = true
			break
		}
		if m.Status.HasCondition(migctl.Succeeded) && !m.Spec.Stage {
			suspended = true
			break
		}
	}

	if suspended {
		plan.Status.SetCondition(migapi.Condition{
			Type:     Suspended,
			Status:   True,
			Category: Advisory,
			Message:  SuspendedMessage,
		})
	}

	return nil
}
