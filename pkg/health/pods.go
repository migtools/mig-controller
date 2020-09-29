package health

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ImagePullBackOff is unhealthy container state
	ImagePullBackOff = "ImagePullBackOff"
	// CrashLoopBackOff is unhealthy container state
	CrashLoopBackOff = "CrashLoopBackOff"
)

var podManagingResources = [...]schema.GroupVersionKind{
	schema.GroupVersionKind{
		Group: "apps",
		Kind:  "statefulset",
	},
	schema.GroupVersionKind{
		Group: "apps",
		Kind:  "deployment",
	},
	schema.GroupVersionKind{
		Group: "",
		Kind:  "replicationcontroller",
	},
	schema.GroupVersionKind{
		Group: "extensions",
		Kind:  "replicaset",
	},
	schema.GroupVersionKind{
		Group: "apps.openshift.io",
		Kind:  "deploymentconfig",
	},
}

// PodManagersRecreated does verify that ReplicaSets, Deployments,
// Statefulsets and DeploymentConfigs pod replicas match expected number
func PodManagersRecreated(client k8sclient.Client, options *k8sclient.ListOptions) (bool, error) {
	podManagersList := unstructured.UnstructuredList{}

	for _, gvk := range podManagingResources {
		podManagersList.SetGroupVersionKind(gvk)
		err := client.List(context.TODO(), options, &podManagersList)
		if err != nil {
			return false, err
		}

		for _, podManager := range podManagersList.Items {
			expectedReplicas, found, err := unstructured.NestedFieldNoCopy(podManager.Object, "spec", "replicas")
			if err != nil {
				return false, err
			}
			if !found {
				err = fmt.Errorf("Replicas field was not found in %s kind", podManager.GetKind())
				return false, err
			}
			replicas, ok := expectedReplicas.(int64)
			if !ok {
				err = fmt.Errorf("Can't convert Replicas field to int64 for %s kind", podManager.GetKind())
				return false, err
			}
			if replicas == 0 {
				continue
			}

			readyReplicas, foundReady, err := unstructured.NestedFieldNoCopy(podManager.Object, "status", "readyReplicas")
			if err != nil {
				return false, err
			}

			if !foundReady || expectedReplicas != readyReplicas {
				return false, nil
			}
		}
	}

	return true, nil
}

// DaemonSetsRecreated checks the number of pod replicas managed by DaemonSets
func DaemonSetsRecreated(client k8sclient.Client, options *k8sclient.ListOptions) (bool, error) {
	daemonSetList := v1beta1.DaemonSetList{}

	err := client.List(context.TODO(), options, &daemonSetList)
	if err != nil {
		return false, err
	}

	for _, ds := range daemonSetList.Items {
		if ds.Status.DesiredNumberScheduled != ds.Status.NumberReady {
			return false, nil
		}
	}

	return true, nil
}

// PodsUnhealthy would collect unhealthy pods in a specific namespace
func PodsUnhealthy(client k8sclient.Client, options *k8sclient.ListOptions) (*[]unstructured.Unstructured, error) {
	podList := corev1.PodList{}

	err := client.List(context.TODO(), options, &podList)
	if err != nil {
		return nil, err
	}

	unhealthy := []unstructured.Unstructured{}
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodFailed &&
			!ContainerUnhealthy(pod) &&
			!UnhealthyStatusConditions(pod) {
			continue
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		if err != nil {
			return nil, err
		}

		unstructuredPod := unstructured.Unstructured{Object: obj}
		unstructuredPod.SetKind("Pod")
		unhealthy = append(unhealthy, unstructuredPod)
	}

	return &unhealthy, nil
}

func UnhealthyStatusConditions(pod corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodRunning {
		return false
	}

	// Unschedulable containers are not expected to be present
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReasonUnschedulable {
			return true
		}
	}
	return false
}

func ContainerUnhealthy(pod corev1.Pod) bool {
	// Consider suspicious containers as an error state
	podContainersStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, containerStatus := range podContainersStatuses {
		if containerStatus.Ready {
			continue
		}
		waitingDetails := containerStatus.State.Waiting
		if waitingDetails == nil {
			continue
		}
		switch waitingDetails.Reason {
		case ImagePullBackOff, CrashLoopBackOff:
			break
		default:
			continue
		}

		return true
	}

	return false
}
