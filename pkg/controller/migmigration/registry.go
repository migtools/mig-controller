package migmigration

import (
	"errors"
	"fmt"

	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Annotations
const (
	MigRegistryAnnotationKey    string = "openshift.io/migration-registry"
	MigRegistryDirAnnotationKey string = "openshift.io/migration-registry-dir"
)

// Returns the right backup/restore annotations including registry-specific ones
func (t *Task) getAnnotations(client k8sclient.Client) (map[string]string, error) {
	annotations := t.Annotations
	registryService, err := t.PlanResources.MigPlan.GetRegistryService(client)
	if err != nil {
		return nil, err
	}
	if registryService == nil {
		return nil, errors.New("migration registry service not found")
	}
	registryDC, err := t.PlanResources.MigPlan.GetRegistryDC(client)
	if err != nil {
		return nil, err
	}
	if registryDC == nil {
		return nil, errors.New("migration registry DeploymentConfig not found")
	}

	if len(registryService.Spec.Ports) == 0 {
		return nil, errors.New("Migration Registry service port not found")
	}
	annotations[MigRegistryAnnotationKey] = fmt.Sprintf("%s:%d", registryService.Spec.ClusterIP,
		registryService.Spec.Ports[0].Port)
	for _, container := range registryDC.Spec.Template.Spec.Containers {
		for _, envVar := range container.Env {
			if envVar.Name == "REGISTRY_STORAGE_S3_ROOTDIRECTORY" {
				annotations[MigRegistryDirAnnotationKey] = envVar.Value
			}
		}
	}
	if t.quiesce() {
		annotations[MigQuiesceAnnotationKey] = "true"
	}
	return annotations, nil
}
