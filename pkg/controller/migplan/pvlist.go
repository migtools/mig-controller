package migplan

import (
	"context"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PvMap map[types.NamespacedName]core.PersistentVolume
type Claims []types.NamespacedName

// Update the PVs listed on the plan.
func (r *ReconcileMigPlan) updatePvs(plan *migapi.MigPlan) error {
	// Get srcMigCluster
	srcMigCluster, err := plan.GetSourceCluster(r.Client)
	if err != nil {
		return err
	}

	client, err := srcMigCluster.GetClient(r)
	if err != nil {
		return err
	}

	plan.Spec.BeginPvStaging()

	// Build PV map.
	table, err := r.getPvMap(client)
	if err != nil {
		return err
	}
	namespaces := plan.Spec.Namespaces
	claims, err := r.getClaims(client, namespaces)
	if err != nil {
		return err
	}
	for _, claim := range claims {
		pv, found := table[claim]
		if !found {
			continue
		}
		plan.Spec.AddPv(
			migapi.PersistentVolume{
				Name:             pv.Name,
				StorageClass:     pv.Spec.StorageClassName,
				SupportedActions: r.getSupportedActions(pv),
			})
	}

	// Set the condition to indicate that discovery has been performed.
	plan.Status.SetCondition(migapi.Condition{
		Type:     PvsDiscovered,
		Status:   True,
		Reason:   Done,
		Category: migapi.Required,
		Message:  PvsDiscoveredMessage,
	})

	// Update
	plan.Spec.PersistentVolumes.EndPvStaging()
	err = r.Update(context.TODO(), plan)
	if err != nil {
		return nil
	}

	return nil
}

// Get a table (map) of PVs keyed by namespaced name.
func (r *ReconcileMigPlan) getPvMap(client k8sclient.Client) (PvMap, error) {
	table := PvMap{}
	list := core.PersistentVolumeList{}
	err := client.List(context.TODO(), &k8sclient.ListOptions{}, &list)
	if err != nil {
		return nil, err
	}
	for _, pv := range list.Items {
		if pv.Status.Phase != core.VolumeBound {
			continue
		}
		claim := pv.Spec.ClaimRef
		if migref.RefSet(claim) {
			key := types.NamespacedName{
				Namespace: claim.Namespace,
				Name:      claim.Name,
			}
			table[key] = pv
		}
	}

	return table, nil
}

// Get a list of PVCs found on pods with the specified namespaces.
func (r *ReconcileMigPlan) getClaims(client k8sclient.Client, namespaces []string) (Claims, error) {
	claims := Claims{}
	list := &core.PodList{}
	for _, ns := range namespaces {
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, list)
		if err != nil {
			return nil, err
		}
		for _, pod := range list.Items {
			for _, volume := range pod.Spec.Volumes {
				claim := volume.VolumeSource.PersistentVolumeClaim
				if claim == nil {
					continue
				}
				ref := types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      claim.ClaimName,
				}
				claims = append(claims, ref)
			}
		}
	}

	return claims, nil
}

// Determine the supported PV actions.
func (r *ReconcileMigPlan) getSupportedActions(pv core.PersistentVolume) []string {
	if pv.Spec.HostPath != nil {
		return []string{}
	}
	if pv.Spec.NFS != nil ||
		pv.Spec.Glusterfs != nil ||
		pv.Spec.AzureDisk != nil ||
		pv.Spec.AzureFile != nil ||
		pv.Spec.AWSElasticBlockStore != nil {
		return []string{
			migapi.PvCopyAction,
			migapi.PvMoveAction,
		}
	}
	return []string{
		migapi.PvCopyAction,
	}
}
