package compat

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/mig-controller/pkg/settings"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dapi "k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var Settings = &settings.Settings

// A smart client.
// Provides seamless API version compatibility.
type Client interface {
	k8sclient.Client
	dapi.DiscoveryInterface
	RestConfig() *rest.Config
	MajorVersion() int
	MinorVersion() int
}

type client struct {
	*rest.Config
	k8sclient.Client
	dapi.DiscoveryInterface
	// major k8s version.
	Major int
	// minor k8s version.
	Minor int
}

// Create a new client.
func NewClient(restCfg *rest.Config, rClient *k8sclient.Client) (Client, error) {
	dClient, err := dapi.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	version, err := dClient.ServerVersion()
	if err != nil {
		return nil, err
	}

	// Attempt parsing version.Major/Minor first, fall back to parsing gitVersion
	major, err1 := strconv.Atoi(version.Major)
	minor, err2 := strconv.Atoi(strings.Trim(version.Minor, "+"))

	if err1 != nil || err2 != nil {
		// gitVersion format ("v1.11.0+d4cacc0")
		r, _ := regexp.Compile(`v[0-9]+\.[0-9]+\.`)
		valid := r.MatchString(version.GitVersion)
		if !valid {
			return nil, errors.New("gitVersion does not match expected format")
		}
		majorMinorArr := strings.Split(strings.Split(version.GitVersion, "v")[1], ".")

		major, err = strconv.Atoi(majorMinorArr[0])
		if err != nil {
			return nil, err
		}
		minor, err = strconv.Atoi(majorMinorArr[1])
		if err != nil {
			return nil, err
		}
	}

	nClient := &client{
		Config:             restCfg,
		Client:             *rClient,
		DiscoveryInterface: dClient,
		Major:              major,
		Minor:              minor,
	}

	return nClient, nil
}

func (c client) RestConfig() *rest.Config {
	return c.Config
}

func (c client) MajorVersion() int {
	return c.Major
}

func (c client) MinorVersion() int {
	return c.Minor
}

// supportedVersion will determine correct version of the object provided, based on cluster version
func (c client) supportedVersion(obj k8sclient.Object) k8sclient.Object {
	if c.Minor < 16 {
		switch obj.(type) {

		// Deployment
		case *appsv1.Deployment:
			return &appsv1beta1.Deployment{}

		// DaemonSet
		case *appsv1.DaemonSet:
			return &extv1beta1.DaemonSet{}

		// ReplicaSet
		case *appsv1.ReplicaSet:
			return &extv1beta1.ReplicaSet{}

		// StatefulSet
		case *appsv1.StatefulSet:
			return &appsv1beta1.StatefulSet{}
		}
	}

	if c.Minor > 24 {
		switch obj.(type) {
		case *batchv1beta1.CronJob:
			return &batchv1.CronJob{}
		}
	}

	return obj
}

// supportedVersion will determine correct version of the object provided, based on cluster version
func (c client) supportedVersionList(obj k8sclient.ObjectList) k8sclient.ObjectList {
	if c.Minor < 16 {
		switch obj.(type) {

		// Deployment
		case *appsv1.DeploymentList:
			return &appsv1beta1.DeploymentList{}

		// DaemonSet
		case *appsv1.DaemonSetList:
			return &extv1beta1.DaemonSetList{}

		// ReplicaSet
		case *appsv1.ReplicaSetList:
			return &extv1beta1.ReplicaSetList{}

		// StatefulSet
		case *appsv1.StatefulSetList:
			return &appsv1beta1.StatefulSetList{}
		}
	}

	if c.Minor > 24 {
		switch obj.(type) {
		case *batchv1beta1.CronJobList:
			return &batchv1.CronJobList{}
		}
	}

	return obj
}

// Down convert a resource as needed based on cluster version.
func (c client) downConvert(ctx context.Context, obj k8sclient.Object) (k8sclient.Object, error) {
	new := c.supportedVersion(obj)
	if new == obj {
		return obj, nil
	}

	err := scheme.Scheme.Convert(obj, new, ctx)
	if err != nil {
		return nil, err
	}

	return new, nil
}

// upConvert will convert src resource to dst as needed based on cluster version
func (c client) upConvert(ctx context.Context, src k8sclient.Object, dst k8sclient.Object) error {
	if c.supportedVersion(dst) == dst {
		dst = src
		return nil
	}

	return scheme.Scheme.Convert(src, dst, ctx)
}

// Down convert a resource as needed based on cluster version.
func (c client) downConvertList(ctx context.Context, obj k8sclient.ObjectList) (k8sclient.ObjectList, error) {
	new := c.supportedVersionList(obj)
	if new == obj {
		return obj, nil
	}

	err := scheme.Scheme.Convert(obj, new, ctx)
	if err != nil {
		return nil, err
	}

	return new, nil
}

// upConvert will convert src resource to dst as needed based on cluster version
func (c client) upConvertList(ctx context.Context, src k8sclient.ObjectList, dst k8sclient.ObjectList) error {
	if c.supportedVersionList(dst) == dst {
		dst = src
		return nil
	}

	return scheme.Scheme.Convert(src, dst, ctx)
}

// Get the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Get(ctx context.Context, key k8sclient.ObjectKey, in k8sclient.Object, options ...k8sclient.GetOption) error {
	start := time.Now()
	defer func() { Metrics.Get(c, in, float64(time.Since(start)/nanoToMilli)) }()

	obj := c.supportedVersion(in)

	err := c.Client.Get(ctx, key, obj)
	if err != nil {
		return err
	}

	return c.upConvert(ctx, obj, in)
}

// List the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) List(ctx context.Context, in k8sclient.ObjectList, opt ...k8sclient.ListOption) error {
	obj, err := c.downConvertList(ctx, in)
	start := time.Now()
	defer func() { Metrics.List(c, in, float64(time.Since(start)/nanoToMilli)) }()

	if err != nil {
		return err
	}

	err = c.Client.List(ctx, obj, opt...)
	if err != nil {
		return err
	}

	return c.upConvertList(ctx, obj, in)
}

// Create the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Create(ctx context.Context, in k8sclient.Object, opt ...k8sclient.CreateOption) error {
	start := time.Now()
	defer func() { Metrics.Create(c, in, float64(time.Since(start)/nanoToMilli)) }()

	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	err = c.Client.Create(ctx, obj, opt...)
	if err != nil {
		return err
	}
	if Settings.EnableCachedClient {
		c.waitForPopulatedCache(obj, 0)
	}

	return nil
}

// Delete the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Delete(ctx context.Context, in k8sclient.Object, opt ...k8sclient.DeleteOption) error {
	start := time.Now()
	defer func() { Metrics.Delete(c, in, float64(time.Since(start)/nanoToMilli)) }()

	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	err = c.Client.Delete(ctx, obj, opt...)
	if err != nil {
		return err
	}

	return nil
}

// Update the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Update(ctx context.Context, in k8sclient.Object, opt ...k8sclient.UpdateOption) error {
	start := time.Now()
	defer func() { Metrics.Update(c, in, float64(time.Since(start)/nanoToMilli)) }()

	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	err = c.Client.Update(ctx, obj, opt...)
	if err != nil {
		return err
	}
	if Settings.EnableCachedClient {
		expectedRV, err := getResourceVersion(obj)
		if err != nil {
			return err
		}
		c.waitForPopulatedCache(obj, expectedRV)
	}

	return nil
}

// Wait until cache contains resource with expected resourceVersion _or_ timeout, whichever comes first.
func (c client) waitForPopulatedCache(resource k8sclient.Object, expectedRV int) error {
	log := logging.WithName("compatclient",
		"func", "waitForPopulatedCache",
		"resource", path.Join(resource.GetName(), resource.GetNamespace()),
		"resourceKind", resource.GetObjectKind())

	timeout := 3 * time.Second
	deadline := time.Now().Add(timeout)
	pollInterval := time.Millisecond * 5

	// Define a placeholder with the same structure as the original resource to be overwritten with client.Get
	placeholder, ok := resource.DeepCopyObject().(k8sclient.Object)
	if !ok {
		log.V(4).Info("Error type asserting placeholder as k8sclient.Object")
		return nil
	}

	key, err := getNsName(placeholder)
	if err != nil {
		log.V(4).Info("Error getting nsName for resource")
		return err
	}
	// If no name is defined on the resource, we can't query for it in the cache
	if key.Name == "" {
		log.V(4).Info("Found empty name for resource, can't query for cache population.")
		return nil
	}
	// Poll for resource in cache every 5ms until 3s deadline.
	for time.Now().Before(deadline) {
		// Poll the cache for the latest version of the object
		err := c.Get(context.TODO(), key, placeholder)
		if err != nil {
			if k8serror.IsNotFound(err) {
				log.V(4).Info("Resource not found in cache, polling again.",
					"pollInterval", pollInterval)
				time.Sleep(pollInterval)
				continue
			} else {
				log.V(4).Info("Cache get failed", "error", err)
				return err
			}
		}
		foundRV, err := getResourceVersion(placeholder)
		if err != nil {
			log.V(4).Info("Error getting ResourceVersion")
			return err
		}
		if foundRV < expectedRV {
			log.V(4).Info("ResourceVersion in cache lower than expected, polling again.",
				"pollInterval", pollInterval,
				"expectedRV", expectedRV,
				"foundRV", foundRV)
			time.Sleep(pollInterval)
			continue
		}
		// Resource found in cache, success
		log.V(4).Info("Success, resource found in cache",
			"expectedRV", expectedRV,
			"foundRV", foundRV)
		return nil
	}
	// Resource not found in cache, stop polling for it
	err = fmt.Errorf("resource key=%v/%v kind=%v not found with "+
		"expectedResourceVersion>=%v in cache before timeout %v",
		key.Namespace, key.Name, resource.GetObjectKind(), expectedRV, timeout)

	log.V(4).Info("Timed out waiting for resource to populate in cache",
		"error", err)

	return err
}

func getNsName(in k8sclient.Object) (types.NamespacedName, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(in)
	if err != nil {
		return types.NamespacedName{}, err
	}
	// Some resources don't have a namespace, ignore this.
	ns, _, _ := unstructured.NestedString(u, "metadata", "namespace")
	// Some resources don't have a name, we can't do a lookup on these
	name, _, err := unstructured.NestedString(u, "metadata", "name")
	if err != nil {
		return types.NamespacedName{}, err
	}
	return types.NamespacedName{Namespace: ns, Name: name}, nil
}

func getResourceVersion(in k8sclient.Object) (int, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(in)
	if err != nil {
		return 0, err
	}
	resourceVersion, found, err := unstructured.NestedString(u, "metadata", "resourceVersion")
	if err != nil || !found {
		return 0, err
	}
	resourceVersionInt, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return 0, err
	}
	return resourceVersionInt, nil
}
