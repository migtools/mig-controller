package migmigration

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	core "k8s.io/api/core/v1"
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
		t.updateRestore(foundRestore)
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
		return err
	}
	foundRestore, err := t.getRestore(true)
	if err != nil {
		return err
	}
	if foundRestore == nil {
		newRestore.Spec.BackupName = t.CopyBackup.Name
		t.CopyRestore = newRestore
		client, err := t.getDestinationClient()
		if err != nil {
			return err
		}
		err = client.Create(context.TODO(), newRestore)
		if err != nil {
			return err
		}
		return nil
	}
	t.CopyRestore = foundRestore
	if !t.equalsRestore(newRestore, foundRestore) {
		t.updateRestore(foundRestore)
		client, err := t.getDestinationClient()
		if err != nil {
			return err
		}
		err = client.Update(context.TODO(), foundRestore)
		if err != nil {
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
	if len(list.Items) > 0 {
		for i, restore := range list.Items {
			if restore.Annotations[copyBackupRestoreAnnotationKey] != "" && copyRestore {
				return &list.Items[i], nil
			}
			if restore.Annotations[copyBackupRestoreAnnotationKey] == "" && !copyRestore {
				return &list.Items[i], nil
			}
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
	// If includeClusterResources isn't set, this means it is first restore to
	// satisfy moving the persistent storage over
	if includeClusterResources == nil {
		annotations[copyBackupRestoreAnnotationKey] = "true"
	} else {
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
	t.updateRestore(restore)
	return restore, nil
}

// Update a Restore as desired for the destination cluster.
func (t *Task) updateRestore(restore *velero.Restore) {
	restorePVs := true
	restore.Spec = velero.RestoreSpec{
		BackupName: t.InitialBackup.Name,
		RestorePVs: &restorePVs,
	}
}

// Delete stage pods
func (t *Task) deleteStagePods() error {
	client, err := t.getDestinationClient()
	if err != nil {
		log.Trace(err)
		return err
	}
	// Find all pods matching the podStageLabel
	list := core.PodList{}
	labels := make(map[string]string)
	labels[podStageLabel] = "true"
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		log.Trace(err)
		return err
	}
	// Delete all pods
	for _, pod := range list.Items {
		err = client.Delete(
			context.TODO(),
			&pod)
		if err != nil {
			log.Trace(err)
			return err
		}
	}
	return nil
}
