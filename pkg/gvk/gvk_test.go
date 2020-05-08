package gvk

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
