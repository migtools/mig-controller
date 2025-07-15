package directvolumemigration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	transfer "github.com/konveyor/crane-lib/state_transfer/transfer"
	"github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	fakecompat "github.com/konveyor/mig-controller/pkg/compat/fake"
	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	virtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sourcePVC                   = "source-pvc"
	sourceNs                    = "source-ns"
	targetPVC                   = "target-pvc"
	targetNs                    = "target-ns"
	targetDv                    = "target-dv"
	testPVCName                 = "test-pvc"
	testStorageClass            = "test-sc"
	testDefaultStorageClass     = "test-default-sc"
	testVirtDefaultStorageClass = "test-virt-default-sc"
)

func TestTask_startLiveMigrations(t *testing.T) {
	tests := []struct {
		name             string
		task             *Task
		client           compat.Client
		pairMap          map[string][]transfer.PVCPair
		expectedReasons  []string
		expectedVMUpdate bool
		wantErr          bool
	}{
		{
			name: "same namespace, no volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeFinal),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachine("vm", testNamespace)),
			pairMap: map[string][]transfer.PVCPair{
				testNamespace + ":" + testNamespace: {
					transfer.NewPVCPair(createPvc("pvc1", testNamespace), createPvc("pvc2", testNamespace)),
				},
			},
		},
		{
			name: "different namespace, no volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachine("vm", testNamespace)),
			pairMap: map[string][]transfer.PVCPair{
				testNamespace + ":" + testNamespace2: {
					transfer.NewPVCPair(createPvc("pvc1", testNamespace), createPvc("pvc2", testNamespace2)),
				},
			},
			wantErr:         true,
			expectedReasons: []string{"source and target namespaces must match"},
		},
		{
			name: "running VM, no volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeFinal),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachine("vm", testNamespace), createVirtlauncherPod("vm", testNamespace, []string{"dv"})),
			pairMap: map[string][]transfer.PVCPair{
				testNamespace + ":" + testNamespace: {
					transfer.NewPVCPair(createPvc("pvc1", testNamespace), createPvc("pvc2", testNamespace)),
				},
			},
		},
		{
			name: "running VM, matching volume",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv",
						},
					},
				},
			}), createVirtlauncherPod("vm", testNamespace, []string{"dv"}),
				createDataVolume("dv", testNamespace)),
			pairMap: map[string][]transfer.PVCPair{
				testNamespace + ":" + testNamespace: {
					transfer.NewPVCPair(createPvc("dv", testNamespace), createPvc("dv2", testNamespace)),
				},
			},
			expectedVMUpdate: true,
		},
		{
			name: "running VM, no matching volume",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeFinal),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv-nomatch",
						},
					},
				},
			}), createVirtlauncherPod("vm", testNamespace, []string{"dv"}),
				createDataVolume("dv", testNamespace)),
			pairMap: map[string][]transfer.PVCPair{
				testNamespace + ":" + testNamespace: {
					transfer.NewPVCPair(createPvc("dv", testNamespace), createPvc("dv2", testNamespace)),
				},
			},
			wantErr:          false,
			expectedVMUpdate: false,
			expectedReasons:  []string{"source and target volumes do not match for VM vm"},
		},
		{
			name: "running VM, but offline migration in progress",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeFinal),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv",
						},
					},
				},
			}), createVirtlauncherPod("vm", testNamespace, []string{"dv"}),
				createDataVolume("dv", testNamespace),
				createBlockRsyncPod("blockrsync", testNamespace, &Task{
					Owner: &v1alpha1.DirectVolumeMigration{
						Spec: v1alpha1.DirectVolumeMigrationSpec{
							MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeFinal),
						},
					},
				}, []string{"dv"})),
			pairMap: map[string][]transfer.PVCPair{
				testNamespace + ":" + testNamespace: {
					transfer.NewPVCPair(createPvc("dv", testNamespace), createPvc("dv2", testNamespace)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			tt.task.destinationClient = tt.client
			reasons, err := tt.task.startLiveMigrations(tt.pairMap)
			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			} else if err == nil && tt.wantErr {
				t.Errorf("expected error, got nil")
				t.FailNow()
			}
			if len(reasons) != len(tt.expectedReasons) {
				t.Errorf("expected %v, got %v", tt.expectedReasons, reasons)
				t.FailNow()
			}
			for i, r := range reasons {
				if r != tt.expectedReasons[i] {
					t.Errorf("expected %v, got %v", tt.expectedReasons, reasons)
					t.FailNow()
				}
			}
			vm := &virtv1.VirtualMachine{}
			err = tt.client.Get(context.Background(), k8sclient.ObjectKey{Name: "vm", Namespace: testNamespace}, vm)
			if err != nil && !k8serrors.IsNotFound(err) {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if tt.expectedVMUpdate {
				if vm.Spec.UpdateVolumesStrategy == nil || *vm.Spec.UpdateVolumesStrategy != virtv1.UpdateVolumesStrategyMigration {
					t.Errorf("expected update volumes strategy to be migration")
					t.FailNow()
				}
			} else {
				if vm.Spec.UpdateVolumesStrategy != nil {
					t.Errorf("expected update volumes strategy to be nil")
					t.FailNow()
				}
			}
		})
	}
}

func TestGetNamespace(t *testing.T) {
	_, err := getNamespace("nocolon")
	if err == nil {
		t.Errorf("expected error, got nil")
		t.FailNow()
	}
}

func TestTaskGetRunningVMVolumes(t *testing.T) {
	tests := []struct {
		name            string
		task            *Task
		client          compat.Client
		expectedVolumes []string
	}{
		{
			name:            "no VMs",
			task:            &Task{},
			client:          getFakeClientWithObjs(),
			expectedVolumes: []string{},
		},
		{
			name:            "no running vms",
			task:            &Task{},
			client:          getFakeClientWithObjs(createVirtualMachine("vm", "ns1"), createVirtualMachine("vm2", "ns2")),
			expectedVolumes: []string{},
		},
		{
			name: "running all vms",
			task: &Task{},
			client: getFakeClientWithObjs(
				createVirtualMachine("vm", "ns1"),
				createVirtualMachine("vm2", "ns2"),
				createVirtlauncherPod("vm", "ns1", []string{"dv"}),
				createVirtlauncherPod("vm2", "ns2", []string{"dv2"})),
			expectedVolumes: []string{"ns1/dv", "ns2/dv2"},
		},
		{
			name: "running single vms",
			task: &Task{},
			client: getFakeClientWithObjs(
				createVirtualMachine("vm", "ns1"),
				createVirtualMachine("vm2", "ns2"),
				createVirtlauncherPod("vm2", "ns2", []string{"dv2"})),
			expectedVolumes: []string{"ns2/dv2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			volumes, err := tt.task.getRunningVMVolumes([]string{"ns1", "ns2"})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if len(volumes) != len(tt.expectedVolumes) {
				t.Errorf("expected %v, got %v", tt.expectedVolumes, volumes)
				t.FailNow()
			}
			for i, v := range volumes {
				if v != tt.expectedVolumes[i] {
					t.Errorf("expected %v, got %v", tt.expectedVolumes, volumes)
					t.FailNow()
				}
			}
		})
	}
}

func TestTaskStorageLiveMigrateVM(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		client  compat.Client
		volumes *vmVolumes
		wantErr bool
	}{
		{
			name: "no vm, no volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client:  getFakeClientWithObjs(),
			volumes: &vmVolumes{},
			wantErr: true,
		},
		{
			name: "vm, no volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client:  getFakeClientWithObjs(createVirtualMachine("vm", testNamespace)),
			volumes: &vmVolumes{},
			wantErr: false,
		},
		{
			name: "vm, matching volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv",
						},
					},
				},
			}), createDataVolume("dv", testNamespace)),
			volumes: &vmVolumes{
				sourceVolumes: []string{"dv"},
				targetVolumes: []string{targetDv},
			},
			wantErr: false,
		},
		{
			name: "vm, matching volumes, already modified",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: targetDv,
						},
					},
				},
			}), createDataVolume("dv", testNamespace)),
			volumes: &vmVolumes{
				sourceVolumes: []string{"dv"},
				targetVolumes: []string{targetDv},
			},
			wantErr: false,
		},
		{
			name: "vm, non matching volumes",
			task: &Task{
				Owner: &v1alpha1.DirectVolumeMigration{
					Spec: v1alpha1.DirectVolumeMigrationSpec{
						MigrationType: ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback),
					},
				},
			},
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "unmatched",
						},
					},
				},
			}), createDataVolume("dv", testNamespace)),
			volumes: &vmVolumes{
				sourceVolumes: []string{"dv"},
				targetVolumes: []string{targetDv},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.task.Owner.Spec.MigrationType == nil || tt.task.Owner.Spec.MigrationType == ptr.To[v1alpha1.DirectVolumeMigrationType](v1alpha1.MigrationTypeRollback) {
				tt.task.sourceClient = tt.client
			} else {
				tt.task.destinationClient = tt.client
			}
			err := tt.task.storageLiveMigrateVM("vm", testNamespace, tt.volumes)
			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			} else if err == nil && tt.wantErr {
				t.Errorf("expected error, got nil")
				t.FailNow()
			}
		})
	}
}

func TestVirtualMachineMigrationStatus(t *testing.T) {
	tests := []struct {
		name           string
		client         k8sclient.Client
		expectedStatus string
		wantErr        bool
	}{
		{
			name:           "In progress VMIM",
			client:         getFakeClientWithObjs(createInProgressVirtualMachineMigration("vmim", testNamespace, "vm")),
			expectedStatus: fmt.Sprintf("VMI %s not found in namespace %s", "vm", testNamespace),
			wantErr:        true,
		},
		{
			name:           "No VMIM or VMI",
			client:         getFakeClientWithObjs(),
			expectedStatus: fmt.Sprintf("VMI %s not found in namespace %s", "vm", testNamespace),
			wantErr:        true,
		},
		{
			name:           "Failed VMIM with message",
			client:         getFakeClientWithObjs(createFailedVirtualMachineMigration("vmim", testNamespace, "vm", "failed")),
			expectedStatus: "failed",
		},
		{
			name:           "Canceled VMIM",
			client:         getFakeClientWithObjs(createCanceledVirtualMachineMigration("vmim", testNamespace, "vm", virtv1.MigrationAbortSucceeded)),
			expectedStatus: "Migration canceled",
		},
		{
			name:           "Canceled VMIM inprogress",
			client:         getFakeClientWithObjs(createCanceledVirtualMachineMigration("vmim", testNamespace, "vm", virtv1.MigrationAbortInProgress)),
			expectedStatus: "Migration cancel in progress",
		},
		{
			name:           "Canceled VMIM failed",
			client:         getFakeClientWithObjs(createCanceledVirtualMachineMigration("vmim", testNamespace, "vm", virtv1.MigrationAbortFailed)),
			expectedStatus: "Migration canceled failed",
		},
		{
			name:           "Completed VMIM",
			client:         getFakeClientWithObjs(createCompletedVirtualMachineMigration("vmim", testNamespace, "vm")),
			expectedStatus: "",
		},
		{
			name:           "VMI without conditions",
			client:         getFakeClientWithObjs(createVirtualMachineInstance("vm", testNamespace, virtv1.Running)),
			expectedStatus: "",
		},
		{
			name: "VMI with update strategy conditions, and live migrate possible",
			client: getFakeClientWithObjs(createVirtualMachineInstanceWithConditions("vm", testNamespace, []virtv1.VirtualMachineInstanceCondition{
				{
					Type:   virtv1.VirtualMachineInstanceVolumesChange,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   virtv1.VirtualMachineInstanceIsMigratable,
					Status: corev1.ConditionTrue,
				},
			})),
			expectedStatus: "",
		},
		{
			name: "VMI with update strategy conditions, and live migrate not possible",
			client: getFakeClientWithObjs(createVirtualMachineInstanceWithConditions("vm", testNamespace, []virtv1.VirtualMachineInstanceCondition{
				{
					Type:   virtv1.VirtualMachineInstanceVolumesChange,
					Status: corev1.ConditionTrue,
				},
				{
					Type:    virtv1.VirtualMachineInstanceIsMigratable,
					Status:  corev1.ConditionFalse,
					Message: "Migration not possible",
				},
			})),
			expectedStatus: "Migration not possible",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := virtualMachineMigrationStatus(tt.client, "vm", testNamespace, log.WithName(tt.name))
			if err != nil && !tt.wantErr {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if status != tt.expectedStatus {
				t.Errorf("expected %s, got %s", tt.expectedStatus, status)
				t.FailNow()
			}
		})
	}
}

func TestCancelLiveMigration(t *testing.T) {
	tests := []struct {
		name      string
		client    k8sclient.Client
		vmVolumes *vmVolumes
		expectErr bool
	}{
		{
			name:      "no changed volumes",
			client:    getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{})),
			vmVolumes: &vmVolumes{},
		},
		{
			name:      "no virtual machine",
			client:    getFakeClientWithObjs(),
			vmVolumes: &vmVolumes{},
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cancelLiveMigration(tt.client, "vm", testNamespace, tt.vmVolumes, log.WithName(tt.name))
			if err != nil && !tt.expectErr {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
		})
	}
}

func TestLiveMigrationsCompleted(t *testing.T) {
	tests := []struct {
		name           string
		client         k8sclient.Client
		vmNames        []string
		expectComplete bool
	}{
		{
			name:           "no VMIMs",
			client:         getFakeClientWithObjs(),
			vmNames:        []string{"vm1", "vm2"},
			expectComplete: true,
		},
		{
			name:           "all VMIMs completed, but no matching VMs",
			client:         getFakeClientWithObjs(createCompletedVirtualMachineMigration("vmim1", testNamespace, "vm")),
			vmNames:        []string{"vm1", "vm2"},
			expectComplete: true,
		},
		{
			name: "all VMIMs completed, and one matching VM",
			client: getFakeClientWithObjs(
				createCompletedVirtualMachineMigration("vmim1", testNamespace, "vm1"),
				createCompletedVirtualMachineMigration("vmim2", testNamespace, "vm2")),
			vmNames:        []string{"vm1"},
			expectComplete: true,
		},
		{
			name: "not all VMIMs completed, and matching VM",
			client: getFakeClientWithObjs(
				createCompletedVirtualMachineMigration("vmim1", testNamespace, "vm1"),
				createInProgressVirtualMachineMigration("vmim2", testNamespace, "vm2")),
			vmNames:        []string{"vm1"},
			expectComplete: true,
		},
		{
			name: "not all VMIMs completed, and matching VM",
			client: getFakeClientWithObjs(
				createCompletedVirtualMachineMigration("vmim1", testNamespace, "vm1"),
				createInProgressVirtualMachineMigration("vmim2", testNamespace, "vm2")),
			vmNames:        []string{"vm1", "vm2"},
			expectComplete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complete, err := liveMigrationsCompleted(tt.client, testNamespace, tt.vmNames)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if complete != tt.expectComplete {
				t.Errorf("expected %t, got %t", tt.expectComplete, complete)
				t.FailNow()
			}
		})
	}
}

func TestUpdateVM(t *testing.T) {
	tests := []struct {
		name           string
		client         k8sclient.Client
		vmName         string
		sourceVolumes  []string
		targetVolumes  []string
		expectedVM     *virtv1.VirtualMachine
		expectedUpdate bool
	}{
		{
			name:   "name nil vm",
			client: getFakeClientWithObjs(),
			vmName: "",
		},
		{
			name:           "vm without volumes",
			client:         getFakeClientWithObjs(createVirtualMachine("vm", testNamespace)),
			vmName:         "vm",
			expectedVM:     createVirtualMachine("vm", testNamespace),
			expectedUpdate: true,
		},
		{
			name:           "already migrated vm, no update",
			client:         getFakeClientWithObjs(createVirtualMachineWithUpdateStrategy("vm", testNamespace, []virtv1.Volume{})),
			vmName:         "vm",
			expectedVM:     createVirtualMachineWithUpdateStrategy("vm", testNamespace, []virtv1.Volume{}),
			expectedUpdate: false,
		},
		{
			name: "update volumes in VM, no datavolume template",
			client: getFakeClientWithObjs(
				createDataVolume("volume-source", testNamespace),
				createDataVolume("volume-source2", testNamespace),
				createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
					{
						Name: "dv",
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "volume-source",
							},
						},
					},
					{
						Name: "dv2",
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "volume-source2",
							},
						},
					},
				})),
			vmName: "vm",
			expectedVM: createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "volume-target",
						},
					},
				},
				{
					Name: "dv2",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "volume-target2",
						},
					},
				},
			}),
			expectedUpdate: true,
			sourceVolumes:  []string{"volume-source", "volume-source2"},
			targetVolumes:  []string{"volume-target", "volume-target2"},
		},
		{
			name: "update volume in VM, with datavolume template",
			client: getFakeClientWithObjs(
				createDataVolume("volume-source", testNamespace),
				createVirtualMachineWithTemplateAndVolumes("vm", testNamespace, []virtv1.Volume{
					{
						Name: "dv",
						VolumeSource: virtv1.VolumeSource{
							DataVolume: &virtv1.DataVolumeSource{
								Name: "volume-source",
							},
						},
					},
				})),
			vmName: "vm",
			expectedVM: createVirtualMachineWithTemplateAndVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "dv",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "volume-target",
						},
					},
				},
			}),
			expectedUpdate: true,
			sourceVolumes:  []string{"volume-source"},
			targetVolumes:  []string{"volume-target"},
		},
		{
			name: "update persisten volumes in VM, no datavolume template",
			client: getFakeClientWithObjs(
				createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
					{
						Name: "pvc",
						VolumeSource: virtv1.VolumeSource{
							PersistentVolumeClaim: &virtv1.PersistentVolumeClaimVolumeSource{
								PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "volume-source",
								},
							},
						},
					},
				})),
			vmName: "vm",
			expectedVM: createVirtualMachineWithVolumes("vm", testNamespace, []virtv1.Volume{
				{
					Name: "pvc",
					VolumeSource: virtv1.VolumeSource{
						PersistentVolumeClaim: &virtv1.PersistentVolumeClaimVolumeSource{
							PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "volume-target",
							},
						},
					},
				},
			}),
			expectedUpdate: true,
			sourceVolumes:  []string{"volume-source"},
			targetVolumes:  []string{"volume-target"},
		}}
	for _, tt := range tests {
		sourceVM := &virtv1.VirtualMachine{}
		err := tt.client.Get(context.Background(), k8sclient.ObjectKey{Name: "vm", Namespace: testNamespace}, sourceVM)
		if err != nil && !k8serrors.IsNotFound(err) {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}
		t.Run(tt.name, func(t *testing.T) {
			err := updateVM(tt.client, sourceVM, tt.sourceVolumes, tt.targetVolumes, log.WithName(tt.name))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if tt.expectedVM != nil {
				vm := &virtv1.VirtualMachine{}
				err = tt.client.Get(context.Background(), k8sclient.ObjectKey{Name: "vm", Namespace: testNamespace}, vm)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					t.FailNow()
				}
				if vm.Spec.Template != nil && len(tt.expectedVM.Spec.Template.Spec.Volumes) != len(vm.Spec.Template.Spec.Volumes) {
					t.Errorf("expected volumes to be equal")
					t.FailNow()
				} else if vm.Spec.Template != nil {
					for i, v := range vm.Spec.Template.Spec.Volumes {
						if v.VolumeSource.DataVolume != nil {
							if v.VolumeSource.DataVolume.Name != tt.expectedVM.Spec.Template.Spec.Volumes[i].VolumeSource.DataVolume.Name {
								t.Errorf("expected volumes to be equal")
								t.FailNow()
							}
						}
						if v.VolumeSource.PersistentVolumeClaim != nil {
							if v.VolumeSource.PersistentVolumeClaim.ClaimName != tt.expectedVM.Spec.Template.Spec.Volumes[i].VolumeSource.PersistentVolumeClaim.ClaimName {
								t.Errorf("expected volumes to be equal")
								t.FailNow()
							}
						}
					}
					for i, tp := range vm.Spec.DataVolumeTemplates {
						if tp.Name != tt.expectedVM.Spec.DataVolumeTemplates[i].Name {
							t.Errorf("expected data volume templates to be equal")
							t.FailNow()
						}
					}
				}
				if vm.Spec.UpdateVolumesStrategy == nil || *vm.Spec.UpdateVolumesStrategy != virtv1.UpdateVolumesStrategyMigration {
					t.Errorf("expected update volumes strategy to be migration")
					t.FailNow()
				}
				if tt.expectedUpdate {
					newVersion, _ := strconv.Atoi(vm.GetResourceVersion())
					oldVersion, _ := strconv.Atoi(sourceVM.GetResourceVersion())
					if newVersion <= oldVersion {
						t.Errorf("expected resource version to be updated, originalVersion: %s, updatedVersion: %s", sourceVM.GetResourceVersion(), vm.GetResourceVersion())
						t.FailNow()
					}
				} else {
					if vm.GetResourceVersion() != sourceVM.GetResourceVersion() {
						t.Errorf("expected resource version to be the same, originalVersion: %s, updatedVersion: %s", sourceVM.GetResourceVersion(), vm.GetResourceVersion())
						t.FailNow()
					}
				}
			}
		})
	}
}

func TestCreateNewDataVolume(t *testing.T) {
	tests := []struct {
		name          string
		client        k8sclient.Client
		sourceDv      *cdiv1.DataVolume
		expectedDv    *cdiv1.DataVolume
		expectedNewDv bool
	}{
		{
			name:   "create new data volume",
			client: getFakeClientWithObjs(createDataVolume("source-dv", testNamespace)),
			sourceDv: &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "source-dv",
				},
			},
			expectedNewDv: true,
			expectedDv:    createDataVolume("source-dv", testNamespace),
		},
		{
			name:   "don't update existing new data volume",
			client: getFakeClientWithObjs(createDataVolume("source-dv", testNamespace), createDataVolume(targetDv, testNamespace)),
			sourceDv: &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "source-dv",
				},
			},
			expectedNewDv: false,
			expectedDv:    createDataVolume("source-dv", testNamespace),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateNewAdoptionDataVolume(tt.client, tt.sourceDv.Name, targetDv, testNamespace, log.WithName(tt.name))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			dv := &cdiv1.DataVolume{}
			err = tt.client.Get(context.Background(), k8sclient.ObjectKey{Name: targetDv, Namespace: testNamespace}, dv)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if tt.expectedNewDv {
				if dv.GetAnnotations()["cdi.kubevirt.io/allowClaimAdoption"] != "true" {
					t.Errorf("expected allowClaimAdoption annotation to be true")
					t.FailNow()
				}
				if dv.Spec.Source == nil {
					t.Errorf("expected source to be set")
					t.FailNow()
				}
				if dv.Spec.Source.Blank == nil {
					t.Errorf("expected source blank to be set")
					t.FailNow()
				}
			} else {
				if _, ok := dv.GetAnnotations()["cdi.kubevirt.io/allowClaimAdoption"]; ok {
					t.Errorf("expected allowClaimAdoption annotation to not be set")
					t.FailNow()
				}
			}
		})
	}

}

func TestTask_getVolumeVMIMInNamespaces(t *testing.T) {
	tests := []struct {
		name            string
		sourceNamespace string
		task            *Task
		client          compat.Client
		expectedVMIM    map[string]*virtv1.VirtualMachineInstanceMigration
	}{
		{
			name:            "empty volume name, due to no VMs",
			task:            &Task{},
			sourceNamespace: sourceNs,
			client:          getFakeClientWithObjs(),
		},
		{
			name:            "empty volume name, due to no running VMs",
			task:            &Task{},
			sourceNamespace: sourceNs,
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", sourceNs, []virtv1.Volume{
				{
					Name: "data",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv",
						},
					},
				},
			})),
		},
		{
			name:            "empty volume name, due to no migrations",
			task:            &Task{},
			sourceNamespace: sourceNs,
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", sourceNs, []virtv1.Volume{
				{
					Name: "data",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv",
						},
					},
				},
			}), createVirtlauncherPod("vm", sourceNs, []string{"dv"})),
		},
		{
			name:            "running VMIM",
			task:            &Task{},
			sourceNamespace: sourceNs,
			client: getFakeClientWithObjs(createVirtualMachineWithVolumes("vm", sourceNs, []virtv1.Volume{
				{
					Name: "data",
					VolumeSource: virtv1.VolumeSource{
						DataVolume: &virtv1.DataVolumeSource{
							Name: "dv",
						},
					},
				},
			}), createVirtlauncherPod("vm", sourceNs, []string{"dv"}),
				createCompletedVirtualMachineMigration("vmim", sourceNs, "vm")),
			expectedVMIM: map[string]*virtv1.VirtualMachineInstanceMigration{
				"source-ns/dv": createCompletedVirtualMachineMigration("vmim", sourceNs, "vm"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			out, err := tt.task.getVolumeVMIMInNamespaces([]string{tt.sourceNamespace})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			for k, v := range out {
				if tt.expectedVMIM[k] == nil {
					t.Errorf("unexpected VMIM %s", k)
					t.FailNow()
				}
				if tt.expectedVMIM[k] == nil {
					t.Errorf("got unexpected VMIM %s", k)
					t.FailNow()
				}
				if v.Name != tt.expectedVMIM[k].Name {
					t.Errorf("expected %s, got %s", tt.expectedVMIM[k].Name, v.Name)
					t.FailNow()
				}
			}
		})
	}
}

func TestGetVMIMElapsedTime(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name          string
		vmim          *virtv1.VirtualMachineInstanceMigration
		expectedValue metav1.Duration
	}{
		{
			name: "nil VMIM",
			vmim: nil,
			expectedValue: metav1.Duration{
				Duration: 0,
			},
		},
		{
			name: "nil VMIM.Status.MigrationState",
			vmim: &virtv1.VirtualMachineInstanceMigration{
				Status: virtv1.VirtualMachineInstanceMigrationStatus{
					MigrationState: nil,
				},
			},
			expectedValue: metav1.Duration{
				Duration: 0,
			},
		},
		{
			name: "VMIM.Status.MigrationState.StartTimestamp nil, and no replacement from PhaseTransition",
			vmim: &virtv1.VirtualMachineInstanceMigration{
				Status: virtv1.VirtualMachineInstanceMigrationStatus{
					MigrationState: &virtv1.VirtualMachineInstanceMigrationState{
						StartTimestamp: nil,
					},
				},
			},
			expectedValue: metav1.Duration{
				Duration: 0,
			},
		},
		{
			name: "VMIM.Status.MigrationState.StartTimestamp nil, and replacement from PhaseTransition",
			vmim: &virtv1.VirtualMachineInstanceMigration{
				Status: virtv1.VirtualMachineInstanceMigrationStatus{
					MigrationState: &virtv1.VirtualMachineInstanceMigrationState{
						StartTimestamp: nil,
					},
					PhaseTransitionTimestamps: []virtv1.VirtualMachineInstanceMigrationPhaseTransitionTimestamp{
						{
							Phase: virtv1.MigrationRunning,
							PhaseTransitionTimestamp: metav1.Time{
								Time: now.Add(-1 * time.Hour),
							},
						},
					},
				},
			},
			expectedValue: metav1.Duration{
				Duration: time.Hour,
			},
		},
		{
			name: "VMIM.Status.MigrationState.StartTimestamp set, and end timestamp set",
			vmim: &virtv1.VirtualMachineInstanceMigration{
				Status: virtv1.VirtualMachineInstanceMigrationStatus{
					MigrationState: &virtv1.VirtualMachineInstanceMigrationState{
						StartTimestamp: &metav1.Time{
							Time: now.Add(-1 * time.Hour),
						},
						EndTimestamp: &now,
					},
				},
			},
			expectedValue: metav1.Duration{
				Duration: time.Hour,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := getVMIMElapsedTime(tt.vmim)
			// Round to nearest to avoid issues with time.Duration precision
			// we could still get really unlucky and just be on the edge of a minute
			// but it is unlikely
			if out.Round(time.Minute) != tt.expectedValue.Round(time.Minute) {
				t.Errorf("expected %s, got %s", tt.expectedValue, out)
			}
		})
	}
}

func TestTaskGetLastObservedProgressPercent(t *testing.T) {
	tests := []struct {
		name            string
		vmName          string
		sourceNamespace string
		currentProgress map[string]*migapi.LiveMigrationProgress
		task            *Task
		client          compat.Client
		expectedPercent string
	}{
		{
			name: "valid result from query",
			task: &Task{
				PrometheusAPI: prometheusv1.NewAPI(&mockPrometheusClient{
					fakeUrl:       "https://fakeurl",
					expectedQuery: "query=kubevirt_vmi_migration_data_processed_bytes",
					responseBody:  "[0,'59.3']",
				}),
				PromQuery: func(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error) {
					return &model.String{
						Value: "=> 59.3 @",
					}, nil, nil
				},
			},
			expectedPercent: "59%",
		},
		{
			name: "invalid result from query",
			task: &Task{
				PrometheusAPI: prometheusv1.NewAPI(&mockPrometheusClient{
					fakeUrl:       "https://fakeurl",
					expectedQuery: "query=kubevirt_vmi_migration_data_processed_bytes",
					responseBody:  "[0,'59.3']",
				}),
				PromQuery: func(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error) {
					return &model.String{
						Value: "=> x59.3 @",
					}, nil, nil
				},
			},
			expectedPercent: "",
		},
		{
			name: "error result from query",
			task: &Task{
				PrometheusAPI: prometheusv1.NewAPI(&mockPrometheusClient{
					fakeUrl:       "https://fakeurl",
					expectedQuery: "query=kubevirt_vmi_migration_data_processed_bytes",
					responseBody:  "[0,'59.3']",
				}),
				PromQuery: func(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error) {
					return &model.String{
						Value: "=> x59.3 @",
					}, nil, fmt.Errorf("error")
				},
			},
			currentProgress: map[string]*migapi.LiveMigrationProgress{
				"source-ns/vm": &migapi.LiveMigrationProgress{
					LastObservedProgressPercent: "43%",
				},
			},
			vmName:          "vm",
			sourceNamespace: sourceNs,
			expectedPercent: "43%",
		},
		{
			name: "error result from query, no existing progress",
			task: &Task{
				PrometheusAPI: prometheusv1.NewAPI(&mockPrometheusClient{
					fakeUrl:       "https://fakeurl",
					expectedQuery: "query=kubevirt_vmi_migration_data_processed_bytes",
					responseBody:  "[0,'59.3']",
				}),
				PromQuery: func(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error) {
					return &model.String{
						Value: "=> x59.3 @",
					}, nil, fmt.Errorf("error")
				},
			},
			vmName:          "vm",
			sourceNamespace: sourceNs,
			expectedPercent: "",
		},
		{
			name: "warning result from query, no existing progress",
			task: &Task{
				PrometheusAPI: prometheusv1.NewAPI(&mockPrometheusClient{
					fakeUrl:       "https://fakeurl",
					expectedQuery: "query=kubevirt_vmi_migration_data_processed_bytes",
					responseBody:  "[0,'59.3']",
				}),
				PromQuery: func(ctx context.Context, query string, ts time.Time, opts ...prometheusv1.Option) (model.Value, prometheusv1.Warnings, error) {
					return &model.String{
						Value: "=> 21.7 @",
					}, prometheusv1.Warnings{"warning"}, nil
				},
			},
			vmName:          "vm",
			sourceNamespace: sourceNs,
			expectedPercent: "21%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			percent, err := tt.task.getLastObservedProgressPercent(tt.vmName, tt.sourceNamespace, tt.currentProgress)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if percent != tt.expectedPercent {
				t.Errorf("expected %s, got %s", tt.expectedPercent, percent)
			}
		})
	}
}

func TestTaskBuildPrometheusAPI(t *testing.T) {
	tests := []struct {
		name   string
		task   *Task
		client compat.Client
		apiNil bool
	}{
		{
			name: "API already built",
			task: &Task{
				PrometheusAPI: prometheusv1.NewAPI(&mockPrometheusClient{}),
			},
			apiNil: false,
		},
		{
			name: "API not built, should build",
			client: getFakeClientWithObjs(
				createControllerConfigMap(
					"migration-controller",
					migapi.OpenshiftMigrationNamespace,
					map[string]string{
						"source-cluster_PROMETHEUS_URL": "http://prometheus",
					},
				),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
						Spec: migapi.MigClusterSpec{
							IsHostCluster: true,
						},
					},
				},
				restConfig: &rest.Config{},
			},
			apiNil: false,
		},
		{
			name: "API not built, should build, no URL",
			client: getFakeClientWithObjs(
				createControllerConfigMap(
					"migration-controller",
					migapi.OpenshiftMigrationNamespace,
					map[string]string{
						"source-cluster_PROMETHEUS_URL": "",
					},
				),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
						Spec: migapi.MigClusterSpec{
							IsHostCluster: true,
						},
					},
				},
				restConfig: &rest.Config{},
			},
			apiNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			err := tt.task.buildPrometheusAPI()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			if tt.apiNil && tt.task.PrometheusAPI != nil {
				t.Errorf("expected API to be nil")
				t.FailNow()
			}
		})
	}
}

func TestParseProgress(t *testing.T) {
	tests := []struct {
		name          string
		intput        string
		expectedValue string
	}{
		{
			name:          "Empty string",
			intput:        "",
			expectedValue: "",
		},
		{
			name:          "Valid progress",
			intput:        "=> 59.3 @",
			expectedValue: "59",
		},
		{
			name:          "Invalid progress",
			intput:        "=> x59.3 @",
			expectedValue: "",
		},
		{
			name:          "Invalid progress over 100",
			intput:        "=> 1159.3 @",
			expectedValue: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := parseProgress(tt.intput)
			if out != tt.expectedValue {
				t.Errorf("expected %s, got %s", tt.expectedValue, out)
			}
		})
	}
}

func TestTaskBuildSourcePrometheusEndPointURL(t *testing.T) {
	tests := []struct {
		name                 string
		client               compat.Client
		task                 *Task
		expectedError        bool
		expectedErrorMessage string
		expectedValue        string
	}{
		{
			name:                 "No prometheus config map, should return not found error",
			client:               getFakeClientWithObjs(),
			task:                 &Task{},
			expectedError:        true,
			expectedErrorMessage: "not found",
		},
		{
			name: "Prometheus config map exists, but no prometheus url, should return empty url",
			client: getFakeClientWithObjs(
				createControllerConfigMap("migration-controller", migapi.OpenshiftMigrationNamespace, nil),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
					},
				},
			},
			expectedValue: "",
		},
		{
			name: "Prometheus config map exists, with prometheus url, should return correct url",
			client: getFakeClientWithObjs(
				createControllerConfigMap(
					"migration-controller",
					migapi.OpenshiftMigrationNamespace,
					map[string]string{
						"source-cluster_PROMETHEUS_URL": "http://prometheus",
					},
				),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
					},
				},
			},
			expectedValue: "https://prometheus",
		},
		{
			name: "Prometheus config map exists, but no prometheus url, but route, should return route url",
			client: getFakeClientWithObjs(
				createControllerConfigMap("migration-controller", migapi.OpenshiftMigrationNamespace, nil),
				createRoute("prometheus-route", "openshift-monitoring", "https://route.prometheus"),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
					},
				},
			},
			expectedValue: "https://route.prometheus",
		},
		{
			name: "Prometheus config map exists, but no prometheus url, but route, should return route url",
			client: getFakeClientWithObjs(
				createControllerConfigMap("migration-controller", migapi.OpenshiftMigrationNamespace, nil),
				createRoute("prometheus-route", "openshift-monitoring", "http://route.prometheus"),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
					},
				},
			},
			expectedValue: "https://route.prometheus",
		},
		{
			name: "Prometheus config map exists, with invalid prometheus url, should return blank",
			client: getFakeClientWithObjs(
				createControllerConfigMap("migration-controller", migapi.OpenshiftMigrationNamespace,
					map[string]string{
						"source-cluster_PROMETHEUS_URL": "%#$invalid",
					},
				),
			),
			task: &Task{
				PlanResources: &migapi.PlanResources{
					SrcMigCluster: &migapi.MigCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "source-cluster",
						},
					},
				},
			},
			expectedValue: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.sourceClient = tt.client
			value, err := tt.task.buildSourcePrometheusEndPointURL()
			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got nil")
					t.FailNow()
				}
				if !strings.Contains(err.Error(), tt.expectedErrorMessage) {
					t.Errorf("expected error message to contain %s, got %s", tt.expectedErrorMessage, err.Error())
				}
			}
			if value != tt.expectedValue {
				t.Errorf("expected %s, got %s", tt.expectedValue, value)
			}
		})
	}
}

func TestGetStorageClassFromName(t *testing.T) {
	tests := []struct {
		name          string
		client        compat.Client
		expectedError bool
		expectedSc    string
	}{
		{
			name:          "no pvcs, no storage class, should return blank",
			client:        getFakeClientWithObjs(),
			expectedError: false,
			expectedSc:    "",
		},
		{
			name:          "pvcs, with storage class, should return name",
			client:        getFakeClientWithObjs(createPVC(testPVCName, testNamespace)),
			expectedError: false,
			expectedSc:    testStorageClass,
		},
		{
			name:          "pvcs, no storage class, no default storage class return blank",
			client:        getFakeClientWithObjs(createNoStorageClassPVC(testPVCName, testNamespace)),
			expectedError: false,
			expectedSc:    "",
		},
		{
			name:          "pvcs, no storage class, default storage class, should return name",
			client:        getFakeClientWithObjs(createNoStorageClassPVC(testPVCName, testNamespace), createDefaultStorageClass(testDefaultStorageClass)),
			expectedError: false,
			expectedSc:    testDefaultStorageClass,
		},
		{
			name:          "pvcs, no storage class, virt default storage class, should return name",
			client:        getFakeClientWithObjs(createNoStorageClassPVC(testPVCName, testNamespace), createDefaultStorageClass(testDefaultStorageClass), createVirtDefaultStorageClass(testVirtDefaultStorageClass)),
			expectedError: false,
			expectedSc:    testVirtDefaultStorageClass,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := getStorageClassFromName(tt.client, testPVCName, testNamespace)
			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got nil")
					t.FailNow()
				}
			}
			if sc != tt.expectedSc {
				t.Errorf("expected %s, got %s", tt.expectedSc, sc)
			}
		})
	}
}

func getFakeClientWithObjs(obj ...k8sclient.Object) compat.Client {
	client, _ := fakecompat.NewFakeClient(obj...)
	return client
}

func createVirtualMachine(name, namespace string) *virtv1.VirtualMachine {
	return &virtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createCompletedVirtualMachineMigration(name, namespace, vmName string) *virtv1.VirtualMachineInstanceMigration {
	return &virtv1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: virtv1.VirtualMachineInstanceMigrationSpec{
			VMIName: vmName,
		},
		Status: virtv1.VirtualMachineInstanceMigrationStatus{
			Phase: virtv1.MigrationSucceeded,
			MigrationState: &virtv1.VirtualMachineInstanceMigrationState{
				EndTimestamp: &metav1.Time{
					Time: metav1.Now().Add(-1 * time.Hour),
				},
				Completed: true,
			},
		},
	}
}

func createFailedVirtualMachineMigration(name, namespace, vmName, failedMessage string) *virtv1.VirtualMachineInstanceMigration {
	return &virtv1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: virtv1.VirtualMachineInstanceMigrationSpec{
			VMIName: vmName,
		},
		Status: virtv1.VirtualMachineInstanceMigrationStatus{
			Phase: virtv1.MigrationFailed,
			MigrationState: &virtv1.VirtualMachineInstanceMigrationState{
				EndTimestamp: &metav1.Time{
					Time: metav1.Now().Add(-1 * time.Hour),
				},
				FailureReason: failedMessage,
				Failed:        true,
			},
		},
	}
}

func createCanceledVirtualMachineMigration(name, namespace, vmName string, reason virtv1.MigrationAbortStatus) *virtv1.VirtualMachineInstanceMigration {
	return &virtv1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: virtv1.VirtualMachineInstanceMigrationSpec{
			VMIName: vmName,
		},
		Status: virtv1.VirtualMachineInstanceMigrationStatus{
			Phase: virtv1.MigrationFailed,
			MigrationState: &virtv1.VirtualMachineInstanceMigrationState{
				EndTimestamp: &metav1.Time{
					Time: metav1.Now().Add(-1 * time.Hour),
				},
				AbortStatus: reason,
			},
		},
	}
}

func createInProgressVirtualMachineMigration(name, namespace, vmName string) *virtv1.VirtualMachineInstanceMigration {
	return &virtv1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: virtv1.VirtualMachineInstanceMigrationSpec{
			VMIName: vmName,
		},
		Status: virtv1.VirtualMachineInstanceMigrationStatus{
			Phase: virtv1.MigrationRunning,
		},
	}
}

func createVirtualMachineWithVolumes(name, namespace string, volumes []virtv1.Volume) *virtv1.VirtualMachine {
	vm := createVirtualMachine(name, namespace)
	vm.Spec = virtv1.VirtualMachineSpec{
		Template: &virtv1.VirtualMachineInstanceTemplateSpec{
			Spec: virtv1.VirtualMachineInstanceSpec{
				Volumes: volumes,
			},
		},
	}
	return vm
}

func createVirtualMachineWithUpdateStrategy(name, namespace string, volumes []virtv1.Volume) *virtv1.VirtualMachine {
	vm := createVirtualMachineWithVolumes(name, namespace, volumes)
	vm.Spec.UpdateVolumesStrategy = ptr.To[virtv1.UpdateVolumesStrategy](virtv1.UpdateVolumesStrategyMigration)
	return vm
}

func createVirtualMachineWithTemplateAndVolumes(name, namespace string, volumes []virtv1.Volume) *virtv1.VirtualMachine {
	vm := createVirtualMachineWithVolumes(name, namespace, volumes)
	for _, volume := range volumes {
		vm.Spec.DataVolumeTemplates = append(vm.Spec.DataVolumeTemplates, virtv1.DataVolumeTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: volume.DataVolume.Name,
			},
		})
	}
	return vm
}

func createControllerConfigMap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func createRoute(name, namespace, url string) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "openshift-monitoring",
			},
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Name: prometheusRoute,
			},
			Host: url,
		},
	}
}

func createPVC(name, namespace string) *corev1.PersistentVolumeClaim {
	pvc := createNoStorageClassPVC(name, namespace)
	pvc.Spec.StorageClassName = ptr.To(testStorageClass)
	return pvc
}

func createNoStorageClassPVC(name, namespace string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
func createStorageClass(name string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func createDefaultStorageClass(name string) *storagev1.StorageClass {
	sc := createStorageClass(name)
	sc.Annotations = map[string]string{
		defaultK8sStorageClass: "true",
	}
	return sc
}

func createVirtDefaultStorageClass(name string) *storagev1.StorageClass {
	sc := createStorageClass(name)
	sc.Annotations = map[string]string{
		defaultVirtStorageClass: "true",
	}
	return sc
}

type mockPrometheusClient struct {
	fakeUrl       string
	responseBody  string
	expectedQuery string
}

func (m *mockPrometheusClient) URL(ep string, args map[string]string) *url.URL {
	url, err := url.Parse(m.fakeUrl)
	if err != nil {
		panic(err)
	}
	return url
}

func (m *mockPrometheusClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if req.Body != nil {
		defer req.Body.Close()
	}
	b, err := io.ReadAll(req.Body)
	queryBody := string(b)
	if !strings.Contains(queryBody, m.expectedQuery) {
		return nil, nil, fmt.Errorf("expected query %s, got %s", m.expectedQuery, queryBody)
	}
	if err != nil {
		return nil, nil, err
	}

	out := []byte(m.responseBody)

	t := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(bytes.NewBuffer(out)),
		ContentLength: int64(len(out)),
		Request:       req,
		Header:        make(http.Header, 0),
	}
	return t, out, nil
}

func createVirtlauncherPod(vmName, namespace string, dataVolumes []string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("virt-launcher-%s", vmName),
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name: vmName,
					Kind: "VirtualMachineInstance",
				},
			},
		},
		Spec: corev1.PodSpec{},
	}
	for _, dv := range dataVolumes {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: dv,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: dv,
				},
			},
		})
	}
	return pod
}

func createRunningBlockRsyncPod(podName, namespace string, t *Task, dataVolumes []string) *corev1.Pod {
	pod := createBlockRsyncPod(podName, namespace, t, dataVolumes)
	pod.Status.Phase = corev1.PodRunning
	return pod
}

func createCompletedBlockRsyncPod(podName, namespace string, t *Task, dataVolumes []string) *corev1.Pod {
	pod := createBlockRsyncPod(podName, namespace, t, dataVolumes)
	pod.Status.Phase = corev1.PodSucceeded
	return pod
}

func createBlockRsyncPod(podName, namespace string, t *Task, dataVolumes []string) *corev1.Pod {
	blockdvmLabels := t.buildDVMLabels()
	blockdvmLabels["app"] = DirectVolumeMigrationRsyncTransferBlock
	blockdvmLabels["purpose"] = DirectVolumeMigrationRsync
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    blockdvmLabels,
		},
		Spec: corev1.PodSpec{},
	}
	for _, dv := range dataVolumes {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: dv,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: dv,
				},
			},
		})
	}
	return pod
}

func createDataVolume(name, namespace string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createVirtualMachineInstance(name, namespace string, phase virtv1.VirtualMachineInstancePhase) *virtv1.VirtualMachineInstance {
	return &virtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: virtv1.VirtualMachineInstanceStatus{
			Phase: phase,
		},
	}
}

func createVirtualMachineInstanceWithConditions(name, namespace string, conditions []virtv1.VirtualMachineInstanceCondition) *virtv1.VirtualMachineInstance {
	vm := createVirtualMachineInstance(name, namespace, virtv1.Running)
	vm.Status.Conditions = append(vm.Status.Conditions, conditions...)
	return vm
}
