package gvk

import (
	"context"
	"sort"
	"strings"

	"github.com/konveyor/controller/pkg/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapset "github.com/deckarep/golang-set"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logging.WithName("gvk")

type CohabitatingResource struct {
	resource       string
	groupResource1 schema.GroupResource
	groupResource2 schema.GroupResource
	Seen           bool
}

func NewCohabitatingResource(resource, group1, group2 string) *CohabitatingResource {
	return &CohabitatingResource{
		resource:       resource,
		groupResource1: schema.GroupResource{Group: group1, Resource: resource},
		groupResource2: schema.GroupResource{Group: group2, Resource: resource},
		Seen:           false,
	}
}

func NewCohabitatingResources() map[string]*CohabitatingResource {
	return map[string]*CohabitatingResource{
		"deployments":     NewCohabitatingResource("deployments", "extensions", "apps"),
		"daemonsets":      NewCohabitatingResource("daemonsets", "extensions", "apps"),
		"replicasets":     NewCohabitatingResource("replicasets", "extensions", "apps"),
		"networkpolicies": NewCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
		"events":          NewCohabitatingResource("events", "", "events.k8s.io"),
	}
}

// Compare is a store for discovery and dynamic clients to do GVK compare
type Compare struct {
	Plan                  *migapi.MigPlan
	SrcDiscovery          discovery.DiscoveryInterface
	DstDiscovery          discovery.DiscoveryInterface
	SrcClient             dynamic.Interface
	DstClient             dynamic.Interface
	CohabitatingResources map[string]*CohabitatingResource
}

// Merge the namespace/gvr mappings from the built-in and CRD validation results
func MergeGVRMaps(map1, map2 map[string][]schema.GroupVersionResource) map[string][]schema.GroupVersionResource {
	for key, gvrList2 := range map2 {
		gvrList1, ok := map1[key]
		if !ok {
			map1[key] = gvrList2
		} else {
			map1[key] = append(gvrList1, gvrList2...)
		}
	}
	return map1
}

// Compare GVKs on both clusters, find incompatible GVKs
// and check each plan source namespace for existence of incompatible GVKs
func (r *Compare) Compare() (map[string][]schema.GroupVersionResource, error) {
	preferredSrcResourceList, err := collectPreferredResources(r.SrcDiscovery)
	if err != nil {
		return nil, err
	}
	srcCRDResource, err := collectPreferredCRDResource(r.SrcDiscovery)
	if err != nil {
		return nil, err
	}

	dstResourceList, err := collectNamespacedResources(r.DstDiscovery)
	if err != nil {
		return nil, err
	}

	preferredSrcResourceList, err = r.excludeCRDs(preferredSrcResourceList, srcCRDResource, r.SrcClient)
	if err != nil {
		return nil, err
	}

	resourcesDiff := r.compareResources(preferredSrcResourceList, dstResourceList)
	incompatibleGVKs, err := convertToGVRList(resourcesDiff)
	if err != nil {
		return nil, err
	}

	// Don't report an incompatibleGVK if user settings will skip resource anyways
	excludedResources := toStringSlice(settings.ExcludedInitialResources.Union(toSet(r.Plan.Status.ExcludedResources)))
	filteredGVKs := []schema.GroupVersionResource{}
	for _, gvr := range incompatibleGVKs {
		skip := false
		for _, resource := range excludedResources {
			if strings.EqualFold(gvr.Resource, resource) {
				skip = true
			}
		}
		if !skip {
			filteredGVKs = append(filteredGVKs, gvr)
		}
	}

	return r.collectIncompatibleMapping(filteredGVKs)
}

// Compare CRDs on both clusters, find incompatible ones
// and check each plan source namespace for existence of incompatible CRDs.
// CRDs will be incompatible if the apiextensions CRD APIs (i.e. v1beta1 from 3.11 vs. v1
// from 4.9) are incompatible and there are CRs for the CRD in the source cluster and the CRD
// does not exist in the destination cluster.
func (r *Compare) CompareCRDs() (map[string][]schema.GroupVersionResource, error) {
	srcCRDResource, err := collectPreferredCRDResource(r.SrcDiscovery)
	if err != nil {
		return nil, err
	}

	dstCRDResourceList, err := collectCRDResources(r.DstDiscovery)
	if err != nil {
		return nil, err
	}

	crdGVDiff := r.compareResources(srcCRDResource, dstCRDResourceList)
	// if len(crdGVDiff)>0, then CRD APIVersion is incompatible between src and dest
	if len(crdGVDiff) > 0 {
		srcCRDs, err := collectPreferredResources(r.SrcDiscovery)
		if err != nil {
			return nil, err
		}
		srcCRDs, err = r.includeCRDsOnly(srcCRDs, srcCRDResource, r.SrcClient)

		dstCRDs, err := collectNamespacedResources(r.DstDiscovery)
		if err != nil {
			return nil, err
		}
		dstCRDs, err = r.includeCRDsOnly(dstCRDs, dstCRDResourceList, r.DstClient)
		if err != nil {
			return nil, err
		}
		crdsDiff := r.compareResources(srcCRDs, dstCRDs)
		incompatibleGVKs, err := convertToGVRList(crdsDiff)
		if err != nil {
			return nil, err
		}
		// Don't report an incompatibleGVK if user settings will skip resource anyways
		excludedResources := toStringSlice(settings.ExcludedInitialResources.Union(toSet(r.Plan.Status.ExcludedResources)))
		filteredGVKs := []schema.GroupVersionResource{}
		for _, gvr := range incompatibleGVKs {
			skip := false
			for _, resource := range excludedResources {
				if strings.EqualFold(gvr.Resource, resource) {
					skip = true
				}
			}
			if !skip {
				filteredGVKs = append(filteredGVKs, gvr)
			}
		}

		return r.collectIncompatibleMapping(filteredGVKs)
	}
	return nil, nil
}

func toStringSlice(set mapset.Set) []string {
	interfaceSlice := set.ToSlice()
	var strSlice []string = make([]string, len(interfaceSlice))
	for i, s := range interfaceSlice {
		strSlice[i] = s.(string)
	}
	return strSlice
}
func toSet(strSlice []string) mapset.Set {
	var interfaceSlice []interface{} = make([]interface{}, len(strSlice))
	for i, s := range strSlice {
		interfaceSlice[i] = s
	}
	return mapset.NewSetFromSlice(interfaceSlice)
}

// collectNamespacedResources collects all namespace-scoped apiResources from the cluster
func collectNamespacedResources(discovery discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
	resources, err := discovery.ServerPreferredNamespacedResources()
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

// collectPreferredResources collects all preferred namespaced scoped apiResources from the cluster
func collectPreferredResources(discovery discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
	resources, err := discovery.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		res.APIResources = excludeSubresources(res.APIResources)
		// Some resources appear not to have permissions to list, need to exclude those.
		res.APIResources = listAllowed(res.APIResources)
	}

	return resources, nil
}

// collectPreferredResources collects all preferred namespaced scoped apiResources from the cluster
func collectPreferredCRDResource(discovery discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
	resources, err := discovery.ServerPreferredResources()
	crdResource := []*metav1.APIResourceList{}
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		gv, err := schema.ParseGroupVersion(res.GroupVersion)
		if err != nil {
			continue
		}
		if gv.Group != "apiextensions.k8s.io" {
			continue
		}
		emptyAPIResourceList := metav1.APIResourceList{
			GroupVersion: res.GroupVersion,
		}
		emptyAPIResourceList.APIResources = findCRDGVRs(res.APIResources)
		crdResource = append(crdResource, &emptyAPIResourceList)
		break
	}

	return crdResource, nil
}

// collectNamespacedResources collects all namespace-scoped apiResources from the cluster
func collectCRDResources(discovery discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
	_, resources, err := discovery.ServerGroupsAndResources()
	crdResources := []*metav1.APIResourceList{}
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		gv, err := schema.ParseGroupVersion(res.GroupVersion)
		if err != nil {
			continue
		}
		if gv.Group != "apiextensions.k8s.io" {
			continue
		}
		emptyAPIResourceList := metav1.APIResourceList{
			GroupVersion: res.GroupVersion,
		}
		emptyAPIResourceList.APIResources = findCRDGVRs(res.APIResources)
		crdResources = append(crdResources, &emptyAPIResourceList)
	}

	return crdResources, nil
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
			gvr := gv.WithResource(resource.Name)
			GVRs = append(GVRs, gvr)
		}
	}

	return GVRs, nil
}

// GetNamespacedGVRsForCluster collects all namespace-scoped GVRs for the provided cluster compatible client
func GetNamespacedGVRsForCluster(cluster *migapi.MigCluster, c client.Client) (dynamic.Interface, []schema.GroupVersionResource, error) {
	compat, err := cluster.GetClient(c)
	if err != nil {
		return nil, nil, err
	}
	dynamic, err := dynamic.NewForConfig(compat.RestConfig())
	if err != nil {
		return nil, nil, err
	}
	resourceList, err := collectNamespacedResources(compat)
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

func findCRDGVRs(resources []metav1.APIResource) []metav1.APIResource {
	CRDList := []metav1.APIResource{}
	for _, res := range resources {
		if res.Name == "customresourcedefinitions" {
			CRDList = append(CRDList, res)
		}
		break
	}

	return CRDList
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
	resourceList, err := r.SrcClient.Resource(gvr).List(context.Background(), options)
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

func (r *Compare) excludeCRDs(resources, crdResources []*metav1.APIResourceList, client dynamic.Interface) ([]*metav1.APIResourceList, error) {
	return r.includeExcludeCRDs(resources, crdResources, client, true)
}

func (r *Compare) includeCRDsOnly(resources, crdResources []*metav1.APIResourceList, client dynamic.Interface) ([]*metav1.APIResourceList, error) {
	return r.includeExcludeCRDs(resources, crdResources, client, false)
}

func (r *Compare) includeExcludeCRDs(resources, crdResources []*metav1.APIResourceList, client dynamic.Interface, exclude bool) ([]*metav1.APIResourceList, error) {
	crdGVRs, err := convertToGVRList(crdResources)
	if err != nil {
		return nil, err
	}

	options := metav1.ListOptions{}
	crdList := []unstructured.Unstructured{}
	for _, crdGVR := range crdGVRs {
		crds, err := client.Resource(crdGVR).List(context.Background(), options)
		if err != nil {
			return nil, err
		}
		crdList = append(crdList, crds.Items...)
	}

	crdGroups := mapset.NewSet()
	groupPath := []string{"spec", "group"}
	for _, crd := range crdList {
		group, _, err := unstructured.NestedString(crd.Object, groupPath...)
		if err != nil {
			return nil, err
		}
		crdGroups.Add(group)
	}

	updatedLists := []*metav1.APIResourceList{}
	for _, resourceList := range resources {
		isCRD := isCRDGroup(resourceList.GroupVersion, crdGroups)
		if !isCRD && exclude || isCRD && !exclude {
			updatedLists = append(updatedLists, resourceList)
		}
	}

	return updatedLists, nil
}

func (r *Compare) compareResources(src []*metav1.APIResourceList, dst []*metav1.APIResourceList) []*metav1.APIResourceList {
	missingResources := []*metav1.APIResourceList{}
	SortResources(src)
	for _, srcList := range src {
		missing := []metav1.APIResource{}
		matchingDstResourceList := findResourceList(srcList.GroupVersion, dst)
		for _, resource := range srcList.APIResources {
			if cohabitator, found := r.CohabitatingResources[resource.Name]; found {
				gv, err := schema.ParseGroupVersion(srcList.GroupVersion)
				if err == nil &&
					(gv.Group == cohabitator.groupResource1.Group || gv.Group == cohabitator.groupResource2.Group) {
					if cohabitator.Seen {
						continue
					}
					cohabitator.Seen = true
				}
			}
			if !resourceExist(resource, matchingDstResourceList) {
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

func resourceExist(resource metav1.APIResource, resources []metav1.APIResource) bool {
	for _, resourceItem := range resources {
		if resource.Name == resourceItem.Name {
			return true
		}
	}

	return false
}

func findResourceList(groupVersion string, list []*metav1.APIResourceList) []metav1.APIResource {
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

func isCRDGroup(group string, crdGroups mapset.Set) bool {
	for _, crdGroup := range toStringSlice(crdGroups) {
		if strings.HasPrefix(group, crdGroup) {
			return true
		}
	}

	return false
}

// SortResources sources resources by moving extensions to the end of the slice. The order of all
// the other resources is preserved.
func SortResources(resources []*metav1.APIResourceList) {
	sort.SliceStable(resources, func(i, j int) bool {
		left := resources[i]
		leftGV, _ := schema.ParseGroupVersion(left.GroupVersion)
		// not checking error because it should be impossible to fail to parse data coming from the
		// apiserver
		if leftGV.Group == "extensions" {
			// always sort extensions at the bottom by saying left is "greater"
			return false
		}

		right := resources[j]
		rightGV, _ := schema.ParseGroupVersion(right.GroupVersion)
		// not checking error because it should be impossible to fail to parse data coming from the
		// apiserver
		if rightGV.Group == "extensions" {
			// always sort extensions at the bottom by saying left is "less"
			return true
		}

		return i < j
	})
}
