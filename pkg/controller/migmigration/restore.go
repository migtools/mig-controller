package migmigration

import (
	"context"
	velero "github.com/heptio/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the restore on the destination cluster has been
// created  and has the proper settings.
func (t *Task) ensureRestore() error {
	newRestore, err := t.buildRestore()
	if err != nil {
		return err
	}
	foundRestore, err := t.getRestore()
	if err != nil {
		return err
	}
	if foundRestore == nil {
		t.Restore = newRestore
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
	t.Restore = foundRestore
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
func (t Task) getRestore() (*velero.Restore, error) {
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
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, nil
}

// Build a Restore as desired for the destination cluster.
func (t *Task) buildRestore() (*velero.Restore, error) {
	annotations, err := t.getAnnotations(t.DestRegistryResources)
	if err != nil {
		return nil, err
	}
	restore := &velero.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       t.Owner.GetCorrelationLabels(),
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    VeleroNamespace,
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
		BackupName: t.Backup.Name,
		RestorePVs: &restorePVs,
	}
}
