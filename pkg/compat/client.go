package compat

import (
	"context"
	appv1 "k8s.io/api/apps/v1"
	appv1beta1 "k8s.io/api/apps/v1beta1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	dapi "k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

//
// A smart client.
// Provides seamless API version compatibility.
type Client struct {
	k8sclient.Client
	dapi.DiscoveryInterface
	// major k8s version.
	Major int
	// minor k8s version.
	Minor int
}

//
// Create a new client.
func NewClient(restCfg *rest.Config) (k8sclient.Client, error) {
	rClient, err := k8sclient.New(
		restCfg,
		k8sclient.Options{
			Scheme: scheme.Scheme,
		})
	if err != nil {
		return nil, err
	}
	dClient, err := dapi.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	version, err := dClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	major, err := strconv.Atoi(version.Major)
	if err != nil {
		return nil, err
	}
	minor, err := strconv.Atoi(strings.Trim(version.Minor, "+"))
	if err != nil {
		return nil, err
	}
	nClient := &Client{
		Client:             rClient,
		DiscoveryInterface: dClient,
		Major:              major,
		Minor:              minor,
	}

	return nClient, nil
}

//
// Down convert a resource as needed based on cluster version.
func (c Client) downConvert(obj runtime.Object) runtime.Object {
	if c.Minor < 16 {
		if _, cast := obj.(*appv1.Deployment); cast {
			return &appv1beta1.Deployment{}
		}
		if _, cast := obj.(*appv1.DeploymentList); cast {
			return &appv1beta1.DeploymentList{}
		}
		if _, cast := obj.(*appv1.DaemonSet); cast {
			return &extv1beta1.DaemonSet{}
		}
		if _, cast := obj.(*appv1.DaemonSetList); cast {
			return &extv1beta1.DaemonSetList{}
		}
	}

	return obj
}

//
// Get the specified resource.
// The resource will be converted to a compatible version as needed.
func (c Client) Get(ctx context.Context, key k8sclient.ObjectKey, in runtime.Object) error {
	obj := c.downConvert(in)
	err := c.Client.Get(ctx, key, obj)
	if err != nil {
		return err
	}
	if in != obj {
		err := scheme.Scheme.Convert(obj, in, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

//
// List the specified resource.
// The resource will be converted to a compatible version as needed.
func (c Client) List(ctx context.Context, opt *k8sclient.ListOptions, in runtime.Object) error {
	obj := c.downConvert(in)
	err := c.Client.List(ctx, opt, obj)
	if err != nil {
		return err
	}
	if in != obj {
		err := scheme.Scheme.Convert(obj, in, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

//
// List the specified resource.
// The resource will be converted to a compatible version as needed.
func (c Client) Create(ctx context.Context, in runtime.Object) error {
	obj := c.downConvert(in)
	return c.Client.Create(ctx, obj)
}

//
// Delete the specified resource.
// The resource will be converted to a compatible version as needed.
func (c Client) Delete(ctx context.Context, in runtime.Object, opt ...k8sclient.DeleteOptionFunc) error {
	obj := c.downConvert(in)
	return c.Client.Delete(ctx, obj, opt...)
}

//
// Update the specified resource.
// The resource will be converted to a compatible version as needed.
func (c Client) Update(ctx context.Context, in runtime.Object) error {
	obj := c.downConvert(in)
	return c.Client.Update(ctx, obj)
}

func (c Client) Status() k8sclient.StatusWriter {
	return c.Client.Status()
}
