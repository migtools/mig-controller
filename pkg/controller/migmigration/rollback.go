package migmigration

import (
	"context"
	"fmt"
	"path"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/gvk"
	ocapi "github.com/openshift/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Delete namespace and cluster-scoped resources on dest cluster
func (t *Task) deleteMigrated() error {
	// Delete 'deployer' and 'hooks' Pods that DeploymentConfig leaves behind upon DC deletion.
	err := t.deleteDeploymentConfigLeftoverPods()
	if err != nil {
		return liberr.Wrap(err)
	}

	err = t.deleteMigratedNamespaceScopedResources()
	if err != nil {
		return liberr.Wrap(err)
	}

	err = t.deleteMovedNfsPVs()
	if err != nil {
		return liberr.Wrap(err)
	}

	err = t.deleteDVMTemporaryResources()
	if err != nil {
		return liberr.Wrap(err)
	}
	return nil
}

// deleteDVMTemporaryResources deletes temporary resources created by DVM such as configmaps, secrets, etc.
// TODO: this logic is copied from DVM controller for the time being. The purpose is to address BZ-1977887
// and avoid cyclic dependencies between controllers. This needs to be updated once crane-lib is integrated
func (t *Task) deleteDVMTemporaryResources() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}
	selector := labels.SelectorFromSet(map[string]string{
		migapi.MigPlanLabel: string(t.PlanResources.MigPlan.UID),
	})
	for _, ns := range t.sourceNamespaces() {
		podList := corev1.PodList{}
		cmList := corev1.ConfigMapList{}
		secretList := corev1.SecretList{}
		// Get Pod list
		err := client.List(
			context.TODO(),
			&podList,
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			})
		if err != nil {
			return err
		}
		// Get Secret list
		err = client.List(
			context.TODO(),
			&secretList,
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			})
		if err != nil {
			return err
		}
		// Get configmap list
		err = client.List(
			context.TODO(),
			&cmList,
			&k8sclient.ListOptions{
				Namespace:     ns,
				LabelSelector: selector,
			})
		if err != nil {
			return err
		}
		// Delete pods
		for _, pod := range podList.Items {
			t.Log.Info("Rollback: Deleting DVM Pods from source cluster",
				"pod", path.Join(pod.Namespace, pod.Name))
			err = client.Delete(context.TODO(), &pod, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
		// Delete secrets
		for _, secret := range secretList.Items {
			t.Log.Info("Rollback: Deleting DVM Secrets from source cluster",
				"secret", path.Join(secret.Namespace, secret.Name))
			err = client.Delete(context.TODO(), &secret, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
		// Delete configmaps
		for _, cm := range cmList.Items {
			t.Log.Info("Rollback: Deleting DVM ConfigMaps from source cluster",
				"configMap", path.Join(cm.Namespace, cm.Name))
			err = client.Delete(context.TODO(), &cm, k8sclient.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !k8serror.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (t *Task) deleteDeploymentConfigLeftoverPods() error {
	// DeploymentConfigs are an exception to the general policy of rollback deleting everything with
	// the label "migrated-by-migplan: migplan-uid" because DCs spawn additional Pods without
	// ownerRefs that will mount PVCs. When we delete the DCs, it doesn't cascade to the
	// all Pods (e.g. deployer) that the DC created, and those Pods sometimes stop PVCs from terminating.
	// This custom deletion routine for DCs is needed to avoid rollback hanging waiting on PVCs
	// to finish terminating.
	for _, ns := range t.destinationNamespaces() {
		destClient, err := t.getDestinationClient()
		if err != nil {
			return liberr.Wrap(err)
		}
		// Iterate over all DeploymentConfigs belonging migrated by current MigPlan in target cluster namespaces
		dcList := ocapi.DeploymentConfigList{}
		err = destClient.List(
			context.TODO(),
			&dcList,
			k8sclient.InNamespace(ns),
			k8sclient.MatchingLabels(map[string]string{migapi.MigPlanLabel: string(t.PlanResources.MigPlan.UID)}),
		)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, dc := range dcList.Items {
			// Iterate over ReplicationControllers associated with DCs
			rcList := corev1.ReplicationControllerList{}
			err = destClient.List(
				context.TODO(),
				&rcList,
				k8sclient.InNamespace(ns),
				k8sclient.MatchingLabels(map[string]string{"openshift.io/deployment-config.name": dc.GetName()}),
			)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, rc := range rcList.Items {
				// Delete Deployer Pod(s) for RC
				podList := corev1.PodList{}
				err = destClient.List(
					context.TODO(),
					&podList,
					k8sclient.InNamespace(ns),
					k8sclient.MatchingLabels(map[string]string{"openshift.io/deployer-pod-for.name": rc.GetName()}),
				)
				for _, pod := range podList.Items {
					t.Log.Info(
						"Rollback: Deleting Deployer Pod associated with migrated DeploymentConfig.",
						"pod", path.Join(pod.Namespace, pod.Name),
						"replicationController", path.Join(rc.Namespace, rc.Name),
						"deploymentConfig", path.Join(dc.Namespace, dc.Name))
					err = destClient.Delete(context.TODO(), &pod)
					if err != nil {
						return liberr.Wrap(err)
					}
				}

				// Delete RCs matching DC name to remove old Pods
				t.Log.Info(
					"Rollback: Deleting ReplicationController associated with migrated DeploymentConfig.",
					"replicationController", path.Join(rc.Namespace, rc.Name),
					"deploymentConfig", path.Join(dc.Namespace, dc.Name))
				err = destClient.Delete(context.TODO(), &rc)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
		}
	}
	return nil
}

// Delete migrated namespace-scoped resources on dest cluster
func (t *Task) deleteMigratedNamespaceScopedResources() error {
	t.Log.Info("Rollback: Scanning all GVKs in all migrated namespaces for " +
		"MigPlan associated resources to delete.")
	client, GVRs, err := gvk.GetNamespacedGVRsForCluster(t.PlanResources.DestMigCluster, t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}

	clientListOptions := k8sclient.ListOptions{}
	matchingLabels := k8sclient.MatchingLabels(map[string]string{
		migapi.MigPlanLabel: string(t.PlanResources.MigPlan.UID),
	})
	matchingLabels.ApplyToList(&clientListOptions)
	listOptions := clientListOptions.AsListOptions()
	for _, gvr := range GVRs {
		for _, ns := range t.destinationNamespaces() {
			gvkCombined := gvr.Group + "/" + gvr.Version + "/" + gvr.Resource
			t.Log.Info(fmt.Sprintf("Rollback: Searching destination cluster namespace for resources "+
				"with migrated-by label."),
				"namespace", ns,
				"gvk", gvkCombined,
				"label", fmt.Sprintf("%v:%v", migapi.MigPlanLabel, string(t.PlanResources.MigPlan.UID)))
			err = client.Resource(gvr).DeleteCollection(context.Background(), metav1.DeleteOptions{}, *listOptions)
			if err == nil {
				continue
			}
			if !k8serror.IsMethodNotSupported(err) && !k8serror.IsNotFound(err) {
				return liberr.Wrap(err)
			}
			list, err := client.Resource(gvr).Namespace(ns).List(context.Background(), *listOptions)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, r := range list.Items {
				err = client.Resource(gvr).Namespace(ns).Delete(context.Background(), r.GetName(), metav1.DeleteOptions{})
				if err != nil {
					// Will ignore the ones that were removed, or for some reason are not supported
					// Assuming that main resources will be removed, such as pods and pvcs
					if k8serror.IsMethodNotSupported(err) || k8serror.IsNotFound(err) {
						continue
					}
					log.Error(err, fmt.Sprintf("Failed to request delete on: %s", gvr.String()))
					return err
				}
				log.Info("DELETION REQUESTED for resource on destination cluster with matching migrated-by label",
					"gvk", gvkCombined,
					"resource", path.Join(ns, r.GetName()))
			}
		}
	}

	return nil
}

// Delete migrated NFS PV resources that were "moved" to the dest cluster
func (t *Task) deleteMovedNfsPVs() error {
	t.Log.Info("Starting deletion of any 'moved' NFS PVs from destination cluster")
	dstClient, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	// Only delete PVs with matching 'migrated-by-migplan' label.
	listOptions := k8sclient.MatchingLabels(map[string]string{
		migapi.MigPlanLabel: string(t.PlanResources.MigPlan.UID),
	})
	list := corev1.PersistentVolumeList{}
	err = dstClient.List(context.TODO(), &list, listOptions)
	if err != nil {
		return err
	}
	for _, pv := range list.Items {
		// Skip unless PV type = NFS
		if pv.Spec.NFS == nil {
			continue
		}
		// Skip delete unless ReclaimPolicy=Retain
		if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
			continue
		}
		t.Log.Info("Deleting 'moved' NFS PV from destination cluster",
			"persistentVolume", pv.Name)
		err := dstClient.Delete(context.TODO(), &pv)
		if err != nil {
			if k8serror.IsMethodNotSupported(err) || k8serror.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to request delete on moved PV",
				"persistentVolume", pv.Name)
			return err
		}
		log.Info("Deleted moved NFS PV from destination cluster", "persistentVolume", pv.Name)
	}

	return nil
}

func (t *Task) ensureMigratedResourcesDeleted() (bool, error) {
	t.Log.Info("Scanning all GVKs in all migrated namespaces to ensure " +
		"resources have finished deleting.")
	client, GVRs, err := gvk.GetNamespacedGVRsForCluster(t.PlanResources.DestMigCluster, t.Client)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	clientListOptions := k8sclient.ListOptions{}
	matchingLabels := k8sclient.MatchingLabels(map[string]string{
		migapi.MigPlanLabel: string(t.PlanResources.MigPlan.UID),
	})
	matchingLabels.ApplyToList(&clientListOptions)
	listOptions := clientListOptions.AsListOptions()
	for _, gvr := range GVRs {
		for _, ns := range t.destinationNamespaces() {
			gvkCombinedName := gvr.Group + "/" + gvr.Version + "/" + gvr.Resource
			log.Info("Rollback: Checking for leftover resources in destination cluster",
				"gvk", gvkCombinedName, "namespace", ns)
			list, err := client.Resource(gvr).Namespace(ns).List(context.Background(), *listOptions)
			if err != nil {
				return false, liberr.Wrap(err)
			}
			// Wait for resources with deletion timestamps
			if len(list.Items) > 0 {
				t.Log.Info("Resource(s) found with in destination cluster "+
					"that have NOT finished terminating. These resource(s) "+
					"are associated with the MigPlan and deletion has been requested.",
					"gvk", gvkCombinedName,
					"namespace", ns)
				return false, err
			}
		}
	}

	return true, nil
}
