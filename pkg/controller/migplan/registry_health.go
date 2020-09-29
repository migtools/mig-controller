package migplan

import (
	"context"
	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	health "github.com/konveyor/mig-controller/pkg/health"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	event2 "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)


// A registry health checker which routinely checks whether registries are healthy or not
type registryHealth struct {
	hostClient client.Client
	srcClient  client.Client
	Interval   time.Duration
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

// Start the health checks
func (r *registryHealth) Start(
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate) error {

	r.handler = handler
	r.queue = queue
	r.predicates = predicates
	go r.run()

	return nil
}

// Enqueue a reconcile request event for migplan
func (r *registryHealth) enqueue(plan v1alpha1.MigPlan) {
	event := event2.GenericEvent{
		Meta:   &plan.ObjectMeta,
		Object: &plan,
	}
	for _, predicate := range r.predicates {
		if !predicate.Generic(event) {
			return
		}
	}

	r.handler.Generic(event, r.queue)
}

//Run the health checks for registry pods
func (r *registryHealth) run() {
	//List all the migplans using the hostClient
	//Now using srcClient for each migplan find the registry pods, if registry container is not ready then enqueue this migplan
	//repeat all the above steps for all the plans for both clusters

	for{
		time.Sleep(r.Interval)
		planList, err := v1alpha1.ListPlans(r.hostClient)
		if err != nil {
			log.Trace(err)
			return
		}

		for _, plan := range planList{
			//TODO avoid race condition check


			srcCluster, err := plan.GetSourceCluster(r.hostClient)
			if err != nil {
				log.Trace(err)
				return
			}
			destCluster, err := plan.GetDestinationCluster(r.hostClient)
			if err != nil {
				log.Trace(err)
				return
			}

			srcClient, err := srcCluster.GetClient(r.hostClient)
			if err != nil {
				log.Trace(err)
				return
			}
			destClient, err := destCluster.GetClient(r.hostClient)
			if err != nil {
				log.Trace(err)
				return
			}

			registryPodsSrc, err := r.getRegistryPods(plan, srcClient)
			if err != nil {
				log.Trace(err)
				return
			}

			registryPodsDest, err := r.getRegistryPods(plan, destClient)
			if err != nil {
				log.Trace(err)
				return
			}

			for _, registryPod := range registryPodsSrc.Items{
				if health.ContainerUnhealthy(registryPod) || health.UnhealthyStatusConditions(registryPod) {
					r.enqueue(plan)
					continue
				}
			}

			for _, registryPod := range registryPodsDest.Items{
				if health.ContainerUnhealthy(registryPod) || health.UnhealthyStatusConditions(registryPod) {
					r.enqueue(plan)
					continue
				}
			}
		}

	}

}

func (r *registryHealth) getRegistryPods(plan v1alpha1.MigPlan, registryClient compat.Client) (corev1.PodList, error){

	registryDeployment, err := plan.GetRegistryDeployment(registryClient)
	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	registryPodList := corev1.PodList{}
	err = registryClient.List(context.TODO(), k8sclient.MatchingLabels(registryDeployment.GetLabels()), &registryPodList)
	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	return registryPodList, nil
}