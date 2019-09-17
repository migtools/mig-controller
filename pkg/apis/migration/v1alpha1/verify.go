package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ImagePullBackOff is unhealthy container state
	ImagePullBackOff = "ImagePullBackOff"
	// CrashLoopBackOff is unhealthy container state
	CrashLoopBackOff = "CrashLoopBackOff"
)

// UnhealthyResources is a store for unhealthy namespaces across clusters
type UnhealthyResources struct {
	Source      []UnhealthyNamespace `json:"source,omitempty"`
	Destination []UnhealthyNamespace `json:"destination,omitempty"`
}

// Workload is a store for unhealthy resource and it's dependents
type Workload struct {
	Name      string   `json:"name"`
	Resources []string `json:"resources,omitempty"`
}

// UnhealthyNamespace is a store for unhealthy resources in a namespace
type UnhealthyNamespace struct {
	UnhealthyNamespace string     `json:"unhealthyNamespace"`
	Workloads          []Workload `json:"workloads"`
}

// VerifyPods would collect, display and notify about the pod health in a specific namespace
func (u *UnhealthyResources) VerifyPods(client k8sclient.Client, options *k8sclient.ListOptions) (*[]unstructured.Unstructured, error) {
	podList := corev1.PodList{}

	err := client.List(context.TODO(), options, &podList)
	if err != nil {
		return nil, err
	}

	unhealthy := []unstructured.Unstructured{}
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodFailed && !containerUnhealthy(pod) {
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

func containerUnhealthy(pod corev1.Pod) bool {
	// Consider suspicious containers as an error state
	podContainersStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, containerStatus := range podContainersStatuses {
		if containerStatus.Ready {
			continue
		}
		terminationDetails := containerStatus.State.Terminated
		if terminationDetails != nil && terminationDetails.ExitCode != 0 {
			return true
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

func (u *UnhealthyResources) findResources(namespace string, unhealthyResources ...*[]UnhealthyNamespace) *UnhealthyNamespace {
	for _, namespaces := range unhealthyResources {
		for _, unhealthyNamespace := range *namespaces {
			if unhealthyNamespace.UnhealthyNamespace == namespace {
				return &unhealthyNamespace
			}
		}
	}
	return nil
}

func (u *UnhealthyResources) findWorkload(resources *UnhealthyNamespace, name string) *Workload {
	for _, workload := range resources.Workloads {
		if workload.Name == name {
			return &workload
		}
		for _, subWorkload := range workload.Resources {
			if subWorkload == name {
				return &workload
			}
		}
	}
	return nil
}

// AddWorkload is adding a workload to selected list of unhealthy namespaces
func (u *UnhealthyResources) AddWorkload(cluster *[]UnhealthyNamespace, namespace, name string, resourceNames ...string) {
	workload := Workload{
		Name:      name,
		Resources: resourceNames,
	}
	resources := u.findResources(namespace, cluster)
	if resources == nil {
		unhealthyNamespace := UnhealthyNamespace{
			UnhealthyNamespace: namespace,
			Workloads:          []Workload{workload},
		}
		*cluster = append(*cluster, unhealthyNamespace)
	} else {
		resources.Workloads = append(resources.Workloads, workload)
	}
}

// AddUnhealthyResources is adding unhealthy workloads and resolves owner references
func (u *UnhealthyResources) AddUnhealthyResources(
	client k8sclient.Client,
	unhealthyNamespaces *[]UnhealthyNamespace,
	namespace string,
	unhealthy *[]unstructured.Unstructured,
) error {
	parentMapping := make(map[string][]string)

	if unhealthyNamespaces == nil {
		unhealthyNamespaces = &[]UnhealthyNamespace{}
	}

	for _, res := range *unhealthy {
		workloadName := fmt.Sprintf("%s/%s", res.GetKind(), res.GetName())

		owners, err := resolveOwners(res.GetNamespace(), res.GetOwnerReferences(), client)
		if err != nil {
			return err
		}

		if len(owners) == 0 {
			u.AddWorkload(unhealthyNamespaces, res.GetNamespace(), workloadName)
			continue
		}

		for _, owner := range owners {
			ownerName := owner.GetKind() + "/" + owner.GetName()
			_, found := parentMapping[ownerName]
			if found {
				parentMapping[ownerName] = append(parentMapping[ownerName], workloadName)
			} else {
				parentMapping[ownerName] = []string{workloadName}
			}
		}
	}

	for owner, resources := range parentMapping {
		u.AddWorkload(unhealthyNamespaces, namespace, owner, resources...)
	}

	return nil
}

func resolveOwners(namespace string, owners []v1.OwnerReference, client k8sclient.Client) ([]*unstructured.Unstructured, error) {
	parentResources := []*unstructured.Unstructured{}
	for _, ownerReference := range owners {
		ownerResource, err := getOwnerFromReference(namespace, ownerReference, client)
		if err != nil {
			return nil, err
		}

		parents := ownerResource.GetOwnerReferences()
		if len(parents) == 0 {
			parentResources = append(parentResources, ownerResource)
		} else {
			resolved, err := resolveOwners(namespace, ownerResource.GetOwnerReferences(), client)
			if err != nil {
				return nil, err
			}
			parentResources = append(parentResources, resolved...)
		}
	}

	return parentResources, nil
}

func getOwnerFromReference(namespace string, owner v1.OwnerReference, client k8sclient.Client) (*unstructured.Unstructured, error) {
	ownerResource := &unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{
		Kind: strings.ToLower(owner.Kind),
	}
	if strings.Contains(owner.APIVersion, "/") {
		groupVersion := strings.SplitN(owner.APIVersion, "/", 2)
		gvk.Group, gvk.Version = groupVersion[0], groupVersion[1]
	} else {
		gvk.Version = owner.APIVersion
	}
	ownerResource.SetGroupVersionKind(gvk)

	ownerReference := types.NamespacedName{
		Namespace: namespace,
		Name:      owner.Name,
	}
	err := client.Get(context.TODO(), ownerReference, ownerResource)
	return ownerResource, err
}
