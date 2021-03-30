package fake

import (
	"context"
	"fmt"

	"github.com/konveyor/mig-controller/pkg/compat"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	krand "k8s.io/apimachinery/pkg/util/rand"
	dapi "k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeCompatClient struct {
	k8sclient.Client
	dapi.DiscoveryInterface
}

func (fake fakeCompatClient) RestConfig() *rest.Config {
	return nil
}
func (fake fakeCompatClient) MajorVersion() int {
	return 0
}
func (fake fakeCompatClient) MinorVersion() int {
	return 0
}
func (fake fakeCompatClient) Get(ctx context.Context, key k8sclient.ObjectKey, obj runtime.Object) error {
	return fake.Client.Get(ctx, key, obj)
}
func (fake fakeCompatClient) List(ctx context.Context, opt *k8sclient.ListOptions, obj runtime.Object) error {
	return fake.Client.List(ctx, opt, obj)
}
func (fake fakeCompatClient) Create(ctx context.Context, obj runtime.Object) error {
	switch obj.(type) {
	case *corev1.Pod:
		s := obj.(*corev1.Pod)
		if s.Name == "" && s.GenerateName != "" {
			s.Name = fmt.Sprintf("%s%s", s.GenerateName, krand.String(10))
		}
		obj = s
	}
	return fake.Client.Create(ctx, obj)
}
func (fake fakeCompatClient) Delete(ctx context.Context, obj runtime.Object, opt ...k8sclient.DeleteOptionFunc) error {
	return fake.Client.Delete(ctx, obj, opt...)
}
func (fake fakeCompatClient) Update(ctx context.Context, obj runtime.Object) error {
	return fake.Client.Update(ctx, obj)
}

func NewFakeClient(obj ...runtime.Object) compat.Client {
	return fakeCompatClient{
		Client: fake.NewFakeClient(obj...),
	}
}
