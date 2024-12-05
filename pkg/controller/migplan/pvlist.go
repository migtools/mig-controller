package migplan

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/compat"
	"github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	migpods "github.com/konveyor/mig-controller/pkg/pods"
	migref "github.com/konveyor/mig-controller/pkg/reference"
	"github.com/opentracing/opentracing-go"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PvMap map[k8sclient.ObjectKey]core.PersistentVolume
type Claims []migapi.PVC

var (
	suffixMatcher = regexp.MustCompile(`(.*)-mig-([\d|[:alpha:]]{4})$`)
)

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
	// Build PV map.
	pvMap, err := r.getPvMap(srcClient, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	claims, err := r.getClaims(srcClient, plan)
	if err != nil {
		return liberr.Wrap(err)
	}
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
					CopyMethods: r.getSupportedCopyMethods(),
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
	allPodList := core.PodList{}
	err = client.List(context.TODO(), &allPodList, &k8sclient.ListOptions{})
	if err != nil {
		return nil, err
	}
	inNamespaces := func(objNamespace string, namespaces []string) bool {
		for _, ns := range namespaces {
			if ns == objNamespace {
				return true
			}
		}
		return false
	}
	podList := []core.Pod{}
	for _, pod := range allPodList.Items {
		if inNamespaces(pod.Namespace, plan.GetSourceNamespaces()) {
			podList = append(podList, pod)
		}
	}

	for _, pv := range list.Items {
		if pv.Status.Phase != core.VolumeBound {
			continue
		}
		claim := pv.Spec.ClaimRef
		if migref.RefSet(claim) {
			key := k8sclient.ObjectKey{
				Namespace: claim.Namespace,
				Name:      getMappedNameForPVC(claim, podList, plan),
			}
			pvMap[key] = pv
		}
	}

	return pvMap, nil
}

// How to determine the new mapped name for a new target PVC.
// If the original PVC doesn't contain -mig-XXXX where XXXX is 4 random letters, then we append -mig-XXXX to the end of the PVC name.
// If the original PVC contains -mig-XXXX, then we replace -mig-XXXX with -mig-YYYY where YYYY is 4 random letters.
// For stateful sets the PVC name is in the form of <string>?<-mig-XXXX>-<setName>-<one or more digits>
// Replace this with <string>-mig-YYYY-<setName>-<one or more digits>
// YYYY can be found in the mig plan.
func getMappedNameForPVC(pvcRef *core.ObjectReference, podList []core.Pod, plan *migapi.MigPlan) string {
	pvcName := pvcRef.Name
	existingPVC := plan.Spec.FindPVC(pvcRef.Namespace, pvcRef.Name)
	if existingPVC != nil && (existingPVC.GetSourceName() != existingPVC.GetTargetName()) {
		if !isStorageConversionPlan(plan) {
			pvcName = fmt.Sprintf("%s:%s", pvcRef.Name, existingPVC.GetTargetName())
		} else {
			pvcName = existingPVC.GetTargetName()
		}
	}
	// if plan is for storage conversion, create a new unique name for the destination PVC
	if isStorageConversionPlan(plan) {
		pvcName = trimSuffix(pvcName)
		// we cannot append a suffix when pvcName itself is 247 characters or more because:
		// 1. total length of new pvc name cannot exceed 253 characters
		// 2. we append '-mig-' char before prefix and pvc names cannot end with '-'
		if len(pvcName) > 247 {
			pvcName = pvcName[:247]
		}
		if isStatefulSet, setName := isStatefulSetVolume(pvcRef, podList); isStatefulSet {
			pvcName = getStatefulSetVolumeName(pvcName, setName, plan)
		} else {
			pvcName = fmt.Sprintf("%s-mig-%s", pvcName, plan.GetSuffix())
		}
		if len(pvcName) > 253 {
			pvcName = pvcName[:253]
		}
		return fmt.Sprintf("%s:%s", pvcRef.Name, pvcName)
	}
	return pvcName
}

// getStatefulSetVolumeName add the storage conversion prefix between the ordeal and the pvc name string
func getStatefulSetVolumeName(pvcName, setName string, plan *migapi.MigPlan) string {
	formattedName := pvcName
	// templated volumes follow a pattern
	matcher := regexp.MustCompile(fmt.Sprintf("(.*)?(-%s)(-\\d+)$", setName))
	if !matcher.MatchString(pvcName) {
		// this is not a templated volume
		// must be treated as any other volume
		pvcName = trimSuffix(pvcName)
		return fmt.Sprintf("%s-mig-%s", pvcName, plan.GetSuffix())
	} else {
		cols := matcher.FindStringSubmatch(pvcName)
		if len(cols) > 3 {
			prefix := cols[1]
			if suffixMatcher.MatchString(cols[1]) {
				suffixCols := suffixMatcher.FindStringSubmatch(cols[1])
				prefix = strings.Replace(prefix, suffixCols[2], plan.GetSuffix(), 1)
				formattedName = fmt.Sprintf("%s%s%s",
					prefix, cols[2], cols[3])
			} else {
				prefix = strings.TrimSuffix(prefix, "-new")
				formattedName = fmt.Sprintf("%s-mig-%s%s%s",
					prefix, plan.GetSuffix(), cols[2], cols[3])
			}
		}
	}
	log.V(3).Info("Returning statefulset formatted Name", "formattedName", formattedName)
	return formattedName
}

func trimSuffix(pvcName string) string {
	suffix := "-new"
	if suffixMatcher.MatchString(pvcName) {
		suffixFixCols := suffixMatcher.FindStringSubmatch(pvcName)
		suffix = "-mig-" + suffixFixCols[2]
	}
	return strings.TrimSuffix(pvcName, suffix)
}

// isStatefulSetVolume given a volume and a list of discovered pods, returns whether the volume is used by a statefulset
func isStatefulSetVolume(pvcRef *core.ObjectReference, podList []core.Pod) (bool, string) {
	for _, pod := range podList {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "StatefulSet" {
				for _, vol := range pod.Spec.Volumes {
					if vol.PersistentVolumeClaim != nil {
						if vol.PersistentVolumeClaim.ClaimName == pvcRef.Name {
							return true, owner.Name
						}
					}
				}
			}
		}
	}
	return false, ""
}

// isStorageConversionPlan tells whether the migration plan is for storage conversion
func isStorageConversionPlan(plan *migapi.MigPlan) bool {
	migrationTypeCond := plan.Status.FindCondition(migapi.MigrationTypeIdentified)
	if migrationTypeCond != nil {
		if migrationTypeCond.Reason == string(migapi.StorageConversionPlan) {
			return true
		}
	}
	return false
}

// Get a list of PVCs found within the specified namespaces.
func (r *ReconcileMigPlan) getClaims(client compat.Client, plan *migapi.MigPlan) (Claims, error) {
	claims := Claims{}
	pvcList := []core.PersistentVolumeClaim{}
	for _, namespace := range plan.GetSourceNamespaces() {
		list := &core.PersistentVolumeClaimList{}
		err := client.List(context.TODO(), list, k8sclient.InNamespace(namespace))
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		pvcList = append(pvcList, list.Items...)
	}

	podList, err := migpods.ListTemplatePods(client, plan.GetSourceNamespaces())
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	for _, namespace := range plan.GetSourceNamespaces() {
		pods := &core.PodList{}
		err = client.List(context.TODO(), pods, k8sclient.InNamespace(namespace))
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == core.PodRunning {
				podList = append(podList, pod)
			}
		}
	}

	alreadyMigrated := func(pvc core.PersistentVolumeClaim) bool {
		if planuid, exists := pvc.Labels[migapi.MigPlanLabel]; exists {
			if planuid == string(plan.UID) {
				return true
			}
		}
		return false
	}

	migrationSourceOtherPlan := func(pvc core.PersistentVolumeClaim) bool {
		if planuid, exists := pvc.Labels[directvolumemigration.MigrationSourceFor]; exists {
			if planuid != string(plan.UID) {
				return true
			}
		}
		return false
	}

	isStorageConversionPlan := isStorageConversionPlan(plan)

	pvcToOwnerMap, err := r.createPVCToOwnerTypeMap(podList)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, pvc := range pvcList {
		if isStorageConversionPlan && (alreadyMigrated(pvc) || migrationSourceOtherPlan(pvc)) {
			continue
		}

		pv := plan.Spec.FindPv(migapi.PV{Name: pvc.Spec.VolumeName})
		volumeMode := core.PersistentVolumeFilesystem
		accessModes := pvc.Spec.AccessModes
		if pv == nil {
			if pvc.Spec.VolumeMode != nil {
				volumeMode = *pvc.Spec.VolumeMode
			}
		} else {
			volumeMode = pv.PVC.VolumeMode
			accessModes = pv.PVC.AccessModes
		}
		claims = append(
			claims, migapi.PVC{
				Namespace: pvc.Namespace,
				Name: getMappedNameForPVC(&core.ObjectReference{
					Name:      pvc.Name,
					Namespace: pvc.Namespace,
				}, podList, plan),
				AccessModes:  accessModes,
				VolumeMode:   volumeMode,
				HasReference: pvcInPodVolumes(pvc, podList),
				OwnerType:    pvcToOwnerMap[pvc.Name],
			})
	}
	return claims, nil
}

func (r *ReconcileMigPlan) createPVCToOwnerTypeMap(podList []core.Pod) (map[string]migapi.OwnerType, error) {
	pvcToOwnerMap := make(map[string]migapi.OwnerType)
	for _, pod := range podList {
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				// Only check for owner references if there is a single owner and the volume wasn't set already.
				ownerType, ok := pvcToOwnerMap[vol.PersistentVolumeClaim.ClaimName]
				if pod.OwnerReferences != nil && len(pod.OwnerReferences) == 1 {
					for _, owner := range pod.OwnerReferences {
						newOwnerType := migapi.Unknown
						if owner.Kind == "StatefulSet" && owner.APIVersion == "apps/v1" {
							newOwnerType = migapi.StatefulSet
						} else if owner.Kind == "ReplicaSet" && owner.APIVersion == "apps/v1" {
							// Check if the owner is a Deployment
							replicaSet := &appsv1.ReplicaSet{}
							if owner.Name != "" {
								err := r.Client.Get(context.TODO(), k8sclient.ObjectKey{
									Namespace: pod.Namespace,
									Name:      owner.Name,
								}, replicaSet)
								if err != nil && !errors.IsNotFound(err) {
									return nil, err
								}
							}
							if len(replicaSet.OwnerReferences) == 1 && replicaSet.OwnerReferences[0].Kind == "Deployment" && replicaSet.OwnerReferences[0].APIVersion == "apps/v1" {
								newOwnerType = migapi.Deployment
							} else {
								newOwnerType = migapi.ReplicaSet
							}
						} else if owner.Kind == "Deployment" && owner.APIVersion == "apps/v1" {
							newOwnerType = migapi.Deployment
						} else if owner.Kind == "DaemonSet" && owner.APIVersion == "apps/v1" {
							newOwnerType = migapi.DaemonSet
						} else if owner.Kind == "Job" && owner.APIVersion == "batch/v1" {
							newOwnerType = migapi.Job
						} else if owner.Kind == "CronJob" && owner.APIVersion == "batch/v1" {
							newOwnerType = migapi.CronJob
						} else if owner.Kind == "VirtualMachineInstance" && (owner.APIVersion == "kubevirt.io/v1" || owner.APIVersion == "kubevirt.io/v1alpha3") {
							newOwnerType = migapi.VirtualMachine
						} else if owner.Kind == "Pod" && strings.HasPrefix(pod.Name, "hp-") {
							newOwnerType = migapi.VirtualMachine
						} else {
							newOwnerType = migapi.Unknown
						}
						if !ok {
							pvcToOwnerMap[vol.PersistentVolumeClaim.ClaimName] = newOwnerType
						} else if ownerType != newOwnerType {
							pvcToOwnerMap[vol.PersistentVolumeClaim.ClaimName] = migapi.Unknown
						}
					}
				} else {
					pvcToOwnerMap[vol.PersistentVolumeClaim.ClaimName] = migapi.Unknown
				}
			}
		}
	}
	return pvcToOwnerMap, nil
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
func (r *ReconcileMigPlan) getSupportedCopyMethods() []string {
	return []string{
		migapi.PvFilesystemCopyMethod,
		migapi.PvBlockCopyMethod,
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
	copyMethod := migapi.PvFilesystemCopyMethod
	if pv.Spec.VolumeMode != nil && *pv.Spec.VolumeMode == core.PersistentVolumeBlock {
		copyMethod = migapi.PvBlockCopyMethod
	}
	log.Info("PV Discovery: Setting default selections for discovered PV.",
		"persistentVolume", pv.Name,
		"pvSelectedAction", selectedAction,
		"pvSelectedStorageClass", selectedStorageClass,
		"pvCopyMethod", copyMethod)
	return migapi.Selection{
		Action:       selectedAction,
		StorageClass: selectedStorageClass,
		CopyMethod:   copyMethod,
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

	isIntraCluster, err := plan.IsIntraCluster(r)
	if err != nil {
		return targetStorageClassName, liberr.Wrap(err)
	}

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
		// don't warn for intra-cluster migrations
		if pv.Spec.NFS == nil && !isIntraCluster {
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
