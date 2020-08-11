package migmigration

import (
	"context"
	"fmt"
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
	newBackup.Labels[InitialBackupLabel] = t.UID()
	newBackup.Labels[migapi.ParentLabel] = t.Owner.Name
	newBackup.Spec.IncludedResources = toStringSlice(settings.IncludedInitialResources.Difference(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	newBackup.Spec.ExcludedResources = toStringSlice(settings.ExcludedInitialResources.Union(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	delete(newBackup.Annotations, QuiesceAnnotation)
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
	labels[InitialBackupLabel] = t.UID()
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
	newBackup, err := t.buildBackup(client)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			IncludedInStageBackupLabel: t.UID(),
		},
	}
	newBackup.Labels[StageBackupLabel] = t.UID()
	newBackup.Labels[migapi.ParentLabel] = t.Owner.Name
	newBackup.Spec.IncludedResources = toStringSlice(settings.IncludedStageResources.Difference(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	newBackup.Spec.ExcludedResources = toStringSlice(settings.ExcludedStageResources.Union(toSet(t.PlanResources.MigPlan.Status.ExcludedResources)))
	newBackup.Spec.LabelSelector = &labelSelector
	err = client.Create(context.TODO(), newBackup)
	if err != nil {
		return nil, err
	}
	return newBackup, nil
}

// Get the stage backup on the source cluster.
func (t *Task) getStageBackup() (*velero.Backup, error) {
	labels := t.Owner.GetCorrelationLabels()
	labels[StageBackupLabel] = t.UID()
	return t.getBackup(labels)
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

// Get whether a backup has completed on the source cluster.
func (t Task) hasBackupCompleted(backup *velero.Backup) (bool, []string) {
	completed := false
	reasons := []string{}
	switch backup.Status.Phase {
	case velero.BackupPhaseCompleted:
		completed = true
	case velero.BackupPhaseFailed:
		completed = true
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup: %s/%s partially failed.",
				backup.Namespace,
				backup.Name))
	case velero.BackupPhasePartiallyFailed:
		completed = true
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup: %s/%s partially failed.",
				backup.Namespace,
				backup.Name))
	case velero.BackupPhaseFailedValidation:
		reasons = backup.Status.ValidationErrors
		reasons = append(
			reasons,
			fmt.Sprintf(
				"Backup: %s/%s validation failed.",
				backup.Namespace,
				backup.Name))
		completed = true
	}

	return completed, reasons
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

func (t *Task) deleteBackups() error {
	client, err := t.getSourceClient()
	if err != nil {
		return liberr.Wrap(err)
	}

	list := velero.BackupList{}
	err = client.List(
		context.TODO(),
		k8sclient.MatchingLabels(t.Owner.GetCorrelationLabels()),
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
		if err := client.Create(context.TODO(), request); err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Determine whether backups are replicated by velero on the destination cluster.
func (t *Task) isBackupReplicated(backup *velero.Backup) (bool, error) {
	client, err := t.getDestinationClient()
	if err != nil {
		return false, err
	}
	replicated := velero.Backup{}
	err = client.Get(
		context.TODO(),
		types.NamespacedName{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		&replicated)
	if err == nil {
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
