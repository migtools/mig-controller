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
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
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

// Returns the restore progress statistics
func getRestoreStats(restore *velero.Restore) (itemsRestored int, totalItems int) {
	if restore == nil || restore.Status.Progress == nil {
		return
	}

	itemsRestored = restore.Status.Progress.ItemsRestored
	totalItems = restore.Status.Progress.TotalItems
	return
}

func getRestoreDuration(restore *velero.Restore) (duration string) {
	if restore.Status.StartTimestamp != nil {
		if restore.Status.CompletionTimestamp == nil {
			duration = fmt.Sprintf("(%s)",
				time.Now().Sub(restore.Status.StartTimestamp.Time).Round(time.Second))
		} else {
			duration = fmt.Sprintf("(%s)",
				restore.Status.CompletionTimestamp.Sub(restore.Status.StartTimestamp.Time).Round(time.Second))
		}
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
		itemsRestored, totalItems := getRestoreStats(restore)
		progress = append(
			progress,
			fmt.Sprintf(
				"Restore %s/%s: %d out of estimated total of %d objects restored %s",
				restore.Namespace,
				restore.Name,
				itemsRestored,
				totalItems,
				getRestoreDuration(restore)))
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
		itemsRestored, totalItems := getRestoreStats(restore)
		progress = append(
			progress,
			fmt.Sprintf(
				"Restore %s/%s: %d out of estimated total of %d objects restored %s",
				restore.Namespace,
				restore.Name,
				itemsRestored,
				totalItems,
				getRestoreDuration(restore)))
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
		itemsRestored, totalItems := getRestoreStats(restore)
		message = fmt.Sprintf(
			"%s %d out of estimated total of %d objects restored %s",
			message,
			itemsRestored,
			totalItems,
			getRestoreDuration(restore))
		progress = append(progress, message)
		progress = append(
			progress,
			getPodVolumeRestoresProgress(pvrs)...)
	case velero.RestorePhasePartiallyFailed:
		completed = true
		itemsRestored, totalItems := getRestoreStats(restore)
		message := fmt.Sprintf(
			"Restore %s/%s: partially failed. %d out of estimated total of %d objects restored %s",
			restore.Namespace,
			restore.Name,
			itemsRestored,
			totalItems,
			getRestoreDuration(restore))
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
		migMigrationUID, ok := restore.ObjectMeta.Labels[migapi.MigMigrationLabel]
		if !ok {
			t.Log.V(4).Info("Restore does not have an attached label "+
				"associating it with a MigMigration. Skipping deletion.",
				"restore", path.Join(restore.Namespace, restore.Name),
				"restorePhase", restore.Status.Phase,
				"associationLabel", migapi.MigMigrationLabel)
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
			migMigrationUID, ok := restore.ObjectMeta.Labels[migapi.MigMigrationLabel]
			if !ok {
				t.Log.V(4).Info("PodVolumeRestore does not have an attached label "+
					"associating it with a MigMigration. Skipping deletion.",
					"podVolumeRestore", path.Join(pvr.Namespace, pvr.Name),
					"podVolumeRestoreStatus", pvr.Status.Phase,
					"associationLabel", migapi.MigMigrationLabel)
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
