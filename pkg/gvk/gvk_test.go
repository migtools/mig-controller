package gvk

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/fake"
	"reflect"
	"testing"
)

//src 4.3
//{
//  "kind": "APIGroup",
//  "apiVersion": "v1",
//  "name": "apps",
//  "versions": [
//    {
//      "groupVersion": "apps/v1",
//      "version": "v1"
//    },
//    {
//      "groupVersion": "apps/v1beta2",
//      "version": "v1beta2"
//    },
//    {
//      "groupVersion": "apps/v1beta1",
//      "version": "v1beta1"
//    }
//  ],
//  "preferredVersion": {
//    "groupVersion": "apps/v1",
//    "version": "v1"
//  }
//}

//dst 4.4
//{
//  "kind": "APIGroup",
//  "apiVersion": "v1",
//  "name": "apps",
//  "versions": [
//    {
//      "groupVersion": "apps/v1",
//      "version": "v1"
//    }
//  ],
//  "preferredVersion": {
//    "groupVersion": "apps/v1",
//    "version": "v1"
//  }
//}
func Test_compareResources(t *testing.T) {
	type args struct {
		src []*metav1.APIResourceList
		dst []*metav1.APIResourceList
	}
	tests := []struct {
		name string
		args args
		want []*metav1.APIResourceList
	}{
		{
			name: "4.3 -> 4.4",
			args: args{
				src: []*metav1.APIResourceList{
					{
						GroupVersion: "apps/v1",
						APIResources: []metav1.APIResource{
							{
								Name:       "deployments",
								Namespaced: true,
								Group:      "apps",
								Version:    "v1beta1",
								Kind:       "Deployment",
								ShortNames: []string{"deploy"},
								Verbs:      []string{"create", "update", "list", "delete"},
							},
						},
					},
				},
				dst: []*metav1.APIResourceList{
					{
						GroupVersion: "apps/v1",
						APIResources: []metav1.APIResource{
							{
								Name:       "deployments",
								Namespaced: true,
								Group:      "apps",
								Version:    "v1",
								Kind:       "Deployment",
								ShortNames: []string{"deploy"},
								Verbs:      []string{"create", "update", "list", "delete"},
							},
						},
					},
					{
						GroupVersion: "apps/v1beta1",
						APIResources: []metav1.APIResource{
							{
								Name:       "deployments",
								Namespaced: true,
								Group:      "apps",
								Version:    "v1beta1",
								Kind:       "Deployment",
								ShortNames: []string{"deploy"},
								Verbs:      []string{"create", "update", "list", "delete"},
							},
						},
					},
					{
						GroupVersion: "apps/v1beta2",
						APIResources: []metav1.APIResource{
							{
								Name:       "deployments",
								Namespaced: true,
								Group:      "apps",
								Version:    "v1",
								Kind:       "Deployment",
								ShortNames: []string{"deploy"},
								Verbs:      []string{"create", "update", "list", "delete"},
							},
						},
					},
				},
			},
			want: []*metav1.APIResourceList{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareResources(tt.args.src, tt.args.dst); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("compareResources() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_collectPreferredResources(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset()
	serverGroups, err := fakeKubeClient.Discovery().ServerGroups()
	var preferredDeployment metav1.GroupVersionForDiscovery
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range serverGroups.Groups {
		if group.Kind == "deployments" {
			preferredDeployment = group.PreferredVersion
		}
	}
	type args struct {
		discovery discovery.DiscoveryInterface
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "client has multiple deployment versions",
			args:    args{fakeKubeClient.Discovery()},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectPreferredResources(tt.args.discovery)
			if (err != nil) != tt.wantErr {
				t.Errorf("collectResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, resources := range got {
				if resources.Kind == "deployments" && resources.GroupVersion != preferredDeployment.GroupVersion {
					t.Fail()
				}
			}
		})
	}
}
