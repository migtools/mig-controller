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

package migcluster

import (
	"context"
	"time"

	"github.com/konveyor/mig-controller/pkg/errorutil"
	"github.com/konveyor/mig-controller/pkg/remote"
	"github.com/konveyor/mig-controller/pkg/settings"
	"github.com/opentracing/opentracing-go"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	Settings = &settings.Settings
	sink     = logging.WithName("cluster")
	log      = sink.Real
)

// Add creates a new MigCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileMigCluster {
	return &ReconcileMigCluster{Client: mgr.GetClient(), scheme: mgr.GetScheme(), EventRecorder: mgr.GetEventRecorderFor("migcluster_controller")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileMigCluster) error {
	// Create a new controller
	c, err := controller.New("migcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MigCluster
	err = c.Watch(
		source.Kind(mgr.GetCache(), &migapi.MigCluster{}),
		&handler.EnqueueRequestForObject{},
		&ClusterPredicate{
			Namespace: migapi.OpenshiftMigrationNamespace,
		})
	if err != nil {
		return err
	}

	// Watch remote clusters for connection problems
	err = c.Watch(
		&RemoteClusterSource{
			Client:    mgr.GetClient(),
			Namespace: migapi.OpenshiftMigrationNamespace,
			Interval:  time.Second * 60},
		&handler.EnqueueRequestForObject{},
		&migref.MigrationNamespacePredicate{Namespace: migapi.OpenshiftMigrationNamespace})
	if err != nil {
		return err
	}

	// Watch for changes to Secrets referenced by MigClusters
	err = c.Watch(
		source.Kind(mgr.GetCache(), &kapi.Secret{}),
		handler.EnqueueRequestsFromMapFunc(func(_ context.Context, a k8sclient.Object) []reconcile.Request {
			return migref.GetRequests(a, migapi.OpenshiftMigrationNamespace, migapi.MigCluster{})
		}))
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigCluster{}

// ReconcileMigCluster reconciles a MigCluster object
type ReconcileMigCluster struct {
	k8sclient.Client
	record.EventRecorder

	scheme     *runtime.Scheme
	Controller controller.Controller
	tracer     opentracing.Tracer
}

func (r *ReconcileMigCluster) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var err error
	sink = logging.WithName("cluster", "migCluster", request.Name)
	log := sink.Real

	// Fetch the MigCluster
	cluster := &migapi.MigCluster{}
	err = r.Get(context.TODO(), request.NamespacedName, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		sink.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Get jaeger span for reconcile, add to ctx
	reconcileSpan := r.initTracer(cluster)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	// Report reconcile error.
	defer func() {
		log.Info("CR", "conditions", cluster.Status.Conditions)
		cluster.Status.Conditions.RecordEvents(cluster, r.EventRecorder)
		if err == nil || errors.IsConflict(errorutil.Unwrap(err)) {
			return
		}
		cluster.Status.SetReconcileFailed(err)
		err := r.Update(context.TODO(), cluster)
		if err != nil {
			sink.Trace(err)
			return
		}
	}()

	// Begin staging conditions.
	cluster.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(ctx, cluster)
	if err != nil {
		sink.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Set Status.RegistryPath
	err = cluster.SetRegistryPath(r)
	if err != nil {
		sink.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Set Status.OperatorVersion
	err = cluster.SetOperatorVersion(r)
	if err != nil {
		sink.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	if Settings.EnableCachedClient {
		if !cluster.Status.HasBlockerCondition() {
			// Remote Watch.
			err = r.setupRemoteWatch(cluster)
			if err != nil {
				sink.Trace(err)
				return reconcile.Result{Requeue: true}, nil
			}
		} else {
			r.shutdownRemoteWatch(cluster)
		}
	}

	// Ready
	cluster.Status.SetReady(
		!cluster.Status.HasBlockerCondition(),
		"The cluster is ready.")

	// End staging conditions.
	cluster.Status.EndStagingConditions()

	// Mark as refreshed
	cluster.Spec.Refresh = false

	// Apply changes.
	cluster.MarkReconciled()
	err = r.Update(context.TODO(), cluster)
	if err != nil {
		sink.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{Requeue: false}, nil
}

// Setup remote watch.
func (r *ReconcileMigCluster) setupRemoteWatch(cluster *migapi.MigCluster) error {
	var err error
	nsName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	var restCfg *rest.Config
	if cluster.Spec.IsHostCluster {
		restCfg, err = config.GetConfig()
		if err != nil {
			return liberr.Wrap(err)
		}
	} else {
		restCfg, err = cluster.BuildRestConfig(r.Client)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	if IsRemoteWatchConsistent(nsName, restCfg) {
		return nil
	}

	r.shutdownRemoteWatch(cluster)

	log.Info("Starting remote manager.", "cluster", cluster.Name)
	StartRemoteWatch(r, remote.ManagerConfig{
		RemoteRestConfig: restCfg,
		ParentNsName:     nsName,
		ParentMeta:       cluster.GetObjectMeta(),
		ParentObject:     cluster,
		Scheme:           r.scheme,
	})
	log.Info("Remote manager started.", "cluster", cluster.Name)

	return nil
}

func (r *ReconcileMigCluster) shutdownRemoteWatch(cluster *migapi.MigCluster) {
	log.Info("Stopping remote manager.", "cluster", cluster.Name)
	nsName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}

	StopRemoteWatch(nsName)
	log.Info("Stopped remote manager.", "cluster", cluster.Name)
}
