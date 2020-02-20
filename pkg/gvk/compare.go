package gvk

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
)

// Compare is a store for discovery and dynamic clients to do GVK compare
type Compare struct {
	Plan         *migapi.MigPlan
	SrcDiscovery *discovery.DiscoveryClient
	DstDiscovery *discovery.DiscoveryClient
	SrcClient    dynamic.Interface
}

// Compare GVKs on both clusters, find unsupported GVRs
// and check each plan source namespace for existence of unsupported GVRs
func (r *Compare) Compare() (map[string][]schema.GroupVersionResource, error) {
	gvDiff, err := r.compareGroupVersions()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to compare GroupVersions between clusters")
	}

	unsupportedGVRs, err := r.unsupportedServerResources(gvDiff)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get unsupported resources for scrCluster")
	}

	err = r.excludeCRDs(&unsupportedGVRs)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to exclude CRs from the unsupported resources")
	}

	return r.collectUnsupportedMapping(unsupportedGVRs)
}

// NewSourceDiscovery initializes source discovery client for a source cluster
func (r *Compare) NewSourceDiscovery(c client.Client) error {
	srcCluster, err := r.Plan.GetSourceCluster(c)
	if err != nil {
		return errors.Wrap(err, "Error reading srcMigCluster")
	}

	discovery, err := r.getDiscovery(c, srcCluster)
	if err != nil {
		return errors.Wrap(err, "Can't compile discovery client for srcCluster")
	}

	r.SrcDiscovery = discovery

	return nil
}

// NewDestinationDiscovery initializes destination discovery client forom a destination cluster
func (r *Compare) NewDestinationDiscovery(c client.Client) error {
	dstCluster, err := r.Plan.GetDestinationCluster(c)
	if err != nil {
		return errors.Wrap(err, "Error reading dstMigCluster")
	}

	discovery, err := r.getDiscovery(c, dstCluster)
	if err != nil {
		return errors.Wrap(err, "Can't compile discovery client for dstCluster")
	}

	r.DstDiscovery = discovery

	return nil
}

// NewSourceClient initializes source discovery client for a source cluster
func (r *Compare) NewSourceClient(c client.Client) error {
	srcCluster, err := r.Plan.GetSourceCluster(c)
	if err != nil {
		return errors.Wrap(err, "Error reading srcMigCluster")
	}

	client, err := r.getClient(c, srcCluster)
	if err != nil {
		return errors.Wrap(err, "Can't compile dynamic client for srcCluster")
	}

	r.SrcClient = client

	return nil
}

func (r *Compare) getDiscovery(c client.Client, cluster *migapi.MigCluster) (*discovery.DiscoveryClient, error) {
	config, err := cluster.BuildRestConfig(c)
	if err != nil {
		return nil, errors.Wrap(err, "Can't get REST config from a cluster")
	}

	return discovery.NewDiscoveryClientForConfig(config)
}

func (r *Compare) getClient(c client.Client, cluster *migapi.MigCluster) (dynamic.Interface, error) {
	config, err := cluster.BuildRestConfig(c)
	if err != nil {
		return nil, errors.Wrap(err, "Can't get REST config from a cluster")
	}

	return dynamic.NewForConfig(config)
}

func (r *Compare) compareGroupVersions() ([]metav1.APIGroup, error) {
	srcGroupList, err := r.SrcDiscovery.ServerGroups()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to fetch server groups for a srcCluster")
	}

	dstGroupList, err := r.DstDiscovery.ServerGroups()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to fetch server groups for a dstCluster")
	}

	missingGroups := missingGroups(srcGroupList.Groups, dstGroupList.Groups)
	matchGroups(srcGroupList, dstGroupList, missingGroups)
	missingVersions := missingVersions(srcGroupList.Groups, dstGroupList.Groups)

	return append(missingGroups, missingVersions...), nil
}

func (r *Compare) collectUnsupportedMapping(unsupportedResources []schema.GroupVersionResource) (map[string][]schema.GroupVersionResource, error) {
	unsupportedNamespaces := map[string][]schema.GroupVersionResource{}
	for _, gvr := range unsupportedResources {
		namespaceOccurence, err := r.occureIn(gvr)
		if err != nil {
			return nil, errors.Wrapf(err, "Unable to collect namespace occurences for GVR: %s", gvr)
		}

		for _, namespace := range namespaceOccurence {
			if inNamespaces(namespace, r.Plan.GetSourceNamespaces()) {
				_, exist := unsupportedNamespaces[namespace]
				if exist {
					unsupportedNamespaces[namespace] = append(unsupportedNamespaces[namespace], gvr)
				} else {
					unsupportedNamespaces[namespace] = []schema.GroupVersionResource{gvr}
				}
			}
		}
	}

	return unsupportedNamespaces, nil
}

func (r *Compare) occureIn(gvr schema.GroupVersionResource) ([]string, error) {
	namespacesOccured := []string{}
	options := metav1.ListOptions{}
	resourceList, err := r.SrcClient.Resource(gvr).List(options)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while listing: %s", gvr)
	}

	for _, res := range resourceList.Items {
		if !inNamespaces(res.GetNamespace(), namespacesOccured) {
			namespacesOccured = append(namespacesOccured, res.GetNamespace())
		}
	}

	return namespacesOccured, nil
}

func (r *Compare) unsupportedServerResources(gvDiff []metav1.APIGroup) ([]schema.GroupVersionResource, error) {
	unsupportedGVRs := []schema.GroupVersionResource{}
	for _, gr := range gvDiff {
		for _, version := range gr.Versions {
			r, err := r.SrcDiscovery.ServerResourcesForGroupVersion(version.GroupVersion)
			if err != nil {
				return nil, errors.Wrap(err, "Unable to get a list of resources for a GroupVersion on srcCluster")
			}

			r.APIResources = namespaced(r.APIResources)
			r.APIResources = excludeSubresources(r.APIResources)

			gv, err := schema.ParseGroupVersion(version.GroupVersion)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing GroupVersion %s", gr)
			}

			for _, resource := range r.APIResources {
				gvr := gv.WithResource(resource.Name)
				unsupportedGVRs = append(unsupportedGVRs, gvr)
			}
		}
	}

	return unsupportedGVRs, nil
}

func (r *Compare) excludeCRDs(unsupportedGVRs *[]schema.GroupVersionResource) error {
	crd := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1beta1",
		Resource: "customresourcedefinitions",
	}

	options := metav1.ListOptions{}
	crdList, err := r.SrcClient.Resource(crd).List(options)
	if err != nil {
		return errors.Wrap(err, "Error while listing CRDs")
	}

	crdGroups := []string{}
	groupPath := []string{"spec", "group"}
	for _, crd := range crdList.Items {
		group, found, err := unstructured.NestedString(crd.Object, groupPath...)
		if !found {
			return errors.Wrap(err, "Error while extracting CRD group: not exist")
		}
		if err != nil {
			return errors.Wrap(err, "Error while extracting CRD group")
		}
		crdGroups = append(crdGroups, group)
	}

	updatedGVRs := []schema.GroupVersionResource{}
	for _, gvr := range *unsupportedGVRs {
		if !isCRDGroup(gvr.Group, crdGroups) {
			updatedGVRs = append(updatedGVRs, gvr)
		}
	}

	*unsupportedGVRs = updatedGVRs

	return nil
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

func missingGroups(srcList []metav1.APIGroup, dstList []metav1.APIGroup) []metav1.APIGroup {
	missing := []metav1.APIGroup{}
	for _, group := range srcList {
		if !groupExist(group, dstList) {
			missing = append(missing, group)
		}
	}

	return missing
}

func groupExist(group metav1.APIGroup, groupList []metav1.APIGroup) bool {
	for _, selectedGroup := range groupList {
		if selectedGroup.Name == group.Name {
			return true
		}
	}

	return false
}

func versionExist(preferredVersion metav1.GroupVersionForDiscovery, versions []metav1.GroupVersionForDiscovery) bool {
	for _, version := range versions {
		if preferredVersion.GroupVersion == version.GroupVersion {
			return true
		}
	}

	return false
}

func missingVersions(src []metav1.APIGroup, dst []metav1.APIGroup) []metav1.APIGroup {
	missingVersion := []metav1.APIGroup{}
	for i, srcGroup := range src {
		missingVersions := []metav1.GroupVersionForDiscovery{}
		for _, version := range srcGroup.Versions {
			if !versionExist(version, dst[i].Versions) {
				missingVersions = append(missingVersions, version)
			}
		}
		if len(missingVersions) > 0 {
			srcGroup.Versions = missingVersions
			missingVersion = append(missingVersion, srcGroup)
		}
	}

	return missingVersion
}

func matchGroups(src *metav1.APIGroupList, dst *metav1.APIGroupList, missing []metav1.APIGroup) {
	reducedSrc := []metav1.APIGroup{}
	for _, group := range src.Groups {
		if !groupExist(group, missing) {
			reducedSrc = append(reducedSrc, group)
		}
	}

	reducedDst := []metav1.APIGroup{}
	for _, group := range dst.Groups {
		if groupExist(group, reducedSrc) {
			reducedDst = append(reducedDst, group)
		}
	}

	sort.Slice(reducedSrc, func(i int, j int) bool {
		return reducedSrc[i].Name < reducedSrc[j].Name
	})
	sort.Slice(reducedDst, func(i int, j int) bool {
		return reducedDst[i].Name < reducedDst[j].Name
	})

	src.Groups = reducedSrc
	dst.Groups = reducedDst
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
		if crdGroup == group {
			return true
		}
	}

	return false
}
