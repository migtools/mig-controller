package migplan

import (
	"context"
	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)


// A registry health checker which routinely checks whether registries are healthy or not
type registryHealth struct {
	hostClient client.Client
	Interval   time.Duration
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
	podLabels map[string]string
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
	event := event.GenericEvent{
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
				//TODO Error condition would result in loosing the watch util the pod restarts
				return
			}
			destCluster, err := plan.GetDestinationCluster(r.hostClient)
			if err != nil {
				log.Trace(err)
				return
			}

			if srcCluster.Status.IsReady() {
				srcClient, err := srcCluster.GetClient(r.hostClient)
				if err != nil {
					log.Trace(err)
					return
				}

				registryPodsSrc, err := r.getRegistryPods(plan, srcClient)
				if err != nil {
					log.Trace(err)
					return
				}

				for _, registryPod := range registryPodsSrc.Items{
					if !registryPod.Status.ContainerStatuses[0].Ready {
						r.enqueue(plan)
						continue
					}
				}
			}

			if destCluster.Status.IsReady() {
				destClient, err := destCluster.GetClient(r.hostClient)
				if err != nil {
					log.Trace(err)
					return
				}

				registryPodsDest, err := r.getRegistryPods(plan, destClient)
				if err != nil {
					log.Trace(err)
					return
				}

				for _, registryPod := range registryPodsDest.Items{
					if !registryPod.Status.ContainerStatuses[0].Ready {
						r.enqueue(plan)
						continue
					}
				}
			}


		}

	}
//TODO: need see if this go routine should be stopped and returned

}

func (r *registryHealth) getRegistryPods(plan v1alpha1.MigPlan, registryClient compat.Client) (corev1.PodList, error){

	registryPodList := corev1.PodList{}
	err := registryClient.List(context.TODO(), k8sclient.MatchingLabels(plan.GetLabels()), &registryPodList)
	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	return registryPodList, nil
}
