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
	"fmt"

	migrationv1alpha1 "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/fusor/mig-controller/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterregv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kapi "k8s.io/api/core/v1"
)

var log = logf.Log.WithName("controller")

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
	err = c.Watch(&source.Kind{Type: &migrationv1alpha1.MigCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// >>>> DWHATLEY - MULTIPLE OWNERS
	// rpm := util.GetResourceParentsMap()

	// migClusterResource := util.KubeResource{NsName: types.NamespacedName{Name: }

	// rpm.GetParentsOfKind(util.KubeResource{Kind: })

	// mapFn := handler.ToRequestsFunc(
	// 	func(a handler.MapObject) []reconcile.Request {
	// 		return []reconcile.Request{
	// 			{NamespacedName: types.NamespacedName{
	// 				Name:      a.Meta.GetName() + "-1",
	// 				Namespace: a.Meta.GetNamespace(),
	// 			}},
	// 			{NamespacedName: types.NamespacedName{
	// 				Name:      a.Meta.GetName() + "-2",
	// 				Namespace: a.Meta.GetNamespace(),
	// 			}},
	// 		}
	// 	})
	// <<<< DWHATLEY - MULITPLE OWNERS

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigCluster{}

// var _ remoteWatchMap = GetRemoteWatchMap()

// ReconcileMigCluster reconciles a MigCluster object
type ReconcileMigCluster struct {
	client.Client
	scheme     *runtime.Scheme
	Controller controller.Controller
}

// Reconcile reads that state of the cluster for a MigCluster object and makes changes based on the state read
// and what is in the MigCluster.Spec
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=migration.openshift.io,resources=migclusters/status,verbs=get;update;patch
func (r *ReconcileMigCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info(fmt.Sprintf("[mCluster] RECONCILE [nsName=%s/%s]", request.Namespace, request.Name))

	// Set up ResourceParentsMap to manage parent-child mapping
	rpm := util.GetResourceParentsMap()
	parentMigCluster := util.KubeResource{Kind: util.KindMigCluster, NsName: request.NamespacedName}

	// Fetch the MigCluster instance
	instance := &migrationv1alpha1.MigCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}

	// Check if this cluster is also hosting the controller
	isHostCluster := instance.Spec.IsHostCluster
	log.Info(fmt.Sprintf("[mCluster] isHostCluster: [%v]", isHostCluster))

	// Get the SA secret attached to MigCluster
	saSecretRef := instance.Spec.ServiceAccountSecretRef
	saSecret := &kapi.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: saSecretRef.Name, Namespace: saSecretRef.Namespace}, saSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}
	// Valid Cluster found, add MigCluster as parent to receive reconciliation events
	childSecret := util.KubeResource{
		Kind: util.KindSecret,
		NsName: types.NamespacedName{
			Name:      saSecretRef.Name,
			Namespace: saSecretRef.Namespace,
		},
	}
	rpm.AddChildToParent(childSecret, parentMigCluster)

	// Get data from saToken secret
	saTokenKey := "saToken"
	saTokenData, ok := saSecret.Data[saTokenKey]
	if !ok {
		log.Info(fmt.Sprintf("[mCluster] saToken: [%v]", ok))
		return reconcile.Result{}, nil // don't requeue
	}
	saToken := string(saTokenData)
	// log.Info(fmt.Sprintf("saToken: [%s]", saToken))

	// Get k8s URL from Cluster associated with MigCluster
	crClusterRef := instance.Spec.ClusterRef
	crCluster := &clusterregv1alpha1.Cluster{}

	err = r.Get(context.TODO(), types.NamespacedName{Name: crClusterRef.Name, Namespace: crClusterRef.Namespace}, crCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil // don't requeue
		}
		return reconcile.Result{}, err // requeue
	}
	// Valid Cluster found, add MigCluster as parent to receive reconciliation events
	childCrCluster := util.KubeResource{
		Kind: util.KindClusterRegCluster,
		NsName: types.NamespacedName{
			Name:      crClusterRef.Name,
			Namespace: crClusterRef.Namespace,
		},
	}
	rpm.AddChildToParent(childCrCluster, parentMigCluster)

	// Get remoteClusterURL from Cluster
	var remoteClusterURL string
	k8sEndpoints := crCluster.Spec.KubernetesAPIEndpoints.ServerEndpoints
	if len(k8sEndpoints) > 0 {
		remoteClusterURL = string(k8sEndpoints[0].ServerAddress)
		log.Info(fmt.Sprintf("[mCluster] remoteClusterURL: [%s]", remoteClusterURL))
	} else {
		log.Info(fmt.Sprintf("[mCluster] remoteClusterURL: [len=0]"))
	}

	// Create a Remote Watch for this MigCluster if one doesn't exist
	rwm := GetRemoteWatchMap()
	rwc := rwm.GetRWC(request.NamespacedName)
	restCfg := util.BuildRestConfig(remoteClusterURL, saToken)

	if rwc == nil {
		log.Info(fmt.Sprintf("[mCluster] Starting RemoteWatch for MigCluster [ns=%s], [name=%s]", request.Namespace, request.Name))
		StartRemoteWatch(r, RemoteManagerConfig{
			RemoteRestConfig: restCfg,
			ParentNsName:     request.NamespacedName,
			ParentResource:   instance,
		})
		log.Info(fmt.Sprintf("[mCluster] RemoteWatch started successfully for MigCluster [ns=%s], [name=%s]", request.Namespace, request.Name))
	}

	return reconcile.Result{}, nil
}
