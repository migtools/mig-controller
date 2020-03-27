package migmigration

import (
	"context"
	"fmt"
	"strings"

	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensure the final restore on the destination cluster has been
// created  and has the proper settings.
func (t *Task) ensureFinalRestore() (*velero.Restore, error) {
	backup, err := t.getInitialBackup()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	if backup == nil {
		return nil, errors.New("Backup not found")
	}

	newRestore, err := t.buildRestore(backup.Name)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	newRestore.Labels[FinalRestoreLabel] = t.UID()
	foundRestore, err := t.getFinalRestore()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	if foundRestore == nil {
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		err = client.Create(context.TODO(), newRestore)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		return newRestore, nil
	}
	if !t.equalsRestore(newRestore, foundRestore) {
		t.updateRestore(foundRestore, backup.Name)
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		err = client.Update(context.TODO(), foundRestore)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
	}

	return foundRestore, nil
}

// Get the final restore on the destination cluster.
func (t *Task) getFinalRestore() (*velero.Restore, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[FinalRestoreLabel] = t.UID()
	return t.getRestore(labels)
}

// Ensure the first restore on the destination cluster has been
// created and has the proper settings.
func (t *Task) ensureStageRestore() (*velero.Restore, error) {
	backup, err := t.getStageBackup()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	if backup == nil {
		return nil, errors.New("Backup not found")
	}

	newRestore, err := t.buildRestore(backup.Name)
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	newRestore.Labels[StageRestoreLabel] = t.UID()
	foundRestore, err := t.getStageRestore()
	if err != nil {
		log.Trace(err)
		return nil, err
	}
	if foundRestore == nil {
		newRestore.Spec.BackupName = backup.Name
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		err = client.Create(context.TODO(), newRestore)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		return newRestore, nil
	}
	if !t.equalsRestore(newRestore, foundRestore) {
		t.updateRestore(foundRestore, backup.Name)
		client, err := t.getDestinationClient()
		if err != nil {
			log.Trace(err)
			return nil, err
		}
		err = client.Update(context.TODO(), foundRestore)
		if err != nil {
			log.Trace(err)
			return nil, err
		}
	}

	return foundRestore, nil
}

// Get the stage restore on the destination cluster.
func (t *Task) getStageRestore() (*velero.Restore, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[StageRestoreLabel] = t.UID()
	return t.getRestore(labels)
}

// Get whether the two Restores are equal.
func (t *Task) equalsRestore(a, b *velero.Restore) bool {
	match := a.Spec.BackupName == b.Spec.BackupName &&
		*a.Spec.RestorePVs == *b.Spec.RestorePVs
	return match
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

// Get whether a resource has completed on the destination cluster.
func (t Task) hasRestoreCompleted(restore *velero.Restore) (bool, []string) {
	completed := false
	reasons := []string{}
	switch restore.Status.Phase {
	case velero.RestorePhaseCompleted:
		completed = true
	case velero.RestorePhaseFailed:
		completed = true
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Restore: %s/%s partially failed.",
				restore.Namespace,
				restore.Name))
	case velero.RestorePhasePartiallyFailed:
		completed = true
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Restore: %s/%s partially failed.",
				restore.Namespace,
				restore.Name))
	case velero.RestorePhaseFailedValidation:
		reasons = restore.Status.ValidationErrors
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Restore: %s/%s validation failed.",
				restore.Namespace,
				restore.Name))
		completed = true
	}

	return completed, reasons
}

// Build a Restore as desired for the destination cluster.
func (t *Task) buildRestore(backupName string) (*velero.Restore, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	annotations, err := t.getAnnotations(client)
	if err != nil {
		log.Trace(err)
		return nil, err
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

	t.updateNamespaceMapping(restore)
}

func (t *Task) deleteRestores() error {
	client, err := t.getDestinationClient()
	if err != nil {
		log.Trace(err)
		return err
	}

	labels := t.Owner.GetCorrelationLabels()
	labels["migmigration"] = string(t.Owner.GetUID())
	list := velero.RestoreList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(labels),
		&list)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, restore := range list.Items {
		err = client.Delete(context.TODO(), &restore)
		if err != nil && !k8serror.IsNotFound(err) {
			log.Trace(err)
			return err
		}
	}

	return nil
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
