package migmigration

import (
	"context"
	appsv1 "github.com/openshift/api/apps/v1"
	coreappsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// All quiesce functionality should be put here
func (t *Task) quiesceApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.suspendCronJobs(client)
	if err != nil {
		log.Trace(err)
	}
	err = t.scaleDownDeploymentConfigs(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownDeployments(client)
	if err != nil {
		log.Trace(err)
	}
	err = t.scaleDownStatefulSets(client)
	if err != nil {
		log.Trace(err)
	}
	err = t.scaleDownReplicaSets(client)
	if err != nil {
		log.Trace(err)
	}
	err = t.scaleDownDaemonSets(client)
	if err != nil {
		log.Trace(err)
	}
	err = t.scaleDownJobs(client)
	if err != nil {
		log.Trace(err)
	}

	return err
}

// Scales down DeploymentConfig on source cluster
func (t *Task) scaleDownDeploymentConfigs(client k8sclient.Client) error {
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := appsv1.DeploymentConfigList{}
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
			if dc.Spec.Replicas == 0 {
				continue
			}
			dc.Spec.Replicas = 0
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
func (t *Task) scaleDownDeployments(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := coreappsv1.DeploymentList{}
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
			if deployment.Spec.Replicas == &zero {
				continue
			}
			deployment.Spec.Replicas = &zero
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
func (t *Task) scaleDownStatefulSets(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := coreappsv1.StatefulSetList{}
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
			if set.Spec.Replicas == &zero {
				continue
			}
			set.Spec.Replicas = &zero
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
func (t *Task) scaleDownReplicaSets(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := coreappsv1.ReplicaSetList{}
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
			if set.Spec.Replicas == &zero {
				continue
			}
			set.Spec.Replicas = &zero
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
func (t *Task) scaleDownDaemonSets(client k8sclient.Client) error {
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := coreappsv1.DaemonSetList{}
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
			if set.Spec.Template.Spec.NodeSelector == nil {
				set.Spec.Template.Spec.NodeSelector = make(map[string]string)
			} else if set.Spec.Template.Spec.NodeSelector[quiesceNodeSelector] == quiesceNodeSelectorVal {
				continue
			}
			set.Spec.Template.Spec.NodeSelector[quiesceNodeSelector] = quiesceNodeSelectorVal
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
	trueVal := true
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
		list := batchv1beta1.CronJobList{}
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
			if job.Spec.Suspend != nil && *job.Spec.Suspend == true {
				continue
			}
			job.Spec.Suspend = &trueVal
			err = client.Update(context.TODO(), &job)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return nil
}

// Scales down all Jobs
func (t *Task) scaleDownJobs(client k8sclient.Client) error {
	zero := int32(0)
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
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
			if job.Spec.Parallelism == &zero {
				continue
			}
			job.Spec.Parallelism = &zero
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
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return false, err
	}
	for _, ns := range t.PlanResources.MigPlan.Spec.Namespaces {
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
			for _, ref := range pod.OwnerReferences {
				if _, found := kinds[ref.Kind]; found {
					return false, nil
				}
			}
		}
	}

	return true, nil
}
