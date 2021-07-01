package health

import (
	"context"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	"k8s.io/apimachinery/pkg/api/meta"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
		Group:   "apps",
		Kind:    "statefulset",
		Version: "v1",
	},
	schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "deployment",
		Version: "v1",
	},
	schema.GroupVersionKind{
		Group:   "",
		Kind:    "replicationcontroller",
		Version: "v1",
	},
	schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "replicaset",
		Version: "v1",
	},
	schema.GroupVersionKind{
		Group:   "apps.openshift.io",
		Kind:    "deploymentconfig",
		Version: "v1",
	},
}

// PodManagersRecreated does verify that ReplicaSets, Deployments,
// Statefulsets and DeploymentConfigs pod replicas match expected number
func PodManagersRecreated(client k8sclient.Client, options *k8sclient.ListOptions) (bool, error) {
	podManagersList := unstructured.UnstructuredList{}

	for _, gvk := range podManagingResources {
		podManagersList.SetGroupVersionKind(gvk)
		err := client.List(context.TODO(), &podManagersList, options)
		if err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			return false, liberr.Wrap(err)
		}

		for _, podManager := range podManagersList.Items {
			expectedReplicas, found, err := unstructured.NestedFieldNoCopy(podManager.Object, "spec", "replicas")
			if err != nil {
				return false, liberr.Wrap(err)
			}
			if !found {
				err = fmt.Errorf("Replicas field was not found in %s kind", podManager.GetKind())
				return false, liberr.Wrap(err)
			}
			replicas, ok := expectedReplicas.(int64)
			if !ok {
				err = fmt.Errorf("Can't convert Replicas field to int64 for %s kind", podManager.GetKind())
				return false, liberr.Wrap(err)
			}
			if replicas == 0 {
				continue
			}

			readyReplicas, foundReady, err := unstructured.NestedFieldNoCopy(podManager.Object, "status", "readyReplicas")
			if err != nil {
				return false, liberr.Wrap(err)
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
	// the use of appsv1 is explicit, the compact client will downconvert to extv1beta1 if needed
	daemonSetList := appsv1.DaemonSetList{}

	err := client.List(context.TODO(), &daemonSetList, options)
	if err != nil {
		return false, liberr.Wrap(err)
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

	err := client.List(context.TODO(), &podList, options)
	if err != nil {
		return nil, err
	}

	unhealthy := []unstructured.Unstructured{}
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodFailed &&
			!containerUnhealthy(pod) &&
			!unhealthyStatusConditions(pod) {
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

func unhealthyStatusConditions(pod corev1.Pod) bool {
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

func containerUnhealthy(pod corev1.Pod) bool {
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
