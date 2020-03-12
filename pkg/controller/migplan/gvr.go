package migplan

import (
	"strings"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (r ReconcileMigPlan) compareGVK(plan *migapi.MigPlan) error {
	// No spec chage this time
	if plan.HasReconciled() || !clustersReady(plan) {
		return nil
	}

	gvkCompare, err := r.newGVKCompare(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	incompatibleMapping, err := gvkCompare.Compare()
	if err != nil {
		log.Trace(err)
		return err
	}

	reportGVK(plan, incompatibleMapping)

	return nil
}

func (r ReconcileMigPlan) newGVKCompare(plan *migapi.MigPlan) (*Compare, error) {
	gvkCompare := &Compare{
		Plan: plan,
	}

	err := gvkCompare.NewSourceDiscovery(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	err = gvkCompare.NewDestinationDiscovery(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	err = gvkCompare.NewSourceClient(r)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	return gvkCompare, nil
}

func reportGVK(plan *migapi.MigPlan, incompatibleMapping map[string][]schema.GroupVersionResource) {
	incompatibleNamespaces := []migapi.IncompatibleNamespace{}

	for namespace, incompatibleGVRs := range incompatibleMapping {
		incompatibleResources := []migapi.IncompatibleGVR{}
		for _, res := range incompatibleGVRs {
			incompatibleResources = append(incompatibleResources, migapi.FromGVR(res))
		}

		incompatibleNamespace := migapi.IncompatibleNamespace{
			Name: namespace,
			GVRs: incompatibleResources,
		}
		incompatibleNamespaces = append(incompatibleNamespaces, incompatibleNamespace)
	}

	if len(incompatibleNamespaces) > 0 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     GVRsIncompatible,
			Status:   True,
			Category: Warn,
			Message:  NsGVRsIncompatible,
		})
	}

	plan.Status.Incompatible = migapi.Incompatible{
		Namespaces: incompatibleNamespaces,
	}
}

// Check if any blocker condition appeared on the migPlan after cluster validation phase
func clustersReady(plan *migapi.MigPlan) bool {
	clustersNotReadyConditions := []string{
		InvalidDestinationClusterRef,
		InvalidDestinationCluster,
		InvalidDestinationClusterRef,
		InvalidSourceClusterRef,
		DestinationClusterNotReady,
		SourceClusterNotReady,
	}
	return !plan.Status.HasAnyCondition(clustersNotReadyConditions...)
}

// Compare GVKs on both clusters, find incompatible GVRs
// and check each plan source namespace for existence of incompatible GVRs
func (r *Compare) Compare() (map[string][]schema.GroupVersionResource, error) {
	srcResourceList, err := collectResources(r.SrcDiscovery)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	dstResourceList, err := collectResources(r.DstDiscovery)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	srcResourceList, err = r.excludeCRDs(srcResourceList)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	resourcesDiff := compareResources(srcResourceList, dstResourceList)
	incompatibleGVRs, err := incompatibleResources(resourcesDiff)
	if err != nil {
		log.Trace(err)
		return nil, err
	}

	return r.collectIncompatibleMapping(incompatibleGVRs)
}

// NewSourceDiscovery initializes source discovery client for a source cluster
func (r *Compare) NewSourceDiscovery(c client.Client) error {
	srcCluster, err := r.Plan.GetSourceCluster(c)
	if err != nil {
		return err
	}

	discovery, err := r.getDiscovery(c, srcCluster)
	if err != nil {
		return err
	}

	r.SrcDiscovery = discovery

	return nil
}

// NewDestinationDiscovery initializes destination discovery client forom a destination cluster
func (r *Compare) NewDestinationDiscovery(c client.Client) error {
	dstCluster, err := r.Plan.GetDestinationCluster(c)
	if err != nil {
		return err
	}

	discovery, err := r.getDiscovery(c, dstCluster)
	if err != nil {
		return err
	}

	r.DstDiscovery = discovery

	return nil
}

// NewSourceClient initializes source discovery client for a source cluster
func (r *Compare) NewSourceClient(c client.Client) error {
	srcCluster, err := r.Plan.GetSourceCluster(c)
	if err != nil {
		return err
	}

	client, err := r.getClient(c, srcCluster)
	if err != nil {
		return err
	}

	r.SrcClient = client

	return nil
}

func (r *Compare) getDiscovery(c client.Client, cluster *migapi.MigCluster) (*discovery.DiscoveryClient, error) {
	config, err := cluster.BuildRestConfig(c)
	if err != nil {
		return nil, err
	}

	return discovery.NewDiscoveryClientForConfig(config)
}

func (r *Compare) getClient(c client.Client, cluster *migapi.MigCluster) (dynamic.Interface, error) {
	config, err := cluster.BuildRestConfig(c)
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func (r *Compare) collectIncompatibleMapping(incompatibleResources []schema.GroupVersionResource) (map[string][]schema.GroupVersionResource, error) {
	incompatibleNamespaces := map[string][]schema.GroupVersionResource{}
	for _, gvr := range incompatibleResources {
		namespaceOccurence, err := r.occurIn(gvr)
		if err != nil {
			return nil, err
		}

		for _, namespace := range namespaceOccurence {
			if inNamespaces(namespace, r.Plan.GetSourceNamespaces()) {
				_, exist := incompatibleNamespaces[namespace]
				if exist {
					incompatibleNamespaces[namespace] = append(incompatibleNamespaces[namespace], gvr)
				} else {
					incompatibleNamespaces[namespace] = []schema.GroupVersionResource{gvr}
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

func incompatibleResources(resourceDiff []*metav1.APIResourceList) ([]schema.GroupVersionResource, error) {
	incompatibleGVRs := []schema.GroupVersionResource{}
	for _, resourceList := range resourceDiff {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, resource := range resourceList.APIResources {
			gvr := gv.WithResource(resource.Name)
			incompatibleGVRs = append(incompatibleGVRs, gvr)
		}
	}

	return incompatibleGVRs, nil
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
