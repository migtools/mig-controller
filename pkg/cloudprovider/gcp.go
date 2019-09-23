package cloudprovider

import (
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	appsv1 "github.com/openshift/api/apps/v1"
	kapi "k8s.io/api/core/v1"
)

type GCPProvider struct {
	BaseProvider
}

func (p *GCPProvider) UpdateBSL(bsl *velero.BackupStorageLocation) {
	bsl.Spec.Provider = GCP
}

func (p *GCPProvider) UpdateVSL(vsl *velero.VolumeSnapshotLocation) {
	vsl.Spec.Provider = GCP
}

func (p *GCPProvider) UpdateCloudSecret(secret, cloudSecret *kapi.Secret) {
}

func (p *GCPProvider) UpdateRegistrySecret(secret, registrySecret *kapi.Secret) {
}

func (p *GCPProvider) UpdateRegistryDC(deploymentconfig *appsv1.DeploymentConfig, name, dirName string) {
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
