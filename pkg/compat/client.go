package compat

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	batchv2alpha "k8s.io/api/batch/v2alpha1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	dapi "k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

//
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

//
// Create a new client.
func NewClient(restCfg *rest.Config, rClient k8sclient.Client) (Client, error) {
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
		Client:             rClient,
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

//
// supportedVersion will determine correct version of the object provided, based on cluster version
func (c client) supportedVersion(obj runtime.Object) runtime.Object {
	if c.Minor < 16 {
		switch obj.(type) {

		// Deployment
		case *appsv1.Deployment:
			return &appsv1beta1.Deployment{}
		case *appsv1.DeploymentList:
			return &appsv1beta1.DeploymentList{}

		// DaemonSet
		case *appsv1.DaemonSet:
			return &extv1beta1.DaemonSet{}
		case *appsv1.DaemonSetList:
			return &extv1beta1.DaemonSetList{}

		// ReplicaSet
		case *appsv1.ReplicaSet:
			return &extv1beta1.ReplicaSet{}
		case *appsv1.ReplicaSetList:
			return &extv1beta1.ReplicaSetList{}

		// StatefulSet
		case *appsv1.StatefulSet:
			return &appsv1beta1.StatefulSet{}
		case *appsv1.StatefulSetList:
			return &appsv1beta1.StatefulSetList{}
		}
	}

	if c.Minor < 8 {
		switch obj.(type) {

		// CronJob
		case *batchv1beta.CronJobList:
			return &batchv2alpha.CronJobList{}
		case *batchv1beta.CronJob:
			return &batchv2alpha.CronJob{}
		}
	}

	return obj
}

//
// Down convert a resource as needed based on cluster version.
func (c client) downConvert(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
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

//
// upConvert will convert src resource to dst as needed based on cluster version
func (c client) upConvert(ctx context.Context, src runtime.Object, dst runtime.Object) error {
	if c.supportedVersion(dst) == dst {
		dst = src
		return nil
	}

	return scheme.Scheme.Convert(src, dst, ctx)
}

//
// Get the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Get(ctx context.Context, key k8sclient.ObjectKey, in runtime.Object) error {
	obj := c.supportedVersion(in)
	start := time.Now()
	err := c.Client.Get(ctx, key, obj)
	if err != nil {
		return err
	}
	elapsed := float64(time.Since(start) / nanoToMilli)
	Metrics.Get(c, in, elapsed)

	return c.upConvert(ctx, obj, in)
}

//
// List the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) List(ctx context.Context, opt *k8sclient.ListOptions, in runtime.Object) error {
	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	start := time.Now()
	err = c.Client.List(ctx, opt, obj)
	if err != nil {
		return err
	}
	elapsed := float64(time.Since(start) / nanoToMilli)
	Metrics.List(c, in, elapsed)

	return c.upConvert(ctx, obj, in)
}

// Create the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Create(ctx context.Context, in runtime.Object) error {
	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	start := time.Now()
	err = c.Client.Create(ctx, obj)
	elapsed := float64(time.Since(start) / nanoToMilli)
	Metrics.Create(c, in, elapsed)

	return err
}

// Delete the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Delete(ctx context.Context, in runtime.Object, opt ...k8sclient.DeleteOptionFunc) error {
	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	start := time.Now()
	err = c.Client.Delete(ctx, obj, opt...)
	elapsed := float64(time.Since(start) / nanoToMilli)
	Metrics.Delete(c, in, elapsed)

	return err
}

// Update the specified resource.
// The resource will be converted to a compatible version as needed.
func (c client) Update(ctx context.Context, in runtime.Object) error {
	obj, err := c.downConvert(ctx, in)
	if err != nil {
		return err
	}

	start := time.Now()
	err = c.Client.Update(ctx, obj)
	elapsed := float64(time.Since(start) / nanoToMilli)
	Metrics.Update(c, in, elapsed)

	return err
}
