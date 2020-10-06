package migplan

import (
	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	podLabels  map[string]string
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

	for {
		time.Sleep(r.Interval)
		planList, err := v1alpha1.ListPlans(r.hostClient)
		if err != nil {
			log.Trace(err)
			return
		}

		for _, plan := range planList {
			//TODO avoid race condition check
			srcCluster, err := plan.GetSourceCluster(r.hostClient)
			if err != nil {
				log.Trace(err)
				//TODO Error condition would result in loosing the watch util the pod restarts
				return
			}

			if !srcCluster.Status.IsReady() {
				log.Info("Cannot check registry pod health, cluster is not ready", srcCluster.Name, plan.Name)
				return
			}

			srcClient, err := srcCluster.GetClient(r.hostClient)
			if err != nil {
				return
			}

			destCluster, err := plan.GetDestinationCluster(r.hostClient)
			if err != nil {
				log.Trace(err)
				return
			}

			if !destCluster.Status.IsReady() {
				log.Info("Cannot check registry pod health, cluster is not ready", destCluster.Name, plan.Name)
				return
			}

			destClient, err := destCluster.GetClient(r.hostClient)
			if err != nil {
				return
			}

			isSrcRegistryPodUnhealthy, _, err := isRegistryPodUnHealthy(&plan, srcClient)
			if err != nil {
				log.Trace(err)
				return
			}

			isDestRegistryPodUnhealthy, _, err := isRegistryPodUnHealthy(&plan, destClient)
			if err != nil {
				log.Trace(err)
				return
			}

			switch  {
			//enqueue a reconcile event when the src registry pod is unhealthy and the plan is ready
			case isSrcRegistryPodUnhealthy && plan.Status.HasCondition(RegistriesHealthy):
				r.enqueue(plan)
				continue

			//enqueue a reconcile event when the src registry pod is healthy and the plan is not ready
			case !isSrcRegistryPodUnhealthy && !plan.Status.HasCondition(RegistriesHealthy):
				r.enqueue(plan)
				continue

			//enqueue a reconcile event when the destination registry pod is unhealthy and the plan is ready
			case isDestRegistryPodUnhealthy && plan.Status.HasCondition(RegistriesHealthy):
				r.enqueue(plan)
				continue

			//enqueue a reconcile event when the destination registry pod is healthy and the plan is not ready
			case !isDestRegistryPodUnhealthy && !plan.Status.HasCondition(RegistriesHealthy):
				r.enqueue(plan)
				continue
			}
		}

	}
	//TODO: need see if this go routine should be stopped and returned
}

