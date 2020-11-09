package migmigration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
	newRestore.Labels[MigMigrationDebugLabel] = t.Owner.Name
	newRestore.Labels[MigPlanDebugLabel] = t.Owner.Spec.MigPlanRef.Name
	newRestore.Labels[MigMigrationLabel] = string(t.Owner.UID)
	newRestore.Labels[MigPlanLabel] = string(t.PlanResources.MigPlan.UID)

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
	newRestore.Labels[MigMigrationDebugLabel] = t.Owner.Name
	newRestore.Labels[MigPlanDebugLabel] = t.Owner.Spec.MigPlanRef.Name
	newRestore.Labels[MigMigrationLabel] = string(t.Owner.UID)
	newRestore.Labels[MigPlanLabel] = string(t.PlanResources.MigPlan.UID)
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
		k8sclient.MatchingLabels(restoreAssociationLabel),
		&list)
	if err != nil {
		log.Trace(err)
	}
	return &list
}

// Get whether a resource has completed on the destination cluster.
func (t *Task) hasRestoreCompleted(restore *velero.Restore) (bool, []string) {
	completed := false
	reasons := []string{}
	progress := []string{}

	pvrs := t.getPodVolumeRestoresForRestore(restore)

	getPodVolumeRestoresProgress := func(pvrList *velero.PodVolumeRestoreList) (progress []string) {
		getDuration := func(pvr *velero.PodVolumeRestore) (duration string) {
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
					getDuration(&pvr))
			case velero.PodVolumeRestorePhaseCompleted:
				msg = fmt.Sprintf(
					"PodVolumeRestore %s/%s: Completed, %s out of %s restored%s",
					pvr.Namespace,
					pvr.Name,
					bytesToSI(pvr.Status.Progress.BytesDone),
					bytesToSI(pvr.Status.Progress.TotalBytes),
					getDuration(&pvr))
			case velero.PodVolumeRestorePhaseFailed:
				msg = fmt.Sprintf(
					"PodVolumeRestore %s/%s: Failed%s",
					pvr.Namespace,
					pvr.Name,
					getDuration(&pvr))
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

	switch restore.Status.Phase {
	case velero.RestorePhaseNew:
		progress = append(
			progress,
			fmt.Sprintf(
				"Backup %s/%s: Not started yet",
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
			"Restore: %s/%s failed.",
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
				"Restore: %s/%s partially failed.",
				restore.Namespace,
				restore.Name))
		reasons = append(reasons, message)
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
				"Restore: %s/%s validation failed.",
				restore.Namespace,
				restore.Name))
		completed = true
	}

	t.setProgress(progress)
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

	migplan, err := t.Owner.GetPlan(client)
	if err != nil {
		return liberr.Wrap(err)
	}

	list := velero.RestoreList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(migplan.GetCorrelationLabels()),
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
		MigPlanLabel: string(t.PlanResources.MigPlan.UID),
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
		MigPlanLabel: string(t.PlanResources.MigPlan.UID),
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
