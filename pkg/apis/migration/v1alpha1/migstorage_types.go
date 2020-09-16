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
	"reflect"

	pvdr "github.com/konveyor/mig-controller/pkg/cloudprovider"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Cloud Providers.
const (
	AWS   = pvdr.AWS
	Azure = pvdr.Azure
	GCP   = pvdr.GCP
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
	ObservedDigest string `json:"observedDigest,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MigStorage is the Schema for the migstorages API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="BackupStorageProvider",type=string,JSONPath=".spec.backupStorageProvider"
// +kubebuilder:printcolumn:name="VolumeSnapshotProvider",type=string,JSONPath=".spec.volumeSnapshotProvider"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
	CredsSecretRef          *kapi.ObjectReference `json:"credsSecretRef,omitempty"`
	SnapshotCreationTimeout string                `json:"snapshotCreationTimeout,omitempty"`
	AwsRegion               string                `json:"awsRegion,omitempty"`
	AzureAPITimeout         string                `json:"azureApiTimeout,omitempty"`
	AzureResourceGroup      string                `json:"azureResourceGroup,omitempty"`
}

// BackupStorageConfig defines config for creating and storing Backups
type BackupStorageConfig struct {
	CredsSecretRef        *kapi.ObjectReference `json:"credsSecretRef,omitempty"`
	AwsBucketName         string                `json:"awsBucketName,omitempty"`
	AwsRegion             string                `json:"awsRegion,omitempty"`
	AwsS3ForcePathStyle   bool                  `json:"awsS3ForcePathStyle,omitempty"`
	AwsS3URL              string                `json:"awsS3Url,omitempty"`
	AwsPublicURL          string                `json:"awsPublicUrl,omitempty"`
	AwsKmsKeyID           string                `json:"awsKmsKeyId,omitempty"`
	AwsSignatureVersion   string                `json:"awsSignatureVersion,omitempty"`
	S3CustomCABundle      []byte                `json:"s3CustomCABundle,omitempty"`
	AzureStorageAccount   string                `json:"azureStorageAccount,omitempty"`
	AzureStorageContainer string                `json:"azureStorageContainer,omitempty"`
	AzureResourceGroup    string                `json:"azureResourceGroup,omitempty"`
	GcpBucket             string                `json:"gcpBucket,omitempty"`
	Insecure              bool                  `json:"insecure,omitempty"`
}

func init() {
	SchemeBuilder.Register(&MigStorage{}, &MigStorageList{})
}

// Build BSL.
func (r *MigStorage) BuildBSL() *velero.BackupStorageLocation {
	bsl := &velero.BackupStorageLocation{
		Spec: velero.BackupStorageLocationSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Labels:       r.GetCorrelationLabels(),
			Namespace:    VeleroNamespace,
			GenerateName: r.Name + "-",
		},
	}

	return bsl
}

// Build VSL.
// planUID param is a workaround for velero, needed until velero
// restore accepts a VSL attibute when the name differs from the backup
func (r *MigStorage) BuildVSL(planUID string) *velero.VolumeSnapshotLocation {
	vsl := &velero.VolumeSnapshotLocation{
		Spec: velero.VolumeSnapshotLocationSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    r.GetCorrelationLabels(),
			Namespace: VeleroNamespace,
			Name:      r.Name + "-" + planUID,
		},
	}

	return vsl
}

// Build backup cloud-secret.
func (r *MigStorage) BuildBSLCloudSecret() *kapi.Secret {
	secret := &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    r.GetCorrelationLabels(),
			Namespace: VeleroNamespace,
			Name:      r.GetBackupStorageProvider().GetCloudSecretName(),
		},
	}

	return secret
}

// Build snapshot cloud-secret.
func (r *MigStorage) BuildVSLCloudSecret() *kapi.Secret {
	secret := &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    r.GetCorrelationLabels(),
			Namespace: VeleroNamespace,
			Name:      r.GetVolumeSnapshotProvider().GetCloudSecretName(),
		},
	}

	return secret
}

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

// Determine if two secrets cloud secrets are equal.
// Returns `true` when equal.
func (r *MigStorage) EqualsCloudSecret(a, b *kapi.Secret) bool {
	return reflect.DeepEqual(a.Data, b.Data)
}

// Get the backup storage cloud provider.
func (r *MigStorage) GetBackupStorageProvider() pvdr.Provider {
	var provider pvdr.Provider
	provider = r.Spec.BackupStorageConfig.GetProvider(r.Spec.BackupStorageProvider)
	return provider
}

// Get the volume snapshot cloud provider.
func (r *MigStorage) GetVolumeSnapshotProvider() pvdr.Provider {
	var provider pvdr.Provider
	if r.Spec.VolumeSnapshotProvider != "" {
		provider = r.Spec.VolumeSnapshotConfig.GetProvider(r.Spec.VolumeSnapshotProvider)
	} else {
		provider = r.Spec.BackupStorageConfig.GetProvider(r.Spec.BackupStorageProvider)
	}
	if provider != nil {
		provider.SetRole(pvdr.VolumeSnapshot)
	}

	return provider
}

// Get the backup credentials secret.
func (r *MigStorage) GetBackupStorageCredSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return r.Spec.BackupStorageConfig.GetCredSecret(client)
}

// Get the backup credentials secret.
func (r *MigStorage) GetVolumeSnapshotCredSecret(client k8sclient.Client) (*kapi.Secret, error) {
	if r.Spec.VolumeSnapshotProvider != "" {
		return r.Spec.VolumeSnapshotConfig.GetCredSecret(client)
	} else {
		return r.Spec.BackupStorageConfig.GetCredSecret(client)
	}
}

// Get the cloud provider.
func (r *BackupStorageConfig) GetProvider(name string) pvdr.Provider {
	var provider pvdr.Provider
	switch name {
	case AWS:
		provider = &pvdr.AWSProvider{
			BaseProvider: pvdr.BaseProvider{
				Role: pvdr.BackupStorage,
				Name: name,
			},
			Region:           r.AwsRegion,
			Bucket:           r.AwsBucketName,
			S3URL:            r.AwsS3URL,
			PublicURL:        r.AwsPublicURL,
			KMSKeyId:         r.AwsKmsKeyID,
			SignatureVersion: r.AwsSignatureVersion,
			S3ForcePathStyle: r.AwsS3ForcePathStyle,
			CustomCABundle:   r.S3CustomCABundle,
			Insecure:         r.Insecure,
		}
	case Azure:
		provider = &pvdr.AzureProvider{
			BaseProvider: pvdr.BaseProvider{
				Role: pvdr.BackupStorage,
				Name: name,
			},
			ResourceGroup:    r.AzureResourceGroup,
			StorageAccount:   r.AzureStorageAccount,
			StorageContainer: r.AzureStorageContainer,
		}
	case GCP:
		provider = &pvdr.GCPProvider{
			BaseProvider: pvdr.BaseProvider{
				Role: pvdr.BackupStorage,
				Name: name,
			},
			Bucket: r.GcpBucket,
		}
	}

	return provider
}

// Get credentials secret.
func (r *BackupStorageConfig) GetCredSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, r.CredsSecretRef)
}

// Get the cloud provider.
func (r *VolumeSnapshotConfig) GetProvider(name string) pvdr.Provider {
	var provider pvdr.Provider
	switch name {
	case AWS:
		provider = &pvdr.AWSProvider{
			BaseProvider: pvdr.BaseProvider{
				Role: pvdr.VolumeSnapshot,
				Name: name,
			},
			Region:                  r.AwsRegion,
			SnapshotCreationTimeout: r.SnapshotCreationTimeout,
		}
	case Azure:
		provider = &pvdr.AzureProvider{
			BaseProvider: pvdr.BaseProvider{
				Role: pvdr.VolumeSnapshot,
				Name: name,
			},
			ResourceGroup:           r.AzureResourceGroup,
			APITimeout:              r.AzureAPITimeout,
			SnapshotCreationTimeout: r.SnapshotCreationTimeout,
		}
	case GCP:
		provider = &pvdr.GCPProvider{
			BaseProvider: pvdr.BaseProvider{
				Role: pvdr.VolumeSnapshot,
				Name: name,
			},
			SnapshotCreationTimeout: r.SnapshotCreationTimeout,
		}
	}

	return provider
}

// Get credentials secret.
func (r *VolumeSnapshotConfig) GetCredSecret(client k8sclient.Client) (*kapi.Secret, error) {
	return GetSecret(client, r.CredsSecretRef)
}
