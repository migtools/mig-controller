package migmigration

import (
	"context"

	"github.com/konveyor/mig-controller/pkg/reference"
	appsv1 "github.com/openshift/api/apps/v1"
	k8sappsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"

	extv1b1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// All quiesce functionality should be put here
func (t *Task) quiesceApplications() error {
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	discovery, err := t.getSourceDiscovery()
	if err != nil {
		log.Trace(err)
		return err
	}
	kubeVersion, err := reference.GetKubeVersion(discovery)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.suspendCronJobs(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownDeploymentConfigs(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownDeployments(client, kubeVersion)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownStatefulSets(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownReplicaSets(client, kubeVersion)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownDaemonSets(client, kubeVersion)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = t.scaleDownJobs(client)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Scales down DeploymentConfig on source cluster
func (t *Task) scaleDownDeploymentConfigs(client k8sclient.Client) error {
	for _, ns := range t.sourceNamespaces() {
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
func (t *Task) scaleDownDeployments(client k8sclient.Client, kubeVersion int) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		dList := &k8sappsv1.DeploymentList{}
		options := k8sclient.InNamespace(ns)
		if kubeVersion >= reference.AppsGap {
			err := client.List(
				context.TODO(),
				options,
				dList)
			if err != nil {
				log.Trace(err)
				return err
			}
		} else {
			dListOld := &v1beta1.DeploymentList{}
			err := client.List(
				context.TODO(),
				options,
				dListOld)
			if err != nil {
				log.Trace(err)
				return err
			}

			err = t.scheme.Convert(dListOld, dList, nil)
			if err != nil {
				log.Trace(err)
				return err
			}
		}

		for _, deployment := range dList.Items {
			if deployment.Spec.Replicas == &zero {
				continue
			}
			deployment.Spec.Replicas = &zero
			err := client.Update(context.TODO(), &deployment)
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
	for _, ns := range t.sourceNamespaces() {
		list := v1beta1.StatefulSetList{}
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
func (t *Task) scaleDownReplicaSets(client k8sclient.Client, kubeVersion int) error {
	zero := int32(0)
	for _, ns := range t.sourceNamespaces() {
		options := k8sclient.InNamespace(ns)
		rsList := &k8sappsv1.ReplicaSetList{}
		if kubeVersion >= reference.AppsGap {
			err := client.List(context.TODO(), options, rsList)
			if err != nil {
				log.Trace(err)
				return err
			}
		} else {
			rsListOld := &extv1b1.ReplicaSetList{}
			err := client.List(context.TODO(), options, rsListOld)
			if err != nil {
				log.Trace(err)
				return err
			}
			err = t.scheme.Convert(rsListOld, rsList, nil)
			if err != nil {
				log.Trace(err)
				return err
			}
		}

		for _, set := range rsList.Items {
			if set.Spec.Replicas == &zero {
				continue
			}
			set.Spec.Replicas = &zero
			err := client.Update(context.TODO(), &set)
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
func (t *Task) scaleDownDaemonSets(client k8sclient.Client, kubeVersion int) error {
	for _, ns := range t.sourceNamespaces() {
		options := k8sclient.InNamespace(ns)
		rsList := &k8sappsv1.DaemonSetList{}
		if kubeVersion >= reference.AppsGap {
			err := client.List(
				context.TODO(),
				options,
				rsList)
			if err != nil {
				log.Trace(err)
				return err
			}
		} else {
			rsListOld := &extv1b1.DaemonSetList{}
			err := client.List(
				context.TODO(),
				options,
				rsListOld)
			if err != nil {
				log.Trace(err)
				return err
			}
			err = t.scheme.Convert(rsListOld, rsList, nil)
			if err != nil {
				log.Trace(err)
				return err
			}
		}

		for _, set := range rsList.Items {
			if set.Spec.Template.Spec.NodeSelector == nil {
				set.Spec.Template.Spec.NodeSelector = make(map[string]string)
			} else if set.Spec.Template.Spec.NodeSelector[quiesceNodeSelector] == quiesceNodeSelectorVal {
				continue
			}
			set.Spec.Template.Spec.NodeSelector[quiesceNodeSelector] = quiesceNodeSelectorVal
			err := client.Update(context.TODO(), &set)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}
	return nil
}

// Suspends all CronJobs
// Using unstructured because OCP 3.7 supports batch/v2alpha1 which is
// not supported in newer versions such as OCP 3.11.
func (t *Task) suspendCronJobs(client k8sclient.Client) error {
	fields := []string{"spec", "suspend"}
	for _, ns := range t.sourceNamespaces() {
		options := k8sclient.InNamespace(ns)
		list := unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "batch",
			Kind:  "cronjob",
		})
		err := client.List(context.TODO(), options, &list)
		if err != nil {
			log.Trace(err)
			return err
		}
		for _, r := range list.Items {
			suspend, found, err := unstructured.NestedBool(r.Object, fields...)
			if err != nil {
				log.Trace(err)
				return err
			}
			if found && suspend {
				continue
			}
			unstructured.SetNestedField(r.Object, true, fields...)
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
func (t *Task) scaleDownJobs(client k8sclient.Client) error {
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
