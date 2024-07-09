package migmigration

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testVMName = "test-vm"
)

func TestQuiesceVirtualMachine(t *testing.T) {
	tests := []struct {
		name       string
		client     k8sclient.Client
		restClient rest.Interface
		task       *Task
		wantErr    bool
	}{
		{
			name:   "If no namespaces, quiesceVirtualMachines should not do anything",
			client: getFakeClientWithObjs(),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{},
				},
			},
			restClient: nil,
			wantErr:    false,
		},
		{
			name:   "If no virtual machines in namespace, quiesceVirtualMachines should not do anything",
			client: getFakeClientWithObjs(),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: nil,
			wantErr:    false,
		},
		{
			name:   "If active virtual machines in namespace, quiesceVirtualMachines should call stop",
			client: getFakeClientWithObjs(createVM(testVMName, "src-namespace"), createVirtlauncherPod(testVMName, "src-namespace")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: getFakeRestClient(),
			wantErr:    false,
		},
		{
			name:   "If no active virtual machines in namespace, quiesceVirtualMachines not do anything",
			client: getFakeClientWithObjs(createVM(testVMName, "src-namespace")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: nil,
			wantErr:    false,
		},
		{
			name:   "If active virtual machines in namespace with runstrategy, quiesceVirtualMachines should call stop",
			client: getFakeClientWithObjs(createVMWithRunStrategy(testVMName, "src-namespace", virtv1.RunStrategyRerunOnFailure), createVirtlauncherPod(testVMName, "src-namespace")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: getFakeRestClient(),
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.task.quiesceVirtualMachines(tt.client, tt.restClient); err != nil && !tt.wantErr {
				t.Errorf("quiesceVirtualMachines() error = %v, wantErr %v", err, tt.wantErr)
			} else if err == nil && tt.wantErr {
				t.Errorf("quiesceVirtualMachines() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.restClient != nil {
				fakeClient := tt.restClient.(*fakerest.RESTClient)
				if fakeClient.Req == nil {
					t.Errorf("Expected RESTClient to be called, but it was not")
				} else {
					if fakeClient.Req.Method != "PUT" {
						t.Errorf("Expected RESTClient to be called with PUT, but got %s", fakeClient.Req.Method)
					}
					if !strings.Contains(fakeClient.Req.URL.Path, "stop") {
						t.Errorf("Expected RESTClient to be called with stop, but got %s", fakeClient.Req.URL.Path)
					}
				}
			}
			if tt.client != nil {
				resVM := &virtv1.VirtualMachine{}
				if err := tt.client.Get(context.Background(), k8sclient.ObjectKey{Name: testVMName, Namespace: "src-namespace"}, resVM); err != nil && !k8serrors.IsNotFound(err) {
					t.Errorf("Expected VirtualMachine to be updated, but got error %v", err)
				}
				if resVM.Spec.RunStrategy != nil {
					if resVM.Annotations[migapi.RunStrategyAnnotation] != string(*resVM.Spec.RunStrategy) {
						t.Errorf("Expected VirtualMachine to be updated with annotation %s, but got %s", string(*resVM.Spec.RunStrategy), resVM.Annotations[migapi.RunStrategyAnnotation])
					}
				}
			}
		})
	}
}

func TestIsVMActive(t *testing.T) {
	vm := createVM(testVMName, "test-namespace")

	tests := []struct {
		name     string
		client   k8sclient.Client
		expected bool
	}{
		{
			name:     "If not pod exists, isVMActive should return false",
			client:   getFakeClientWithObjs(),
			expected: false,
		},
		{
			name: "If pod exists with owner ref, isVMActive should return true",
			client: getFakeClientWithObjs(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-virt-launcher",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "kubevirt.io/v1",
							Kind:       "VirtualMachineInstance",
							Name:       testVMName,
						},
					},
				},
			}),
			expected: true,
		},
		{
			name: "If pod exists with owner ref in wrong namespace, isVMActive should return false",
			client: getFakeClientWithObjs(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-virt-launcher",
					Namespace: "test-namespace2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "kubevirt.io/v1",
							Kind:       "VirtualMachineInstance",
							Name:       testVMName,
						},
					},
				},
			}),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isVMActive(vm, tt.client)
			if res != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, res)
			}
		})
	}
}

func TestUnQuiesceVirtualMachine(t *testing.T) {
	tests := []struct {
		name                string
		client              k8sclient.Client
		restClient          rest.Interface
		task                *Task
		namespaces          []string
		wantErr             bool
		wantRestCall        bool
		expectedRunStrategy *virtv1.VirtualMachineRunStrategy
	}{
		{
			name:   "If no namespaces, unQuiesceVirtualMachines should not do anything",
			client: getFakeClientWithObjs(),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: getFakeRestClient(),
			wantErr:    false,
			namespaces: nil,
		},
		{
			name:   "If no virtual machine in namespace, unQuiesceVirtualMachines should not do anything",
			client: getFakeClientWithObjs(),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: getFakeRestClient(),
			wantErr:    false,
			namespaces: []string{"tgt-namespace"},
		},
		{
			name:   "If active virtual machine in namespace, unQuiesceVirtualMachines should not do anything",
			client: getFakeClientWithObjs(createVM(testVMName, "tgt-namespace"), createVirtlauncherPod(testVMName, "tgt-namespace")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: getFakeRestClient(),
			wantErr:    false,
			namespaces: []string{"tgt-namespace"},
		},
		{
			name:   "If inactive virtual machine in namespace, but marked to not start, unQuiesceVirtualMachines should not do anything",
			client: getFakeClientWithObjs(createVMWithAnnotation(testVMName, "tgt-namespace", map[string]string{migapi.StartVMAnnotation: "false"})),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient: getFakeRestClient(),
			wantErr:    false,
			namespaces: []string{"tgt-namespace"},
		},
		{
			name:   "If inactive virtual machine in namespace, and marked to start, unQuiesceVirtualMachines should call subresource",
			client: getFakeClientWithObjs(createVMWithAnnotation(testVMName, "tgt-namespace", map[string]string{migapi.StartVMAnnotation: "true"})),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient:   getFakeRestClient(),
			wantErr:      false,
			wantRestCall: true,
			namespaces:   []string{"tgt-namespace"},
		},
		{
			name: "If inactive virtual machine in namespace, and marked to start with run strategy, unQuiesceVirtualMachines should call subresource",
			client: getFakeClientWithObjs(createVMWithAnnotation(testVMName, "tgt-namespace", map[string]string{
				migapi.StartVMAnnotation:     "true",
				migapi.RunStrategyAnnotation: string(virtv1.RunStrategyRerunOnFailure),
			})),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			restClient:          getFakeRestClient(),
			wantErr:             false,
			wantRestCall:        true,
			namespaces:          []string{"tgt-namespace"},
			expectedRunStrategy: ptr.To[virtv1.VirtualMachineRunStrategy](virtv1.RunStrategyRerunOnFailure),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.task.unQuiesceVirtualMachines(tt.client, tt.restClient, tt.namespaces)
			if res != nil && !tt.wantErr {
				t.Errorf("Expected no error, got %v", res)
			}
			if tt.restClient == nil {
				t.Errorf("Expected RESTClient to not be nil")
			}
			fakeClient := tt.restClient.(*fakerest.RESTClient)
			if !tt.wantRestCall {
				if fakeClient.Req != nil {
					t.Errorf("Expected RESTClient to not be called, but it was")
				}
			} else {
				if fakeClient.Req == nil {
					t.Errorf("Expected RESTClient to be called, but it was not")
				} else {
					if fakeClient.Req.Method != "PUT" {
						t.Errorf("Expected RESTClient to be called with PUT, but got %s", fakeClient.Req.Method)
					}
					if !strings.Contains(fakeClient.Req.URL.Path, "start") {
						t.Errorf("Expected RESTClient to be called with start, but got %s", fakeClient.Req.URL.Path)
					}
				}
				if tt.client != nil {
					resVM := &virtv1.VirtualMachine{}
					if err := tt.client.Get(context.Background(), k8sclient.ObjectKey{Name: testVMName, Namespace: "tgt-namespace"}, resVM); err != nil {
						t.Errorf("Expected VirtualMachine to be updated, but got error %v", err)
					}
					if _, ok := resVM.Annotations[migapi.StartVMAnnotation]; ok {
						t.Errorf("Expected startVM annotation to be deleted, but it is there")
					}
					if _, ok := resVM.Annotations[migapi.RunStrategyAnnotation]; ok {
						t.Errorf("Expected run strategy annotation to be deleted, but it is there")
					}
					if resVM.Spec.RunStrategy != nil && tt.expectedRunStrategy != nil && *resVM.Spec.RunStrategy != *tt.expectedRunStrategy {
						t.Errorf("Expected VirtualMachine to be updated with run strategy %s, but got %s", *tt.expectedRunStrategy, *resVM.Spec.RunStrategy)
					} else if resVM.Spec.RunStrategy == nil && tt.expectedRunStrategy != nil ||
						resVM.Spec.RunStrategy != nil && tt.expectedRunStrategy == nil {
						t.Errorf("runstrategy nil when other is not")
					}
				}
			}
		})
	}
}

func TestEnsureDestinationQuiescedPodsTerminated(t *testing.T) {
	tests := []struct {
		name          string
		client        compat.Client
		task          *Task
		allterminated bool
	}{
		{
			name:   "no pods",
			client: getFakeClientWithObjs(),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: true,
		},
		{
			name:   "no pods with owner ref",
			client: getFakeClientWithObjs(createPodWithOwner("pod", "tgt-namespace", "", "", "")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: true,
		},
		{
			name:   "pods with deployment owner ref",
			client: getFakeClientWithObjs(createPodWithOwner("pod", "tgt-namespace", "v1", "ReplicaSet", "deployment")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: false,
		},
		{
			name:   "skipped pods with vm owner ref",
			client: getFakeClientWithObjs(createPodWithOwnerAndPhase("pod", "tgt-namespace", "v1", "VirtualMachineInstance", "virt-launcher", corev1.PodSucceeded)),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.destinationClient = tt.client
			allTerminated, err := tt.task.ensureDestinationQuiescedPodsTerminated()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if allTerminated != tt.allterminated {
				t.Errorf("ensureDestinationQuiescedPodsTerminated() allTerminated = %v, want %v", allTerminated, tt.allterminated)
			}
		})
	}
}

func TestEnsureSourceQuiescedPodsTerminated(t *testing.T) {
	tests := []struct {
		name          string
		client        compat.Client
		task          *Task
		allterminated bool
	}{
		{
			name:   "no pods",
			client: getFakeClientWithObjs(),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: true,
		},
		{
			name:   "no pods with owner ref",
			client: getFakeClientWithObjs(createPodWithOwner("pod", "src-namespace", "", "", "")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: true,
		},
		{
			name:   "pods with deployment owner ref",
			client: getFakeClientWithObjs(createPodWithOwner("pod", "src-namespace", "v1", "ReplicationController", "controller")),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: false,
		},
		{
			name:   "skipped pods with vm owner ref",
			client: getFakeClientWithObjs(createPodWithOwnerAndPhase("pod", "src-namespace", "v1", "VirtualMachineInstance", "virt-launcher", corev1.PodFailed)),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					MigPlan: &migapi.MigPlan{
						Spec: migapi.MigPlanSpec{
							Namespaces: []string{"src-namespace:tgt-namespace"},
						},
					},
				},
			},
			allterminated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			allTerminated, err := tt.task.ensureSourceQuiescedPodsTerminated()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if allTerminated != tt.allterminated {
				t.Errorf("ensureDestinationQuiescedPodsTerminated() allTerminated = %v, want %v", allTerminated, tt.allterminated)
			}
		})
	}
}

func getFakeClientWithObjs(obj ...k8sclient.Object) compat.Client {
	client, _ := fakecompat.NewFakeClient(obj...)
	return client
}

func getFakeRestClient() rest.Interface {
	return &fakerest.RESTClient{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Resp: &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
}

func createVM(name, namespace string) *virtv1.VirtualMachine {
	return &virtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createVMWithRunStrategy(name, namespace string, runStrategy virtv1.VirtualMachineRunStrategy) *virtv1.VirtualMachine {
	vm := createVM(name, namespace)
	vm.Spec = virtv1.VirtualMachineSpec{
		RunStrategy: ptr.To[virtv1.VirtualMachineRunStrategy](runStrategy),
	}
	return vm
}

func createVMWithAnnotation(name, namespace string, ann map[string]string) *virtv1.VirtualMachine {
	vm := createVM(name, namespace)
	vm.Annotations = ann
	return vm
}

func createVirtlauncherPod(vmName, namespace string) *corev1.Pod {
	pod := createPodWithOwner(vmName+"-virt-launcher", namespace, "kubevirt.io/v1", "VirtualMachineInstance", vmName)
	pod.Labels = map[string]string{
		"kubevirt.io": "virt-launcher",
	}
	return pod
}

func createPodWithOwner(name, namespace, apiversion, kind, ownerName string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	if apiversion != "" && kind != "" && ownerName != "" {
		pod.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: apiversion,
				Kind:       kind,
				Name:       ownerName,
			},
		}
	}
	return pod
}

func createPodWithOwnerAndPhase(name, namespace, apiversion, kind, ownerName string, phase corev1.PodPhase) *corev1.Pod {
	pod := createPodWithOwner(name, namespace, apiversion, kind, ownerName)
	pod.Status.Phase = phase
	return pod
}
