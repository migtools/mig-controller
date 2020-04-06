package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// Incompatible - list of namespaces containing incompatible resources for migration
// which are being selected in the MigPlan
type Incompatible struct {
	Namespaces []IncompatibleNamespace `json:"incompatibleNamespaces,omitempty"`
}

// IncompatibleNamespace - namespace, which is noticed
// to contain resources incompatible by the migration
type IncompatibleNamespace struct {
	Name string            `json:"name"`
	GVKs []IncompatibleGVK `json:"gvks"`
}

// IncompatibleGVK - custom structure for printing GVKs lowercase
type IncompatibleGVK struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// FromGVR - allows to convert the scheme.GVR into lowercase IncompatibleGVK
func FromGVR(gvr schema.GroupVersionResource) IncompatibleGVK {
	return IncompatibleGVK{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    gvr.Resource,
	}
}

// ResourceList returns a list of collected resources, which are not supported by an apiServer on a destination cluster
func (i *Incompatible) ResourceList() (incompatible []string) {
	for _, ns := range i.Namespaces {
		for _, gvk := range ns.GVKs {
			resource := schema.GroupResource{
				Group:    gvk.Group,
				Resource: gvk.Kind,
			}
			incompatible = append(incompatible, resource.String())
		}
	}
	return
}

func CollectResources(discovery discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
	resources, err := discovery.ServerResources()
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		res.APIResources = namespaced(res.APIResources)
		res.APIResources = excludeSubresources(res.APIResources)
		// Some resources appear not to have permissions to list, need to exclude those.
		res.APIResources = listAllowed(res.APIResources)
	}

	return resources, nil
}

func ConvertToGVRList(resourceList []*metav1.APIResourceList) ([]schema.GroupVersionResource, error) {
	GVRs := []schema.GroupVersionResource{}
	for _, resourceList := range resourceList {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, resource := range resourceList.APIResources {
			gvk := gv.WithResource(resource.Name)
			GVRs = append(GVRs, gvk)
		}
	}

	return GVRs, nil
}

func excludeSubresources(resources []metav1.APIResource) []metav1.APIResource {
	filteredList := []metav1.APIResource{}
	for _, res := range resources {
		if !strings.Contains(res.Name, "/") {
			filteredList = append(filteredList, res)
		}
	}

	return filteredList
}

func namespaced(resources []metav1.APIResource) []metav1.APIResource {
	filteredList := []metav1.APIResource{}
	for _, res := range resources {
		if res.Namespaced {
			filteredList = append(filteredList, res)
		}
	}

	return filteredList
}

func listAllowed(resources []metav1.APIResource) []metav1.APIResource {
	filteredList := []metav1.APIResource{}
	for _, res := range resources {
		for _, verb := range res.Verbs {
			if verb == "list" {
				filteredList = append(filteredList, res)
				break
			}
		}
	}

	return filteredList
}
