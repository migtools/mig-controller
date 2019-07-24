package cloudprovider

import (
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
)

type AzureProvider struct {
	BaseProvider
	StorageAccount string
	ResourceGroup  string
	APITimeout     string
}

func (p *AzureProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = Azure
}

func (p *AzureProvider) UpdateVSL(vsl *velero.VolumeSnapshotLocation) {
	vsl.Spec.Provider = Azure
}

func (p *AzureProvider) UpdateCloudSecret(ecret, cloudSecret *kapi.Secret) {
}

func (p *AzureProvider) Validate(secret *kapi.Secret) []string {
	fields := []string{}

	if p.ResourceGroup == "" {
		fields = append(fields, "ResourceGroup")
	}

	switch p.Role {
	case BackupStorage:
		if p.StorageAccount == "" {
			fields = append(fields, "StorageAccount")
		}
	case VolumeSnapshot:
		if p.APITimeout == "" {
			fields = append(fields, "APITimeout")
		}
	}

	return fields
}

func (p *AzureProvider) Test(secret *kapi.Secret) error {
	if secret == nil {
		return nil
	}

	return nil
}
