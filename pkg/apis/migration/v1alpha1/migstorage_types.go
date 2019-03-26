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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MigStorageSpec defines the desired state of MigStorage
type MigStorageSpec struct {
	VolumeSnapshotProvider string `json:"volumeSnapshotProvider"`
	VolumeSnapshotConfig   `json:"volumeSnapshotConfig"`

	BackupStorageProvider string `json:"backupStorageProvider"`
	BackupStorageConfig   `json:"backupStorageConfig"`
}

// MigStorageStatus defines the observed state of MigStorage
type MigStorageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
	AwsRegion          string `json:"awsRegion"`
	AzureAPITimeout    string `json:"azureApiTimeout"`
	AzureResourceGroup string `json:"azureResourceGroup"`
}

// BackupStorageConfig defines config for creating and storing Backups
type BackupStorageConfig struct {
	AwsBucketName       string `json:"awsBucketName"`
	AwsRegion           string `json:"awsRegion"`
	AwsS3ForcePathStyle bool   `json:"awsS3ForcePathStyle"`
	AwsPublicURL        string `json:"awsPublicUrl"`
	AwsKmsKeyID         string `json:"awsKmsKeyId"`
	AwsSignatureVersion string `json:"awsSignatureVersion"`
	AzureStorageAccount string `json:"azureStorageAccount"`
	AzureResourceGroup  string `json:"azureResourceGroup"`
}

func init() {
	SchemeBuilder.Register(&MigStorage{}, &MigStorageList{})
}
