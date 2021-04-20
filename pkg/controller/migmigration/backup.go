package migmigration

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"time"

	mapset "github.com/deckarep/golang-set"
	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var Settings = &settings.Settings

// Ensure the initial backup on the source cluster has been created
// and has the proper settings.
func (t *Task) ensureInitialBackup() (*velero.Backup, error) {
	backup, err := t.getInitialBackup()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if backup != nil {
		return backup, nil
	}

	client, err := t.getSourceClient()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newBackup, err := t.buildBackup(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	newBackup.Labels[migapi.InitialBackupLabel] = t.UID()
	newBackup.Labels[migapi.MigMigrationDebugLabel] = t.Owner.Name
	newBackup.Labels[migapi.MigPlanDebugLabel] = t.Owner.Spec.MigPlanRef.Name
	newBackup.Labels[migapi.MigMigrationLabel] = string(t.Owner.UID)
	newBackup.Labels[migapi.MigPlanLabel] = string(t.PlanResources.MigPlan.UID)
	newBackup.Spec.IncludedResources = toStringSlice(settings.IncludedInitialResources.Difference(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	newBackup.Spec.ExcludedResources = toStringSlice(settings.ExcludedInitialResources.Union(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	delete(newBackup.Annotations, migapi.QuiesceAnnotation)

	if Settings.DisImgCopy {
		if newBackup.Annotations == nil {
			newBackup.Annotations = map[string]string{}
		}
		newBackup.Annotations[migapi.DisableImageCopy] = strconv.FormatBool(Settings.DisImgCopy)
	}

	err = client.Create(context.TODO(), newBackup)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return newBackup, nil
}

func toStringSlice(set mapset.Set) []string {
	interfaceSlice := set.ToSlice()
	var strSlice []string = make([]string, len(interfaceSlice))
	for i, s := range interfaceSlice {
		strSlice[i] = s.(string)
	}
	return strSlice
}
func toSet(strSlice []string) mapset.Set {
	var interfaceSlice []interface{} = make([]interface{}, len(strSlice))
	for i, s := range strSlice {
		interfaceSlice[i] = s
	}
	return mapset.NewSetFromSlice(interfaceSlice)
}

// Get the initial backup on the source cluster.
func (t *Task) getInitialBackup() (*velero.Backup, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[migapi.InitialBackupLabel] = t.UID()
	return t.getBackup(labels)
}

// Ensure the second backup on the source cluster has been created and
// has the proper settings.
func (t *Task) ensureStageBackup() (*velero.Backup, error) {
	backup, err := t.getStageBackup()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if backup != nil {
		return backup, nil
	}

	client, err := t.getSourceClient()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	t.Log.Info("Building Stage Velero Backup resource definition")
	newBackup, err := t.buildBackup(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			migapi.IncludedInStageBackupLabel: t.UID(),
		},
	}
	newBackup.Labels[migapi.StageBackupLabel] = t.UID()
	newBackup.Labels[migapi.MigMigrationDebugLabel] = t.Owner.Name
	newBackup.Labels[migapi.MigPlanDebugLabel] = t.Owner.Spec.MigPlanRef.Name
	newBackup.Labels[migapi.MigMigrationLabel] = string(t.Owner.UID)
	newBackup.Labels[migapi.MigPlanLabel] = string(t.PlanResources.MigPlan.UID)
	var includedResources mapset.Set

	if t.indirectImageMigration() || Settings.DisImgCopy {
		includedResources = settings.IncludedStageResources
	} else {
		includedResources = settings.IncludedStageResources.Difference(mapset.NewSetFromSlice([]interface{}{settings.ISResource}))
	}
	newBackup.Spec.IncludedResources = toStringSlice(includedResources.Difference(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	newBackup.Spec.ExcludedResources = toStringSlice(settings.ExcludedStageResources.Union(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	newBackup.Spec.LabelSelector = &labelSelector
	if Settings.DisImgCopy {
		if newBackup.Annotations == nil {
			newBackup.Annotations = map[string]string{}
		}
		newBackup.Annotations[migapi.DisableImageCopy] = strconv.FormatBool(Settings.DisImgCopy)
	}
	t.Log.Info("Creating Stage Velero Backup on source cluster.",
		"backup", path.Join(newBackup.Namespace, newBackup.Name))
	err = client.Create(context.TODO(), newBackup)
	if err != nil {
		return nil, err
	}
	return newBackup, nil
}

// Get the stage backup on the source cluster.
func (t *Task) getStageBackup() (*velero.Backup, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[migapi.StageBackupLabel] = t.UID()
	return t.getBackup(labels)
}

func (t *Task) getPodVolumeBackupsForBackup(backup *velero.Backup) *velero.PodVolumeBackupList {
	list := velero.PodVolumeBackupList{}
	backupAssociationLabel := map[string]string{
		velero.BackupNameLabel: backup.Name,
	}
	client, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return &list
	}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(backupAssociationLabel),
		&list)
	if err != nil {
		log.Trace(err)
	}
	return &list
}

// Get an existing Backup on the source cluster.
func (t Task) getBackup(labels map[string]string) (*velero.Backup, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	list := velero.BackupList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// getPVBDuration returns duration of in-progress PVB
func getPVBDuration(pvb *velero.PodVolumeBackup) (duration string) {
	if pvb.Status.StartTimestamp != nil {
		if pvb.Status.CompletionTimestamp == nil {
			duration = fmt.Sprintf(" (%s)",
				time.Now().Sub(pvb.Status.StartTimestamp.Time).Round(time.Second))
		} else {
			duration = fmt.Sprintf(" (%s)",
				pvb.Status.CompletionTimestamp.Sub(pvb.Status.StartTimestamp.Time).Round(time.Second))
		}
	}
	return
}

// getBackupDuration returns duration of in-progress Backup
func getBackupDuration(bkp *velero.Backup) (duration string) {
	if bkp.Status.StartTimestamp != nil {
		if bkp.Status.CompletionTimestamp == nil {
			duration = fmt.Sprintf(" (%s)",
				time.Now().Sub(bkp.Status.StartTimestamp.Time).Round(time.Second))
		} else {
			duration = fmt.Sprintf(" (%s)",
				bkp.Status.CompletionTimestamp.Sub(bkp.Status.StartTimestamp.Time).Round(time.Second))
		}
	}
	return
}

// getPodVolumeBackupsProgress returns progress information of PVB resources
// Given a list of PVBs, iterates over the list, reads progress reported in status field of PVB,
// converts to compatible progress messages and returns an ordered list of formatted messages
func getPodVolumeBackupsProgress(pvbList *velero.PodVolumeBackupList) (progress []string) {
	m, keys, msg := make(map[string]string), make([]string, 0), ""

	for _, pvb := range pvbList.Items {
		switch pvb.Status.Phase {
		case velero.PodVolumeBackupPhaseInProgress:
			msg = fmt.Sprintf(
				"PodVolumeBackup %s/%s: %s out of %s backed up%s",
				pvb.Namespace,
				pvb.Name,
				bytesToSI(pvb.Status.Progress.BytesDone),
				bytesToSI(pvb.Status.Progress.TotalBytes),
				getPVBDuration(&pvb))
		case velero.PodVolumeBackupPhaseCompleted:
			msg = fmt.Sprintf(
				"PodVolumeBackup %s/%s: %s out of %s backed up%s",
				pvb.Namespace,
				pvb.Name,
				bytesToSI(pvb.Status.Progress.BytesDone),
				bytesToSI(pvb.Status.Progress.TotalBytes),
				getPVBDuration(&pvb))
		case velero.PodVolumeBackupPhaseFailed:
			msg = fmt.Sprintf(
				"PodVolumeBackup %s/%s: Failed. %s out of %s backed up%s",
				pvb.Namespace,
				pvb.Name,
				bytesToSI(pvb.Status.Progress.BytesDone),
				bytesToSI(pvb.Status.Progress.TotalBytes),
				getPVBDuration(&pvb))
		default:
			msg = fmt.Sprintf(
				"PodVolumeBackup %s/%s: Waiting for ongoing volume backup(s) to complete",
				pvb.Namespace,
				pvb.Name)
		}
		m[pvb.Namespace+"/"+pvb.Name] = msg
		keys = append(keys, pvb.Namespace+"/"+pvb.Name)
	}
	// sort the progress array to maintain order everytime it's updated
	sort.Strings(keys)
	for _, k := range keys {
		progress = append(progress, m[k])
	}
	return
}

// getBackupStats returns backup progress statistics
func getBackupStats(backup *velero.Backup) (itemsBackedUp int, totalItems int) {
	if backup.Status.Progress == nil {
		return
	}
	totalItems = backup.Status.Progress.TotalItems
	itemsBackedUp = backup.Status.Progress.ItemsBackedUp
	return
}

// Get whether a backup has completed on the source cluster.
func (t *Task) hasBackupCompleted(backup *velero.Backup) (bool, []string) {
	completed := false
	reasons := []string{}
	progress := []string{}

	pvbs := t.getPodVolumeBackupsForBackup(backup)

	switch backup.Status.Phase {
	case velero.BackupPhaseNew:
		progress = append(
			progress,
			fmt.Sprintf(
				"Backup %s/%s: Not started",
				backup.Namespace,
				backup.Name))
	case velero.BackupPhaseInProgress:
		itemsBackedUp, totalItems := getBackupStats(backup)
		progress = append(
			progress,
			fmt.Sprintf(
				"Backup %s/%s: %d out of estimated total of %d objects backed up%s",
				backup.Namespace,
				backup.Name,
				itemsBackedUp,
				totalItems,
				getBackupDuration(backup)))
		progress = append(
			progress,
			getPodVolumeBackupsProgress(pvbs)...)
	case velero.BackupPhaseCompleted:
		completed = true
		itemsBackedUp, totalItems := getBackupStats(backup)
		progress = append(
			progress,
			fmt.Sprintf(
				"Backup %s/%s: %d out of estimated total of %d objects backed up%s",
				backup.Namespace,
				backup.Name,
				itemsBackedUp,
				totalItems,
				getBackupDuration(backup)))
		progress = append(
			progress,
			getPodVolumeBackupsProgress(pvbs)...)
	case velero.BackupPhaseFailed:
		completed = true
		message := fmt.Sprintf(
			"Backup %s/%s: failed.",
			backup.Namespace,
			backup.Name)
		reasons = append(reasons, message)
		itemsBackedUp, totalItems := getBackupStats(backup)
		message = fmt.Sprintf(
			"%s %d out of estimated total of %d objects backed up%s",
			message,
			itemsBackedUp,
			totalItems,
			getBackupDuration(backup))
		progress = append(progress, message)
		progress = append(
			progress,
			getPodVolumeBackupsProgress(pvbs)...)
	case velero.BackupPhasePartiallyFailed:
		completed = true
		itemsBackedUp, totalItems := getBackupStats(backup)
		message := fmt.Sprintf(
			"Backup %s/%s: partially failed. %d out of estimated total of %d objects backed up%s",
			backup.Namespace,
			backup.Name,
			itemsBackedUp,
			totalItems,
			getBackupDuration(backup))
		progress = append(progress, message)
		progress = append(
			progress,
			getPodVolumeBackupsProgress(pvbs)...)
	case velero.BackupPhaseFailedValidation:
		reasons = backup.Status.ValidationErrors
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup %s/%s: validation failed.",
				backup.Namespace,
				backup.Name))
		completed = true
	}
	t.Log.Info("Velero Backup progress report",
		"backup", path.Join(backup.Namespace, backup.Name),
		"backupProgress", progress)

	t.setProgress(progress)
	return completed, reasons
}

// Set warning conditions on migmigration if there were any partial failures
func (t *Task) setInitialBackupPartialFailureWarning(backup *velero.Backup) {
	if backup.Status.Phase == velero.BackupPhasePartiallyFailed {
		message := fmt.Sprintf(
			"Backup %s/%s: partially failed on source cluster", backup.GetNamespace(), backup.GetName())
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     VeleroInitialBackupPartiallyFailed,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
}

// Set warning conditions on migmigration if there were any partial failures
func (t *Task) setStageBackupPartialFailureWarning(backup *velero.Backup) {
	if backup.Status.Phase == velero.BackupPhasePartiallyFailed {
		message := fmt.Sprintf(
			"Stage Backup %s/%s: partially failed on source cluster", backup.GetNamespace(), backup.GetName())
		t.Log.Info(message)
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     VeleroStageBackupPartiallyFailed,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
}

// Get the existing BackupStorageLocation on the source cluster.
func (t *Task) getBSL() (*velero.BackupStorageLocation, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	plan := t.PlanResources.MigPlan
	location, err := plan.GetBSL(client)
	if err != nil {
		return nil, err
	}
	if location == nil {
		return nil, errors.New("BSL not found")
	}

	return location, nil
}

// Get the existing VolumeSnapshotLocation on the source cluster
func (t *Task) getVSL() (*velero.VolumeSnapshotLocation, error) {
	client, err := t.getSourceClient()
	if err != nil {
		return nil, err
	}
	plan := t.PlanResources.MigPlan
	location, err := plan.GetVSL(client)
	if err != nil {
		return nil, err
	}
	if location == nil {
		return nil, errors.New("VSL not found")
	}

	return location, nil
}

// Build a Backups as desired for the source cluster.
func (t *Task) buildBackup(client k8sclient.Client) (*velero.Backup, error) {
	var includeClusterResources *bool = nil
	annotations, err := t.getAnnotations(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	backupLocation, err := t.getBSL()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	snapshotLocation, err := t.getVSL()
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	backup := &velero.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    migapi.VeleroNamespace,
			Annotations:  annotations,
		},
		Spec: velero.BackupSpec{
			IncludeClusterResources: includeClusterResources,
			StorageLocation:         backupLocation.Name,
			VolumeSnapshotLocations: []string{snapshotLocation.Name},
			TTL:                     metav1.Duration{Duration: 720 * time.Hour},
			IncludedNamespaces:      t.sourceNamespaces(),
			Hooks: velero.BackupHooks{
				Resources: []velero.BackupResourceHookSpec{},
			},
		},
	}
	return backup, nil
}

// Delete all Velero Backups correlated with the running MigPlan
func (t *Task) deleteCorrelatedBackups() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	list := velero.BackupList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(t.PlanResources.MigPlan.GetCorrelationLabels()),
		&list)
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, backup := range list.Items {
		request := &velero.DeleteBackupRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    migapi.VeleroNamespace,
				GenerateName: backup.Name + "-",
			},
			Spec: velero.DeleteBackupRequestSpec{
				BackupName: backup.Name,
			},
		}
		t.Log.Info("Deleting Velero Backup on source cluster "+
			"due to correlation with MigPlan",
			"backup", path.Join(backup.Namespace, backup.Name))
		if err := client.Create(context.TODO(), request); err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Delete stale Velero Backups/Restores, set condition indicating
// Velero restart should be performed later if anything was deleted.
func (t *Task) deleteStaleVeleroCRs() error {
	// Delete source cluster Velero Backups and Restores
	srcCluster := t.PlanResources.SrcMigCluster
	nBackupsDeleted, nInProgressBackupsDeleted, err := t.deleteStaleBackupsOnCluster(srcCluster)
	if err != nil {
		return liberr.Wrap(err)
	}
	nRestoresDeleted, nInProgressRestoresDeleted, err := t.deleteStaleRestoresOnCluster(srcCluster)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Only set condition if 'InProgress' Backups/Restores were deleted
	if nInProgressBackupsDeleted > 0 || nInProgressRestoresDeleted > 0 {
		msg := fmt.Sprintf(
			"Deleted stale Velero CRs: %v Backups, %v Restores. "+
				"Velero will be restarted on source MigCluster %v/%v.",
			nBackupsDeleted, nRestoresDeleted, srcCluster.Namespace, srcCluster.Name)
		t.Log.Info(msg)
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StaleSrcVeleroCRsDeleted,
			Status:   True,
			Category: migapi.Required,
			Message:  msg,
			Durable:  true,
		})
	}

	// Delete target cluster Velero Backups and Restores
	dstCluster := t.PlanResources.DestMigCluster
	nBackupsDeleted, nInProgressBackupsDeleted, err = t.deleteStaleBackupsOnCluster(dstCluster)
	if err != nil {
		return liberr.Wrap(err)
	}
	nRestoresDeleted, nInProgressRestoresDeleted, err = t.deleteStaleRestoresOnCluster(dstCluster)
	if err != nil {
		return liberr.Wrap(err)
	}
	// Only set condition if 'InProgress' Backups/Restores were deleted
	if nInProgressBackupsDeleted > 0 || nInProgressRestoresDeleted > 0 {
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StaleDestVeleroCRsDeleted,
			Status:   True,
			Category: migapi.Required,
			Message: fmt.Sprintf("Deleted stale Velero CRs: %v Backups, %v Restores. "+
				"Velero will be restarted on target MigCluster %v/%v.",
				nBackupsDeleted, nRestoresDeleted, dstCluster.Namespace, dstCluster.Name),
			Durable: true,
		})
	}
	return nil
}

// Delete stale Restic CRs, set condition indicating
// Restic Pod restarts may be needed, but we leave this up to the user.
func (t *Task) deleteStaleResticCRs() error {
	pvbsDeleted := 0
	pvrsDeleted := 0
	for _, cluster := range t.getBothClusters() {
		clusterPVBsDeleted, err := t.deleteStalePVBsOnCluster(cluster)
		if err != nil {
			return liberr.Wrap(err)
		}
		pvbsDeleted += clusterPVBsDeleted

		clusterPVRsDeleted, err := t.deleteStalePVRsOnCluster(cluster)
		if err != nil {
			return liberr.Wrap(err)
		}
		pvrsDeleted += clusterPVRsDeleted
	}

	if pvbsDeleted > 0 || pvrsDeleted > 0 {
		msg := fmt.Sprintf("Deleted stale Restic CRs: "+
			"%v PodVolumeBackups,  %v PodVolumeRestores. ",
			pvbsDeleted, pvrsDeleted)
		t.Log.Info(msg)

		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     StaleResticCRsDeleted,
			Status:   True,
			Category: migapi.Required,
			Message:  msg,
			Durable:  true,
		})
	}
	return nil
}

func (t *Task) migrationUIDisRunning(migrationUID string) (bool, error) {
	t.Log.Info("Checking if migration with UID is running",
		"migrationUID", migrationUID)
	corrKey, _ := t.Owner.GetCorrelationLabel()
	corrLabel := map[string]string{corrKey: migrationUID}
	migrationList := migapi.MigMigrationList{}
	err := t.Client.List(
		context.TODO(),
		k8sclient.MatchingLabels(corrLabel),
		&migrationList,
	)
	if err != nil {
		return false, liberr.Wrap(err)
	}
	migrationIsRunning := false
	for _, migration := range migrationList.Items {
		if migration.Status.Phase != Completed {
			t.Log.Info("Migration IS running.",
				"migration", path.Join(migration.Namespace, migration.Name),
				"migrationUID", migrationUID,
				"migrationPhase", migration.Status.Phase)
			migrationIsRunning = true
		} else {
			t.Log.Info("Migration is NOT running.",
				"migration", path.Join(migration.Namespace, migration.Name),
				"migrationUID", migrationUID,
				"migrationPhase", migration.Status.Phase)
		}
	}

	return migrationIsRunning, nil
}

// Delete stale Velero Backups in the controller namespace to empty
// the work queue for next migration.
func (t *Task) deleteStaleBackupsOnCluster(cluster *migapi.MigCluster) (int, int, error) {
	t.Log.Info("Checking for stale Velero Backups on MigCluster",
		"migCluster", path.Join(cluster.Namespace, cluster.Name))
	nDeleted := 0
	nInProgressDeleted := 0

	clusterClient, err := cluster.GetClient(t.Client)
	if err != nil {
		return 0, 0, liberr.Wrap(err)
	}

	list := velero.BackupList{}
	err = clusterClient.List(
		context.TODO(),
		k8sclient.InNamespace(migapi.VeleroNamespace),
		&list)
	if err != nil {
		return 0, 0, liberr.Wrap(err)
	}
	for _, backup := range list.Items {
		// Skip delete unless phase is "", "New" or "InProgress"
		if backup.Status.Phase != velero.BackupPhaseNew &&
			backup.Status.Phase != velero.BackupPhaseInProgress &&
			backup.Status.Phase != "" {
			t.Log.V(4).Info("Backup phase is not 'New' or 'InProgress'. Skipping deletion.",
				"backup", path.Join(backup.Namespace, backup.Name),
				"backupPhase", backup.Status.Phase)
			continue
		}
		// Skip if missing a migmigration correlation label (only delete our own CRs)
		// Example 'migmigration: 4c9d317f-f410-430b-af8f-4ecd7d17a7de'
		corrKey, _ := t.Owner.GetCorrelationLabel()
		migMigrationUID, ok := backup.ObjectMeta.Labels[corrKey]
		if !ok {
			t.Log.V(4).Info("Backup  does not have an attached label "+
				"associating it with a MigMigration. Skipping deletion.",
				"backup", path.Join(backup.Namespace, backup.Name),
				"backupPhase", backup.Status.Phase,
				"associationLabel", corrKey)
			continue
		}
		// Skip if correlation label points to an existing, running migration
		isRunning, err := t.migrationUIDisRunning(migMigrationUID)
		if err != nil {
			return nDeleted, nInProgressDeleted, liberr.Wrap(err)
		}
		if isRunning {
			t.Log.Info("Backup is running. Skipping deletion.",
				"backup", path.Join(backup.Namespace, backup.Name),
				"backupPhase", backup.Status.Phase)
			continue
		}
		// Submit a request to remove backup assets from storage
		// Note, this will probably be a no-op. See velero backup_deletion_controller.go
		deleteBackupRequest := &velero.DeleteBackupRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    migapi.VeleroNamespace,
				GenerateName: backup.Name + "-",
			},
			Spec: velero.DeleteBackupRequestSpec{
				BackupName: backup.Name,
			},
		}
		t.Log.Info("CREATING Velero DeleteBackupRequest for Backup on MigCluster",
			"backup", path.Join(backup.Namespace, backup.Name),
			"backupPhase", backup.Status.Phase,
			"migCluster", path.Join(cluster.Namespace, cluster.Name))
		err = clusterClient.Create(context.TODO(), deleteBackupRequest)
		if err != nil {
			return nDeleted, nInProgressDeleted, liberr.Wrap(err)
		}
		// Also delete the backup CR directly
		// This should work since backup is still in-progress.
		t.Log.Info("DELETING stale Velero Backup from MigCluster",
			"backup", path.Join(backup.Namespace, backup.Name),
			"backupPhase", backup.Status.Phase,
			"migCluster", path.Join(cluster.Namespace, cluster.Name))
		err = clusterClient.Delete(context.TODO(), &backup)
		if err != nil && !k8serrors.IsNotFound(err) {
			return nDeleted, nInProgressDeleted, liberr.Wrap(err)
		}
		nDeleted++
		// Separate count for InProgress, used to determine if restart needed
		if backup.Status.Phase == velero.BackupPhaseInProgress {
			nInProgressDeleted++
		}
	}

	return nDeleted, nInProgressDeleted, err
}

// Delete stale Velero PodVolumeBackups in the controller namespace to empty
// the work queue for next migration.
func (t *Task) deleteStalePVBsOnCluster(cluster *migapi.MigCluster) (int, error) {
	t.Log.Info("Checking for stale PodVolumeBackups on MigCluster",
		"migCluster", path.Join(cluster.Namespace, cluster.Name))
	nDeleted := 0
	clusterClient, err := cluster.GetClient(t.Client)
	if err != nil {
		return 0, liberr.Wrap(err)
	}

	list := velero.PodVolumeBackupList{}
	err = clusterClient.List(
		context.TODO(),
		k8sclient.InNamespace(migapi.VeleroNamespace),
		&list)
	if err != nil {
		return 0, liberr.Wrap(err)
	}
	for _, pvb := range list.Items {
		// Skip delete unless phase is "", "New" or "InProgress"
		if pvb.Status.Phase != velero.PodVolumeBackupPhaseNew &&
			pvb.Status.Phase != velero.PodVolumeBackupPhaseInProgress &&
			pvb.Status.Phase != "" {
			t.Log.V(4).Info("PodVolumeBackup with is not 'New' or 'InProgress'. Skipping deletion.",
				"podVolumeBackup", path.Join(pvb.Namespace, pvb.Name),
				"podVolumeBackupPhase", pvb.Status.Phase)
			continue
		}
		// Skip delete if PVB is associated with running migration
		pvbHasRunningMigration := false
		for _, ownerRef := range pvb.OwnerReferences {
			if ownerRef.Kind != "Backup" {
				t.Log.V(4).Info("PodVolumeBackup does not have an OwnerRef associated "+
					"with a Velero Backup. Skipping deletion.",
					"podVolumeBackup", path.Join(pvb.Namespace, pvb.Name),
					"podVolumeBackupPhase", pvb.Status.Phase)
				continue
			}
			backup := velero.Backup{}
			err := clusterClient.Get(
				context.TODO(),
				types.NamespacedName{
					Namespace: migapi.VeleroNamespace,
					Name:      ownerRef.Name,
				},
				&backup,
			)
			if err != nil {
				return nDeleted, liberr.Wrap(err)
			}
			// Skip delete if missing a migmigration correlation label (only delete our own CRs)
			// Example 'migmigration: 4c9d317f-f410-430b-af8f-4ecd7d17a7de'
			corrKey, _ := t.Owner.GetCorrelationLabel()
			migMigrationUID, ok := backup.ObjectMeta.Labels[corrKey]
			if !ok {
				t.Log.V(4).Info("PodVolumeBackup does not have an attached label "+
					"associating it with a MigMigration. Skipping deletion.",
					"podVolumeBackup", path.Join(pvb.Namespace, pvb.Name),
					"podVolumeBackupPhase", pvb.Status.Phase,
					"associationLabel", corrKey)
				continue
			}
			isRunning, err := t.migrationUIDisRunning(migMigrationUID)
			if err != nil {
				return nDeleted, liberr.Wrap(err)
			}
			if isRunning {
				pvbHasRunningMigration = true
			}
		}
		if pvbHasRunningMigration == true {
			t.Log.Info("PodVolumeBackup with is associated with a running migration. Skipping deletion.",
				"podVolumeBackup", path.Join(pvb.Namespace, pvb.Name),
				"podVolumeBackupPhase", pvb.Status.Phase)
			continue
		}

		// Delete the PVB
		t.Log.Info("DELETING stale Velero PodVolumeBackup from MigCluster",
			"podVolumeBackup", path.Join(pvb.Namespace, pvb.Name),
			"podVolumeBackupPhase", pvb.Status.Phase,
			"migCluster", path.Join(cluster.Namespace, cluster.Name))
		err = clusterClient.Delete(context.TODO(), &pvb)
		if err != nil && !k8serrors.IsNotFound(err) {
			return nDeleted, liberr.Wrap(err)
		}
		nDeleted++
	}

	return nDeleted, nil
}

// Determine whether backups are replicated by velero on the destination cluster.
func (t *Task) isBackupReplicated(backup *velero.Backup) (bool, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}
	replicated := velero.Backup{}
	t.Log.Info("Checking if Velero Backup has been replicated to destination cluster",
		"backup", path.Join(backup.Namespace, backup.Name))
	err = client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		&replicated)
	if err == nil {
		t.Log.Info("FOUND Velero Backup has been replicated to destination cluster",
			"backup", path.Join(replicated.Namespace, replicated.Name))
		return true, nil
	}
	if k8serrors.IsNotFound(err) {
		err = nil
	}
	return false, err
}

func findPVAction(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.Action
		}
	}
	return ""
}

func findPVStorageClass(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.StorageClass
		}
	}
	return ""
}

func findPVAccessMode(pvList migapi.PersistentVolumes, pvName string) corev1.PersistentVolumeAccessMode {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.AccessMode
		}
	}
	return ""
}

func findPVCopyMethod(pvList migapi.PersistentVolumes, pvName string) string {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.CopyMethod
		}
	}
	return ""
}

func findPVVerify(pvList migapi.PersistentVolumes, pvName string) bool {
	for _, pv := range pvList.List {
		if pv.Name == pvName {
			return pv.Selection.Verify
		}
	}
	return false
}

// converts raw 'bytes' to nearest possible SI unit
// with a precision of 2 decimal digits
func bytesToSI(bytes int64) string {
	const baseUnit = 1000
	if bytes < baseUnit {
		return fmt.Sprintf("%d bytes", bytes)
	}
	const siUnits = "kMGTPE"
	div, exp := int64(baseUnit), 0
	for n := bytes / baseUnit; n >= baseUnit; n /= baseUnit {
		div *= baseUnit
		exp++
	}
	return fmt.Sprintf("%.2f %cB",
		float64(bytes)/float64(div), siUnits[exp])
}
