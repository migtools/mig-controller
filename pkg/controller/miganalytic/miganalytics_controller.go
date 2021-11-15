/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package miganalytic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/errorutil"
	"github.com/konveyor/mig-controller/pkg/gvk"
	"github.com/konveyor/mig-controller/pkg/settings"
	"github.com/openshift/api/image/docker10"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/opentracing/opentracing-go"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	imagev1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MiBSuffix               = "Mi"
	MiB                     = 1048576
	RequeueInterval         = 10
	maxConcurrentReconciles = 2
)

var log = logging.WithName("analytics")
var Settings = &settings.Settings

// Add creates a new MigAnalytic Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager, unscopedMgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, unscopedMgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, unscopedMgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMigAnalytic{
		Client:        unscopedMgr.GetClient(),
		watchClient:   mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetEventRecorderFor("miganalytic_controller")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("miganalytic-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	})
	if err != nil {
		log.Trace(err)
		return err
	}

	// Watch for changes to MigAnalytic
	err = c.Watch(
		&source.Kind{Type: &migapi.MigAnalytic{}},
		&handler.EnqueueRequestForObject{},
		&AnalyticPredicate{})
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMigAnalytic{}

// ReconcileMigAnalytic reconciles a MigAnalytic object
type ReconcileMigAnalytic struct {
	client.Client
	record.EventRecorder
	watchClient client.Client

	scheme *runtime.Scheme
	tracer opentracing.Tracer
}

// MigAnalyticPersistentVolumeDetails defines extended properties of a volume discovered by MigAnalytic
type MigAnalyticPersistentVolumeDetails struct {
	Name                string
	RequestedCapacity   resource.Quantity
	PodUID              types.UID
	ProvisionedCapacity resource.Quantity
	StorageClass        *string
	Namespace           string
	VolumeName          string
}

func (r *ReconcileMigAnalytic) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var err error
	log = logging.WithName("analytics", "migAnalytic", request.Name)

	// Fetch the MigAnalytic instance
	analytic := &migapi.MigAnalytic{}

	err = r.watchClient.Get(context.TODO(), request.NamespacedName, analytic)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Get jaeger span for reconcile, add to ctx
	reconcileSpan := r.initTracer(analytic)
	if reconcileSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, reconcileSpan)
		defer reconcileSpan.Finish()
	}

	// Exit early if the MigAnalytic already has a ready condition
	// and Refresh boolean is unset
	if analytic.Status.IsReady() && !analytic.Spec.Refresh {
		return reconcile.Result{Requeue: false}, nil
	}

	// Report reconcile error.
	defer func() {
		log.Info("CR", "conditions", analytic.Status.Conditions)
		analytic.Status.Conditions.RecordEvents(analytic, r.EventRecorder)
		if err == nil || errors.IsConflict(errorutil.Unwrap(err)) {
			return
		}
		analytic.Status.SetReconcileFailed(err)
		err := r.watchClient.Update(context.TODO(), analytic)
		if err != nil {
			log.Trace(err)
			return
		}
	}()

	// Begin staging conditions.
	analytic.Status.BeginStagingConditions()

	// Add Labels and Annoations
	analytic.ObjectMeta.Labels = map[string]string{"migplan": analytic.Spec.MigPlanRef.Name}
	analytic.ObjectMeta.Annotations = map[string]string{"migplan": analytic.Spec.MigPlanRef.Name}

	// Validations.
	err = r.validate(analytic)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Update Validation Status
	err = r.watchClient.Update(context.TODO(), analytic)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Check for blocker conditions
	if analytic.Status.HasBlockerCondition() {
		return reconcile.Result{RequeueAfter: time.Second * RequeueInterval}, nil
	}

	// Analyze
	err = r.analyze(analytic)
	if err != nil {
		log.Info("Error calculating migration analytics", "error", err.Error())
		return reconcile.Result{RequeueAfter: time.Second * RequeueInterval}, nil
	}

	// Ready
	analytic.Status.SetReady(
		!analytic.Status.HasBlockerCondition(),
		"The analytic is ready.")

	analytic.Spec.Refresh = false

	// End staging conditions.
	analytic.Status.EndStagingConditions()

	// Apply changes.
	analytic.MarkReconciled()
	err = r.watchClient.Update(context.TODO(), analytic)
	if err != nil {
		log.Trace(err)
		return reconcile.Result{Requeue: true}, nil
	}

	// Done
	return reconcile.Result{Requeue: false}, nil
}

func (r *ReconcileMigAnalytic) analyze(analytic *migapi.MigAnalytic) error {
	plan := &migapi.MigPlan{}

	err := r.getAnalyticPlan(analytic, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	if plan == nil {
		return fmt.Errorf("migplan %v/%v referenced from miganalytic %v/%v not found",
			plan.Namespace, plan.Name, analytic.Namespace, analytic.Name)
	}

	if analytic.Status.Analytics.PercentComplete == 100 && !analytic.Spec.Refresh {
		return nil
	}

	srcCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcCluster == nil {
		return fmt.Errorf("source migcluster %v/%v referenced by migplan %v/%v could not be found",
			plan.Spec.SrcMigClusterRef.Namespace, plan.Spec.SrcMigClusterRef.Name,
			plan.Namespace, plan.Name)
	}

	client, err := srcCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if client == nil {
		return fmt.Errorf("source migcluster %v/%v client could not be built",
			srcCluster.Namespace, srcCluster.Name)
	}

	resources, err := collectResources(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	dynamic, err := dynamic.NewForConfig(client.RestConfig())
	if err != nil {
		return liberr.Wrap(err)
	}

	nodeToPVMap := make(map[string][]MigAnalyticPersistentVolumeDetails)

	analytic.Status.Analytics.Plan = plan.Name
	analytic.Status.Analytics.Namespaces = make([]migapi.MigAnalyticNamespace, 0)
	for i, namespace := range plan.GetSourceNamespaces() {
		for _, ns := range analytic.Status.Analytics.Namespaces {
			if ns.Namespace == namespace {
				continue
			}
		}
		ns := migapi.MigAnalyticNamespace{
			Namespace: namespace,
		}

		excludedResources := plan.Status.ExcludedResources
		incompatible := plan.Status.Incompatible

		if analytic.Spec.AnalyzeK8SResources {
			err := r.analyzeK8SResources(dynamic, resources, &ns, excludedResources, incompatible)
			if err != nil {
				return liberr.Wrap(err)
			}
		}
		if analytic.Spec.AnalyzeImageCount && !isExcluded("imagestreams", excludedResources) && !Settings.DisImgCopy {
			err := r.analyzeImages(client, &ns, analytic.Spec.ListImages, analytic.Spec.ListImagesLimit)
			if err != nil {
				return liberr.Wrap(err)
			}
		}

		if analytic.Spec.AnalyzePVCapacity && !(isExcluded("persistentvolumes", excludedResources) &&
			isExcluded("persistentvolumeclaims", excludedResources)) {

			err := r.analyzePVCapacity(client, &ns)
			if err != nil {
				return liberr.Wrap(err)
			}
		}

		if analytic.Spec.AnalyzeExtendedPVCapacity {
			analytic.Status.SetCondition(migapi.Condition{
				Type:     migapi.ExtendedPVAnalysisStarted,
				Category: migapi.Advisory,
				Status:   True,
				Message:  "Started collecting detailed PV information",
				Durable:  true,
			})
			namespacedNodeToPVMap, err := r.getNodeToPVCMapForNS(&ns, client)
			for node, pvcs := range namespacedNodeToPVMap {
				if _, exists := nodeToPVMap[node]; !exists {
					nodeToPVMap[node] = make([]MigAnalyticPersistentVolumeDetails, 0)
				}
				nodeToPVMap[node] = append(nodeToPVMap[node], pvcs...)
			}
			if err != nil {
				return liberr.Wrap(err)
			}
		}

		analytic.Status.Analytics.Namespaces = append(analytic.Status.Analytics.Namespaces, ns)
		analytic.Status.Analytics.K8SResourceTotal += ns.K8SResourceTotal
		analytic.Status.Analytics.ExcludedK8SResourceTotal += ns.ExcludedK8SResourceTotal
		analytic.Status.Analytics.IncompatibleK8SResourceTotal += ns.IncompatibleK8SResourceTotal
		analytic.Status.Analytics.ImageCount += ns.ImageCount
		analytic.Status.Analytics.ImageSizeTotal.Add(ns.ImageSizeTotal)
		analytic.Status.Analytics.PVCapacity.Add(ns.PVCapacity)
		analytic.Status.Analytics.PVCount += ns.PVCount
		analytic.Status.Analytics.PercentComplete = (i + 1) * 100 / len(plan.Spec.Namespaces)

		err = r.watchClient.Update(context.TODO(), analytic)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	if analytic.Spec.AnalyzeExtendedPVCapacity {
		err := r.analyzeExtendedPVCapacity(client, analytic, nodeToPVMap)
		if err != nil {
			return liberr.Wrap(err)
		}
		analytic.Status.DeleteCondition(migapi.ExtendedPVAnalysisStarted)
		err = r.watchClient.Update(context.TODO(), analytic)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// analyzeExtendedPVCapacity given a source client, owner cr and map of nodeNames -> pvs, run volume adjustment
func (r *ReconcileMigAnalytic) analyzeExtendedPVCapacity(sourceClient compat.Client, analytic *migapi.MigAnalytic, nodeToPVMap map[string][]MigAnalyticPersistentVolumeDetails) error {
	volumeAdjuster := PersistentVolumeAdjuster{
		Owner:  analytic,
		Client: r.Client,
		DFExecutor: &ResticDFCommandExecutor{
			Namespace: migapi.VeleroNamespace,
			Client:    sourceClient,
		},
	}
	err := volumeAdjuster.Run(nodeToPVMap)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// getNodeToPVCMapForNS given a ns and client, returns map of nodeName -> PV Info present on that node
func (r *ReconcileMigAnalytic) getNodeToPVCMapForNS(ns *migapi.MigAnalyticNamespace, client compat.Client) (map[string][]MigAnalyticPersistentVolumeDetails, error) {
	podList := corev1.PodList{}
	nodeToPVDetails := map[string][]MigAnalyticPersistentVolumeDetails{}
	err := client.List(
		context.TODO(),
		&podList,
		k8sclient.InNamespace(ns.Namespace))
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					pvcObject := corev1.PersistentVolumeClaim{}
					err := client.Get(
						context.TODO(),
						types.NamespacedName{
							Name:      vol.PersistentVolumeClaim.ClaimName,
							Namespace: pod.Namespace,
						}, &pvcObject)

					if err != nil {
						return nodeToPVDetails, liberr.Wrap(err)
					}

					if _, exists := nodeToPVDetails[pod.Spec.NodeName]; !exists {
						nodeToPVDetails[pod.Spec.NodeName] = make([]MigAnalyticPersistentVolumeDetails, 0)
					}

					nodeToPVDetails[pod.Spec.NodeName] = append(nodeToPVDetails[pod.Spec.NodeName],
						MigAnalyticPersistentVolumeDetails{
							Name:                vol.PersistentVolumeClaim.ClaimName,
							Namespace:           pvcObject.Namespace,
							RequestedCapacity:   pvcObject.Spec.Resources.Requests[corev1.ResourceStorage],
							PodUID:              pod.UID,
							ProvisionedCapacity: pvcObject.Status.Capacity[corev1.ResourceStorage],
							StorageClass:        pvcObject.Spec.StorageClassName,
							VolumeName:          pvcObject.Spec.VolumeName,
						})
				}
			}
		}
	}
	return nodeToPVDetails, nil
}

func (r *ReconcileMigAnalytic) analyzeK8SResources(dynamic dynamic.Interface,
	resources []*metav1.APIResourceList,
	ns *migapi.MigAnalyticNamespace,
	excludedResources []string, incompatible v1alpha1.Incompatible) error {
	gvk.SortResources(resources)
	cohabitatingResources := gvk.NewCohabitatingResources()

	for _, res := range resources {
		for _, r := range res.APIResources {
			// skip resource if we have already handle it via an alternate group
			// i.e. apps/v1beta1 deployment vs extension/v1beta1 deployment
			if cohabitator, found := cohabitatingResources[r.Name]; found {
				if cohabitator.Seen {
					continue
				}
				cohabitator.Seen = true
			}

			excluded := false
			compatible := true
			gvr := schema.GroupVersionResource{}
			gv := strings.Split(res.GroupVersion, "/")
			if len(gv) > 1 {
				gvr = schema.GroupVersionResource{
					Group:    gv[0],
					Version:  gv[1],
					Resource: r.Name,
				}
			} else {
				gvr = schema.GroupVersionResource{
					Group:    "",
					Version:  gv[0],
					Resource: r.Name,
				}
			}

			list, err := dynamic.Resource(gvr).Namespace(ns.Namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return liberr.Wrap(err)
			}

			// If no resources of this type we won't add it to any lists
			if len(list.Items) > 0 {

				// Check if the resource type is on the plan exludedResources list
				excluded = isExcluded(gvr.Resource, excludedResources)

				// The lists are mutually exclusive. Only if not excluded check the incompatible GVK list
				if !excluded {
					compatible = isCompatible(gvr, ns.Namespace, incompatible.Namespaces)
				}

				NamespaceResource := migapi.MigAnalyticNSResource{
					Group:   gvr.Group,
					Version: gvr.Version,
					Kind:    gvr.Resource,
					Count:   len(list.Items),
				}
				if !compatible {
					ns.IncompatibleK8SResources = append(ns.IncompatibleK8SResources, NamespaceResource)
					ns.IncompatibleK8SResourceTotal += len(list.Items)
				} else if excluded {
					ns.ExcludedK8SResources = append(ns.ExcludedK8SResources, NamespaceResource)
					ns.ExcludedK8SResourceTotal += len(list.Items)
				} else {
					ns.K8SResources = append(ns.K8SResources, NamespaceResource)
					ns.K8SResourceTotal += len(list.Items)
				}
			}
		}
	}

	return nil
}

func (r *ReconcileMigAnalytic) analyzeImages(client compat.Client,
	namespace *migapi.MigAnalyticNamespace,
	listImages bool,
	listImagesLimit int) error {
	imageStreamList := imagev1.ImageStreamList{}

	major, minor := client.MajorVersion(), client.MinorVersion()
	internalRegistry, err := GetRegistryInfo(major, minor, client)
	if err != nil {
		return liberr.Wrap(err)
	}

	err = client.List(context.TODO(), &imageStreamList, k8sclient.InNamespace(namespace.Namespace))
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, im := range imageStreamList.Items {
		for _, tag := range im.Status.Tags {
			for i := len(tag.Items) - 1; i >= 0; i-- {
				dockerImageReference := tag.Items[i].DockerImageReference
				if len(internalRegistry) > 0 && strings.HasPrefix(dockerImageReference, internalRegistry) {
					image, size, err := getImageDetails(tag.Items[i].Image, client)
					if err != nil {
						return liberr.Wrap(err)
					}

					namespace.ImageCount += 1
					namespace.ImageSizeTotal.Add(size)
					if listImages && namespace.ImageCount <= listImagesLimit {
						namespace.Images = append(namespace.Images,
							migapi.MigAnalyticNSImage{
								Name:      image.Name,
								Reference: image.DockerImageReference,
								Size:      size,
							})
					}
				}
			}
		}
	}
	return nil
}

func (r *ReconcileMigAnalytic) analyzePVCapacity(client compat.Client,
	namespace *migapi.MigAnalyticNamespace) error {
	pvcList := corev1.PersistentVolumeClaimList{}
	err := client.List(context.TODO(), &pvcList, k8sclient.InNamespace(namespace.Namespace))
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, pvc := range pvcList.Items {
		namespace.PVCapacity.Add(*pvStorage(pvc.Spec.Resources.Requests))
		namespace.PVCount += 1
	}
	return nil
}

func (r *ReconcileMigAnalytic) getAnalyticPlan(analytic *migapi.MigAnalytic,
	plan *migapi.MigPlan) error {
	err := r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      analytic.Spec.MigPlanRef.Name,
			Namespace: analytic.Spec.MigPlanRef.Namespace,
		},
		plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

func pvStorage(self corev1.ResourceList) *resource.Quantity {
	if val, ok := (self)["storage"]; ok {
		return &val
	}
	return &resource.Quantity{Format: resource.BinarySI}
}

func collectResources(client compat.Client) ([]*metav1.APIResourceList, error) {
	resources, err := client.ServerResources()
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		res.APIResources = namespaced(res.APIResources)
		res.APIResources = excludeSubresources(res.APIResources)
		res.APIResources = listAllowed(res.APIResources)
	}

	return resources, nil
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

func findSpecTag(tags []imagev1.TagReference, name string) *imagev1.TagReference {
	for _, tag := range tags {
		if tag.Name == name {
			return &tag
		}
	}
	return nil
}

func GetRegistryInfo(major, minor int, client compat.Client) (string, error) {
	imageStreams := imagev1.ImageStreamList{}

	err := client.List(context.TODO(), &imageStreams, k8sclient.InNamespace("openshift"))
	if err == nil && len(imageStreams.Items) > 0 {
		if value := imageStreams.Items[0].Status.DockerImageRepository; len(value) > 0 {
			ref, err := reference.Parse(value)
			if err == nil {
				return ref.Registry, nil
			}
		}
	}

	if major != 1 {
		return "", fmt.Errorf("server version %v.%v not supported. Must be 1.x", major, minor)
	}

	if err != nil {
		return "", err
	}
	if minor < 7 {
		return "", fmt.Errorf("Kubernetes version 1.%v not supported. Must be 1.7 or greater", minor)
	} else if minor <= 11 {

		registrySvc := corev1.Service{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "docker-registry",
				Namespace: "default",
			},
			&registrySvc)
		if err != nil {
			return "", nil
		}
		registryPort := strconv.Itoa(int(registrySvc.Spec.Ports[0].Port))
		internalRegistry := registrySvc.Spec.ClusterIP + ":" + registryPort
		return internalRegistry, nil
	} else {
		config := corev1.ConfigMap{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      "config",
				Namespace: "openshift-apisrver",
			},
			&config)
		if err != nil {
			return "", err
		}
		serverConfig := apiServerConfig{}
		err = json.Unmarshal([]byte(config.Data["config.yaml"]), &serverConfig)
		if err != nil {
			return "", err
		}
		internalRegistry := serverConfig.ImagePolicyConfig.InternalRegistryHostname
		if len(internalRegistry) == 0 {
			return "", nil
		}
		return internalRegistry, nil
	}
}

func isExcluded(name string, excludedResources []string) bool {
	for _, exclude := range excludedResources {
		if exclude == name {
			return true
		}
	}
	return false
}

func isCompatible(gvr schema.GroupVersionResource,
	namespace string,
	namespaces []v1alpha1.IncompatibleNamespace) bool {
	for _, ns := range namespaces {
		if namespace == ns.Name {
			for _, gvk := range ns.GVKs {
				if gvk.Group == gvr.Group && gvk.Version == gvr.Version && gvk.Kind == gvr.Resource {
					return false
				}
			}
		}
	}
	return true
}

func getImageDetails(tagItemImageName string,
	client k8sclient.Client) (*imagev1.Image, resource.Quantity, error) {
	var size resource.Quantity
	image := &imagev1.Image{}
	err := client.Get(
		context.TODO(),
		types.NamespacedName{
			Name: tagItemImageName,
		},
		image)
	if err != nil {
		return image, size, liberr.Wrap(err)
	}

	dockerImageMetadata := docker10.DockerImage{}
	err = json.Unmarshal(image.DockerImageMetadata.Raw, &dockerImageMetadata)
	if err != nil {
		return image, size, liberr.Wrap(err)
	}
	size = resource.MustParse((strconv.Itoa(int(dockerImageMetadata.Size)/MiB) + MiBSuffix))

	return image, size, nil
}

type apiServerConfig struct {
	ImagePolicyConfig imagePolicyConfig `json:"imagePolicyConfig"`
	RoutingConfig     routingConfig     `json:"routingConfig"`
}

type routingConfig struct {
	Subdomain string `json:"subdomain"`
}

type imagePolicyConfig struct {
	InternalRegistryHostname string `json:"internalRegistryHostname"`
}
