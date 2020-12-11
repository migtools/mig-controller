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
	"sort"
	"strconv"
	"time"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migctl "github.com/konveyor/mig-controller/pkg/controller/migmigration"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/konveyor/mig-controller/pkg/settings"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("plan")

// Application settings.
var Settings = &settings.Settings

// Add creates a new MigPlan Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigPlan{Client: mgr.GetClient(), scheme: mgr.GetScheme(), EventRecorder: mgr.GetRecorder("migplan_controller")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("migplan-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MigPlan
	err = c.Watch(&source.Kind{
		Type: &migapi.MigPlan{}},
		&handler.EnqueueRequestForObject{},
		&PlanPredicate{},
	)
	if err != nil {
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
		return err
	}
	// Pod
	err = indexer.IndexField(
		&kapi.Pod{},
		"status.phase",
		func(rawObj runtime.Object) []string {
			p, cast := rawObj.(*kapi.Pod)
			if !cast {
				return nil
			}
			return []string{
				string(p.Status.Phase),
			}
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigPlan{}

// ReconcileMigPlan reconciles a MigPlan object
type ReconcileMigPlan struct {
	client.Client
	record.EventRecorder

	scheme *runtime.Scheme
}

func (r *ReconcileMigPlan) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var err error
	log.Reset()
	log.SetValues("plan", request)

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

	// Report reconcile error.
	defer func() {
		log.Info("CR", "conditions", plan.Status.Conditions)
		plan.Status.Conditions.RecordEvents(plan, r.EventRecorder)
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

	// Set excluded resources on Status.
	err = r.setExcludedResourceList(plan)
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

	// Validate NFS PV accessibility.
	nfsValidation := NfsValidation{Plan: plan}
	err = nfsValidation.Run(r.Client)
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

	// Ready
	plan.Status.SetReady(
		plan.Status.HasCondition(StorageEnsured, PvsDiscovered) &&
			!plan.Status.HasBlockerCondition(),
		"The migration plan is ready.")

	// End staging conditions.
	plan.Status.EndStagingConditions()

	// Mark as refreshed
	plan.Spec.Refresh = false

	// Apply changes.
	plan.MarkReconciled()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Timed requeue on Plan conflict.
	if plan.Status.HasCondition(PlanConflict) {
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Done
	return reconcile.Result{}, nil
}

// Detect that a plan is been closed and ensure all its referenced
// resources have been cleaned up.
func (r *ReconcileMigPlan) handleClosed(plan *migapi.MigPlan) (bool, error) {
	closed := plan.Spec.Closed
	if !closed || plan.Status.HasCondition(Closed) {
		return closed, nil
	}

	plan.MarkReconciled()
	plan.Status.SetReady(false, "The migration plan is ready.")
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
		return liberr.Wrap(err)
	}
	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		err = cluster.DeleteResources(r, plan.GetCorrelationLabels())
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	plan.Status.DeleteCondition(StorageEnsured, RegistriesEnsured, Suspended)
	plan.Status.SetCondition(migapi.Condition{
		Type:     Closed,
		Status:   True,
		Category: Advisory,
		Message:  "The migration plan is closed.",
	})
	// Apply changes.
	plan.MarkReconciled()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		return liberr.Wrap(err)
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
		return liberr.Wrap(err)
	}

	// Sort migrations by timestamp, newest first.
	sort.Slice(migrations, func(i, j int) bool {
		ts1 := migrations[i].CreationTimestamp
		ts2 := migrations[j].CreationTimestamp
		return ts1.Time.After(ts2.Time)
	})

	for _, m := range migrations {
		// If a migration is running, plan should be suspended
		if m.Status.HasCondition(migctl.Running) {
			suspended = true
			break
		}
		// If the newest final migration is successful, suspend plan
		if m.Status.HasCondition(migctl.Succeeded) && !m.Spec.Stage && !m.Spec.Rollback {
			suspended = true
			break
		}
		// If the newest migration is a successful rollback, unsuspend plan
		if m.Status.HasCondition(migctl.Succeeded) && m.Spec.Rollback {
			suspended = false
			break
		}
	}

	// If refresh requested on plan, temporarily un-suspend
	if plan.Spec.Refresh == true {
		suspended = false
	}

	if suspended {
		plan.Status.SetCondition(migapi.Condition{
			Type:     Suspended,
			Status:   True,
			Category: Advisory,
			Message: "The migrations plan is in suspended state; Limited validation enforced; PV discovery and " +
				"resource reconciliation suspended.",
		})
	}

	return nil
}

// Update Status.ExcludedResources based on settings
func (r *ReconcileMigPlan) setExcludedResourceList(plan *migapi.MigPlan) error {
	excludedResources := Settings.Plan.ExcludedResources
	plan.Status.ExcludedResources = excludedResources
	return nil
}

func (r ReconcileMigPlan) deleteImageRegistryResourcesForClient(client k8sclient.Client, plan *migapi.MigPlan) error {
	plan.Status.Conditions.DeleteCondition(RegistriesEnsured)
	secret, err := plan.GetRegistrySecret(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if secret != nil {
		err := client.Delete(context.Background(), secret)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	err = r.deleteImageRegistryDeploymentForClient(client, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	foundService, err := plan.GetRegistryService(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundService != nil {
		err := client.Delete(context.Background(), foundService)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

func (r ReconcileMigPlan) deleteImageRegistryDeploymentForClient(client k8sclient.Client, plan *migapi.MigPlan) error {
	plan.Status.Conditions.DeleteCondition(RegistriesEnsured)
	foundDeployment, err := plan.GetRegistryDeployment(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundDeployment != nil {
		err := client.Delete(context.Background(), foundDeployment, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}
