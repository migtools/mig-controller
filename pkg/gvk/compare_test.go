package gvk

import (
	"testing"

	"github.com/onsi/gomega"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

var (
	sourceGroups = []metav1.APIGroup{
		{
			Name: "apps",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1",
					Version:      "v1",
				},
				metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1beta2",
					Version:      "v1beta2",
				},
				metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1beta1",
					Version:      "v1beta1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "apps/v1",
				Version:      "v1",
			},
		},
		{
			Name: "batch",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "batch/v1",
					Version:      "v1",
				},
				metav1.GroupVersionForDiscovery{
					GroupVersion: "batch/v1beta1",
					Version:      "v1beta1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "batch/v1",
				Version:      "v1",
			},
		},
		{
			Name: "velero.io",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "velero.io/v1",
					Version:      "v1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "velero.io/v1",
				Version:      "v1",
			},
		},
	}

	destinationGroups = []metav1.APIGroup{
		{
			Name: "apps",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1",
					Version:      "v1",
				},
				metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1beta2",
					Version:      "v1beta2",
				},
				metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1beta1",
					Version:      "v1beta1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "apps/v1",
				Version:      "v1",
			},
		},
		{
			Name: "batch",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "batch/v1",
					Version:      "v1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "batch/v1",
				Version:      "v1",
			},
		},
		{
			Name: "crd.new.io",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "crd.new.io/v1",
					Version:      "v1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "crd.new.io/v1",
				Version:      "v1",
			},
		},
	}
)

func TestCompareGroupVersions(t *testing.T) {
	g := gomega.NewWithT(t)

	expect := []metav1.APIGroup{
		{
			Name: "velero.io",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "velero.io/v1",
					Version:      "v1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "velero.io/v1",
				Version:      "v1",
			},
		},
		{
			Name: "batch",
			Versions: []metav1.GroupVersionForDiscovery{
				metav1.GroupVersionForDiscovery{
					GroupVersion: "batch/v1beta1",
					Version:      "v1beta1",
				},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "batch/v1",
				Version:      "v1",
			},
		},
	}

	t.Run("Full GV diff", func(t *testing.T) {
		srcGroups := &metav1.APIGroupList{
			Groups: sourceGroups,
		}
		dstGroups := &metav1.APIGroupList{
			Groups: destinationGroups,
		}

		gvDiff := compareGroupVersions(srcGroups, dstGroups)
		g.Expect(expect).To(gomega.Equal(gvDiff))
	})
}

func TestExcludeCRDs(t *testing.T) {
	g := gomega.NewWithT(t)

	veleroGVR := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backup",
	}
	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pod",
	}
	resources := []schema.GroupVersionResource{veleroGVR, podGVR}
	expect := []schema.GroupVersionResource{podGVR}
	CRD := apiextv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "backups.velero.io",
		},
		Spec: apiextv1beta1.CustomResourceDefinitionSpec{
			Group: "velero.io",
			Names: apiextv1beta1.CustomResourceDefinitionNames{
				Kind:     "Backup",
				ListKind: "BackupList",
				Plural:   "backups",
				Singular: "backup",
			},
			Scope: apiextv1beta1.NamespaceScoped,
		},
	}

	t.Run("Exclude velero CRD", func(t *testing.T) {
		scheme := runtime.NewScheme()

		// Workaround for "can't assign or convert v1beta1.CustomResourceDefinition into unstructured.Unstructured"
		obj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&CRD)
		// Workaroung for "Object 'apiVersion/kind' is missing in 'unstructured object has no version'"
		obj["kind"] = "CustomResourceDefinition"
		obj["apiVersion"] = "apiextensions.k8s.io/v1beta1"

		cmp := Compare{
			SrcClient: fake.NewSimpleDynamicClient(scheme, &unstructured.Unstructured{Object: obj}),
		}

		err := cmp.excludeCRDs(&resources)
		g.Expect(err).ShouldNot(gomega.HaveOccurred())

		g.Expect(expect).To(gomega.Equal(resources))

	})
}
