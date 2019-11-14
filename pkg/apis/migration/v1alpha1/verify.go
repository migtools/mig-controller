package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// UnhealthyResources is a store for unhealthy namespaces across clusters
type UnhealthyResources struct {
	Namespaces []UnhealthyNamespace `json:"namespaces,omitempty"`
}

// Workload is a store for unhealthy resource and it's dependents
type Workload struct {
	Name      string   `json:"name"`
	Resources []string `json:"resources,omitempty"`
}

// UnhealthyNamespace is a store for unhealthy resources in a namespace
type UnhealthyNamespace struct {
	Name      string     `json:"name"`
	Workloads []Workload `json:"workloads"`
}

// IsHealthy will report if there are any unhealthy namespaces were present
func (m *MigMigration) IsHealthy() bool {
	return m.Status.UnhealthyResources.Namespaces == nil || len(m.Status.UnhealthyResources.Namespaces) == 0
}

// FindResources is responsible for returning a pointer on a single unhealthy namespace entry
func (u *UnhealthyResources) FindResources(namespace string, unhealthyResources ...*[]UnhealthyNamespace) *UnhealthyNamespace {
	for _, namespaces := range unhealthyResources {
		for _, unhealthyNamespace := range *namespaces {
			if unhealthyNamespace.Name == namespace {
				return &unhealthyNamespace
			}
		}
	}
	return nil
}

// FindWorkload will return a workload from the namespace, which match the name. If there is any.
func (u *UnhealthyResources) FindWorkload(resources *UnhealthyNamespace, name string) *Workload {
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

// AddNamespace is adding a workload to selected list of unhealthy namespaces
func (u *UnhealthyResources) AddNamespace(cluster *[]UnhealthyNamespace, namespace, name string, resourceNames ...string) {
	workload := Workload{
		Name:      name,
		Resources: resourceNames,
	}
	resources := u.FindResources(namespace, cluster)
	if resources == nil {
		unhealthyNamespace := UnhealthyNamespace{
			Name:      namespace,
			Workloads: []Workload{workload},
		}
		*cluster = append(*cluster, unhealthyNamespace)
	} else {
		resources.Workloads = append(resources.Workloads, workload)
	}
}

// AddResources is adding unhealthy workloads and resolves owner references
func (u *UnhealthyResources) AddResources(
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
			u.AddNamespace(unhealthyNamespaces, res.GetNamespace(), workloadName)
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
		u.AddNamespace(unhealthyNamespaces, namespace, owner, resources...)
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
