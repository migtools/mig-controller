package gvk

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1beta1", // Should become v1 after 1.17, needs downscaling
	Resource: "customresourcedefinitions",
}

// Compare is a store for discovery and dynamic clients to do GVK compare
type Compare struct {
	Plan         *migapi.MigPlan
	SrcDiscovery discovery.DiscoveryInterface
	DstDiscovery discovery.DiscoveryInterface
	SrcClient    dynamic.Interface
}

// Compare GVKs on both clusters, find incompatible GVKs
// and check each plan source namespace for existence of incompatible GVKs
func (r *Compare) Compare() (map[string][]schema.GroupVersionResource, error) {
	srcResourceList, err := collectResources(r.SrcDiscovery)
	if err != nil {
		return nil, err
	}

	dstResourceList, err := collectResources(r.DstDiscovery)
	if err != nil {
		return nil, err
	}

	srcResourceList, err = r.excludeCRDs(srcResourceList)
	if err != nil {
		return nil, err
	}

	resourcesDiff := compareResources(srcResourceList, dstResourceList)
	incompatibleGVKs, err := convertToGVRList(resourcesDiff)
	if err != nil {
		return nil, err
	}

	return r.collectIncompatibleMapping(incompatibleGVKs)
}

// CollectResources collects all namespaced scoped apiResources from the cluster
func collectResources(discovery discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
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

// convertToGVRList converts provided apiResourceList to list of GroupVersionResources from the server
func convertToGVRList(resourceList []*metav1.APIResourceList) ([]schema.GroupVersionResource, error) {
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

// GetGVRsForCluster collects all namespaced scoped GVRs for the provided cluster compatible client
func GetGVRsForCluster(cluster *migapi.MigCluster, c client.Client) (dynamic.Interface, []schema.GroupVersionResource, error) {
	compat, err := cluster.GetClient(c)
	if err != nil {
		return nil, nil, err
	}
	dynamic, err := dynamic.NewForConfig(compat.RestConfig())
	if err != nil {
		return nil, nil, err
	}
	resourceList, err := collectResources(compat)
	if err != nil {
		return nil, nil, err
	}
	GVRs, err := convertToGVRList(resourceList)
	if err != nil {
		return nil, nil, err
	}
	return dynamic, GVRs, nil
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

func (r *Compare) collectIncompatibleMapping(incompatibleResources []schema.GroupVersionResource) (map[string][]schema.GroupVersionResource, error) {
	incompatibleNamespaces := map[string][]schema.GroupVersionResource{}
	for _, gvk := range incompatibleResources {
		namespaceOccurence, err := r.occurIn(gvk)
		if err != nil {
			return nil, err
		}

		for _, namespace := range namespaceOccurence {
			if inNamespaces(namespace, r.Plan.GetSourceNamespaces()) {
				_, exist := incompatibleNamespaces[namespace]
				if exist {
					incompatibleNamespaces[namespace] = append(incompatibleNamespaces[namespace], gvk)
				} else {
					incompatibleNamespaces[namespace] = []schema.GroupVersionResource{gvk}
				}
			}
		}
	}

	return incompatibleNamespaces, nil
}

func (r *Compare) occurIn(gvr schema.GroupVersionResource) ([]string, error) {
	namespacesOccurred := []string{}
	options := metav1.ListOptions{}
	resourceList, err := r.SrcClient.Resource(gvr).List(options)
	if err != nil {
		return nil, err
	}

	for _, res := range resourceList.Items {
		if !inNamespaces(res.GetNamespace(), namespacesOccurred) {
			namespacesOccurred = append(namespacesOccurred, res.GetNamespace())
		}
	}

	return namespacesOccurred, nil
}

func (r *Compare) excludeCRDs(resources []*metav1.APIResourceList) ([]*metav1.APIResourceList, error) {
	options := metav1.ListOptions{}
	crdList, err := r.SrcClient.Resource(crdGVR).List(options)
	if err != nil {
		return nil, err
	}

	crdGroups := []string{}
	groupPath := []string{"spec", "group"}
	for _, crd := range crdList.Items {
		group, _, err := unstructured.NestedString(crd.Object, groupPath...)
		if err != nil {
			return nil, err
		}
		crdGroups = append(crdGroups, group)
	}

	updatedLists := []*metav1.APIResourceList{}
	for _, resourceList := range resources {
		if !isCRDGroup(resourceList.GroupVersion, crdGroups) {
			updatedLists = append(updatedLists, resourceList)
		}
	}

	return updatedLists, nil
}

func resourceExist(resource metav1.APIResource, resources []metav1.APIResource) bool {
	for _, resourceItem := range resources {
		if resource.Name == resourceItem.Name {
			return true
		}
	}

	return false
}

func compareResources(src []*metav1.APIResourceList, dst []*metav1.APIResourceList) []*metav1.APIResourceList {
	missingResources := []*metav1.APIResourceList{}
	for _, srcList := range src {
		missing := []metav1.APIResource{}
		for _, resource := range srcList.APIResources {
			if !resourceExist(resource, fingResourceList(srcList.GroupVersion, dst)) {
				missing = append(missing, resource)
			}
		}

		if len(missing) > 0 {
			missingList := &metav1.APIResourceList{
				GroupVersion: srcList.GroupVersion,
				APIResources: missing,
			}
			missingResources = append(missingResources, missingList)
		}
	}

	return missingResources
}

func fingResourceList(groupVersion string, list []*metav1.APIResourceList) []metav1.APIResource {
	for _, l := range list {
		if l.GroupVersion == groupVersion {
			return l.APIResources
		}
	}

	return nil
}

func inNamespaces(item string, namespaces []string) bool {
	for _, ns := range namespaces {
		if item == ns {
			return true
		}
	}

	return false
}

func isCRDGroup(group string, crdGroups []string) bool {
	for _, crdGroup := range crdGroups {
		if strings.HasPrefix(group, crdGroup) {
			return true
		}
	}

	return false
}
