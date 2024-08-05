package migmigration

import (
	"context"
	"slices"
	"testing"

	"github.com/go-logr/logr"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	virtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTask_updateDataVolumeRef(t *testing.T) {
	tests := []struct {
		name            string
		objects         []runtime.Object
		dvSource        *virtv1.DataVolumeSource
		ns              string
		mapping         pvcNameMapping
		log             logr.Logger
		expectedFailure bool
		wantErr         bool
		newDV           *cdiv1.DataVolume
	}{
		{
			name:            "no dv source",
			expectedFailure: false,
			wantErr:         false,
		},
		{
			name: "dv source set to unknown dv",
			dvSource: &virtv1.DataVolumeSource{
				Name: "dv-0",
			},
			expectedFailure: true,
			wantErr:         true,
		},
		{
			name: "dv source set to known dv, but missing in mapping",
			objects: []runtime.Object{
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			dvSource: &virtv1.DataVolumeSource{
				Name: "dv-0",
			},
			ns:              "ns-0",
			expectedFailure: true,
			wantErr:         false,
		},
		{
			name: "dv source set to known dv, but mapping as value only",
			objects: []runtime.Object{
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			dvSource: &virtv1.DataVolumeSource{
				Name: "dv-0",
			},
			mapping:         pvcNameMapping{"ns-0/src-0": "dv-0"},
			ns:              "ns-0",
			expectedFailure: false,
			wantErr:         false,
		},
		{
			name: "dv source set to known dv, but mapping as key",
			objects: []runtime.Object{
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			dvSource: &virtv1.DataVolumeSource{
				Name: "dv-0",
			},
			mapping:         pvcNameMapping{"ns-0/dv-0": "tgt-0"},
			ns:              "ns-0",
			expectedFailure: false,
			wantErr:         false,
			newDV: &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tgt-0",
					Namespace: "ns-0",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			err := cdiv1.AddToScheme(s)
			if err != nil {
				panic(err)
			}
			c := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(tt.objects...).Build()
			res, err := updateDataVolumeRef(c, tt.dvSource, tt.ns, tt.mapping, tt.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("updateDataVolumeRef() error = %v, wantErr %v", err, tt.wantErr)
				t.FailNow()
			}
			if res != tt.expectedFailure {
				t.Errorf("updateDataVolumeRef() expected failure = %v, want %v", res, tt.expectedFailure)
				t.FailNow()
			}
			if tt.newDV != nil {
				err := c.Get(context.TODO(), client.ObjectKeyFromObject(tt.newDV), tt.newDV)
				if err != nil {
					t.Errorf("updateDataVolumeRef() failed to create new DV: %v", err)
					t.FailNow()
				}
			} else {
				dvs := &cdiv1.DataVolumeList{}
				err := c.List(context.TODO(), dvs, client.InNamespace(tt.ns))
				if err != nil && !k8serrors.IsNotFound(err) {
					t.Errorf("Error reading datavolumes: %v", err)
					t.FailNow()
				} else if err == nil && len(dvs.Items) > 0 {
					for _, dv := range dvs.Items {
						if dv.Name != tt.dvSource.Name {
							t.Errorf("updateDataVolumeRef() created new DV when it shouldn't have, %v", dvs.Items)
							t.FailNow()
						}
					}
				}
			}
		})
	}
}

func TestTask_swapVirtualMachinePVCRefs(t *testing.T) {
	tests := []struct {
		name             string
		objects          []runtime.Object
		restConfig       rest.Interface
		pvcMapping       pvcNameMapping
		expectedFailures []string
		expectedNewName  string
		shouldStartVM    bool
	}{
		{
			name:       "no VMs, should return no failed VMs",
			restConfig: getFakeRestClient(),
		},
		{
			name:       "VMs without volumes, should return no failed VMs",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				&virtv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vm-0",
						Namespace: "ns-0",
					},
				},
			},
		},
		{
			name:       "VMs with DVs, no mapping, should return all VMs",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				createVirtualMachine("vm-0", "ns-0", []virtv1.Volume{
					{
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "dv-0",
							},
						},
					},
				}),
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			expectedFailures: []string{"ns-0/vm-0"},
		},
		{
			name:       "VMs with PVCs, no mapping, should return all VMs",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				createVirtualMachine("vm-0", "ns-0", []virtv1.Volume{
					{
						VolumeSource: virtv1.VolumeSource{
							PersistentVolumeClaim: &virtv1.PersistentVolumeClaimVolumeSource{
								PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc-0",
								},
							},
						},
					},
				}),
			},
			expectedFailures: []string{"ns-0/vm-0"},
		},
		{
			name:       "VMs with DVs, mapping to new name, should return no failures",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				createVirtualMachine("vm-0", "ns-0", []virtv1.Volume{
					{
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "dv-0",
							},
						},
					},
				}),
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			pvcMapping:      pvcNameMapping{"ns-0/dv-0": "dv-1"},
			expectedNewName: "dv-1",
		},
		{
			name:       "VMs with DVTemplatess, mapping to new name, should return no failures",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				createVirtualMachineWithDVTemplate("vm-0", "ns-0", []virtv1.Volume{
					{
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "dv-0",
							},
						},
					},
				}),
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			pvcMapping:      pvcNameMapping{"ns-0/dv-0": "dv-1"},
			expectedNewName: "dv-1",
		},
		{
			name:       "VMs with DVs, mapping to new name, but running VM, should return no failures, and no updates",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				createVirtualMachine("vm-0", "ns-0", []virtv1.Volume{
					{
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "dv-0",
							},
						},
					},
				}),
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
				createVirtlauncherPod("vm-0", "ns-0"),
			},
			pvcMapping: pvcNameMapping{"ns-0/dv-0": "dv-1"},
		},
		{
			name:       "VMs with DVs, mapping to new name, should return no failures",
			restConfig: getFakeRestClient(),
			objects: []runtime.Object{
				createVirtualMachineWithAnnotation("vm-0", "ns-0", migapi.StartVMAnnotation, "true", []virtv1.Volume{
					{
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "dv-0",
							},
						},
					},
				}),
				&cdiv1.DataVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dv-0",
						Namespace: "ns-0",
					},
				},
			},
			pvcMapping:      pvcNameMapping{"ns-0/dv-0": "dv-1"},
			expectedNewName: "dv-1",
			shouldStartVM:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"ns-0:ns-0", "ns-1:ns-1"},
						},
					},
				},
			}
			s := scheme.Scheme
			err := cdiv1.AddToScheme(s)
			if err != nil {
				panic(err)
			}
			err = virtv1.AddToScheme(s)
			if err != nil {
				panic(err)
			}
			c := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(tt.objects...).Build()
			failedVMs := task.swapVirtualMachinePVCRefs(c, tt.restConfig, tt.pvcMapping)
			if len(failedVMs) != len(tt.expectedFailures) {
				t.Errorf("swapVirtualMachinePVCRefs() failed to swap PVC refs for VMs: %v, expected failures: %v", failedVMs, tt.expectedFailures)
				t.FailNow()
			}
			for _, failedVM := range failedVMs {
				if !slices.Contains(tt.expectedFailures, failedVM) {
					t.Errorf("unexpected failed VM: %s, expected failures: %v", failedVM, tt.expectedFailures)
					t.FailNow()
				}
			}
			vm := &virtv1.VirtualMachine{}
			if err := c.Get(context.TODO(), client.ObjectKey{Namespace: "ns-0", Name: "vm-0"}, vm); err != nil && !k8serrors.IsNotFound(err) {
				t.Errorf("failed to get VM: %v", err)
				t.FailNow()
			}
			if vm.Spec.Template != nil {
				found := false
				for _, volume := range vm.Spec.Template.Spec.Volumes {
					if volume.VolumeSource.DataVolume != nil && volume.VolumeSource.DataVolume.Name == tt.expectedNewName {
						found = true
					}
					if volume.VolumeSource.PersistentVolumeClaim != nil && volume.VolumeSource.PersistentVolumeClaim.ClaimName == tt.expectedNewName {
						found = true
					}
				}
				if !found && tt.expectedNewName != "" {
					t.Errorf("Didn't find new volume name %s", tt.expectedNewName)
					t.FailNow()
				} else if tt.expectedNewName == "" && vm.ObjectMeta.ResourceVersion == "1000" {
					t.Errorf("VM updated when it shouldn't have")
					t.FailNow()
				}
				// Check DVTemplates
				if len(vm.Spec.DataVolumeTemplates) > 0 {
					found = false
					for _, dvTemplate := range vm.Spec.DataVolumeTemplates {
						if dvTemplate.Name == tt.expectedNewName {
							found = true
						}
					}
					if !found && tt.expectedNewName != "" {
						t.Errorf("Didn't find new volume name %s in DVTemplate", tt.expectedNewName)
						t.FailNow()
					} else if found && tt.expectedNewName == "" {
						t.Errorf("Found new volume name %s in DVTemplate when it shouldn't have", tt.expectedNewName)
						t.FailNow()
					}
				}
				if tt.shouldStartVM {
					if _, ok := vm.GetAnnotations()[migapi.StartVMAnnotation]; ok {
						t.Errorf("VM should have started")
						t.FailNow()
					}
				}
			}
		})
	}
}

func createVirtualMachine(name, namespace string, volumes []virtv1.Volume) *virtv1.VirtualMachine {
	return &virtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: virtv1.VirtualMachineSpec{
			Template: &virtv1.VirtualMachineInstanceTemplateSpec{
				Spec: virtv1.VirtualMachineInstanceSpec{
					Volumes: volumes,
				},
			},
		},
	}
}

func createVirtualMachineWithDVTemplate(name, namespace string, volumes []virtv1.Volume) *virtv1.VirtualMachine {
	vm := createVirtualMachine(name, namespace, volumes)
	for _, volume := range volumes {
		if volume.VolumeSource.DataVolume != nil {
			vm.Spec.DataVolumeTemplates = append(vm.Spec.DataVolumeTemplates, virtv1.DataVolumeTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      volume.VolumeSource.DataVolume.Name,
					Namespace: namespace,
				},
				Spec: cdiv1.DataVolumeSpec{
					Storage: &cdiv1.StorageSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			})
		}
	}
	return vm
}

func createVirtualMachineWithAnnotation(name, namespace, key, value string, volumes []virtv1.Volume) *virtv1.VirtualMachine {
	vm := createVirtualMachine(name, namespace, volumes)
	vm.Annotations = map[string]string{key: value}
	return vm
}
