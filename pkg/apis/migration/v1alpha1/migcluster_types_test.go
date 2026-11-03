/*
Copyright 2019 Red Hat Inc.

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

package v1alpha1

import (
	"context"
	"reflect"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStorageMigCluster(t *testing.T) {
	key := types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}
	created := &MigCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MigCluster",
			APIVersion: "migration.openshift.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &MigCluster{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), key, fetched)).To(gomega.HaveOccurred())
}

var getClusterConfigMapWithData = func(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterConfigMapName,
			Namespace: VeleroNamespace,
		},
		Data: data,
	}
}

func TestMigCluster_GetRegistryReadinessTimeout(t *testing.T) {
	type args struct {
		c k8sclient.Client
	}
	tests := []struct {
		name    string
		args    args
		want    int32
		wantErr bool
	}{
		{
			name: "When no registry timeout value is defined, should return default value",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						"fake-value": "0",
					},
				)),
			},
			want:    RegistryDefaultProbeTimeout,
			wantErr: false,
		},
		{
			name: "When there is error finding the configmap, should return error",
			args: args{
				c: fake.NewFakeClient(),
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "When a +ve integer timeout value is set, should return that value",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						RegistryReadinessProbeTimeout: "4",
					},
				)),
			},
			want:    4,
			wantErr: false,
		},
		{
			name: "When a non +ve integer timeout value is set, should return error",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						RegistryReadinessProbeTimeout: "-10",
					},
				)),
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "When a non integer timeout value is set, should return error",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						RegistryReadinessProbeTimeout: "ab",
					},
				)),
			},
			want:    -1,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MigCluster{}
			got, err := m.GetRegistryReadinessTimeout(tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("MigCluster.GetRegistryReadinessTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MigCluster.GetRegistryReadinessTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMigCluster_GetRegistryLivenessTimeout(t *testing.T) {
	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
		Spec       MigClusterSpec
		Status     MigClusterStatus
	}
	type args struct {
		c k8sclient.Client
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int32
		wantErr bool
	}{
		{
			name: "When no registry timeout value is defined, should return default value",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						"fake-value": "0",
					},
				)),
			},
			want:    RegistryDefaultProbeTimeout,
			wantErr: false,
		},
		{
			name: "When there is error finding the configmap, should return error",
			args: args{
				c: fake.NewFakeClient(),
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "When a +ve integer timeout value is set, should return that value",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						RegistryLivenessProbeTimeout: "4",
					},
				)),
			},
			want:    4,
			wantErr: false,
		},
		{
			name: "When a non +ve integer timeout value is set, should return error",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						RegistryLivenessProbeTimeout: "-10",
					},
				)),
			},
			want:    -1,
			wantErr: true,
		},
		{
			name: "When a non integer timeout value is set, should return error",
			args: args{
				c: fake.NewFakeClient(getClusterConfigMapWithData(
					map[string]string{
						RegistryLivenessProbeTimeout: "ab",
					},
				)),
			},
			want:    -1,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MigCluster{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			got, err := m.GetRegistryLivenessTimeout(tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("MigCluster.GetRegistryLivenessTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MigCluster.GetRegistryLivenessTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMigCluster_accessModesForProvisioner(t *testing.T) {
	m := &MigCluster{}
	tests := []struct {
		name        string
		provisioner string
		volumeMode  kapi.PersistentVolumeMode
		want        []kapi.PersistentVolumeAccessMode
	}{
		{
			name:        "csi.trident.netapp.io Filesystem should return RWO and ROX",
			provisioner: "csi.trident.netapp.io",
			volumeMode:  kapi.PersistentVolumeFilesystem,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany},
		},
		{
			name:        "csi.trident.netapp.io Block should return RWO, ROX, and RWX",
			provisioner: "csi.trident.netapp.io",
			volumeMode:  kapi.PersistentVolumeBlock,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
		},
		{
			name:        "legacy netapp.io/trident Filesystem should return RWO and ROX",
			provisioner: "netapp.io/trident",
			volumeMode:  kapi.PersistentVolumeFilesystem,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany},
		},
		{
			name:        "legacy netapp.io/trident Block should return RWO, ROX, and RWX",
			provisioner: "netapp.io/trident",
			volumeMode:  kapi.PersistentVolumeBlock,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
		},
		{
			name:        "CSI and legacy Trident should return identical Filesystem modes",
			provisioner: "csi.trident.netapp.io",
			volumeMode:  kapi.PersistentVolumeFilesystem,
			want:        m.accessModesForProvisioner("netapp.io/trident", kapi.PersistentVolumeFilesystem),
		},
		{
			name:        "CSI and legacy Trident should return identical Block modes",
			provisioner: "csi.trident.netapp.io",
			volumeMode:  kapi.PersistentVolumeBlock,
			want:        m.accessModesForProvisioner("netapp.io/trident", kapi.PersistentVolumeBlock),
		},
		{
			name:        "rbd.csi.ceph.com suffix match for Filesystem should return RWO",
			provisioner: "openshift-storage.rbd.csi.ceph.com",
			volumeMode:  kapi.PersistentVolumeFilesystem,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
		},
		{
			name:        "rbd.csi.ceph.com suffix match for Block should return RWO, ROX, and RWX",
			provisioner: "openshift-storage.rbd.csi.ceph.com",
			volumeMode:  kapi.PersistentVolumeBlock,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
		},
		{
			name:        "unknown provisioner Filesystem should fall back to RWO",
			provisioner: "example.com/unknown-driver",
			volumeMode:  kapi.PersistentVolumeFilesystem,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
		},
		{
			name:        "unknown provisioner Block should fall back to nil",
			provisioner: "example.com/unknown-driver",
			volumeMode:  kapi.PersistentVolumeBlock,
			want:        nil,
		},
		{
			name:        "kubernetes.io/aws-ebs Filesystem should return RWO",
			provisioner: "kubernetes.io/aws-ebs",
			volumeMode:  kapi.PersistentVolumeFilesystem,
			want:        []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
		},
		{
			name:        "kubernetes.io/aws-ebs Block should return nil (not in map)",
			provisioner: "kubernetes.io/aws-ebs",
			volumeMode:  kapi.PersistentVolumeBlock,
			want:        nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.accessModesForProvisioner(tt.provisioner, tt.volumeMode)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accessModesForProvisioner(%q, %q) = %v, want %v",
					tt.provisioner, tt.volumeMode, got, tt.want)
			}
		})
	}
}
