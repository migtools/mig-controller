package migcluster

import (
	"context"
	"time"

	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RemoteClusterSource is a `watch` source used to
// checkup on the connectivity of remote clusters.
//	  Client - A controller-runtime client.
//	  Interval - The connection test interval
type RemoteClusterSource struct {
	Client     client.Client
	Interval   time.Duration
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

// Start the source.
func (r *RemoteClusterSource) Start(
	ctx context.Context,
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate) error {

	r.handler = handler
	r.queue = queue
	r.predicates = predicates
	go r.run()

	return nil
}

// Run the scheduled connection and version match tests.
func (r *RemoteClusterSource) run() {
	for {
		time.Sleep(r.Interval)
		list, err := v1alpha1.ListClusters(r.Client)
		if err != nil {
			log.Trace(err)
			return
		}

		for _, cluster := range list {
			if cluster.Status.HasAnyCondition(
				InvalidURL,
				InvalidSaSecretRef,
				InvalidSaToken,
				SaTokenNotPrivileged) {
				continue
			}

			// Enqueue if our view of the cluster disagrees with the result of the connectivity test
			timeout := time.Duration(time.Second * 5)
			err := cluster.TestConnection(r.Client, timeout)
			if (cluster.Status.HasCondition(TestConnectFailed) && err == nil) ||
				(!cluster.Status.HasCondition(TestConnectFailed) && err != nil) {
				r.enqueue(cluster)
				continue
			}

			// Enqueue if our view of the cluster disagrees with the result of the registry status test
			registryOK := true
			if cluster.Spec.ExposedRegistryPath != "" {
				statusCode, registryErr, err := checkRegistryConnection(&cluster, r.Client)
				registryOK = err == nil && registryErr == nil && statusCode == 200
			}
			if (cluster.Status.HasCondition(InvalidRegistryRoute) && registryOK) ||
				(!cluster.Status.HasCondition(TestConnectFailed) && !registryOK) {
				r.enqueue(cluster)
				continue
			}

			// Enqueue if MigCluster configmap OPERATOR_VERSION disagrees with
			// MigCluster.Status.OperatorVersion
			versionsMatched, err := cluster.OperatorVersionMatchesConfigmap(r.Client)
			if err != nil {
				continue
			}
			if !versionsMatched {
				r.enqueue(cluster)
				continue
			}

		}
	}
}

// Enqueue a reconcile request.
func (r *RemoteClusterSource) enqueue(cluster v1alpha1.MigCluster) {
	clusterEvent := event.GenericEvent{
		Object: &cluster,
	}
	for _, p := range r.predicates {
		if !p.Generic(clusterEvent) {
			return
		}
	}

	r.handler.Generic(clusterEvent, r.queue)
}
