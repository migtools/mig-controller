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
	"k8s.io/apiserver/pkg/storage/names"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	kapi "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	crapi "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("cluster")

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
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigCluster{}

// ReconcileMigCluster reconciles a MigCluster object
type ReconcileMigCluster struct {
	client.Client
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
	log = logf.Log.WithName(names.SimpleNameGenerator.GenerateName("cluster|"))

	// Fetch the MigCluster
	migCluster := &migapi.MigCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, migCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	// Validations.
	nSet, err := r.validate(migCluster)
	if err != nil {
		if errors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		} else {
			return reconcile.Result{}, err
		}
	}
	if nSet > 0 {
		return reconcile.Result{}, nil
	}

	// Create a Remote Watch for this MigCluster if one doesn't exist
	remoteWatchMap := GetRemoteWatchMap()
	remoteWatchCluster := remoteWatchMap.Get(request.NamespacedName)

	if remoteWatchCluster == nil {
		log.Info("Starting remote watch.", "cluster", request.Name)

		var restCfg *rest.Config
		if migCluster.Spec.IsHostCluster {
			restCfg, err = config.GetConfig()
			if err != nil {
				return reconcile.Result{}, err
			}
		} else {
			restCfg, err = migCluster.BuildRestConfig(r.Client)
			if err != nil {
				return reconcile.Result{}, nil // don't requeue
			}
		}

		StartRemoteWatch(r, RemoteManagerConfig{
			RemoteRestConfig: restCfg,
			ParentNsName:     request.NamespacedName,
			ParentResource:   migCluster,
		})

		log.Info("Remote watch started.", "cluster", request.Name)
	}

	// Done
	return reconcile.Result{}, nil
}
