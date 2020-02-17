package gvkcompare

import (
	"sort"
	"strings"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/pkg/errors"
	"k8s.io/client-go/discovery"
)

// GVKCompare reconciles a GVKCompare object
type GVKCompare struct {
	Plan         *migapi.MigPlan
	SrcDiscovery *discovery.DiscoveryClient
	DstDiscovery *discovery.DiscoveryClient
	SrcClient    dynamic.Interface
}

// Compare GVKs on both clusters, find unsupported GVRs
// and check each plan source namespace for existence of unsupported GVRs
func (r *GVKCompare) Compare() error {
	gvDiff, err := r.compareGroupVersions()
	if err != nil {
		return errors.Wrap(err, "Unable to compare GroupVersions between clusters")
	}

	unsupportedGVRs, err := r.unsupportedServerResources(gvDiff)
	if err != nil {
		return errors.Wrap(err, "Unable to get unsupported resources for scrCluster")
	}

	err = r.collectNamespaceReport(unsupportedGVRs)
	if err != nil {
		return errors.Wrap(err, "Unable to evaluate GVR gaps for migrated resources")
	}

	return nil
}

func (r *GVKCompare) PrepareSourceDiscovery(c client.Client) error {
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

func (r *GVKCompare) PrepareDestinationDiscovery(c client.Client) error {
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

func (r *GVKCompare) getDiscovery(c client.Client, cluster *migapi.MigCluster) (*discovery.DiscoveryClient, error) {
	config, err := cluster.BuildRestConfig(c)
	if err != nil {
		return nil, errors.Wrap(err, "Can't get REST config from a cluster")
	}

	return discovery.NewDiscoveryClientForConfig(config)
}

func (r *GVKCompare) getClient(c client.Client, cluster *migapi.MigCluster) (dynamic.Interface, error) {
	config, err := cluster.BuildRestConfig(c)
	if err != nil {
		return nil, errors.Wrap(err, "Can't get REST config from a cluster")
	}

	return dynamic.NewForConfig(config)
}

func (r *GVKCompare) PrepareSourceClient(c client.Client) error {
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

func (r *GVKCompare) compareGroupVersions() ([]metav1.APIGroup, error) {
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

func (r *GVKCompare) collectNamespaceReport(unsupportedResources []schema.GroupVersionResource) error {
	for _, namespace := range r.Plan.GetSourceNamespaces() {
		unsupportedGVRs := []string{}
		for _, gvr := range unsupportedResources {
			options := metav1.ListOptions{}
			resourceList, err := r.SrcClient.Resource(gvr).Namespace(namespace).List(options)
			if err != nil {
				return errors.Wrapf(err, "error listing '%s' in namespace %s", gvr, namespace)
			}

			if len(resourceList.Items) > 0 {
				unsupportedGVRs = append(unsupportedGVRs, gvr.String())
			}
		}

		if len(unsupportedGVRs) > 0 {
			r.Plan.Status.AppendUnsupported(namespace, unsupportedGVRs)
		}
	}

	return nil
}

func (r *GVKCompare) unsupportedServerResources(gvDiff []metav1.APIGroup) ([]schema.GroupVersionResource, error) {
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
