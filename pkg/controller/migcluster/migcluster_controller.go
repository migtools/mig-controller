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

package migcluster

import (
	"context"
	"github.com/fusor/mig-controller/pkg/logging"
	"k8s.io/apimachinery/pkg/types"
	"strconv"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	"github.com/fusor/mig-controller/pkg/remote"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	crapi "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.WithName("cluster")

// Add creates a new MigCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileMigCluster {
	return &ReconcileMigCluster{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileMigCluster) error {
	// Create a new controller
	c, err := controller.New("migcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Add reference to controller on ReconcileMigCluster object to be used
	// for adding remote watches at a later time
	r.Controller = c

	// Watch for changes to MigCluster
	err = c.Watch(
		&source.Kind{Type: &migapi.MigCluster{}},
		&handler.EnqueueRequestForObject{},
		&ClusterPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to Clusters referenced by MigClusters
	err = c.Watch(
		&source.Kind{Type: &crapi.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigCluster{})
				}),
		})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to Secrets referenced by MigClusters
	err = c.Watch(
		&source.Kind{Type: &kapi.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					return migref.GetRequests(a, migapi.MigCluster{})
				}),
		})
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigCluster{}

// ReconcileMigCluster reconciles a MigCluster object
type ReconcileMigCluster struct {
	k8sclient.Client
	scheme     *runtime.Scheme
	Controller controller.Controller
}

// Reconcile reads that state of the cluster for a MigCluster object and makes changes based on the state read
// and what is in the MigCluster.Spec
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=clusterregistry.k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clusterregistry.k8s.io,resources=clusters/status,verbs=get;update;patch
func (r *ReconcileMigCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Reset()

	// Fetch the MigCluster
	cluster := &migapi.MigCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Finalizer
	added := cluster.EnsureFinalizer()
	if added {
		err = r.Update(context.TODO(), cluster)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Cluster deleted.
	if cluster.DeletionTimestamp != nil {
		retry := false
		err := r.clusterDeleted(cluster)
		if err != nil {
			log.Trace(err)
			retry = r.retryFinalizer(cluster)
		}
		return reconcile.Result{Requeue: retry}, nil
	}

	// Begin staging conditions.
	cluster.Status.BeginStagingConditions()

	// Validations.
	err = r.validate(cluster)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	if !cluster.Status.HasBlockerCondition() {
		// Remote Watch.
		err = r.setupRemoteWatch(cluster)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}

		// Storage Classes
		err = r.setStorageClasses(cluster)
		if err != nil {
			log.Trace(err)
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Ready
	cluster.Status.SetReady(
		!cluster.Status.HasBlockerCondition(),
		ReadyMessage)

	// End staging conditions.
	cluster.Status.EndStagingConditions()

	// Apply changes.
	cluster.Touch()
	err = r.Update(context.TODO(), cluster)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{}, nil
}

// Setup remote watch.
func (r *ReconcileMigCluster) setupRemoteWatch(cluster *migapi.MigCluster) error {
	nsName := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	remoteWatchMap := remote.GetWatchMap()
	remoteWatchCluster := remoteWatchMap.Get(nsName)
	if remoteWatchCluster != nil {
		return nil
	}

	log.Info("Starting remote watch.", "cluster", cluster.Name)

	var err error
	var restCfg *rest.Config
	if cluster.Spec.IsHostCluster {
		restCfg, err = config.GetConfig()
		if err != nil {
			log.Trace(err)
			return err
		}
	} else {
		restCfg, err = cluster.BuildRestConfig(r.Client)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	StartRemoteWatch(r, remote.ManagerConfig{
		RemoteRestConfig: restCfg,
		ParentNsName:     nsName,
		ParentMeta:       cluster.GetObjectMeta(),
		ParentObject:     cluster,
	})

	log.Info("Remote watch started.", "cluster", cluster.Name)

	return nil
}

// The cluster has been deleted.
// Delete all `remote` resources created by the application
// on the deleted cluster.
func (r *ReconcileMigCluster) clusterDeleted(cluster *migapi.MigCluster) error {
	var err error
	err = cluster.DeleteResources(r, nil)
	if err != nil {
		log.Trace(err)
	}
	cluster.Touch()
	cluster.DeleteFinalizer()
	err = r.Update(context.TODO(), cluster)
	if err != nil {
		log.Trace(err)
	}

	return err
}

// Get whether the finalizer may retry.
func (r *ReconcileMigCluster) retryFinalizer(cluster *migapi.MigCluster) bool {
	retries := 3
	key := "retry-finalizer"
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	n := 0
	if v, found := cluster.Annotations[key]; found {
		n, _ = strconv.Atoi(v)
	} else {
		n = retries
	}
	if n > 0 {
		n--
		cluster.Annotations[key] = strconv.Itoa(n)
	} else {
		cluster.DeleteFinalizer()
	}
	err := r.Update(context.TODO(), cluster)
	if err != nil {
		log.Trace(err)
	}

	return n > 0
}
