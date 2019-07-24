package cloudprovider

import (
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
)

type GCPProvider struct {
	BaseProvider
}

func (p *GCPProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = Azure
}

func (p *GCPProvider) UpdateVSL(vsl *velero.VolumeSnapshotLocation) {
	vsl.Spec.Provider = GCP
}

func (p *GCPProvider) UpdateCloudSecret(secret, cloudSecret *kapi.Secret) {
}

func (p *GCPProvider) Validate(secret *kapi.Secret) []string {
	fields := []string{}

	switch p.Role {
	case BackupStorage:
	case VolumeSnapshot:
	}

	return fields
}

func (p *GCPProvider) Test(secret *kapi.Secret) error {
	if secret == nil {
		return nil
	}

	return nil
}
