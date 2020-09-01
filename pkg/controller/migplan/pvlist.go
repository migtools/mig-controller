package migplan

import (
	"context"
	"fmt"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migpods "github.com/konveyor/mig-controller/pkg/pods"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PvMap map[k8sclient.ObjectKey]core.PersistentVolume
type Claims []migapi.PVC

// Update the PVs listed on the plan.
func (r *ReconcileMigPlan) updatePvs(plan *migapi.MigPlan) error {
	if plan.Status.HasAnyCondition(Suspended, migapi.RefreshInProgress) {
		plan.Status.StageCondition(PvsDiscovered)
		plan.Status.StageCondition(PvNoSupportedAction)
		plan.Status.StageCondition(PvNoStorageClassSelection)
		plan.Status.StageCondition(PvWarnAccessModeUnavailable)
		plan.Status.StageCondition(PvWarnCopyMethodSnapshot)
		plan.Status.StageCondition(PvWarnNoCephAvailable)
		plan.Status.StageCondition(PvLimitExceeded)
		return nil
	}
	if plan.Status.HasAnyCondition(
		InvalidSourceClusterRef,
		SourceClusterNotReady,
		InvalidDestinationClusterRef,
		DestinationClusterNotReady,
		NsNotFoundOnSourceCluster,
		NsLimitExceeded,
		NsListEmpty) {
		return nil
	}

	// Get srcMigCluster
	srcMigCluster, err := plan.GetSourceCluster(r.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcMigCluster == nil || !srcMigCluster.Status.IsReady() {
		return nil
	}

	client, err := srcMigCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Get destMigCluster
	destMigCluster, err := plan.GetDestinationCluster(r.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if destMigCluster == nil || !destMigCluster.Status.IsReady() {
		return nil
	}

	srcStorageClasses := srcMigCluster.Spec.StorageClasses
	destStorageClasses := destMigCluster.Spec.StorageClasses

	plan.Spec.BeginPvStaging()
	if plan.IsResourceExcluded("persistentvolumeclaims") {
		plan.Spec.ResetPvs()
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvsDiscovered,
			Status:   True,
			Reason:   Done,
			Category: migapi.Required,
			Message:  PvsDiscoveredMessage,
		})
		plan.Spec.PersistentVolumes.EndPvStaging()
		return nil
	}
	// Build PV map.
	pvMap, err := r.getPvMap(client)
	if err != nil {
		return liberr.Wrap(err)
	}
	claims, err := r.getClaims(client, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, claim := range claims {
		key := k8sclient.ObjectKey{
			Namespace: claim.Namespace,
			Name:      claim.Name,
		}
		pv, found := pvMap[key]
		if !found {
			continue
		}
		selection, err := r.getDefaultSelection(pv, claim, plan, srcStorageClasses, destStorageClasses)
		if err != nil {
			return liberr.Wrap(err)
		}
		plan.Spec.AddPv(
			migapi.PV{
				Name:         pv.Name,
				Capacity:     pv.Spec.Capacity[core.ResourceStorage],
				StorageClass: getStorageClassName(pv),
				Supported: migapi.Supported{
					Actions:     r.getSupportedActions(pv, claim),
					CopyMethods: r.getSupportedCopyMethods(pv),
				},
				Selection: selection,
				PVC:       claim,
				NFS:       pv.Spec.NFS,
			})
	}

	// Remove PvWarnNoCephAvailable for move operations
	existingWarnCondition := plan.Status.FindCondition(PvWarnNoCephAvailable)
	if existingWarnCondition != nil {
		for _, pv := range plan.Spec.PersistentVolumes.List {
			if pv.Selection.Action == migapi.PvMoveAction {
				for i, pvName := range existingWarnCondition.Items {
					if pvName == pv.Name {
						existingWarnCondition.Items = append(existingWarnCondition.Items[:i], existingWarnCondition.Items[i+1:]...)
						break
					}
				}
			}
		}
		if len(existingWarnCondition.Items) == 0 {
			plan.Status.DeleteCondition(PvWarnNoCephAvailable)
		}
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
			key := k8sclient.ObjectKey{
				Namespace: claim.Namespace,
				Name:      claim.Name,
			}
			pvMap[key] = pv
		}
	}

	return pvMap, nil
}

// Get a list of PVCs found within the specified namespaces.
func (r *ReconcileMigPlan) getClaims(client k8sclient.Client, plan *migapi.MigPlan) (Claims, error) {
	claims := Claims{}
	list := &core.PersistentVolumeClaimList{}
	err := client.List(context.TODO(), &k8sclient.ListOptions{}, list)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	podList, err := migpods.ListTemplatePods(client, plan.GetSourceNamespaces())
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	runningPods := &core.PodList{}
	err = client.List(context.TODO(), &k8sclient.ListOptions{}, runningPods)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	inNamespaces := func(objNamespace string, namespaces []string) bool {
		for _, ns := range namespaces {
			if ns == objNamespace {
				return true
			}
		}
		return false
	}

	for _, pod := range runningPods.Items {
		if inNamespaces(pod.Namespace, plan.GetSourceNamespaces()) {
			podList = append(podList, pod)
		}
	}

	for _, pvc := range list.Items {
		if !inNamespaces(pvc.Namespace, plan.GetSourceNamespaces()) {
			continue
		}
		claims = append(
			claims, migapi.PVC{
				Namespace:    pvc.Namespace,
				Name:         pvc.Name,
				AccessModes:  pvc.Spec.AccessModes,
				HasReference: pvcInPodVolumes(pvc, podList),
			})
	}

	return claims, nil
}

// Determine the supported PV actions.
func (r *ReconcileMigPlan) getSupportedActions(pv core.PersistentVolume, claim migapi.PVC) []string {
	suportedActions := []string{}
	if !claim.HasReference {
		suportedActions = append(suportedActions, migapi.PvSkipAction)
	}
	if pv.Spec.HostPath != nil {
		return suportedActions
	}
	if pv.Spec.NFS != nil ||
		pv.Spec.Glusterfs != nil ||
		pv.Spec.AWSElasticBlockStore != nil {
		return append(suportedActions,
			migapi.PvCopyAction,
			migapi.PvMoveAction)
	}
	return append(suportedActions, migapi.PvCopyAction)
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

// Gets the StorageClass name for the PV
func getStorageClassName(pv core.PersistentVolume) string {
	storageClassName := pv.Spec.StorageClassName
	if storageClassName == "" {
		storageClassName = pv.Annotations[core.BetaStorageClassAnnotation]
	}
	return storageClassName
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
	actions := r.getSupportedActions(pv, claim)
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
	srcStorageClassName := getStorageClassName(pv)

	srcProvisioner := findProvisionerForName(srcStorageClassName, srcStorageClasses)
	targetProvisioner := ""
	targetStorageClassName := ""
	warnIfTargetUnavailable := false

	// For gluster src volumes, migrate to cephfs or cephrbd (warn if unavailable)
	// For nfs src volumes, migrate to cephfs or cephrbd (no warning if unavailable)
	if srcProvisioner == "kubernetes.io/glusterfs" ||
		strings.HasPrefix(srcProvisioner, "gluster.org/glusterblock") ||
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

func pvcInPodVolumes(pvc core.PersistentVolumeClaim, pods []core.Pod) bool {
	for _, pod := range pods {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return true
			}
		}
	}

	return false
}
