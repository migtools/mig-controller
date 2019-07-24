package cloudprovider

import (
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	kapi "k8s.io/api/core/v1"
)

// Providers.
const (
	AWS   = "aws"
	Azure = "azure"
	GCP   = "gcp"
)

// Roles
const (
	BackupStorage  = "BackupStorage"
	VolumeSnapshot = "VolumeSnapshot"
)

type Provider interface {
	SetRole(role string)
	UpdateBSL(location *velero.BackupStorageLocation)
	UpdateVSL(location *velero.VolumeSnapshotLocation)
	UpdateCloudSecret(secret, cloudSecret *kapi.Secret)
	Validate(secret *kapi.Secret) []string
	Test(secret *kapi.Secret) error
}

type BaseProvider struct {
	Role string
}

func (p *BaseProvider) SetRole(role string) {
	p.Role = role
}
