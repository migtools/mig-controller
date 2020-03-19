package migmigration

import (
	"context"
	"encoding/json"
	"strconv"

	ocappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ReplicasAnnotation - annotation to set during quisce phase, which will allow to recreate the app back to it's initial state
// in case of a migration failure or user's intent
const (
	SuspendAnnotation      = "migration.openshift.io/preQuiesceSuspend"
	ReplicasAnnotation     = "migration.openshift.io/preQuiesceReplicas"
	NodeSelectorAnnotation = "migration.openshift.io/preQuiesceNodeSelector"
)

// All quiesce functionality should be put here
func (t *Task) quiesceApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceCronJobs(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceDeploymentConfigs(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceDeployments(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceStatefulSets(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceReplicaSets(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceDaemonSets(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceJobs(client, true)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// All quiesce undo functionality should be put here
func (t *Task) reactivateApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceCronJobs(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceDeploymentConfigs(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceDeployments(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceStatefulSets(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceReplicaSets(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceDaemonSets(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.quiesceJobs(client, false)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Scales down DeploymentConfig on source cluster
func (t *Task) quiesceDeploymentConfigs(client k8sclient.Client, suspend bool) error {
	for _, ns := range t.sourceNamespaces() {
		list := ocappsv1.DeploymentConfigList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, dc := range list.Items {
			if suspend {
				if dc.Spec.Replicas == 0 {
					continue
				}
				dc.Annotations[ReplicasAnnotation] = string(dc.Spec.Replicas)
				dc.Spec.Replicas = 0
			} else {
				replicas, exist := dc.Annotations[ReplicasAnnotation]
				if !exist {
					continue
				}
				number, err := strconv.Atoi(replicas)
				if err != nil {
					log.Trace(err)
					return err
				}
				dc.Annotations = removeAnnotation(dc.Annotations, ReplicasAnnotation)
				dc.Spec.Replicas = int32(number)
			}

			err = client.Update(context.TODO(), &dc)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}

// Scales down all Deployments
func (t *Task) quiesceDeployments(client k8sclient.Client, suspend bool) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, deployment := range list.Items {
			if suspend {
				if *deployment.Spec.Replicas == zero {
					continue
				}
				deployment.Annotations[ReplicasAnnotation] = string(*deployment.Spec.Replicas)
				deployment.Spec.Replicas = &zero
			} else {
				replicas, exist := deployment.Annotations[ReplicasAnnotation]
				if !exist {
					continue
				}
				number, err := strconv.Atoi(replicas)
				if err != nil {
					log.Trace(err)
					return err
				}
				deployment.Annotations = removeAnnotation(deployment.Annotations, ReplicasAnnotation)
				restoredReplicas := int32(number)
				deployment.Spec.Replicas = &restoredReplicas
			}
			err = client.Update(context.TODO(), &deployment)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}

// Scales down all StatefulSets.
func (t *Task) quiesceStatefulSets(client k8sclient.Client, suspend bool) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.StatefulSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, set := range list.Items {
			if suspend {
				if *set.Spec.Replicas == zero {
					continue
				}
				set.Annotations[ReplicasAnnotation] = string(*set.Spec.Replicas)
				set.Spec.Replicas = &zero
			} else {
				replicas, exist := set.Annotations[ReplicasAnnotation]
				if !exist {
					continue
				}
				number, err := strconv.Atoi(replicas)
				if err != nil {
					log.Trace(err)
					return err
				}
				set.Annotations = removeAnnotation(set.Annotations, ReplicasAnnotation)
				restoredReplicas := int32(number)
				set.Spec.Replicas = &restoredReplicas
			}
			err = client.Update(context.TODO(), &set)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}
	return nil
}

// Scales down all ReplicaSets.
func (t *Task) quiesceReplicaSets(client k8sclient.Client, suspend bool) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.ReplicaSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, set := range list.Items {
			if suspend {
				if *set.Spec.Replicas == zero {
					continue
				}
				set.Annotations[ReplicasAnnotation] = string(*set.Spec.Replicas)
				set.Spec.Replicas = &zero
			} else {
				replicas, exist := set.Annotations[ReplicasAnnotation]
				if !exist {
					continue
				}
				number, err := strconv.Atoi(replicas)
				if err != nil {
					log.Trace(err)
					return err
				}
				set.Annotations = removeAnnotation(set.Annotations, ReplicasAnnotation)
				restoredReplicas := int32(number)
				set.Spec.Replicas = &restoredReplicas
			}
			err = client.Update(context.TODO(), &set)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}
	return nil
}

// values to support nodeSelector-based DaemonSet quiesce
const (
	quiesceNodeSelector    = "openshift.io/quiesceDaemonSet"
	quiesceNodeSelectorVal = "quiesce"
)

// Scales down all DaemonSets.
func (t *Task) quiesceDaemonSets(client k8sclient.Client, suspend bool) error {
	for _, ns := range t.sourceNamespaces() {
		list := appsv1.DaemonSetList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, set := range list.Items {
			if suspend {
				if set.Spec.Template.Spec.NodeSelector == nil {
					set.Spec.Template.Spec.NodeSelector = map[string]string{}
				} else if set.Spec.Template.Spec.NodeSelector[quiesceNodeSelector] == quiesceNodeSelectorVal {
					continue
				}
				selector, err := json.Marshal(set.Spec.Template.Spec.NodeSelector)
				if err != nil {
					log.Trace(err)
					return err
				}
				set.Annotations[NodeSelectorAnnotation] = string(selector)
				set.Spec.Template.Spec.NodeSelector[quiesceNodeSelector] = quiesceNodeSelectorVal
			} else {
				selector, exist := set.Annotations[NodeSelectorAnnotation]
				if !exist {
					continue
				}
				nodeSelector := map[string]string{}
				err := json.Unmarshal([]byte(selector), &nodeSelector)
				if err != nil {
					log.Trace(err)
					return err
				}
				set.Annotations = removeAnnotation(set.Annotations, NodeSelectorAnnotation)
				set.Spec.Template.Spec.NodeSelector = nodeSelector
			}
			err = client.Update(context.TODO(), &set)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}
	return nil
}

// Suspends all CronJobs
func (t *Task) suspendCronJobs(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
		list := batchv1beta.CronJobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, &list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, r := range list.Items {
			if r.Spec.Suspend != nil && *r.Spec.Suspend {
				continue
			}
			r.Spec.Suspend = pointer.BoolPtr(true)
			err = client.Update(context.TODO(), &r)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}

// Scales down all Jobs
func (t *Task) quiesceJobs(client k8sclient.Client, suspend bool) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		list := batchv1.JobList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, job := range list.Items {
			if suspend {
				if job.Spec.Parallelism == &zero {
					continue
				}
				job.Annotations[ReplicasAnnotation] = string(*job.Spec.Parallelism)
				job.Spec.Parallelism = &zero
			} else {
				replicas, exist := job.Annotations[ReplicasAnnotation]
				if !exist {
					continue
				}
				number, err := strconv.Atoi(replicas)
				if err != nil {
					log.Trace(err)
					return err
				}
				job.Annotations = removeAnnotation(job.Annotations, ReplicasAnnotation)
				parallelReplicas := int32(number)
				job.Spec.Parallelism = &parallelReplicas
			}
			err = client.Update(context.TODO(), &job)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}

// Ensure scaled down pods have terminated.
// Returns: `true` when all pods terminated.
func (t *Task) ensureQuiescedPodsTerminated() (bool, error) {
	kinds := map[string]bool{
		"ReplicationController": true,
		"StatefulSet":           true,
		"ReplicaSet":            true,
		"DaemonSet":             true,
		"Job":                   true,
	}
	skippedPhases := map[v1.PodPhase]bool{
		v1.PodSucceeded: true,
		v1.PodFailed:    true,
		v1.PodUnknown:   true,
	}
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return false, err
	}
	for _, ns := range t.sourceNamespaces() {
		list := v1.PodList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(
			context.TODO(),
			options,
			&list)
		if err != nil {
			log.Trace(err)
			return false, err
		}
		for _, pod := range list.Items {
			if _, found := skippedPhases[pod.Status.Phase]; found {
				continue
			}
			for _, ref := range pod.OwnerReferences {
				if _, found := kinds[ref.Kind]; found {
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func removeAnnotation(annotations map[string]string, toRemove string) map[string]string {
	updatedAnnotations := make(map[string]string)
	for key, value := range updatedAnnotations {
		if key != toRemove {
			updatedAnnotations[key] = value
		}
	}

	return updatedAnnotations
}
