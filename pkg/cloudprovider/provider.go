package cloudprovider

import (
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	appsv1 "github.com/openshift/api/apps/v1"
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
	UpdateRegistrySecret(secret, registrySecret *kapi.Secret) error
	UpdateRegistryDC(dc *appsv1.DeploymentConfig, name, dirName string)
	Validate(secret *kapi.Secret) []string
	Test(secret *kapi.Secret) error
}

type BaseProvider struct {
	Role string
}

func (p *BaseProvider) SetRole(role string) {
	p.Role = role
}
