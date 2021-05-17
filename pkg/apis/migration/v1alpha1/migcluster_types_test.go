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
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
