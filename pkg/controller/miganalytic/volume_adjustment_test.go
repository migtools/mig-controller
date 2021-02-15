/*
Copyright 2021 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package miganalytic

import (
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func getTestPersistentVolume(name string, ns string, podUID string, volName string, requested string, provisioned string) MigAnalyticPersistentVolumeDetails {
	return MigAnalyticPersistentVolumeDetails{
		Name:                name,
		Namespace:           ns,
		PodUID:              types.UID(podUID),
		VolumeName:          volName,
		ProvisionedCapacity: resource.MustParse(provisioned),
		RequestedCapacity:   resource.MustParse(requested),
	}
}

var pvd1 = getTestPersistentVolume("pvc-1", "test-ns", "0000", "pvc-1-vol", "1Mi", "1G")
var pvd2 = getTestPersistentVolume("pvc-2", "test-ns", "0000", "pvc-2-vol", "1G", "1G")
var pvd3 = getTestPersistentVolume("pvc-3", "test-ns", "0000", "pvc-2-vol", "1Ti", "100000M")
var pvd4 = getTestPersistentVolume("pvc-4", "test-ns", "0000", "pvc-2-vol", "1Ki", "22M")
var pvd5 = getTestPersistentVolume("pvc-5", "test-ns", "0000", "pvc-2-vol", "200Ki", "1M")

var testDFStderr = `
df: /host_pods/280f7572-6590-11eb-b436-0a916cc7c396/volumes/kubernetes.io~empty-dir/tmp-volum: No such file or directory
df: /l: No such file or directory
`
var testDFStdout = `
Filesystem     1M-blocks  Used Available Use% Mounted on
tmpfs              7942M    1M     7942M   1% /host_pods/280f7572-6590-11eb-b436-0a916cc7c396/volumes/kubernetes.io~secret/sock-shop-token-pn8n9
shm                  64M    0M       64M   0% /dev/shm
tmpfs              7942M    1M     7942M   1% /credentials
/dev/xvda2        51188M 7613M    43576M  15% /host_pods
`

func TestDFCommand_GetPVUsage(t *testing.T) {
	type fields struct {
		StdOut       string
		StdErr       string
		BlockSize    DFBaseUnit
		BaseLocation string
	}
	type args struct {
		volName string
		podUID  types.UID
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		wantPv DFOutput
	}{
		{
			name: "given a volume that we know exists in Stdout, correct pv usage info should be returned",
			fields: fields{
				BlockSize:    BinarySIMega,
				BaseLocation: "/host_pods",
				StdOut:       testDFStdout,
				StdErr:       testDFStderr,
			},
			args: args{
				volName: "sock-shop-token-pn8n9",
				podUID:  types.UID("280f7572-6590-11eb-b436-0a916cc7c396"),
			},
			wantPv: DFOutput{
				IsError:         false,
				TotalSize:       resource.MustParse("7942Mi"),
				UsagePercentage: 1,
			},
		},
		{
			name: "given a volume that we know exists in Stderr, correct pv usage info should be returned",
			fields: fields{
				BlockSize:    BinarySIMega,
				BaseLocation: "/host_pods",
				StdOut:       testDFStdout,
				StdErr:       testDFStderr,
			},
			args: args{
				volName: "tmp-volum",
				podUID:  types.UID("280f7572-6590-11eb-b436-0a916cc7c396"),
			},
			wantPv: DFOutput{
				IsError: true,
			},
		},
		{
			name: "given a volume that we know doesn't exist anywhere, correct pv usage info should be returned",
			fields: fields{
				BlockSize:    BinarySIMega,
				BaseLocation: "/host_pods",
				StdOut:       testDFStdout,
				StdErr:       testDFStderr,
			},
			args: args{
				volName: "tmp-vo",
				podUID:  types.UID("280f7572-6590-11eb-b436-0a916cc7c396"),
			},
			wantPv: DFOutput{
				IsError: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &DFCommand{
				StdOut:       tt.fields.StdOut,
				StdErr:       tt.fields.StdErr,
				BlockSize:    tt.fields.BlockSize,
				BaseLocation: tt.fields.BaseLocation,
			}
			if gotPv := cmd.GetDFOutputForPV(tt.args.volName, tt.args.podUID); !reflect.DeepEqual(gotPv, tt.wantPv) {
				t.Errorf("DFCommand.GetPVUsage() = %v, want %v", gotPv, tt.wantPv)
			}
		})
	}
}

type testQuantity struct {
	dfQuantity  string
	k8sQuantity string
}

var testQuantities = []testQuantity{
	{
		dfQuantity:  "1090MB",
		k8sQuantity: "1090M",
	},
	{
		dfQuantity:  "1000GB",
		k8sQuantity: "1T",
	},
	{
		dfQuantity:  "1048576G",
		k8sQuantity: "1Pi",
	},
}

func TestDFCommand_convertDFQuantityToKubernetesResource(t *testing.T) {
	type fields struct {
		StdOut       string
		StdErr       string
		ExitCode     int
		BlockSize    DFBaseUnit
		BaseLocation string
	}
	type args struct {
		quantity string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "given a valid decimal SI quantity, should return valid k8s resource.Quantity",
			fields: fields{
				BlockSize: DecimalSIMega,
			},
			args: args{
				quantity: testQuantities[0].dfQuantity,
			},
			want:    testQuantities[0].k8sQuantity,
			wantErr: false,
		},
		{
			name: "given a valid decimal SI quantity, should return valid k8s resource.Quantity",
			fields: fields{
				BlockSize: DecimalSIGiga,
			},
			args: args{
				quantity: testQuantities[1].dfQuantity,
			},
			want:    testQuantities[1].k8sQuantity,
			wantErr: false,
		},
		{
			name: "given a valid binary SI quantity, should return valid k8s resource.Quantity",
			fields: fields{
				BlockSize: BinarySIGiga,
			},
			args: args{
				quantity: testQuantities[2].dfQuantity,
			},
			want:    testQuantities[2].k8sQuantity,
			wantErr: false,
		},
		{
			name: "given an invalid pair of SI quantity and block size unit, should return error",
			fields: fields{
				BlockSize: DecimalSIGiga,
			},
			args: args{
				quantity: testQuantities[0].dfQuantity,
			},
			want:    "0",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &DFCommand{
				StdOut:       tt.fields.StdOut,
				StdErr:       tt.fields.StdErr,
				BlockSize:    tt.fields.BlockSize,
				BaseLocation: tt.fields.BaseLocation,
			}
			got, err := cmd.convertDFQuantityToKubernetesResource(tt.args.quantity)
			if (err != nil) != tt.wantErr {
				t.Errorf("DFCommand.convertDFQuantityToKubernetesResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.String(), tt.want) {
				t.Errorf("DFCommand.convertDFQuantityToKubernetesResource() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

func TestPersistentVolumeAdjuster_calculateProposedVolumeSize(t *testing.T) {
	type fields struct {
		Owner      *migapi.MigAnalytic
		Client     client.Client
		DFExecutor DFCommandExecutor
	}
	type args struct {
		usagePercentage   int64
		actualCapacity    resource.Quantity
		requestedCapacity resource.Quantity
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantProposedSize resource.Quantity
		wantReason       string
	}{
		{
			name: "Given values of usagePercentage, actualCapacity and requestedCapacity, appropriate volume size is returned, here proposed size = actual capacity",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				usagePercentage:   50,
				actualCapacity:    resource.MustParse("200"),
				requestedCapacity: resource.MustParse("20"),
			},
			wantProposedSize: resource.MustParse("200"),
			wantReason:       VolumeAdjustmentCapacityMismatch,
		},
		{
			name: "Given values of usagePercentage, actualCapacity and requestedCapacity, appropriate proposed volume size is returned, here proposed size = volume with threshold size",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				usagePercentage:   100,
				actualCapacity:    resource.MustParse("200"),
				requestedCapacity: resource.MustParse("20"),
			},
			wantProposedSize: resource.MustParse("206"),
			wantReason:       VolumeAdjustmentUsageExceeded,
		},
		{
			name: "Given values of usagePercentage, actualCapacity and requestedCapacity, appropriate proposed volume size is returned, here proposed size = requested capacity",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				usagePercentage:   100,
				actualCapacity:    resource.MustParse("200"),
				requestedCapacity: resource.MustParse("250"),
			},
			wantProposedSize: resource.MustParse("250"),
			wantReason:       VolumeAdjustmentNoOp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pva := &PersistentVolumeAdjuster{
				Owner:      tt.fields.Owner,
				Client:     tt.fields.Client,
				DFExecutor: tt.fields.DFExecutor,
			}
			gotProposedSize, gotReason := pva.calculateProposedVolumeSize(tt.args.usagePercentage, tt.args.actualCapacity, tt.args.requestedCapacity)
			if !reflect.DeepEqual(gotProposedSize, tt.wantProposedSize) {
				t.Errorf("calculateProposedVolumeSize() gotProposedSize = %v, want %v", gotProposedSize, tt.wantProposedSize)
			}
			if gotReason != tt.wantReason {
				t.Errorf("calculateProposedVolumeSize() gotReason = %v, want %v", gotReason, tt.wantReason)
			}
		})
	}
}
