package migmigration

import (
	"context"
	"fmt"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/go-logr/logr"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Annotation Keys
const (
	MigQuiesceAnnotationKey        = "openshift.io/migrate-quiesce-pods"
	copyBackupRestoreAnnotationKey = "openshift.io/copy-backup-restore"
)

// Phases
const (
	Started                 = "Started"
	WaitOnResticRestart     = "WaitOnResticRestart"
	ResticRestartCompleted  = "ResticRestartCompleted"
	InitialBackupStarted    = "InitialBackupStarted"
	InitialBackupCompleted  = "InitialBackupCompleted"
	InitialBackupFailed     = "InitialBackupFailed"
	CopyBackupStarted       = "CopyBackupStarted"
	CopyBackupCompleted     = "CopyBackupCompleted"
	CopyBackupFailed        = "CopyBackupFailed"
	CreatingStagePods       = "CreatingStagePods"
	StagePodsCreated        = "StagePodsCreated"
	WaitOnBackupReplication = "WaitOnBackupReplication"
	BackupReplicated        = "BackupReplicated"
	CopyRestoreStarted      = "CopyRestoreStarted"
	CopyRestoreCompleted    = "CopyRestoreCompleted"
	CopyRestoreFailed       = "CopyRestoreFailed"
	DeletingStagePods       = "DeletingStagePods"
	StagePodsDeleted        = "StagePodsDeleted"
	FinalRestoreStarted     = "FinalRestoreStarted"
	FinalRestoreCompleted   = "FinalRestoreCompleted"
	FinalRestoreFailed      = "FinalRestoreFailed"
	Completed               = "Completed"
)

// A Velero task that provides the complete backup & restore workflow.
// Log - A controller's logger.
// Client - A controller's (local) client.
// Owner - A MigMigration resource.
// PlanResources - A PlanRefResources.
// Annotations - Map of annotations to applied to the backup & restore
// BackupResources - Resource types to be included in the backup.
// Phase - The task phase.
// Errors - Migration errors.
// Backup - A Backup created on the source cluster.
// Restore - A Restore created on the destination cluster.
type Task struct {
	Log             logr.Logger
	Client          k8sclient.Client
	Owner           *migapi.MigMigration
	PlanResources   *migapi.PlanResources
	Annotations     map[string]string
	BackupResources []string
	Phase           string
	Errors          []string
	InitialBackup   *velero.Backup
	CopyBackup      *velero.Backup
	CopyRestore     *velero.Restore
	FinalRestore    *velero.Restore
}

// Reconcile() Example:
//
// task := Task{
//     Log: log,
//     Client: r,
//     Owner: migration,
//     PlanResources: plan.GetPlanResources(),
// }
//
// err := task.Run()
// switch task.Phase {
//     case Complete:
//         ...
// }
//

// Run the task.
// Return `true` when run to completion.
func (t *Task) Run() error {
	t.logEnter()
	defer t.logExit()

	// Started
	if t.Phase == "" {
		t.Phase = Started
	}

	// Mount propagation workaround
	// TODO: Only bounce restic pod if cluster version is 3.7-3.9,
	// would require passing in cluster version to the controller.
	err := t.bounceResticPod()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Return unless restic restart has finished
	if t.Phase == Started || t.Phase == WaitOnResticRestart {
		return nil
	}

	// Run initial Backup if this is not a stage
	// The initial backup captures the state of the applications on the source
	// cluster while explicity setting `includePersistentVolumes` value to
	// `false`. This will capture everything except PVs
	if !t.stage() {
		err = t.ensureInitialBackup()
		if err != nil {
			log.Trace(err)
			return err
		}
		switch t.InitialBackup.Status.Phase {
		case velero.BackupPhaseCompleted:
			t.Phase = InitialBackupCompleted
		case velero.BackupPhaseFailed:
			reason := fmt.Sprintf(
				"Backup: %s/%s failed.",
				t.InitialBackup.Namespace,
				t.InitialBackup.Name)
			t.addErrors([]string{reason})
			t.Phase = InitialBackupFailed
			return nil
		case velero.BackupPhasePartiallyFailed:
			reason := fmt.Sprintf(
				"Backup: %s/%s partially failed.",
				t.InitialBackup.Namespace,
				t.InitialBackup.Name)
			t.addErrors([]string{reason})
			t.Phase = InitialBackupFailed
			return nil
		case velero.BackupPhaseFailedValidation:
			t.addErrors(t.InitialBackup.Status.ValidationErrors)
			t.Phase = InitialBackupFailed
			return nil
		default:
			t.Phase = InitialBackupStarted
			return nil
		}
	}

	t.Phase = InitialBackupCompleted

	// Annotate persistent storage resources with PV actions
	// This will also return the number of pods we have annotated to be backed up
	// by restic. This is useful for knowing whether or not we need to create
	// staging pods
	err, resticAnnotationCount := t.annotateStorageResources()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Check if stage pods are created and running
	created, err := t.areStagePodsCreated()
	if err != nil {
		log.Trace(err)
		return err
	}

	// If all stage pods are created and running OR there are no stage pods we
	// need to create, continue
	if created || resticAnnotationCount == 0 {
		t.Phase = StagePodsCreated
	} else if t.Owner.Annotations["openshift.io/stage-completed"] == "" {
		t.Phase = CreatingStagePods
		// Swap out all copy pods with dummy pods
		err = t.createStagePods()
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}

	// Scale down all owning resources
	if t.quiesce() {
		err = t.quiesceApplications()
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	// Run second backup to copy PV data
	err = t.ensureCopyBackup()
	if err != nil {
		log.Trace(err)
		return err
	}

	switch t.CopyBackup.Status.Phase {
	case velero.BackupPhaseCompleted:
		t.Phase = CopyBackupCompleted
	case velero.BackupPhaseFailed:
		reason := fmt.Sprintf(
			"Backup: %s/%s failed.",
			t.CopyBackup.Namespace,
			t.CopyBackup.Name)
		t.addErrors([]string{reason})
		t.Phase = CopyBackupFailed
		return nil
	case velero.BackupPhasePartiallyFailed:
		reason := fmt.Sprintf(
			"Backup: %s/%s partially failed.",
			t.CopyBackup.Namespace,
			t.CopyBackup.Name)
		t.addErrors([]string{reason})
		t.Phase = CopyBackupFailed
		return nil
	case velero.BackupPhaseFailedValidation:
		t.addErrors(t.CopyBackup.Status.ValidationErrors)
		t.Phase = CopyBackupFailed
		return nil
	default:
		t.Phase = CopyBackupStarted
		return nil
	}

	// Delete storage annotations
	err = t.removeStorageResourceAnnotations()
	if err != nil {
		log.Trace(err)
		return err
	}

	// Wait on Backup replication.
	t.Phase = WaitOnBackupReplication
	backup, err := t.getReplicatedBackup()
	if err != nil {
		log.Trace(err)
		return err
	}
	if backup != nil {
		t.Phase = BackupReplicated
	} else {
		return nil
	}

	// Copy restore
	err = t.ensureCopyRestore()
	if err != nil {
		log.Trace(err)
		return err
	}

	switch t.CopyRestore.Status.Phase {
	case velero.RestorePhaseCompleted:
		t.Phase = CopyRestoreCompleted
	case velero.RestorePhaseFailedValidation:
		t.addErrors(t.CopyRestore.Status.ValidationErrors)
		t.Phase = CopyRestoreFailed
		return nil
	case velero.RestorePhaseFailed:
		reason := fmt.Sprintf(
			"Restore: %s/%s failed.",
			t.CopyRestore.Namespace,
			t.CopyRestore.Name)
		t.addErrors([]string{reason})
		t.Phase = CopyRestoreFailed
		return nil
	case velero.RestorePhasePartiallyFailed:
		reason := fmt.Sprintf(
			"Restore: %s/%s partially failed.",
			t.CopyRestore.Namespace,
			t.CopyRestore.Name)
		t.addErrors([]string{reason})
		t.Phase = CopyRestoreFailed
		return nil
	default:
		t.Phase = CopyRestoreStarted
		return nil
	}
	t.Phase = CopyRestoreCompleted

	deleted, err := t.areStagePodsDeleted()
	if err != nil {
		log.Trace(err)
		return err
	}

	if deleted {
		t.Phase = StagePodsDeleted
	} else {
		t.Phase = DeletingStagePods
		// Remove staging pods on source and destination cluster
		err = t.removeStagePods()
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}

	// Final Restore if not a stage
	if !t.stage() {
		err = t.ensureFinalRestore()
		if err != nil {
			log.Trace(err)
			return err
		}
		switch t.FinalRestore.Status.Phase {
		case velero.RestorePhaseCompleted:
			t.Phase = FinalRestoreCompleted
		case velero.RestorePhaseFailedValidation:
			t.addErrors(t.FinalRestore.Status.ValidationErrors)
			t.Phase = FinalRestoreFailed
			return nil
		case velero.RestorePhasePartiallyFailed:
			reason := fmt.Sprintf(
				"Restore: %s/%s partially failed.",
				t.FinalRestore.Namespace,
				t.FinalRestore.Name)
			t.addErrors([]string{reason})
			t.Phase = FinalRestoreFailed
			return nil
		case velero.RestorePhaseFailed:
			reason := fmt.Sprintf(
				"Restore: %s/%s failed.",
				t.FinalRestore.Namespace,
				t.FinalRestore.Name)
			t.addErrors([]string{reason})
			t.Phase = FinalRestoreFailed
			return nil
		default:
			t.Phase = FinalRestoreStarted
			return nil
		}
	}
	t.Phase = FinalRestoreCompleted

	// Done
	t.Phase = Completed

	return nil
}

// Get a client for the source cluster.
func (t *Task) getSourceClient() (k8sclient.Client, error) {
	return t.PlanResources.SrcMigCluster.GetClient(t.Client)
}

// Get a client for the destination cluster.
func (t *Task) getDestinationClient() (k8sclient.Client, error) {
	return t.PlanResources.DestMigCluster.GetClient(t.Client)
}

// Get whether the migration is stage.
func (t *Task) stage() bool {
	return t.Owner.Spec.Stage
}

// Get whether to quiesce pods.
func (t *Task) quiesce() bool {
	return t.Owner.Spec.QuiescePods
}

// Log task start/resumed.
func (t *Task) logEnter() {
	if t.Phase == Started {
		t.Log.Info(
			"Migration: started.",
			"name",
			t.Owner.Name,
			"stage",
			t.stage())
		return
	}
	t.Log.Info(
		"Migration: resumed.",
		"name",
		t.Owner.Name,
		"phase",
		t.Phase)
}

// Log task exit/interrupted.
func (t *Task) logExit() {
	if t.Phase == Completed {
		t.Log.Info("Migration completed.")
		return
	}
	backup := ""
	restore := ""
	if t.InitialBackup != nil {
		backup = t.InitialBackup.Name
	}
	if t.FinalRestore != nil {
		restore = t.FinalRestore.Name
	}
	t.Log.Info(
		"Migration: waiting.",
		"name",
		t.Owner.Name,
		"phase", t.Phase,
		"backup",
		backup,
		"restore",
		restore)
}

// Add errors.
func (t *Task) addErrors(errors []string) {
	for _, error := range errors {
		t.Errors = append(t.Errors, error)
	}
}

// Remove stage pods on source and destination cluster
func (t *Task) removeStagePods() error {
	srcClient, err := t.getSourceClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	// Find all stage pods
	uniqueBackupLabelKey := fmt.Sprintf("%s-%s-copy", pvBackupLabelKey, t.PlanResources.MigPlan.UID)
	labelSelector := map[string]string{
		uniqueBackupLabelKey: "true",
	}
	options := k8sclient.MatchingLabels(labelSelector)
	podList := corev1.PodList{}
	err = srcClient.List(context.TODO(), options, &podList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, pod := range podList.Items {
		err = srcClient.Delete(
			context.TODO(),
			&pod)
		if err != nil {
			log.Trace(err)
			return err
		}
	}
	podList = corev1.PodList{}
	err = destClient.List(context.TODO(), options, &podList)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, pod := range podList.Items {
		err = destClient.Delete(
			context.TODO(),
			&pod)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

func (t *Task) areStagePodsDeleted() (bool, error) {
	srcClient, err := t.getSourceClient()
	if err != nil {
		return false, err
	}
	destClient, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}
	uniqueBackupLabelKey := fmt.Sprintf("%s-%s-copy", pvBackupLabelKey, t.PlanResources.MigPlan.UID)
	labelSelector := map[string]string{
		uniqueBackupLabelKey: "true",
	}
	options := k8sclient.MatchingLabels(labelSelector)
	podList := corev1.PodList{}
	// Find all stage pods on source cluster
	err = srcClient.List(context.TODO(), options, &podList)
	if err != nil {
		return false, err
	}
	if len(podList.Items) != 0 {
		return false, nil
	}
	// Reset podlist
	podList = corev1.PodList{}
	err = destClient.List(context.TODO(), options, &podList)
	if err != nil {
		return false, err
	}
	if len(podList.Items) != 0 {
		return false, nil
	}

	return true, nil
}
