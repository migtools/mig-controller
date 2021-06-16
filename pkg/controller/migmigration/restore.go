package migmigration

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/gvk"
	ocapi "github.com/openshift/api/apps/v1"
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the final restore on the destination cluster has been
// created  and has the proper settings.
func (t *Task) ensureFinalRestore() (*velero.Restore, error) {
	backup, err := t.getInitialBackup()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if backup == nil {
		return nil, errors.New("Backup not found")
	}

	restore, err := t.getFinalRestore()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if restore != nil {
		return restore, nil
	}

	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	newRestore, err := t.buildRestore(client, backup.Name, "final")
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newRestore.Labels[migapi.FinalRestoreLabel] = t.UID()
	newRestore.Labels[migapi.MigMigrationDebugLabel] = t.Owner.Name
	newRestore.Labels[migapi.MigPlanDebugLabel] = t.Owner.Spec.MigPlanRef.Name
	newRestore.Labels[migapi.MigMigrationLabel] = string(t.Owner.UID)
	newRestore.Labels[migapi.MigPlanLabel] = string(t.PlanResources.MigPlan.UID)

	t.Log.Info("Creating Velero Final Restore on target cluster.",
		"restore", path.Join(newRestore.Namespace, newRestore.Name))
	err = client.Create(context.TODO(), newRestore)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return newRestore, nil
}

// Get the final restore on the destination cluster.
func (t *Task) getFinalRestore() (*velero.Restore, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[migapi.FinalRestoreLabel] = t.UID()
	return t.getRestore(labels)
}

// Ensure the first restore on the destination cluster has been
// created and has the proper settings.
func (t *Task) ensureStageRestore() (*velero.Restore, error) {
	backup, err := t.getStageBackup()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if backup == nil {
		return nil, errors.New("Backup not found")
	}

	restore, err := t.getStageRestore()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if restore != nil {
		return restore, nil
	}

	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	newRestore, err := t.buildRestore(client, backup.Name, "stage")
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newRestore.Labels[migapi.StageRestoreLabel] = t.UID()
	newRestore.Labels[migapi.MigMigrationDebugLabel] = t.Owner.Name
	newRestore.Labels[migapi.MigPlanDebugLabel] = t.Owner.Spec.MigPlanRef.Name
	newRestore.Labels[migapi.MigMigrationLabel] = string(t.Owner.UID)
	newRestore.Labels[migapi.MigPlanLabel] = string(t.PlanResources.MigPlan.UID)
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newRestore.Annotations[migapi.StagePodImageAnnotation] = stagePodImage
	err = client.Create(context.TODO(), newRestore)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return newRestore, nil
}

// Get the stage restore on the destination cluster.
func (t *Task) getStageRestore() (*velero.Restore, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[migapi.StageRestoreLabel] = t.UID()
	return t.getRestore(labels)
}

// Get an existing Restore on the destination cluster.
func (t Task) getRestore(labels map[string]string) (*velero.Restore, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	list := velero.RestoreList{}
	err = client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(labels))
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Get PVRs associated with a Restore
func (t *Task) getPodVolumeRestoresForRestore(restore *velero.Restore) *velero.PodVolumeRestoreList {
	list := velero.PodVolumeRestoreList{}
	restoreAssociationLabel := map[string]string{
		velero.RestoreNameLabel: restore.Name,
	}
	client, err := t.getDestinationClient()
	if err != nil {
		log.Trace(err)
		return &list
	}
	err = client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(restoreAssociationLabel))
	if err != nil {
		log.Trace(err)
	}
	return &list
}

// getPVRDuration returns duration of in-progress PVR
func getPVRDuration(pvr *velero.PodVolumeRestore) (duration string) {
	if pvr.Status.StartTimestamp != nil {
		if pvr.Status.CompletionTimestamp == nil {
			duration = fmt.Sprintf(" (%s)",
				time.Now().Sub(pvr.Status.StartTimestamp.Time).Round(time.Second))
		} else {
			duration = fmt.Sprintf(" (%s)",
				pvr.Status.CompletionTimestamp.Sub(pvr.Status.StartTimestamp.Time).Round(time.Second))
		}
	}
	return
}

// getPodVolumeRestoresProgress returns progress information of PVRs
// Given a list of PVRs, iterates over the list, reads progress reported in status field of PVR,
// converts to compatible progress messages and returns an ordered list of formatted messages
func getPodVolumeRestoresProgress(pvrList *velero.PodVolumeRestoreList) (progress []string) {
	m, keys, msg := make(map[string]string), make([]string, 0), ""

	for _, pvr := range pvrList.Items {
		switch pvr.Status.Phase {
		case velero.PodVolumeRestorePhaseInProgress:
			msg = fmt.Sprintf(
				"PodVolumeRestore %s/%s: %s out of %s restored%s",
				pvr.Namespace,
				pvr.Name,
				bytesToSI(pvr.Status.Progress.BytesDone),
				bytesToSI(pvr.Status.Progress.TotalBytes),
				getPVRDuration(&pvr))
		case velero.PodVolumeRestorePhaseCompleted:
			msg = fmt.Sprintf(
				"PodVolumeRestore %s/%s: %s out of %s restored%s",
				pvr.Namespace,
				pvr.Name,
				bytesToSI(pvr.Status.Progress.BytesDone),
				bytesToSI(pvr.Status.Progress.TotalBytes),
				getPVRDuration(&pvr))
		case velero.PodVolumeRestorePhaseFailed:
			msg = fmt.Sprintf(
				"PodVolumeRestore %s/%s: Failed. %s out of %s restored%s",
				pvr.Namespace,
				pvr.Name,
				bytesToSI(pvr.Status.Progress.BytesDone),
				bytesToSI(pvr.Status.Progress.TotalBytes),
				getPVRDuration(&pvr))
		default:
			msg = fmt.Sprintf(
				"PodVolumeRestore %s/%s: Waiting for ongoing volume restore(s) to complete",
				pvr.Namespace,
				pvr.Name)
		}
		m[pvr.Namespace+"/"+pvr.Name] = msg
		keys = append(keys, pvr.Namespace+"/"+pvr.Name)
	}
	// sort the progress array to maintain order everytime it's updated
	sort.Strings(keys)
	for _, k := range keys {
		progress = append(progress, m[k])
	}
	return
}

// Get whether a resource has completed on the destination cluster.
func (t *Task) hasRestoreCompleted(restore *velero.Restore) (bool, []string) {
	completed := false
	reasons := []string{}
	progress := []string{}

	pvrs := t.getPodVolumeRestoresForRestore(restore)

	switch restore.Status.Phase {
	case velero.RestorePhaseNew:
		progress = append(
			progress,
			fmt.Sprintf(
				"Restore %s/%s: Not started",
				restore.Namespace,
				restore.Name))
	case velero.RestorePhaseInProgress:
		progress = append(
			progress,
			fmt.Sprintf(
				"Restore %s/%s: %s",
				restore.Namespace,
				restore.Name,
				restore.Status.Phase))
		progress = append(
			progress,
			getPodVolumeRestoresProgress(pvrs)...)
		stageReport, err := t.allStagePodsMatch()
		if err != nil {
			log.Trace(err)
		}
		progress = append(progress, stageReport...)
	case velero.RestorePhaseCompleted:
		completed = true
		progress = append(
			progress,
			fmt.Sprintf(
				"Restore %s/%s: %s",
				restore.Namespace,
				restore.Name,
				restore.Status.Phase))
		progress = append(
			progress,
			getPodVolumeRestoresProgress(pvrs)...)
		stageReport, err := t.allStagePodsMatch()
		if err != nil {
			log.Trace(err)
		}
		progress = append(progress, stageReport...)
	case velero.RestorePhaseFailed:
		completed = true
		message := fmt.Sprintf(
			"Restore %s/%s: Failed.",
			restore.Namespace,
			restore.Name)
		reasons = append(reasons, message)
		progress = append(progress, message)
		progress = append(
			progress,
			getPodVolumeRestoresProgress(pvrs)...)
	case velero.RestorePhasePartiallyFailed:
		completed = true
		message := fmt.Sprintf(
			fmt.Sprintf(
				"Restore %s/%s: partially failed.",
				restore.Namespace,
				restore.Name))
		progress = append(progress, message)
		progress = append(
			progress,
			getPodVolumeRestoresProgress(pvrs)...)
		stageReport, err := t.allStagePodsMatch()
		if err != nil {
			log.Trace(err)
		}
		progress = append(progress, stageReport...)
	case velero.RestorePhaseFailedValidation:
		reasons = restore.Status.ValidationErrors
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Restore %s/%s: validation failed.",
				restore.Namespace,
				restore.Name))
		completed = true
	}

	t.Log.Info("Velero Restore progress report",
		"restore", path.Join(restore.Namespace, restore.Name),
		"restoreProgress", progress)

	t.setProgress(progress)
	return completed, reasons
}

// Set warning conditions on migmigration if there were restic errors
func (t *Task) setResticConditions(restore *velero.Restore) {
	if len(restore.Status.PodVolumeRestoreErrors) > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     ResticErrors,
			Status:   True,
			Category: migapi.Warn,
			Message: fmt.Sprintf("There were errors found in %d Restic volume restores. See restore `%s` for details",
				len(restore.Status.PodVolumeRestoreErrors), restore.Name),
			Durable: true,
		})
	}
	if len(restore.Status.PodVolumeRestoreVerifyErrors) > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     ResticVerifyErrors,
			Status:   True,
			Category: migapi.Warn,
			Message: fmt.Sprintf("There were verify errors found in %d Restic volume restores. See restore `%s` for details",
				len(restore.Status.PodVolumeRestoreVerifyErrors), restore.Name),
			Durable: true,
		})
	}
}

// Set warning conditions on migmigration if there were any partial failures
func (t *Task) setStageRestorePartialFailureWarning(restore *velero.Restore) {
	if restore.Status.Phase == velero.RestorePhasePartiallyFailed {
		message := fmt.Sprintf(
			"Stage Restore %s/%s: partially failed on destination cluster", restore.GetNamespace(), restore.GetName())
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     VeleroStageRestorePartiallyFailed,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
}

// Set warning conditions on migmigration if there were any partial failures
func (t *Task) setFinalRestorePartialFailureWarning(restore *velero.Restore) {
	if restore.Status.Phase == velero.RestorePhasePartiallyFailed {
		message := fmt.Sprintf(
			"Final Restore %s/%s: partially failed on destination cluster", restore.GetNamespace(), restore.GetName())
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     VeleroFinalRestorePartiallyFailed,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
}

// Build a Restore as desired for the destination cluster.
func (t *Task) buildRestore(client k8sclient.Client, backupName string, restoreTypePrefix string) (*velero.Restore, error) {
	annotations, err := t.getAnnotations(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Construct a name like "$migrationname-54823-stage" or "$migrationname-54823-final".
	// This will produce a 57 character string max. Note that generateName gracefully handles strings >63 char.
	fmtString := fmt.Sprintf("%%.%ds", 55-len(restoreTypePrefix))
	migrationNameTruncated := fmt.Sprintf(fmtString, t.Owner.GetName())
	truncatedGenerateName := fmt.Sprintf("%s-%s-", migrationNameTruncated, restoreTypePrefix)

	restore := &velero.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: truncatedGenerateName,
			Namespace:    migapi.VeleroNamespace,
			Annotations:  annotations,
		},
	}
	t.updateRestore(restore, backupName)
	return restore, nil
}

// Update a Restore as desired for the destination cluster.
func (t *Task) updateRestore(restore *velero.Restore, backupName string) {
	restore.Spec = velero.RestoreSpec{
		BackupName:        backupName,
		RestorePVs:        pointer.BoolPtr(true),
		ExcludedResources: t.PlanResources.MigPlan.Status.ResourceList(),
	}

	t.updateNamespaceMapping(restore)
}

// Update namespace mapping for restore
func (t *Task) updateNamespaceMapping(restore *velero.Restore) {
	namespaceMapping := make(map[string]string)
	for _, namespace := range t.namespaces() {
		mapping := strings.Split(namespace, ":")
		if len(mapping) == 2 {
			if mapping[0] == mapping[1] {
				continue
			}
			if mapping[1] != "" {
				namespaceMapping[mapping[0]] = mapping[1]
			}
		}
	}

	if len(namespaceMapping) != 0 {
		restore.Spec.NamespaceMapping = namespaceMapping
	}
}

// Delete all Velero Restores correlated with the running MigPlan
func (t *Task) deleteCorrelatedRestores() error {
	client, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	list := velero.RestoreList{}
	err = client.List(
		context.TODO(),
		&list,
		k8sclient.MatchingLabels(t.PlanResources.MigPlan.GetCorrelationLabels()))
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, restore := range list.Items {
		t.Log.Info("Deleting Velero Restore on target cluster "+
			"due to correlation with MigPlan",
			"restore", path.Join(restore.Namespace, restore.Name))
		err = client.Delete(context.TODO(), &restore)
		if err != nil && !k8serror.IsNotFound(err) {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Delete stale Velero Restores in the controller namespace to empty
// the work queue for next migration.
func (t *Task) deleteStaleRestoresOnCluster(cluster *migapi.MigCluster) (int, int, error) {
	t.Log.Info("Checking for stale Velero Restore on MigCluster",
		"migCluster", path.Join(cluster.Namespace, cluster.Name))
	nDeleted := 0
	nInProgressDeleted := 0

	clusterClient, err := cluster.GetClient(t.Client)
	if err != nil {
		return 0, 0, liberr.Wrap(err)
	}

	list := velero.RestoreList{}
	err = clusterClient.List(
		context.TODO(),
		&list,
		k8sclient.InNamespace(migapi.VeleroNamespace))
	if err != nil {
		return 0, 0, liberr.Wrap(err)
	}
	for _, restore := range list.Items {
		// Skip delete unless phase is "", "New" or "InProgress"
		if restore.Status.Phase != velero.RestorePhaseNew &&
			restore.Status.Phase != velero.RestorePhaseInProgress &&
			restore.Status.Phase != "" {
			t.Log.V(4).Info("Restore with is not 'New' or 'InProgress'. Skipping deletion.",
				"restore", path.Join(restore.Namespace, restore.Name),
				"restorePhase", restore.Status.Phase)
			continue
		}
		// Skip if missing a migmigration correlation label (only delete our own CRs)
		// Example 'migmigration: 4c9d317f-f410-430b-af8f-4ecd7d17a7de'
		corrKey, _ := t.Owner.GetCorrelationLabel()
		migMigrationUID, ok := restore.ObjectMeta.Labels[corrKey]
		if !ok {
			t.Log.V(4).Info("Restore does not have an attached label "+
				"associating it with a MigMigration. Skipping deletion.",
				"restore", path.Join(restore.Namespace, restore.Name),
				"restorePhase", restore.Status.Phase,
				"associationLabel", corrKey)
			continue
		}
		// Skip if correlation label points to an existing, running migration
		isRunning, err := t.migrationUIDisRunning(migMigrationUID)
		if err != nil {
			return nDeleted, nInProgressDeleted, liberr.Wrap(err)
		}
		if isRunning {
			t.Log.Info("Restore is running. Skipping deletion.",
				"restore", path.Join(restore.Namespace, restore.Name),
				"restorePhase", restore.Status.Phase)
			continue
		}
		// Delete the Restore
		t.Log.Info("DELETING stale Velero Restore on MigCluster",
			"restore", path.Join(restore.Namespace, restore.Name),
			"restorePhase", restore.Status.Phase,
			"migCluster", path.Join(cluster.Namespace, cluster.Name))
		err = clusterClient.Delete(context.TODO(), &restore)
		if err != nil && !k8serror.IsNotFound(err) {
			return nDeleted, nInProgressDeleted, liberr.Wrap(err)
		}
		nDeleted++
		// Separate count for InProgress, used to determine if restart needed
		if restore.Status.Phase == velero.RestorePhaseInProgress {
			nInProgressDeleted++
		}
	}

	return nDeleted, nInProgressDeleted, nil
}

// Delete stale Velero PodVolumeRestores in the controller namespace to empty
// the work queue for next migration.
func (t *Task) deleteStalePVRsOnCluster(cluster *migapi.MigCluster) (int, error) {
	t.Log.Info("Checking for stale PodVolumeRestores on MigCluster",
		"migCluster", path.Join(cluster.Namespace, cluster.Name))
	nDeleted := 0
	clusterClient, err := cluster.GetClient(t.Client)
	if err != nil {
		return 0, liberr.Wrap(err)
	}

	list := velero.PodVolumeRestoreList{}
	err = clusterClient.List(
		context.TODO(),
		&list,
		k8sclient.InNamespace(migapi.VeleroNamespace))
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	for _, pvr := range list.Items {
		// Skip delete unless phase is "", "New" or "InProgress"
		if pvr.Status.Phase != velero.PodVolumeRestorePhaseNew &&
			pvr.Status.Phase != velero.PodVolumeRestorePhaseInProgress &&
			pvr.Status.Phase != "" {
			t.Log.V(4).Info("PodVolumeRestore is not 'New' or 'InProgress'. Skipping deletion.",
				"podVolumeRestore", path.Join(pvr.Namespace, pvr.Name),
				"podVolumeRestoreStatus", pvr.Status.Phase)
			continue
		}

		// Skip delete if PVR is associated with running migration
		pvrHasRunningMigration := false
		for _, ownerRef := range pvr.OwnerReferences {
			if ownerRef.Kind != "Restore" {
				t.Log.V(4).Info("PodVolumeRestore does not have an OwnerRef associated "+
					"with a Velero Backup. Skipping deletion.",
					"podVolumeRestore", path.Join(pvr.Namespace, pvr.Name),
					"podVolumeRestoreStatus", pvr.Status.Phase)
				continue
			}
			restore := velero.Restore{}
			err := clusterClient.Get(
				context.TODO(),
				types.NamespacedName{
					Namespace: migapi.VeleroNamespace,
					Name:      ownerRef.Name,
				},
				&restore,
			)
			if err != nil {
				return nDeleted, liberr.Wrap(err)
			}
			// Skip delete if missing a migmigration correlation label (only delete our own CRs)
			// Example 'migmigration: 4c9d317f-f410-430b-af8f-4ecd7d17a7de'
			corrKey, _ := t.Owner.GetCorrelationLabel()
			migMigrationUID, ok := restore.ObjectMeta.Labels[corrKey]
			if !ok {
				t.Log.V(4).Info("PodVolumeRestore does not have an attached label "+
					"associating it with a MigMigration. Skipping deletion.",
					"podVolumeRestore", path.Join(pvr.Namespace, pvr.Name),
					"podVolumeRestoreStatus", pvr.Status.Phase,
					"associationLabel", corrKey)
				continue
			}
			isRunning, err := t.migrationUIDisRunning(migMigrationUID)
			if err != nil {
				return nDeleted, liberr.Wrap(err)
			}
			if isRunning {
				pvrHasRunningMigration = true
			}
		}
		if pvrHasRunningMigration == true {
			t.Log.Info("PodVolumeRestore is associated with a running migration. Skipping deletion.",
				"podVolumeRestore", path.Join(pvr.Namespace, pvr.Name),
				"podVolumeRestoreStatus", pvr.Status.Phase)
			continue
		}

		// Delete the PVR
		t.Log.Info(
			"DELETING stale Velero PodVolumeRestore from MigCluster",
			"podVolumeRestore", path.Join(pvr.Namespace, pvr.Name),
			"podVolumeRestoreStatus", pvr.Status.Phase,
			"migCluster", path.Join(cluster.Namespace, cluster.Name))
		err = clusterClient.Delete(context.TODO(), &pvr)
		if err != nil && !k8serror.IsNotFound(err) {
			return nDeleted, liberr.Wrap(err)
		}
		nDeleted++
	}

	return nDeleted, nil
}

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

	return nil
}

func (t *Task) deleteDeploymentConfigLeftoverPods() error {
	// DeploymentConfigs are an exception to the general policy of rollback deleting everything with
	// the label "migrated-by-migplan: migplan-uid" because DCs spawn additional Pods without
	// ownerRefs that will mount PVCs. When we delete the DCs, it doesn't cascade to the
	// deployer and hooks Pods that the DC created, and those Pods sometimes stop PVCs from termianting.
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
