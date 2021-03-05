package migmigration

import (
	"errors"
	"fmt"

	liberr "github.com/konveyor/controller/pkg/error"

	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"context"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Annotations
const (
	MigRegistryAnnotationKey    string = "openshift.io/migration-registry"
	MigRegistryDirAnnotationKey string = "openshift.io/migration-registry-dir"
)

// Returns the right backup/restore annotations including registry-specific ones
func (t *Task) getAnnotations(client k8sclient.Client) (map[string]string, error) {
	annotations := t.Annotations
	hasImageStreams, err := t.hasImageStreams()
	if err != nil {
		return nil, err
	}
	if t.PlanResources.MigPlan.Spec.IndirectImageMigration && !t.PlanResources.MigPlan.IsImageMigrationDisabled() && hasImageStreams {
		registryService, err := t.PlanResources.MigPlan.GetRegistryService(client)
		if err != nil {
			return nil, err
		}
		if registryService == nil {
			return nil, liberr.Wrap(errors.New("migration registry service not found"))
		}
		registryDC, err := t.PlanResources.MigPlan.GetRegistryDeployment(client)
		if err != nil {
			return nil, err
		}
		if registryDC == nil {
			return nil, liberr.Wrap(errors.New("migration registry DeploymentConfig not found"))
		}

		if registryDC.DeletionTimestamp != nil {
			return nil, liberr.Wrap(errors.New(fmt.Sprintf("Deployment %s/%s is being garbage collected with deletion timestamp %s", registryDC.Namespace, registryDC.Name, registryDC.DeletionTimestamp)))
		}

		if len(registryService.Spec.Ports) == 0 {
			return nil, liberr.Wrap(errors.New("Migration Registry service port not found"))
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
	}
	if t.quiesce() {
		annotations[QuiesceAnnotation] = "true"
	}
	return annotations, nil
}

// Ensure the migration registries on both source and dest clusters have been created
func (t *Task) ensureMigRegistries() (int, error) {
	nEnsured := 0

	plan := t.PlanResources.MigPlan
	if plan == nil {
		return nEnsured, nil
	}
	storage := t.PlanResources.MigStorage
	if storage == nil {
		return nEnsured, nil
	}

	clusters := t.getBothClusters()

	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err := cluster.GetClient(t.Client)
		if err != nil {
			return nEnsured, liberr.Wrap(err)
		}

		// Migration Registry Secret
		secret, err := t.ensureRegistrySecret(client)
		if err != nil {
			return nEnsured, liberr.Wrap(err)
		}

		// Get cluster specific registry image
		registryImage, err := cluster.GetRegistryImage(client)
		if err != nil {
			return nEnsured, liberr.Wrap(err)
		}

		// Migration Registry DeploymentConfig
		err = t.ensureRegistryDeployment(client, secret, registryImage)
		if err != nil {
			return nEnsured, liberr.Wrap(err)
		}

		// Migration Registry Service
		err = t.ensureRegistryService(client, secret)
		if err != nil {
			return nEnsured, liberr.Wrap(err)
		}

		nEnsured++
	}

	return nEnsured, nil
}

// Ensure the credentials secret for the migration registry on the specified cluster has been created
// If the secret is updated, it will return delete the imageregistry resources
func (t *Task) ensureRegistrySecret(client k8sclient.Client) (*kapi.Secret, error) {
	plan := t.PlanResources.MigPlan
	if plan == nil {
		return nil, nil
	}
	storage := t.PlanResources.MigStorage
	if storage == nil {
		return nil, nil
	}

	newSecret, err := plan.BuildRegistrySecret(t.Client, storage)
	if err != nil {
		return nil, err
	}
	foundSecret, err := plan.GetRegistrySecret(client)
	if err != nil {
		return nil, err
	}
	if foundSecret == nil {
		// if for some reason secret was deleted, we need to make sure we redeploy
		deleteErr := t.deleteImageRegistryDeploymentForClient(client, plan)
		if deleteErr != nil {
			return nil, liberr.Wrap(deleteErr)
		}
		err = client.Create(context.TODO(), newSecret)
		if err != nil {
			return nil, err
		}
		return newSecret, nil
	}
	if plan.EqualsRegistrySecret(newSecret, foundSecret) {
		return foundSecret, nil
	}
	// secret is not same, we need to redeploy
	deleteErr := t.deleteImageRegistryDeploymentForClient(client, plan)
	if deleteErr != nil {
		return nil, liberr.Wrap(deleteErr)
	}
	err = plan.UpdateRegistrySecret(t.Client, storage, foundSecret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	err = client.Update(context.TODO(), foundSecret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return foundSecret, nil
}

func (t *Task) deleteImageRegistryResourcesForClient(client k8sclient.Client, plan *migapi.MigPlan) error {
	t.Owner.Status.Conditions.DeleteCondition(RegistriesHealthy)
	secret, err := plan.GetRegistrySecret(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if secret != nil {
		err := client.Delete(context.Background(), secret)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	err = t.deleteImageRegistryDeploymentForClient(client, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	foundService, err := plan.GetRegistryService(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundService != nil {
		err := client.Delete(context.Background(), foundService)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

func (t *Task) deleteImageRegistryDeploymentForClient(client k8sclient.Client, plan *migapi.MigPlan) error {
	t.Owner.Status.Conditions.DeleteCondition(RegistriesHealthy)
	foundDeployment, err := plan.GetRegistryDeployment(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundDeployment != nil {
		err := client.Delete(context.Background(), foundDeployment, k8sclient.PropagationPolicy(metav1.DeletePropagationForeground))
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

// Ensure the deployment for the migration registry on the specified cluster has been created
func (t *Task) ensureRegistryDeployment(client k8sclient.Client, secret *kapi.Secret, registryImage string) error {
	plan := t.PlanResources.MigPlan
	if plan == nil {
		return nil
	}
	storage := t.PlanResources.MigStorage
	if plan == nil {
		return nil
	}

	name := secret.GetName()
	dirName := storage.GetName() + "-registry-" + string(storage.UID)

	// Get Proxy Env Vars for DC
	proxySecret, err := plan.GetProxySecret(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Construct Registry DC
	newDeployment := plan.BuildRegistryDeployment(storage, proxySecret, name, dirName, registryImage)
	foundDeployment, err := plan.GetRegistryDeployment(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundDeployment == nil {
		err = client.Create(context.TODO(), newDeployment)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	}
	if plan.EqualsRegistryDeployment(newDeployment, foundDeployment) {
		return nil
	}
	plan.UpdateRegistryDeployment(storage, foundDeployment, proxySecret, name, dirName, registryImage)
	err = client.Update(context.TODO(), foundDeployment)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Ensure the service for the migration registry on the specified cluster has been created
func (t *Task) ensureRegistryService(client k8sclient.Client, secret *kapi.Secret) error {
	plan := t.PlanResources.MigPlan
	if plan == nil {
		return nil
	}
	name := secret.GetName()
	newService := plan.BuildRegistryService(name)
	foundService, err := plan.GetRegistryService(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundService == nil {
		err = client.Create(context.TODO(), newService)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	}
	if plan.EqualsRegistryService(newService, foundService) {
		return nil
	}
	plan.UpdateRegistryService(foundService, name)
	// need to delete and update because the version of controller-runtime
	// client we are using does not support Patch yet.
	err = client.Delete(context.Background(), foundService)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = client.Create(context.Background(), newService)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

func (t *Task) deleteImageRegistryResources() error {
	t.Owner.Status.Conditions.DeleteCondition(RegistriesHealthy)
	t.Owner.Status.Conditions.DeleteCondition(RegistriesUnhealthy)

	plan := t.PlanResources.MigPlan
	if plan == nil {
		return nil
	}
	clients, err := t.getBothClients()
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, client := range clients {
		err := t.deleteImageRegistryResourcesForClient(client, plan)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}
