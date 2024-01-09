package fake

import (
	"context"
	"fmt"

	"github.com/konveyor/mig-controller/pkg/compat"
	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	krand "k8s.io/apimachinery/pkg/util/rand"
	dapi "k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeCompatClient struct {
	k8sclient.Client
	dapi.DiscoveryInterface

	Major int
	Minor int
}

func (fake fakeCompatClient) RestConfig() *rest.Config {
	return nil
}

func (fake fakeCompatClient) MajorVersion() int {
	return fake.Major
}

func (fake fakeCompatClient) MinorVersion() int {
	return fake.Minor
}

func (fake fakeCompatClient) Get(ctx context.Context, key k8sclient.ObjectKey, obj k8sclient.Object) error {
	return fake.Client.Get(ctx, key, obj)
}

func (fake fakeCompatClient) List(ctx context.Context, obj k8sclient.ObjectList, opt ...k8sclient.ListOption) error {
	return fake.Client.List(ctx, obj, opt...)
}

func (fake fakeCompatClient) Create(ctx context.Context, obj k8sclient.Object, opt ...k8sclient.CreateOption) error {
	switch obj.(type) {
	case *corev1.Pod:
		s := obj.(*corev1.Pod)
		if s.Name == "" && s.GenerateName != "" {
			s.Name = fmt.Sprintf("%s%s", s.GenerateName, krand.String(10))
		}
		obj = s
	}
	return fake.Client.Create(ctx, obj, opt...)
}

func (fake fakeCompatClient) Delete(ctx context.Context, obj k8sclient.Object, opt ...k8sclient.DeleteOption) error {
	return fake.Client.Delete(ctx, obj, opt...)
}

func (fake fakeCompatClient) Update(ctx context.Context, obj k8sclient.Object, opt ...k8sclient.UpdateOption) error {
	return fake.Client.Update(ctx, obj, opt...)
}

func NewFakeClient(obj ...k8sclient.Object) (compat.Client, error) {
	scheme, err := getSchemeForFakeClient()
	if err != nil {
		return nil, err
	}
	return fakeCompatClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj...).Build(),
	}, nil
}

func NewFakeClientWithVersion(Major int, Minor int, obj ...k8sclient.Object) (compat.Client, error) {
	scheme, err := getSchemeForFakeClient()
	if err != nil {
		return nil, err
	}
	return fakeCompatClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj...).Build(),
		Major:  Major,
		Minor:  Minor,
	}, nil
}

func getSchemeForFakeClient() (*runtime.Scheme, error) {
	if err := routev1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}
	if err := configv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}
	return scheme.Scheme, nil
}
