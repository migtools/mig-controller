package migplan

import (
	"context"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	kapi "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the migration registries on both source and dest clusters have been created
func (r ReconcileMigPlan) ensureMigRegistries(plan *migapi.MigPlan) error {
	var client k8sclient.Client
	nEnsured := 0

	storage, err := plan.GetStorage(r)
	if err != nil {
		return err
	}
	if storage == nil {
		return nil
	}
	clusters, err := r.planClusters(plan)
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err = cluster.GetClient(r)
		if err != nil {
			return err
		}

		// Migration Registry Secret
		secret, err := r.ensureRegistrySecret(client, plan, storage)
		if err != nil {
			return err
		}

		// Migration Registry ImageStream
		err = r.ensureRegistryImageStream(client, plan, secret)
		if err != nil {
			return err
		}

		// Migration Registry DeploymentConfig
		err = r.ensureRegistryDC(client, plan, storage, secret)
		if err != nil {
			return err
		}

		// Migration Registry Service
		err = r.ensureRegistryService(client, plan, secret)
		if err != nil {
			return err
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
		err = client.Create(context.TODO(), newSecret)
		if err != nil {
			return nil, err
		}
		return newSecret, nil
	}
	if plan.EqualsRegistrySecret(newSecret, foundSecret) {
		return foundSecret, nil
	}
	plan.UpdateRegistrySecret(r, storage, foundSecret)
	err = client.Update(context.TODO(), foundSecret)
	if err != nil {
		return nil, err
	}

	return foundSecret, nil
}

// Ensure the imagestream for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryImageStream(client k8sclient.Client, plan *migapi.MigPlan, secret *kapi.Secret) error {
	newImageStream, err := plan.BuildRegistryImageStream(secret.GetName())
	if err != nil {
		return err
	}
	foundImageStream, err := plan.GetRegistryImageStream(client)
	if err != nil {
		return err
	}
	if foundImageStream == nil {
		err = client.Create(context.TODO(), newImageStream)
		if err != nil {
			return err
		}
		return nil
	}
	if plan.EqualsRegistryImageStream(newImageStream, foundImageStream) {
		return nil
	}
	plan.UpdateRegistryImageStream(foundImageStream)
	err = client.Update(context.TODO(), foundImageStream)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the deploymentconfig for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryDC(client k8sclient.Client, plan *migapi.MigPlan, storage *migapi.MigStorage, secret *kapi.Secret) error {
	name := secret.GetName()
	dirName := plan.GetName() + "-registry-" + string(plan.UID)
	newDC, err := plan.BuildRegistryDC(storage, name, dirName)
	if err != nil {
		return err
	}
	foundDC, err := plan.GetRegistryDC(client)
	if err != nil {
		return err
	}
	if foundDC == nil {
		err = client.Create(context.TODO(), newDC)
		if err != nil {
			return err
		}
		return nil
	}
	if plan.EqualsRegistryDC(newDC, foundDC) {
		return nil
	}
	plan.UpdateRegistryDC(storage, foundDC, name, dirName)
	err = client.Update(context.TODO(), foundDC)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the service for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryService(client k8sclient.Client, plan *migapi.MigPlan, secret *kapi.Secret) error {
	name := secret.GetName()
	newService, err := plan.BuildRegistryService(name)
	if err != nil {
		return err
	}
	foundService, err := plan.GetRegistryService(client)
	if err != nil {
		return err
	}
	if foundService == nil {
		err = client.Create(context.TODO(), newService)
		if err != nil {
			return err
		}
		return nil
	}
	if plan.EqualsRegistryService(newService, foundService) {
		return nil
	}
	plan.UpdateRegistryService(foundService, name)
	err = client.Update(context.TODO(), foundService)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the migration registries on both source and dest clusters have been removed
func (r ReconcileMigPlan) ensureMigRegistriesDelete(plan *migapi.MigPlan) error {
	var client k8sclient.Client

	clusters, err := migapi.ListClusters(r, "")
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err = cluster.GetClient(r)
		if err != nil {
			return err
		}

		// Migration Registry Service
		err = r.ensureRegistryServiceDelete(client, plan)
		if err != nil {
			return err
		}

		// Migration Registry DeploymentConfig
		err = r.ensureRegistryDCDelete(client, plan)
		if err != nil {
			return err
		}
		// Migration Registry ImageStream
		err = r.ensureRegistryImageStreamDelete(client, plan)
		if err != nil {
			return err
		}

		// Migration Registry Secret
		err := r.ensureRegistrySecretDelete(client, plan)
		if err != nil {
			return err
		}
	}

	return nil
}

// Ensure the service for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryServiceDelete(client k8sclient.Client, plan *migapi.MigPlan) error {
	foundService, err := plan.GetRegistryService(client)
	if err != nil {
		return err
	}
	if foundService == nil {
		return nil
	}
	err = client.Delete(context.TODO(), foundService)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the deploymentconfig for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryDCDelete(client k8sclient.Client, plan *migapi.MigPlan) error {
	foundDC, err := plan.GetRegistryDC(client)
	if err != nil {
		return err
	}
	if foundDC == nil {
		return nil
	}
	err = client.Delete(context.TODO(), foundDC)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the imagestream for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryImageStreamDelete(client k8sclient.Client, plan *migapi.MigPlan) error {
	foundImageStream, err := plan.GetRegistryImageStream(client)
	if err != nil {
		return err
	}
	if foundImageStream == nil {
		return nil
	}
	err = client.Delete(context.TODO(), foundImageStream)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the credentials secret for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistrySecretDelete(client k8sclient.Client, plan *migapi.MigPlan) error {
	foundSecret, err := plan.GetRegistrySecret(client)
	if err != nil {
		return err
	}
	if foundSecret == nil {
		return nil
	}
	err = client.Delete(context.TODO(), foundSecret)
	if err != nil {
		return err
	}

	return nil
}
