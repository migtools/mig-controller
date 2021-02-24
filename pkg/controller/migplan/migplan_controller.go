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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/konveyor/mig-controller/pkg/errorutil"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	miganalytic "github.com/konveyor/mig-controller/pkg/controller/miganalytic"
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

const (
	MigPlan   = "migplan"
	CreatedBy = "CreatedBy"
)

// define maximum waiting time for mig analytic to be ready
var migAnalyticsTimeout = 2 * time.Minute

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
		if err == nil || errors.IsConflict(errorutil.Unwrap(err)) {
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

	// If intelligent pv resizing is enabled, Check if migAnalytics exists
	if Settings.EnableIntelligentPVResize {
		err = r.ensureMigAnalytics(plan)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
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

	// If intelligent pv resizing is enabled, Wait for the migAnalytics to be ready
	if Settings.EnableIntelligentPVResize {
		migAnalytic, err := r.checkIfMigAnalyticsReady(plan)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
		if migAnalytic != nil {
			// Process PV Capacity and generate conditions
			r.processProposedPVCapacities(plan, migAnalytic)
		}
		if migAnalytic == nil && !plan.Status.HasCondition(PvUsageAnalysisFailed) {
			return reconcile.Result{Requeue: true}, nil
		}
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

func (r *ReconcileMigPlan) ensureMigAnalytics(plan *migapi.MigPlan) error {
	migAnalytics := &migapi.MigAnalyticList{}
	err := r.List(context.TODO(), k8sclient.MatchingLabels(map[string]string{MigPlan: plan.Name}), migAnalytics)
	if err != nil {
		if !errors.IsNotFound(err) {
			return liberr.Wrap(err)
		}
	}
	for _, migAnalytic := range migAnalytics.Items {
		if migAnalytic.Spec.AnalyzeExtendedPVCapacity {
			if !migAnalytic.Spec.Refresh && (plan.Spec.Refresh || !plan.HasReconciled()) {
				migAnalytic.Spec.Refresh = true
				err := r.Update(context.TODO(), &migAnalytic)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
			return nil
		}
	}
	pvMigAnalytics := &migapi.MigAnalytic{}
	pvMigAnalytics.GenerateName = plan.Name + "-"
	pvMigAnalytics.Namespace = plan.Namespace
	pvMigAnalytics.Spec.AnalyzeExtendedPVCapacity = true
	pvMigAnalytics.Annotations = map[string]string{MigPlan: plan.Name, CreatedBy: plan.Name}
	pvMigAnalytics.Labels = map[string]string{MigPlan: plan.Name, CreatedBy: plan.Name}
	pvMigAnalytics.OwnerReferences = append(pvMigAnalytics.OwnerReferences, metav1.OwnerReference{
		APIVersion: plan.APIVersion,
		Kind:       plan.Kind,
		Name:       plan.Name,
		UID:        plan.UID,
	})
	pvMigAnalytics.Spec.MigPlanRef = &kapi.ObjectReference{Namespace: plan.Namespace, Name: plan.Name}
	err = r.Create(context.TODO(), pvMigAnalytics)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func (r *ReconcileMigPlan) checkIfMigAnalyticsReady(plan *migapi.MigPlan) (*migapi.MigAnalytic, error) {
	migAnalytics := &migapi.MigAnalyticList{}
	err := r.List(context.TODO(), k8sclient.MatchingLabels(map[string]string{MigPlan: plan.Name}), migAnalytics)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for i := range migAnalytics.Items {
		migAnalytic := &migAnalytics.Items[i]
		if migAnalytic.Spec.AnalyzeExtendedPVCapacity == true {
			if migAnalytic.Status.IsReady() && !migAnalytic.Spec.Refresh {
				return migAnalytic, nil
			}
			pvAnalysisStartedCondition := migAnalytic.Status.FindCondition(miganalytic.ExtendedPVAnalysisStarted)
			if pvAnalysisStartedCondition != nil {
				if time.Now().Sub(pvAnalysisStartedCondition.LastTransitionTime.Time) > migAnalyticsTimeout {
					plan.Status.SetCondition(migapi.Condition{
						Type:     PvUsageAnalysisFailed,
						Status:   True,
						Category: Warn,
						Reason:   NotDone,
						Message:  "Failed to gather reliable PV usage data from the source cluster, PV resizing will be disabled.",
					})
					plan.Status.DeleteCondition(PvCapacityAdjustmentRequired)
				}
			}
		}
	}
	return nil, nil
}

// processProposedPVCapacities reads miganalytic status to find proposed capacities of volumes
func (r *ReconcileMigPlan) processProposedPVCapacities(plan *migapi.MigPlan, analytic *migapi.MigAnalytic) {
	pvResizingRequiredVolumes := []string{}
	plan.Spec.PersistentVolumes.BeginPvStaging()
	for i := range plan.Spec.PersistentVolumes.List {
		planVol := &plan.Spec.PersistentVolumes.List[i]
		for _, analyticNS := range analytic.Status.Analytics.Namespaces {
			if planVol.PVC.Namespace == analyticNS.Namespace {
				for _, analyticNSVol := range analyticNS.PersistentVolumes {
					if planVol.PVC.Name == analyticNSVol.Name {
						// If new proposed capacity is greater than previous proposed capacity, set confirmed to False
						if planVol.ProposedCapacity.Cmp(analyticNSVol.ProposedCapacity) > 0 {
							planVol.CapacityConfirmed = false
						}
						if analyticNSVol.Comment != miganalytic.VolumeAdjustmentNoOp {
							pvResizingRequiredVolumes = append(pvResizingRequiredVolumes, planVol.Name)
						}
						planVol.ProposedCapacity = analyticNSVol.ProposedCapacity
					}
				}
			}
		}
		plan.Spec.AddPv(*planVol)
	}
	plan.Spec.PersistentVolumes.EndPvStaging()
	r.generatePVResizeConditions(pvResizingRequiredVolumes, plan, analytic)
}

// generatePVResizeConditions generates conditions for PV resizing
func (r ReconcileMigPlan) generatePVResizeConditions(pvResizingRequiredVolumes []string, plan *migapi.MigPlan, migAnalytic *migapi.MigAnalytic) {
	if migAnalytic.Status.HasCondition(miganalytic.ExtendedPVAnalysisFailed) {
		plan.Status.SetCondition(migapi.Condition{
			Category: Warn,
			Status:   True,
			Type:     miganalytic.ExtendedPVAnalysisFailed,
			Reason:   miganalytic.FailedRunningDf,
			Message: fmt.Sprintf(
				"Failed gathering extended PV usage information for some or all PVs, please see MigAnalytic %s/%s for details",
				migAnalytic.Namespace, migAnalytic.Name),
		})
	}
	// remove pv analysis related conditions as we are here processing pvs
	plan.Status.DeleteCondition(PvUsageAnalysisFailed)
	if len(pvResizingRequiredVolumes) > 0 {
		if !Settings.DvmOpts.EnablePVResizing {
			plan.Status.SetCondition(migapi.Condition{
				Type:     PvCapacityAdjustmentRequired,
				Status:   True,
				Category: Warn,
				Reason:   NotDone,
				Message: fmt.Sprintf(
					"Migrating data of following volumes may result in a failure either due to mismatch in their requested and actual capacities or disk usage being close to 100%% :  [%s]",
					strings.Join(pvResizingRequiredVolumes, ",")),
				Durable: true,
			})
		} else {
			plan.Status.SetCondition(migapi.Condition{
				Type:     PvCapacityAdjustmentRequired,
				Status:   False,
				Category: Warn,
				Reason:   Done,
				Message: fmt.Sprintf(
					"Capacity of the following volumes will be automatically adjusted to avoid disk capacity issues in the target cluster  :  [%s]",
					strings.Join(pvResizingRequiredVolumes, ",")),
				Durable: true,
			})
		}
	} else {
		plan.Status.DeleteCondition(PvCapacityAdjustmentRequired)
	}
}
