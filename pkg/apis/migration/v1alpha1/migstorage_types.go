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
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MigStorageSpec defines the desired state of MigStorage
type MigStorageSpec struct {
	BackupStorageProvider  string `json:"backupStorageProvider"`
	BackupStorageConfig    `json:"backupStorageConfig"`
	VolumeSnapshotProvider string `json:"volumeSnapshotProvider,omitempty"`
	VolumeSnapshotConfig   `json:"volumeSnapshotConfig,omitempty"`
}

// MigStorageStatus defines the observed state of MigStorage
type MigStorageStatus struct {
	Conditions
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigStorage is the Schema for the migstorages API
// +k8s:openapi-gen=true
type MigStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigStorageSpec   `json:"spec,omitempty"`
	Status MigStorageStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigStorageList contains a list of MigStorage
type MigStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigStorage `json:"items"`
}

// VolumeSnapshotConfig defines config for taking Volume Snapshots
type VolumeSnapshotConfig struct {
	CredsSecretRef     *kapi.ObjectReference `json:"credsSecretRef,omitempty"`
	AwsRegion          string                `json:"awsRegion,omitempty"`
	AzureAPITimeout    string                `json:"azureApiTimeout,omitempty"`
	AzureResourceGroup string                `json:"azureResourceGroup,omitempty"`
}

// BackupStorageConfig defines config for creating and storing Backups
type BackupStorageConfig struct {
	CredsSecretRef      *kapi.ObjectReference `json:"credsSecretRef,omitempty"`
	AwsBucketName       string                `json:"awsBucketName,omitempty"`
	AwsRegion           string                `json:"awsRegion,omitempty"`
	AwsS3ForcePathStyle bool                  `json:"awsS3ForcePathStyle,omitempty"`
	AwsPublicURL        string                `json:"awsPublicUrl,omitempty"`
	AwsKmsKeyID         string                `json:"awsKmsKeyId,omitempty"`
	AwsSignatureVersion string                `json:"awsSignatureVersion,omitempty"`
	AzureStorageAccount string                `json:"azureStorageAccount,omitempty"`
	AzureResourceGroup  string                `json:"azureResourceGroup,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MigStorage{}, &MigStorageList{})
}

// Determine if two BSLs are equal based on relevant fields in the Spec.
// Returns `true` when equal.
func (r *MigStorage) Equals(a, b *velero.BackupStorageLocation) bool {
	return a.Spec.Provider == b.Spec.Provider &&
		reflect.DeepEqual(a.Spec.Config, b.Spec.Config) &&
		reflect.DeepEqual(
			a.Spec.ObjectStorage,
			b.Spec.ObjectStorage)
}

// Build a velero backup storage location.
func (r *MigStorage) BuildBSL() *velero.BackupStorageLocation {
	location := &velero.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       CorrelationLabels(r, r.Namespace, r.Name),
			Namespace:    "velero",
			GenerateName: r.Name + "-",
		},
		Spec: velero.BackupStorageLocationSpec{
			Provider: r.Spec.BackupStorageProvider,
		},
	}
	r.UpdateBSL(location)
	return location
}

// Update a velero backup storage location.
func (r *MigStorage) UpdateBSL(location *velero.BackupStorageLocation) {
	location.Spec.Provider = r.Spec.BackupStorageProvider
	switch r.Spec.BackupStorageProvider {
	case "aws":
		r.updateAwsBSL(location)
	case "azure":
	case "gcp":
	case "":
	}
}

// Update a velero backup storage location for the AWS provider.
func (r *MigStorage) updateAwsBSL(location *velero.BackupStorageLocation) {
	config := r.Spec.BackupStorageConfig
	location.Spec.StorageType = velero.StorageType{
		ObjectStorage: &velero.ObjectStorageLocation{
			Bucket: config.AwsBucketName,
		},
	}
	location.Spec.Config = map[string]string{
		"region": config.AwsRegion,
	}
}

// Get existing backup-storage-location by Label search.
// Returns `nil` when not found.
func (r *MigStorage) GetBSL(client k8sclient.Client) (*velero.BackupStorageLocation, error) {
	list := velero.BackupStorageLocationList{}
	labels := CorrelationLabels(r, r.Namespace, r.Name)
	err := client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}
