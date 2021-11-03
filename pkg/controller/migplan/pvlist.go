package migplan

import (
	"context"
	"fmt"
	"path"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	migpods "github.com/konveyor/mig-controller/pkg/pods"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PvMap map[k8sclient.ObjectKey]core.PersistentVolume
type Claims []migapi.PVC

// Update the PVs listed on the plan.
func (r *ReconcileMigPlan) updatePvs(ctx context.Context, plan *migapi.MigPlan) error {
	if opentracing.SpanFromContext(ctx) != nil {
		span, _ := opentracing.StartSpanFromContextWithTracer(ctx, r.tracer, "updatePvs")
		defer span.Finish()
	}
	if plan.Status.HasAnyCondition(Suspended) {
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

	log.Info("PV Discovery: Starting for Migration Plan",
		"migPlan", path.Join(plan.Namespace, plan.Name),
		"migPlanNamespaces", plan.Spec.Namespaces)

	// Get srcMigCluster
	srcMigCluster, err := plan.GetSourceCluster(r.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	if srcMigCluster == nil || !srcMigCluster.Status.IsReady() {
		return nil
	}

	srcClient, err := srcMigCluster.GetClient(r)
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

	destClient, err := destMigCluster.GetClient(r)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Get StorageClasses
	srcStorageClasses, err := srcMigCluster.GetStorageClasses(srcClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SrcStorageClasses = srcStorageClasses

	destStorageClasses, err := destMigCluster.GetStorageClasses(destClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.DestStorageClasses = destStorageClasses

	plan.Spec.BeginPvStaging()
	if plan.IsResourceExcluded("persistentvolumeclaims") {
		log.Info("PV Discovery: 'persistentvolumeclaims' found in MigPlan "+
			"Status.ExcludedResources, ending PV discovery",
			"migPlan", path.Join(plan.Namespace, plan.Name))
		plan.Spec.ResetPvs()
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvsDiscovered,
			Status:   True,
			Reason:   Done,
			Category: migapi.Required,
			Message:  "The `persistentVolumes` list has been updated with discovered PVs.",
		})
		plan.Spec.PersistentVolumes.EndPvStaging()
		return nil
	}
	// Build PV map.
	pvMap, err := r.getPvMap(srcClient, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	claims, err := r.getClaims(srcClient, plan)
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
		Message:  "The `persistentVolumes` list has been updated with discovered PVs.",
	})

	plan.Spec.PersistentVolumes.EndPvStaging()

	// Limits
	limit := Settings.Plan.PvLimit
	count := len(plan.Spec.PersistentVolumes.List)
	if count > limit {
		plan.Status.SetCondition(migapi.Condition{
			Type:     PvLimitExceeded,
			Status:   True,
			Reason:   LimitExceeded,
			Category: Warn,
			Message:  fmt.Sprintf("PV limit: %d exceeded, found: %d.", limit, count),
		})
	}

	log.Info("PV Discovery: Finished for Migration Plan",
		"migPlan", path.Join(plan.Namespace, plan.Name),
		"migPlanNamespaces", plan.Spec.Namespaces)
	return nil
}

// Get a table (map) of PVs keyed by PVC namespaced name.
func (r *ReconcileMigPlan) getPvMap(client k8sclient.Client, plan *migapi.MigPlan) (PvMap, error) {
	pvMap := PvMap{}
	list := core.PersistentVolumeList{}
	err := client.List(context.TODO(), &list, &k8sclient.ListOptions{})
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
				Name:      getMappedNameForPVC(claim, plan),
			}
			pvMap[key] = pv
		}
	}

	return pvMap, nil
}

func getMappedNameForPVC(pvcRef *core.ObjectReference, plan *migapi.MigPlan) string {
	pvcName := pvcRef.Name
	existingPVC := plan.Spec.FindPVC(pvcRef.Namespace, pvcRef.Name)
	if existingPVC != nil &&
		(existingPVC.GetSourceName() != existingPVC.GetTargetName()) {
		pvcName = fmt.Sprintf("%s:%s", pvcRef.Name, existingPVC.GetTargetName())
	}
	// if plan is for storage conversion, create a new unique name for the destination PVC
	if isStorageConversionPlan(plan) &&
		(existingPVC == nil || existingPVC.GetSourceName() == existingPVC.GetTargetName()) {
		// we cannot append a prefix when pvcName itself is 252 characters or more because:
		// 1. total length of new pvc name cannot exceed 253 characters
		// 2. we append a '-' char before prefix and pvc names cannot end with '-'
		if len(pvcName) > 251 {
			return pvcName
		} else {
			destName := fmt.Sprintf("%s:%s-%s", pvcName, pvcName, migapi.StorageConversionPVCNamePrefix)
			if len(destName) > 253 {
				return destName[:253]
			} else {
				return destName
			}
		}
	}
	return pvcName
}

// isStorageConversionPlan tells whether the migration plan is for storage conversion
func isStorageConversionPlan(plan *migapi.MigPlan) bool {
	migrationTypeCond := plan.Status.FindCondition(MigrationTypeIdentified)
	if migrationTypeCond != nil {
		if migrationTypeCond.Reason == StorageConversionPlan {
			return true
		}
	}
	return false
}

// Get a list of PVCs found within the specified namespaces.
func (r *ReconcileMigPlan) getClaims(client k8sclient.Client, plan *migapi.MigPlan) (Claims, error) {
	claims := Claims{}
	list := &core.PersistentVolumeClaimList{}
	err := client.List(context.TODO(), list, &k8sclient.ListOptions{})
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	podList, err := migpods.ListTemplatePods(client, plan.GetSourceNamespaces())
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	runningPods := &core.PodList{}
	err = client.List(context.TODO(), runningPods, &k8sclient.ListOptions{})
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

	alreadyMigrated := func(pvc core.PersistentVolumeClaim) bool {
		if _, exists := pvc.Labels[migapi.MigMigrationLabel]; exists {
			return true
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

		if alreadyMigrated(pvc) {
			continue
		}

		claims = append(
			claims, migapi.PVC{
				Namespace: pvc.Namespace,
				Name: getMappedNameForPVC(&core.ObjectReference{
					Name:      pvc.Name,
					Namespace: pvc.Namespace,
				}, plan),
				AccessModes:  pvc.Spec.AccessModes,
				HasReference: pvcInPodVolumes(pvc, podList),
			})
	}
	return claims, nil
}

// Determine the supported PV actions.
func (r *ReconcileMigPlan) getSupportedActions(pv core.PersistentVolume, claim migapi.PVC) []string {
	supportedActions := []string{}
	supportedActions = append(supportedActions, migapi.PvSkipAction)
	if pv.Spec.HostPath != nil {
		return supportedActions
	}
	// TODO: Consider adding Cinder to this default list
	if pv.Spec.NFS != nil ||
		pv.Spec.AWSElasticBlockStore != nil {
		return append(supportedActions,
			migapi.PvCopyAction,
			migapi.PvMoveAction)
	}
	for _, sc := range Settings.Plan.MoveStorageClasses {
		if pv.Spec.StorageClassName == sc {
			return append(supportedActions,
				migapi.PvCopyAction,
				migapi.PvMoveAction)
		}
	}
	return append(supportedActions, migapi.PvCopyAction)
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
	log.Info("PV Discovery: Setting default selections for discovered PV.",
		"persistentVolume", pv.Name,
		"pvSelectedAction", selectedAction,
		"pvSelectedStorageClass", selectedStorageClass,
		"pvCopyMethod", migapi.PvFilesystemCopyMethod)
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
					Message: "Ceph is not available on destination. If this is desired, please install the rook" +
						" operator. The following PVs will use the default storage class instead: []",
					Items: []string{pv.Name},
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
