package migplan

import (
	"context"
	"fmt"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	kapi "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the migration registries on both source and dest clusters have been created
func (r ReconcileMigPlan) ensureMigRegistries(plan *migapi.MigPlan) error {
	var client k8sclient.Client
	nEnsured := 0

	if plan.Status.HasCriticalCondition() ||
		plan.Status.HasAnyCondition(Suspended) {
		plan.Status.StageCondition(RegistriesEnsured)
		return nil
	}
	storage, err := plan.GetStorage(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	if storage == nil {
		return nil
	}
	if !storage.Status.IsReady() {
		err = r.deleteImageRegistryResources(plan)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	clusters, err := r.planClusters(plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err = cluster.GetClient(r)
		if err != nil {
			return liberr.Wrap(err)
		}

		// Migration Registry Secret
		secret, err := r.ensureRegistrySecret(client, plan, storage)
		if err != nil {
			return liberr.Wrap(err)
		}

		// Get cluster specific registry image
		registryImage, err := cluster.GetRegistryImage(client)
		if err != nil {
			return liberr.Wrap(err)
		}

		// Migration Registry ImageStream
		err = r.ensureRegistryImageStream(client, plan, secret, registryImage)
		if err != nil {
			return liberr.Wrap(err)
		}

		// Migration Registry DeploymentConfig
		err = r.ensureRegistryDC(client, plan, storage, secret, registryImage)
		if err != nil {
			return liberr.Wrap(err)
		}

		// Migration Registry Service
		err = r.ensureRegistryService(client, plan, secret)
		if err != nil {
			return liberr.Wrap(err)
		}

		nEnsured++
	}

	// Condition
	ensured := nEnsured == 2 // Both clusters.
	if ensured {
		plan.Status.SetCondition(migapi.Condition{
			Type:     RegistriesEnsured,
			Status:   True,
			Category: migapi.Required,
			Message:  RegistriesEnsuredMessage,
		})
	}

	return err
}

// Ensure the credentials secret for the migration registry on the specified cluster has been created
// If the secret is updated, it will return delete the imageregistry reesources
func (r ReconcileMigPlan) ensureRegistrySecret(client k8sclient.Client, plan *migapi.MigPlan, storage *migapi.MigStorage) (*kapi.Secret, error) {
	newSecret, err := plan.BuildRegistrySecret(r, storage)
	if err != nil {
		return nil, err
	}
	foundSecret, err := plan.GetRegistrySecret(client)
	if err != nil {
		return nil, err
	}
	if foundSecret == nil {
		// if for some reason secret was deleted, we need to make sure we redeploy
		deleteErr := r.deleteImageRegistryResourcesForClient(client, plan)
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
	deleteErr := r.deleteImageRegistryResourcesForClient(client, plan)
	if deleteErr != nil {
		return nil, liberr.Wrap(deleteErr)
	}
	plan.UpdateRegistrySecret(r, storage, foundSecret)
	err = client.Update(context.TODO(), foundSecret)
	if err != nil {
		return nil, err
	}

	return foundSecret, nil
}

// Ensure the imagestream for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryImageStream(client k8sclient.Client, plan *migapi.MigPlan, secret *kapi.Secret, registryImage string) error {
	newImageStream, err := plan.BuildRegistryImageStream(secret.GetName(), registryImage)
	if err != nil {
		return liberr.Wrap(err)
	}
	foundImageStream, err := plan.GetRegistryImageStream(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundImageStream == nil {
		err = client.Create(context.TODO(), newImageStream)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	}
	if plan.EqualsRegistryImageStream(newImageStream, foundImageStream) {
		return nil
	}
	plan.UpdateRegistryImageStream(foundImageStream, registryImage)
	err = client.Update(context.TODO(), foundImageStream)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Ensure the deploymentconfig for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryDC(client k8sclient.Client, plan *migapi.MigPlan,
	storage *migapi.MigStorage, secret *kapi.Secret, registryImage string) error {

	name := secret.GetName()
	dirName := storage.GetName() + "-registry-" + string(storage.UID)

	// Get Proxy Env Vars for DC
	proxySecret, err := plan.GetRegistryProxySecret(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	//Construct Registry DC
	newDC, err := plan.BuildRegistryDC(storage, proxySecret, name, dirName, registryImage)
	if err != nil {
		return liberr.Wrap(err)
	}
	foundDC, err := plan.GetRegistryDC(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if foundDC == nil {
		err = client.Create(context.TODO(), newDC)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	}
	if plan.EqualsRegistryDC(newDC, foundDC) {
		return nil
	}
	plan.UpdateRegistryDC(storage, foundDC, proxySecret, name, dirName, registryImage)
	err = client.Update(context.TODO(), foundDC)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Ensure the service for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryService(client k8sclient.Client, plan *migapi.MigPlan, secret *kapi.Secret) error {
	name := secret.GetName()
	newService, err := plan.BuildRegistryService(name)
	if err != nil {
		return liberr.Wrap(err)
	}
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
	err = client.Update(context.TODO(), foundService)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

func (r ReconcileMigPlan) getBothClients(plan *migapi.MigPlan) ([]compat.Client, error) {
	sourceCluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return []compat.Client{}, liberr.Wrap(err)
	}
	if sourceCluster == nil || !sourceCluster.Status.IsReady() {
		return []compat.Client{}, liberr.Wrap(fmt.Errorf("either source cluster is nil or not ready"))
	}
	sourceClient, err := sourceCluster.GetClient(r)
	if err != nil {
		return []compat.Client{}, liberr.Wrap(err)
	}

	destinationCluster, err := plan.GetDestinationCluster(r)
	if err != nil {
		return []compat.Client{}, liberr.Wrap(err)
	}
	if destinationCluster == nil || !destinationCluster.Status.IsReady() {
		return []compat.Client{}, liberr.Wrap(fmt.Errorf("either destination cluster is nil or not ready"))
	}
	destinationClient, err := destinationCluster.GetClient(r)
	if err != nil {
		return []compat.Client{}, liberr.Wrap(err)
	}

	return []compat.Client{sourceClient, destinationClient}, nil
}

func (r ReconcileMigPlan) deleteImageRegistryResources(plan *migapi.MigPlan) error {
	clients, err := r.getBothClients(plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, client := range clients {
		err := r.deleteImageRegistryResourcesForClient(client, plan)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}
