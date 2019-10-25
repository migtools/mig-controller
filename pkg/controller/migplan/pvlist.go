package migplan

import (
	"context"
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	migref "github.com/fusor/mig-controller/pkg/reference"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type PvMap map[types.NamespacedName]core.PersistentVolume
type Claims []migapi.PVC

// Update the PVs listed on the plan.
func (r *ReconcileMigPlan) updatePvs(plan *migapi.MigPlan) error {
	if plan.Status.HasCondition(Suspended) {
		plan.Status.StageCondition(PvsDiscovered)
		plan.Status.StageCondition(PvLimitExceeded)
		return nil
	}
	if plan.Status.HasAnyCondition(
		InvalidSourceClusterRef,
		SourceClusterNotReady,
		NsListEmpty,
		NsLimitExceeded,
		NsNotFoundOnSourceCluster) {
		return nil
	}

	// Get srcMigCluster
	srcMigCluster, err := plan.GetSourceCluster(r.Client)
	if err != nil {
		log.Trace(err)
		return err
	}

	client, err := srcMigCluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return err
	}

	// Get destMigCluster
	destMigCluster, err := plan.GetDestinationCluster(r.Client)
	if err != nil {
		log.Trace(err)
		return err
	}

	srcStorageClasses := srcMigCluster.Spec.StorageClasses
	destStorageClasses := destMigCluster.Spec.StorageClasses

	plan.Spec.BeginPvStaging()

	// Build PV map.
	pvMap, err := r.getPvMap(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	namespaces := plan.Spec.Namespaces
	claims, err := r.getClaims(client, namespaces)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, claim := range claims {
		key := types.NamespacedName{
			Namespace: claim.Namespace,
			Name:      claim.Name,
		}
		pv, found := pvMap[key]
		if !found {
			continue
		}
		selection, err := r.getDefaultSelection(pv, claim, plan, srcStorageClasses, destStorageClasses)
		if err != nil {
			log.Trace(err)
			return err
		}
		plan.Spec.AddPv(
			migapi.PV{
				Name:         pv.Name,
				Capacity:     pv.Spec.Capacity[core.ResourceStorage],
				StorageClass: pv.Spec.StorageClassName,
				Supported: migapi.Supported{
					Actions:     r.getSupportedActions(pv),
					CopyMethods: r.getSupportedCopyMethods(pv),
				},
				Selection: selection,
				PVC:       claim,
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

	plan.Spec.PersistentVolumes.EndPvStaging()

	// Limits
	limit := Settings.Plan.PvLimit
	count := len(plan.Spec.PersistentVolumes.List)
	if count > limit {
		message := fmt.Sprintf(PvLimitExceededMessage, limit, count)
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvLimitExceeded,
			Status:   True,
			Reason:   LimitExceeded,
			Category: Warn,
			Message:  message,
		})
	}

	return nil
}

// Get a table (map) of PVs keyed by PVC namespaced name.
func (r *ReconcileMigPlan) getPvMap(client k8sclient.Client) (PvMap, error) {
	pvMap := PvMap{}
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
			pvMap[key] = pv
		}
	}

	return pvMap, nil
}

// Get a list of PVCs found on pods with the specified namespaces.
func (r *ReconcileMigPlan) getClaims(client k8sclient.Client, namespaces []string) (Claims, error) {
	claims := Claims{}
	for _, ns := range namespaces {
		list := &core.PodList{}
		options := k8sclient.InNamespace(ns)
		err := client.List(context.TODO(), options, list)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		for _, pod := range list.Items {
			for _, volume := range pod.Spec.Volumes {
				claimRef := volume.VolumeSource.PersistentVolumeClaim
				if claimRef == nil {
					continue
				}
				pvc := core.PersistentVolumeClaim{}
				// Get PVC
				ref := types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      claimRef.ClaimName,
				}
				err := client.Get(context.TODO(), ref, &pvc)
				if err != nil {
					log.Trace(err)
					return nil, err
				}

				claims = append(
					claims, migapi.PVC{
						Namespace:   pod.Namespace,
						Name:        claimRef.ClaimName,
						AccessModes: pvc.Spec.AccessModes,
					})
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

// Determine the supported PV copy methods.
// This is static for now but should eventually take into account
// src volume type and dest cluster available storage classes
func (r *ReconcileMigPlan) getSupportedCopyMethods(pv core.PersistentVolume) []string {
	return []string{
		migapi.PvFilesystemCopyMethod,
		migapi.PvSnapshotCopyMethod,
	}
}

// Gets the default selection values for a PV
func (r *ReconcileMigPlan) getDefaultSelection(pv core.PersistentVolume,
	claim migapi.PVC,
	plan *migapi.MigPlan,
	srcStorageClasses []migapi.StorageClass,
	destStorageClasses []migapi.StorageClass) (migapi.Selection, error) {
	selectedStorageClass, err := r.getDestStorageClass(pv, claim, plan, srcStorageClasses, destStorageClasses)
	if err != nil {
		return migapi.Selection{}, err
	}
	actions := r.getSupportedActions(pv)
	selectedAction := ""
	// if there's only one action, make that the default, otherwise select "copy" (if available)
	if len(actions) == 1 {
		selectedAction = actions[0]
	} else {
		for _, a := range actions {
			if a == migapi.PvCopyAction {
				selectedAction = a
				break
			}
		}
	}

	return migapi.Selection{
		Action:       selectedAction,
		StorageClass: selectedStorageClass,
		CopyMethod:   migapi.PvFilesystemCopyMethod,
	}, nil
}

// Determine the initial value for the destination storage class.
func (r *ReconcileMigPlan) getDestStorageClass(pv core.PersistentVolume,
	claim migapi.PVC,
	plan *migapi.MigPlan,
	srcStorageClasses []migapi.StorageClass,
	destStorageClasses []migapi.StorageClass) (string, error) {
	srcStorageClassName := pv.Spec.StorageClassName

	srcProvisioner := findProvisionerForName(srcStorageClassName, srcStorageClasses)
	targetProvisioner := ""
	targetStorageClassName := ""
	warnIfTargetUnavailable := false

	// For gluster src volumes, migrate to cephfs or cephrbd (warn if unavailable)
	// For nfs src volumes, migrate to cephfs or cephrbd (no warning if unavailable)
	// FIXME: Is there a corresponding pv.Spec.<volumeSource> for glusterblock?
	// FIXME: Do we want to check for a provisioner for NFS or just pv.Spec.NFS?
	if srcProvisioner == "kubernetes.io/glusterfs" ||
		srcProvisioner == "gluster.org/glusterblock" ||
		pv.Spec.Glusterfs != nil ||
		pv.Spec.NFS != nil {
		if isRWX(claim.AccessModes) {
			targetProvisioner = findProvisionerForSuffix("cephfs.csi.ceph.com", destStorageClasses)
		} else if isRWO(claim.AccessModes) {
			targetProvisioner = findProvisionerForSuffix("rbd.csi.ceph.com", destStorageClasses)
		} else {
			targetProvisioner = findProvisionerForSuffix("cephfs.csi.ceph.com", destStorageClasses)
		}
		// warn for gluster but not NFS
		if pv.Spec.NFS == nil {
			warnIfTargetUnavailable = true
		}
		// For all other pvs, migrate to storage class with the same provisioner, if available
		// FIXME: Are there any other types where we want to target a matching dynamic provisioner even if the src
		// pv doesn't have a storage class (i.e. matching targetProvisioner based on pv.Spec.PersistentVolumeSource)?
	} else {
		targetProvisioner = srcProvisioner
	}
	matchingStorageClasses := findStorageClassesForProvisioner(targetProvisioner, destStorageClasses)
	if len(matchingStorageClasses) > 0 {
		if findProvisionerForName(srcStorageClassName, matchingStorageClasses) != "" {
			targetStorageClassName = srcStorageClassName
		} else {
			targetStorageClassName = matchingStorageClasses[0].Name
		}
	} else {
		targetStorageClassName = findDefaultStorageClassName(destStorageClasses)
		// if we're using a default storage class and we need to warn
		if targetStorageClassName != "" && warnIfTargetUnavailable {
			existingWarnCondition := plan.Status.FindCondition(PvWarnNoCephAvailable)
			if existingWarnCondition == nil {
				plan.Status.SetCondition(migapi.Condition{
					Type:     PvWarnNoCephAvailable,
					Status:   True,
					Category: Warn,
					Message:  PvWarnNoCephAvailableMessage,
					Items:    []string{pv.Name},
				})
			} else {
				existingWarnCondition.Items = append(existingWarnCondition.Items, pv.Name)
			}
		}
	}
	return targetStorageClassName, nil
}

func findDefaultStorageClassName(storageClasses []migapi.StorageClass) string {
	for _, storageClass := range storageClasses {
		if storageClass.Default {
			return storageClass.Name
		}
	}
	return ""
}

func findProvisionerForName(name string, storageClasses []migapi.StorageClass) string {
	for _, storageClass := range storageClasses {
		if name == storageClass.Name {
			return storageClass.Provisioner
		}
	}
	return ""
}

// If not found, just return the suffix. It will fail matching later, but it avoids having an empty target string
func findProvisionerForSuffix(suffix string, storageClasses []migapi.StorageClass) string {
	for _, storageClass := range storageClasses {
		if strings.HasSuffix(storageClass.Provisioner, suffix) {
			return storageClass.Provisioner
		}
	}
	return suffix
}

func findStorageClassesForProvisioner(provisioner string, storageClasses []migapi.StorageClass) []migapi.StorageClass {
	var matchingClasses []migapi.StorageClass
	for _, storageClass := range storageClasses {
		if provisioner == storageClass.Provisioner {
			matchingClasses = append(matchingClasses, storageClass)
		}
	}
	return matchingClasses
}

func isRWX(accessModes []core.PersistentVolumeAccessMode) bool {
	for _, accessMode := range accessModes {
		if accessMode == core.ReadWriteMany {
			return true
		}
	}
	return false
}
func isRWO(accessModes []core.PersistentVolumeAccessMode) bool {
	for _, accessMode := range accessModes {
		if accessMode == core.ReadWriteOnce {
			return true
		}
	}
	return false
}
