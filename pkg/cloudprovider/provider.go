package cloudprovider

import (
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	appsv1 "k8s.io/api/apps/v1"
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
	GetName() string
	GetCloudSecretName() string
	GetCloudCredentialsPath() string
	SetRole(role string)
	UpdateBSL(location *velero.BackupStorageLocation)
	UpdateVSL(location *velero.VolumeSnapshotLocation)
	UpdateCloudSecret(secret, cloudSecret *kapi.Secret) error
	UpdateRegistrySecret(secret, registrySecret *kapi.Secret) error
	UpdateRegistryDeployment(deployment *appsv1.Deployment, name, dirName string)
	Validate(secret *kapi.Secret) []string
	Test(secret *kapi.Secret) error
}

type BaseProvider struct {
	Name string
	Role string
}

func (p *BaseProvider) SetRole(role string) {
	p.Role = role
}

func (p *BaseProvider) GetName() string {
	return p.Name
}
