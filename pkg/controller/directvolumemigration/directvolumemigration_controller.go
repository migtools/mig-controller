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
	"github.com/konveyor/mig-controller/pkg/compat"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	dvmFinalizer = "migration.openshift.io/directvolumemigrationfinalizer"
)

var (
	sink = logging.WithName("directvolume")
	log  = sink.Real
)

// Add creates a new DirectVolumeMigration Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDirectVolumeMigration{Config: mgr.GetConfig(), Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("directvolumemigration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DirectVolumeMigration
	err = c.Watch(source.Kind(mgr.GetCache(), &migapi.DirectVolumeMigration{}),
		&handler.EnqueueRequestForObject{},
		&migref.MigrationNamespacePredicate{Namespace: migapi.OpenshiftMigrationNamespace})
	if err != nil {
		return err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &migapi.DirectVolumeMigrationProgress{}),
		handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetClient().RESTMapper(),
			&migapi.DirectVolumeMigration{},
			handler.OnlyControllerOwner()),
		&migref.MigrationNamespacePredicate{Namespace: migapi.OpenshiftMigrationNamespace})
	if err != nil {
		return err
	}

	// TODO: Modify this to watch the proper list of resources

	return nil
}

var _ reconcile.Reconciler = &ReconcileDirectVolumeMigration{}

// ReconcileDirectVolumeMigration reconciles a DirectVolumeMigration object
type ReconcileDirectVolumeMigration struct {
	*rest.Config
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
	tracer := logging.WithName("directvolume", "dvm", request.Name)
	log := tracer.Real
	// Default to PollReQ, can be overridden by r.migrate phase-specific ReQ interval
	requeueAfter := time.Duration(PollReQ)

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
	if err != nil {
		return reconcile.Result{}, err
	}
	if migration != nil {
		log = log.WithValues("migMigration", migration.Name)
	}

	// Set up jaeger tracing, add to ctx
	reconcileSpan := r.initTracer(direct)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	if direct.DeletionTimestamp != nil {
		sourceDeleted, err := r.cleanupSourceResourcesInNamespace(direct)
		if err != nil {
			return reconcile.Result{}, err
		}
		targetDeleted, err := r.cleanupTargetResourcesInNamespaces(direct)
		if err != nil {
			return reconcile.Result{}, err
		}
		if sourceDeleted && targetDeleted {
			log.V(5).Info("DirectVolumeMigration resources deleted. removing finalizer")
			// Remove finalizer
			RemoveFinalizer(direct, dvmFinalizer)
		} else {
			// Requeue
			log.V(5).Info("Requeing waiting for cleanup", "after", requeueAfter)
			return reconcile.Result{RequeueAfter: requeueAfter}, nil
		}
	} else {
		// Add finalizer
		AddFinalizer(direct, dvmFinalizer)
	}

	// Check if completed and return if not deleted, otherwise need to update to
	// remove the finalizer.
	if direct.Status.Phase == Completed && direct.DeletionTimestamp == nil {
		return reconcile.Result{Requeue: false}, nil
	}

	// Begin staging conditions
	direct.Status.BeginStagingConditions()

	// Validation
	err = r.validate(ctx, direct)
	if err != nil {
		tracer.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	if !direct.Status.HasBlockerCondition() && direct.DeletionTimestamp == nil {
		requeueAfter, err = r.migrate(ctx, direct)
		if err != nil {
			tracer.Trace(err)
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
		tracer.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Requeue
	if requeueAfter > 0 {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	// Done
	return reconcile.Result{Requeue: false}, nil
}

// AddFinalizer adds a finalizer to a resource
func AddFinalizer(obj metav1.Object, name string) {
	if HasFinalizer(obj, name) {
		return
	}

	obj.SetFinalizers(append(obj.GetFinalizers(), name))
}

// RemoveFinalizer removes a finalizer from a resource
func RemoveFinalizer(obj metav1.Object, name string) {
	if !HasFinalizer(obj, name) {
		return
	}

	var finalizers []string
	for _, f := range obj.GetFinalizers() {
		if f != name {
			finalizers = append(finalizers, f)
		}
	}

	obj.SetFinalizers(finalizers)
}

// HasFinalizer returns true if a resource has a specific finalizer
func HasFinalizer(object metav1.Object, value string) bool {
	for _, f := range object.GetFinalizers() {
		if f == value {
			return true
		}
	}
	return false
}

func (r *ReconcileDirectVolumeMigration) cleanupSourceResourcesInNamespace(direct *migapi.DirectVolumeMigration) (bool, error) {
	sourceCluster, err := direct.GetSourceCluster(r)
	if err != nil {
		return false, err
	}

	// Cleanup source resources
	client, err := sourceCluster.GetClient(r)
	if err != nil {
		return false, err
	}

	liveMigrationCanceled, err := r.cancelLiveMigrations(client, direct)
	if err != nil {
		return false, err
	}

	completed, err := r.cleanupResourcesInNamespaces(client, direct.GetUID(), direct.GetSourceNamespaces())
	return completed && liveMigrationCanceled, err
}

func (r *ReconcileDirectVolumeMigration) cancelLiveMigrations(client compat.Client, direct *migapi.DirectVolumeMigration) (bool, error) {
	if direct.IsCutover() && direct.IsLiveMigrate() {
		// Cutover and live migration is enabled, attempt to cancel migrations in progress.
		namespaces := direct.GetSourceNamespaces()
		allLiveMigrationCompleted := true
		for _, ns := range namespaces {
			volumeVmMap, err := getRunningVmVolumeMap(client, ns)
			if err != nil {
				return false, err
			}
			vmVolumeMap := make(map[string]*vmVolumes)
			for i := range direct.Spec.PersistentVolumeClaims {
				sourceName := direct.Spec.PersistentVolumeClaims[i].Name
				targetName := direct.Spec.PersistentVolumeClaims[i].TargetName
				vmName, found := volumeVmMap[sourceName]
				if !found {
					continue
				}
				volumes := vmVolumeMap[vmName]
				if volumes == nil {
					volumes = &vmVolumes{
						sourceVolumes: []string{sourceName},
						targetVolumes: []string{targetName},
					}
				} else {
					volumes.sourceVolumes = append(volumes.sourceVolumes, sourceName)
					volumes.targetVolumes = append(volumes.targetVolumes, targetName)
				}
				vmVolumeMap[vmName] = volumes
			}
			vmNames := make([]string, 0)
			for vmName, volumes := range vmVolumeMap {
				if err := cancelLiveMigration(client, vmName, ns, volumes, log); err != nil {
					return false, err
				}
				vmNames = append(vmNames, vmName)
			}
			migrated, err := liveMigrationsCompleted(client, ns, vmNames)
			if err != nil {
				return false, err
			}
			allLiveMigrationCompleted = allLiveMigrationCompleted && migrated
		}
		return allLiveMigrationCompleted, nil
	}
	return true, nil
}

func (r *ReconcileDirectVolumeMigration) cleanupTargetResourcesInNamespaces(direct *migapi.DirectVolumeMigration) (bool, error) {
	destinationCluster, err := direct.GetDestinationCluster(r)
	if err != nil {
		return false, err
	}

	// Cleanup source resources
	client, err := destinationCluster.GetClient(r)
	if err != nil {
		return false, err
	}
	return r.cleanupResourcesInNamespaces(client, direct.GetUID(), direct.GetDestinationNamespaces())
}

func (r *ReconcileDirectVolumeMigration) cleanupResourcesInNamespaces(client compat.Client, uid types.UID, namespaces []string) (bool, error) {
	selector := labels.SelectorFromSet(map[string]string{
		"directvolumemigration": string(uid),
	})

	sourceDeleted := true
	for _, ns := range namespaces {
		if err := findAndDeleteNsResources(client, ns, selector, log); err != nil {
			return false, err
		}
		err, deleted := areRsyncNsResourcesDeleted(client, ns, selector, log)
		if err != nil {
			return false, err
		}
		sourceDeleted = sourceDeleted && deleted
	}
	return sourceDeleted, nil
}
