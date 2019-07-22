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
	"fmt"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/pkg/errors"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Cloud Secret Format
const AwsCloudSecretContent = `
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
`

// Cred Secret Fields
const (
	AwsAccessKeyId     = "aws-access-key-id"
	AwsSecretAccessKey = "aws-secret-access-key"
)

// Error
var CredSecretNotFound = errors.New("Cred secret not found.")

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
	AwsS3URL            string                `json:"awsS3Url,omitempty"`
	AwsPublicURL        string                `json:"awsPublicUrl,omitempty"`
	AwsKmsKeyID         string                `json:"awsKmsKeyId,omitempty"`
	AwsSignatureVersion string                `json:"awsSignatureVersion,omitempty"`
	AzureStorageAccount string                `json:"azureStorageAccount,omitempty"`
	AzureResourceGroup  string                `json:"azureResourceGroup,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MigStorage{}, &MigStorageList{})
}

//
// BSL
//

// Determine if two BSLs are equal based on relevant fields in the Spec.
// Returns `true` when equal.
func (r *MigStorage) EqualsBSL(a, b *velero.BackupStorageLocation) bool {
	return a.Spec.Provider == b.Spec.Provider &&
		reflect.DeepEqual(a.Spec.Config, b.Spec.Config) &&
		reflect.DeepEqual(
			a.Spec.ObjectStorage,
			b.Spec.ObjectStorage)
}

// Determine if two VSLs are equal based on relevant fields in the Spec.
// Returns `true` when equal.
func (r *MigStorage) EqualsVSL(a, b *velero.VolumeSnapshotLocation) bool {
	return a.Spec.Provider == b.Spec.Provider &&
		reflect.DeepEqual(a.Spec.Config, b.Spec.Config)
}

// Build a BSL.
func (r *MigStorage) BuildBSL() *velero.BackupStorageLocation {
	location := &velero.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       r.GetCorrelationLabels(),
			Namespace:    VeleroNamespace,
			GenerateName: r.Name + "-",
		},
		Spec: velero.BackupStorageLocationSpec{
			Provider: r.Spec.BackupStorageProvider,
		},
	}
	r.UpdateBSL(location)
	return location
}

// Update a BSL.
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

// Update a BSL for the AWS provider.
func (r *MigStorage) updateAwsBSL(location *velero.BackupStorageLocation) {
	config := r.Spec.BackupStorageConfig
	location.Spec.StorageType = velero.StorageType{
		ObjectStorage: &velero.ObjectStorageLocation{
			Bucket: config.AwsBucketName,
			Prefix: "velero",
		},
	}
	location.Spec.Config = map[string]string{
		"s3ForcePathStyle": fmt.Sprintf("%t", config.AwsS3ForcePathStyle),
		"region":           config.AwsRegion,
	}
	if config.AwsS3URL != "" {
		location.Spec.Config["s3Url"] = config.AwsS3URL
	}
	if config.AwsPublicURL != "" {
		location.Spec.Config["publicUrl"] = config.AwsPublicURL
	}
	if config.AwsKmsKeyID != "" {
		location.Spec.Config["kmsKeyId"] = config.AwsKmsKeyID
	}
	if config.AwsSignatureVersion != "" {
		location.Spec.Config["signatureVersion"] = config.AwsSignatureVersion
	}
}

//
// VSL
//

// Default the VSL settings (when not defined) using the BSL settings.
func (r *MigStorage) DefaultVSLSettings() {
	if r.Spec.VolumeSnapshotProvider != "" {
		return
	}
	r.Spec.VolumeSnapshotProvider = r.Spec.BackupStorageProvider
	r.Spec.VolumeSnapshotConfig.CredsSecretRef = r.Spec.BackupStorageConfig.CredsSecretRef
	switch r.Spec.BackupStorageProvider {
	case "aws":
		r.Spec.VolumeSnapshotConfig.AwsRegion = r.Spec.BackupStorageConfig.AwsRegion
	case "azure":
	case "gcp":
	case "":
	}
}

// Build a VSL.
func (r *MigStorage) BuildVSL() *velero.VolumeSnapshotLocation {
	location := &velero.VolumeSnapshotLocation{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       r.GetCorrelationLabels(),
			Namespace:    VeleroNamespace,
			GenerateName: r.Name + "-",
		},
		Spec: velero.VolumeSnapshotLocationSpec{
			Provider: r.Spec.VolumeSnapshotProvider,
		},
	}
	r.UpdateVSL(location)
	return location
}

// Update a VSL.
func (r *MigStorage) UpdateVSL(location *velero.VolumeSnapshotLocation) {
	location.Spec.Provider = r.Spec.VolumeSnapshotProvider
	switch r.Spec.VolumeSnapshotProvider {
	case "aws":
		r.updateAwsVSL(location)
	case "azure":
	case "gcp":
	case "":
	}
}

// Update a VSL for the AWS provider.
func (r *MigStorage) updateAwsVSL(location *velero.VolumeSnapshotLocation) {
	config := r.Spec.VolumeSnapshotConfig
	location.Spec.Config = map[string]string{
		"region": config.AwsRegion,
	}
}

//
// Cloud-Secret
//

// Determine if two secrets cloud secrets are equal.
// Returns `true` when equal.
func (r *MigStorage) EqualsCloudSecret(a, b *kapi.Secret) bool {
	return reflect.DeepEqual(a.Data, b.Data)
}

// Build the cloud credentials secret.
func (r *MigStorage) BuildCloudSecret(client k8sclient.Client) (*kapi.Secret, error) {
	secret := &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    r.GetCorrelationLabels(),
			Namespace: VeleroNamespace,
			Name:      "cloud-credentials",
		},
	}
	err := r.UpdateCloudSecret(client, secret)
	return secret, err
}

// Update the cloud credentials secret.
func (r *MigStorage) UpdateCloudSecret(client k8sclient.Client, secret *kapi.Secret) error {
	credSecret, err := r.Spec.BackupStorageConfig.GetCredsSecret(client)
	if err != nil {
		return err
	}
	if credSecret == nil {
		return CredSecretNotFound
	}
	secret.Data = map[string][]byte{
		"cloud": []byte(
			fmt.Sprintf(
				AwsCloudSecretContent,
				credSecret.Data[AwsAccessKeyId],
				credSecret.Data[AwsSecretAccessKey]),
		),
	}
	return nil
}

// Get the referenced Cred secret.
func (r *BackupStorageConfig) GetCredsSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, r.CredsSecretRef)
}

// Get the referenced Cred secret.
func (r *VolumeSnapshotConfig) GetCredSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, r.CredsSecretRef)
}
