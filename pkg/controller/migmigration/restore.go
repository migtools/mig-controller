package migmigration

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const podStageLabel = "migration-stage-pod"

// Ensure the final restore on the destination cluster has been
// created  and has the proper settings.
func (t *Task) ensureFinalRestore() error {
	includeClusterResources := false
	newRestore, err := t.buildRestore(&includeClusterResources)
	if err != nil {
		log.Trace(err)
		return err
	}
	foundRestore, err := t.getRestore(false)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundRestore == nil {
		t.FinalRestore = newRestore
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = client.Create(context.TODO(), newRestore)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	t.FinalRestore = foundRestore
	if !t.equalsRestore(newRestore, foundRestore) {
		t.updateRestore(foundRestore, t.InitialBackup.Name)
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = client.Update(context.TODO(), foundRestore)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Ensure the first restore on the destination cluster has been
// created  and has the proper settings.
func (t *Task) ensureCopyRestore() error {
	newRestore, err := t.buildRestore(nil)
	if err != nil {
		log.Trace(err)
		return err
	}
	foundRestore, err := t.getRestore(true)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundRestore == nil {
		newRestore.Spec.BackupName = t.CopyBackup.Name
		t.CopyRestore = newRestore
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = client.Create(context.TODO(), newRestore)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	t.CopyRestore = foundRestore
	if !t.equalsRestore(newRestore, foundRestore) {
		t.updateRestore(foundRestore, t.CopyBackup.Name)
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return err
		}
		err = client.Update(context.TODO(), foundRestore)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	return nil
}

// Get whether the two Restores are equal.
func (t *Task) equalsRestore(a, b *velero.Restore) bool {
	match := a.Spec.BackupName == b.Spec.BackupName &&
		*a.Spec.RestorePVs == *b.Spec.RestorePVs
	return match
}

// Get an existing Restore on the destination cluster.
func (t Task) getRestore(copyRestore bool) (*velero.Restore, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	list := velero.RestoreList{}
	labels := t.Owner.GetCorrelationLabels()
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		return nil, err
	}
	// Find proper restore to return
	for i, restore := range list.Items {
		// Avoid nil dereference
		if restore.Annotations == nil {
			restore.Annotations = make(map[string]string)
		}
		if restore.Annotations[copyBackupRestoreAnnotationKey] != "" && copyRestore {
			return &list.Items[i], nil
		}
		if restore.Annotations[copyBackupRestoreAnnotationKey] == "" && !copyRestore {
			return &list.Items[i], nil
		}
	}

	return nil, nil
}

// Build a Restore as desired for the destination cluster.
func (t *Task) buildRestore(includeClusterResources *bool) (*velero.Restore, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	annotations, err := t.getAnnotations(client)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	// Set it to copy backup name since initial backup isn't set on stage
	backupName := t.CopyBackup.Name
	// If includeClusterResources isn't set, this means it is first restore to
	// satisfy moving the persistent storage over
	if includeClusterResources == nil {
		backupName = t.CopyBackup.Name
		annotations[copyBackupRestoreAnnotationKey] = "true"
	} else {
		backupName = t.InitialBackup.Name
		delete(annotations, copyBackupRestoreAnnotationKey)
	}
	restore := &velero.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    migapi.VeleroNamespace,
			Annotations:  annotations,
		},
	}
	t.updateRestore(restore, backupName)
	return restore, nil
}

// Update a Restore as desired for the destination cluster.
func (t *Task) updateRestore(restore *velero.Restore, backupName string) {
	restorePVs := true
	restore.Spec = velero.RestoreSpec{
		BackupName: backupName,
		RestorePVs: &restorePVs,
	}
}
