package migmigration

import (
	"context"
	"fmt"
	"strings"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/gvk"
	"github.com/pkg/errors"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	newRestore, err := t.buildRestore(client, backup.Name)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newRestore.Labels[FinalRestoreLabel] = t.UID()
	newRestore.Labels[migapi.ParentLabel] = t.Owner.Name
	err = client.Create(context.TODO(), newRestore)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return newRestore, nil
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
	newRestore, err := t.buildRestore(client, backup.Name)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newRestore.Labels[StageRestoreLabel] = t.UID()
	newRestore.Labels[migapi.ParentLabel] = t.Owner.Name
	stagePodImage, err := t.getStagePodImage(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	newRestore.Annotations[StagePodImageAnnotation] = stagePodImage
	err = client.Create(context.TODO(), newRestore)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return newRestore, nil
}

// Get the stage restore on the destination cluster.
func (t *Task) getStageRestore() (*velero.Restore, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[StageRestoreLabel] = t.UID()
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

// Set warning conditions on migmigration if there were restic errors
func (t *Task) setResticConditions(restore *velero.Restore) {
	if len(restore.Status.PodVolumeRestoreErrors) > 0 {
		message := fmt.Sprintf(ResticErrorsMessage, len(restore.Status.PodVolumeRestoreErrors), restore.Name)
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     ResticErrors,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
	if len(restore.Status.PodVolumeRestoreVerifyErrors) > 0 {
		message := fmt.Sprintf(ResticVerifyErrorsMessage, len(restore.Status.PodVolumeRestoreVerifyErrors), restore.Name)
		t.Owner.Status.SetCondition(migapi.Condition{
			Type:     ResticVerifyErrors,
			Status:   True,
			Category: migapi.Warn,
			Message:  message,
			Durable:  true,
		})
	}
}

// Build a Restore as desired for the destination cluster.
func (t *Task) buildRestore(client k8sclient.Client, backupName string) (*velero.Restore, error) {
	annotations, err := t.getAnnotations(client)
	if err != nil {
		return nil, liberr.Wrap(err)
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

func (t *Task) deleteRestores() error {
	client, err := t.getDestinationClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	list := velero.RestoreList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels()),
		&list)
	if err != nil {
		return liberr.Wrap(err)
	}
	for _, restore := range list.Items {
		err = client.Delete(context.TODO(), &restore)
		if err != nil && !k8serror.IsNotFound(err) {
			return liberr.Wrap(err)
		}
	}

	return nil
}

func (t *Task) deleteMigrated() error {
	client, GVRs, err := gvk.GetGVRsForCluster(t.PlanResources.DestMigCluster, t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}

	listOptions := k8sclient.MatchingLabels(map[string]string{
		MigratedByLabel: string(t.Owner.UID),
	}).AsListOptions()

	for _, gvr := range GVRs {
		for _, ns := range t.destinationNamespaces() {
			err = client.Resource(gvr).DeleteCollection(&metav1.DeleteOptions{}, *listOptions)
			if err == nil {
				continue
			}
			if !k8serror.IsMethodNotSupported(err) && !k8serror.IsNotFound(err) {
				return liberr.Wrap(err)
			}
			list, err := client.Resource(gvr).Namespace(ns).List(*listOptions)
			if err != nil {
				return liberr.Wrap(err)
			}
			for _, r := range list.Items {
				err = client.Resource(gvr).Namespace(ns).Delete(r.GetName(), nil)
				if err != nil {
					// Will ignore the ones that were removed, or for some reason are not supported
					// Assuming that main resources will be removed, such as pods and pvcs
					if k8serror.IsMethodNotSupported(err) || k8serror.IsNotFound(err) {
						continue
					}
					log.Error(err, fmt.Sprintf("Failed to request delete on: %s", gvr.String()))
					return err
				}
			}
		}
	}

	return nil
}

func (t *Task) ensureMigratedResourcesDeleted() (bool, error) {
	client, GVRs, err := gvk.GetGVRsForCluster(t.PlanResources.DestMigCluster, t.Client)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	listOptions := k8sclient.MatchingLabels(map[string]string{
		MigratedByLabel: string(t.Owner.UID),
	}).AsListOptions()
	for _, gvr := range GVRs {
		for _, ns := range t.destinationNamespaces() {
			list, err := client.Resource(gvr).Namespace(ns).List(*listOptions)
			if err != nil {
				return false, liberr.Wrap(err)
			}
			// Wait for resources with deletion timestamps
			if len(list.Items) > 0 {
				return false, err
			}
		}
	}

	return true, nil
}
