package migplan

import (
	"context"
	"fmt"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	corev1 "k8s.io/api/core/v1"
	kapi "k8s.io/api/core/v1"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the migration registries on both source and dest clusters have been created
func (r ReconcileMigPlan) ensureMigRegistries(plan *migapi.MigPlan) error {
	var client k8sclient.Client
	nEnsured := 0

	if plan.Status.HasCondition(StorageNotReady) {
		plan.Status.DeleteCondition(RegistriesEnsured)
		err := r.deleteImageRegistryResources(plan)
		if err != nil {
			return liberr.Wrap(err)
		}
		return nil
	}

	if plan.Status.HasCriticalCondition() || plan.Status.HasAnyCondition(Suspended) {
		// TODO: need to make sure that if plan is suspended due to final migration is complete
		// 		we garbage collect the mig registry
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

		// Migration Registry DeploymentConfig
		err = r.ensureRegistryDeployment(client, plan, storage, secret, registryImage)
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

// Ensure the migration registries on both source and dest clusters are healthy
func (r ReconcileMigPlan) ensureRegistryHealth(plan *migapi.MigPlan) error {
	nEnsured := 0
	unHealthyPod := corev1.Pod{}
	unHealthyClusterName := ""
	clusters, err := r.planClusters(plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, cluster := range clusters {

		if !cluster.Status.IsReady() {
			continue
		}

		client, err := cluster.GetClient(r)
		if err != nil {
			return liberr.Wrap(err)
		}

		registryPods, err := getRegistryPods(plan, client)
		if err != nil {
			log.Trace(err)
			return liberr.Wrap(err)
		}

		registryStatusUnhealthy, podObj := isRegistryPodUnHealthy(registryPods)

		if !registryStatusUnhealthy {
			nEnsured++
		} else {
			unHealthyPod = podObj
			unHealthyClusterName = cluster.ObjectMeta.Name
		}
	}

	if nEnsured == 2 {
		plan.Status.SetCondition(migapi.Condition{
			Type:     RegistriesHealthy,
			Status:   True,
			Category: migapi.Required,
			Message:  RegistriesHealthyMessage,
		})
	} else {
		plan.Status.SetCondition(migapi.Condition{
			Type:     RegistriesHealthy,
			Status:   False,
			Category: migapi.Required,
			Message: fmt.Sprintf("The Migration registry pod %s/%s is not in a healthy state on cluster %s",
				unHealthyPod.Namespace, unHealthyPod.Name, unHealthyClusterName),
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
		deleteErr := r.deleteImageRegistryDeploymentForClient(client, plan)
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
	deleteErr := r.deleteImageRegistryDeploymentForClient(client, plan)
	if deleteErr != nil {
		return nil, liberr.Wrap(deleteErr)
	}
	err = plan.UpdateRegistrySecret(r, storage, foundSecret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	err = client.Update(context.TODO(), foundSecret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return foundSecret, nil
}

// Ensure the deployment for the migration registry on the specified cluster has been created
func (r ReconcileMigPlan) ensureRegistryDeployment(client k8sclient.Client, plan *migapi.MigPlan,
	storage *migapi.MigStorage, secret *kapi.Secret, registryImage string) error {

	name := secret.GetName()
	dirName := storage.GetName() + "-registry-" + string(storage.UID)

	// Get Proxy Env Vars for DC
	proxySecret, err := plan.GetRegistryProxySecret(client)
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
func (r ReconcileMigPlan) ensureRegistryService(client k8sclient.Client, plan *migapi.MigPlan, secret *kapi.Secret) error {
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
	err = client.Update(context.TODO(), foundService)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

func (r ReconcileMigPlan) deleteImageRegistryResources(plan *migapi.MigPlan) error {
	plan.Status.Conditions.DeleteCondition(RegistriesEnsured)
	planRefResources, err := plan.GetRefResources(r)
	if err != nil {
		return liberr.Wrap(err)
	}
	srcClient, err := planRefResources.SrcMigCluster.GetClient(r.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	destClient, err := planRefResources.DestMigCluster.GetClient(r.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	clients := []compat.Client{srcClient, destClient}
	for _, client := range clients {
		err := r.deleteImageRegistryResourcesForClient(client, plan)
		if err != nil {
			return liberr.Wrap(err)
		}
	}
	return nil
}

func isRegistryPodUnHealthy(registryPods corev1.PodList) (bool, corev1.Pod) {
	unHealthyPod := corev1.Pod{}
	for _, registryPod := range registryPods.Items {
		if !registryPod.Status.ContainerStatuses[0].Ready {
			return true, registryPod
		}
	}
	return false, unHealthyPod
}

func getRegistryPods(plan *migapi.MigPlan, registryClient compat.Client) (corev1.PodList, error) {

	registryPodList := corev1.PodList{}
	err := registryClient.List(context.TODO(), &k8sclient.ListOptions{
		LabelSelector: k8sLabels.SelectorFromSet(map[string]string{
			"migplan": string(plan.UID),
		}),
	}, &registryPodList)

	if err != nil {
		log.Trace(err)
		return corev1.PodList{}, err
	}
	return registryPodList, nil
}
