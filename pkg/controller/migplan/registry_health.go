package migplan

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// A registry health checker which routinely checks whether registries are healthy or not for migplan's running migrations
type registryHealth struct {
	hostClient client.Client
	Interval   time.Duration
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
	podLabels  map[string]string
	planLabels map[string]string
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
	e := event.GenericEvent{
		Meta:   &plan.ObjectMeta,
		Object: &plan,
	}
	for _, p := range r.predicates {
		if !p.Generic(e) {
			return
		}
	}

	r.handler.Generic(e, r.queue)
}

//Run the health checks for registry pods
func (r *registryHealth) run() {
	//List all the migplans that are in running state using the hostClient
	//Now using srcClient and destClient for each migplan find the registry pods, if registry container is not ready then enqueue this migplan
	//repeat all the above steps for all the plans for both clusters

	for {
		time.Sleep(r.Interval)
		planList, err := v1alpha1.ListPlansWithLabels(r.hostClient, r.planLabels)
		if err != nil {
			log.Trace(err)
			return
		}
		if planList != nil {
			for _, plan := range planList {
				//TODO avoid race condition check
				srcCluster, err := plan.GetSourceCluster(r.hostClient)
				if err != nil {
					log.Trace(err)
					//TODO Error condition would result in loosing the watch util the pod restarts
				}

				if !srcCluster.Status.IsReady() {
					log.Info("Cannot check registry pod health, cluster is not ready", srcCluster.Name, plan.Name)
				}

				srcClient, err := srcCluster.GetClient(r.hostClient)
				if err != nil {
					log.Trace(err)
				}

				destCluster, err := plan.GetDestinationCluster(r.hostClient)
				if err != nil {
					log.Trace(err)
				}

				if !destCluster.Status.IsReady() {
					log.Info("Cannot check registry pod health, cluster is not ready", destCluster.Name, plan.Name)
				}

				destClient, err := destCluster.GetClient(r.hostClient)
				if err != nil {
					log.Trace(err)
				}

				srcRegistryPods, err := getRegistryPods(&plan, srcClient)
				if err != nil {
					log.Trace(err)
				}

				destRegistryPods, err := getRegistryPods(&plan, destClient)
				if err != nil {
					log.Trace(err)
				}

				if r.checkPodHealthAndPlanConditionToEnqueue(srcRegistryPods, &plan) {
					r.enqueue(plan)
					continue
				}

				if r.checkPodHealthAndPlanConditionToEnqueue(destRegistryPods, &plan) {
					r.enqueue(plan)
					continue
				}
			}
		}

	}
	//TODO: need see if this go routine should be stopped and returned
}

func (r *registryHealth) checkPodHealthAndPlanConditionToEnqueue(podList corev1.PodList, plan *v1alpha1.MigPlan) bool {

	podStateUnhealthy, _ := isRegistryPodUnHealthy(podList)
	enqueue := false

	switch {

	case podStateUnhealthy && plan.Status.HasCondition(RegistriesHealthy) && plan.Status.IsReady():
		//enqueue a reconcile event when the registry pod is unhealthy and the plan is ready
		enqueue = true

	case !podStateUnhealthy && !plan.Status.HasCondition(RegistriesHealthy) && !plan.Status.IsReady():
		//enqueue a reconcile event when the registry pod is healthy and the plan is not ready
		enqueue = true
	}

	return enqueue
}
