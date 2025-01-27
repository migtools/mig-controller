package migmigration

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	dvmc "github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *Task) createDirectVolumeMigration(migType *migapi.DirectVolumeMigrationType) error {
	existingDvm, err := t.getDirectVolumeMigration()
	if err != nil {
		return err
	}
	// If already created exit with no error
	if existingDvm != nil {
		return nil
	}
	t.Log.Info("Building DirectVolumeMigration resource definition")
	dvm := t.buildDirectVolumeMigration()
	if dvm == nil {
		return errors.New("failed to build directvolumeclaim list")
	}
	if migType != nil {
		dvm.Spec.MigrationType = migType
	}
	t.Log.Info("Creating DirectVolumeMigration on host cluster",
		"directVolumeMigration", path.Join(dvm.Namespace, dvm.Name), "dvm", dvm)
	err = t.Client.Create(context.TODO(), dvm)
	return err
}

func (t *Task) buildDirectVolumeMigration() *migapi.DirectVolumeMigration {
	// Set correlation labels
	labels := t.Owner.GetCorrelationLabels()
	labels[migapi.DirectVolumeMigrationLabel] = t.UID()
	pvcList := t.getDirectVolumeClaimList()
	if pvcList == nil {
		return nil
	}
	migrationType := migapi.MigrationTypeStage
	if t.Owner.Spec.QuiescePods {
		migrationType = migapi.MigrationTypeFinal
	}
	dvm := &migapi.DirectVolumeMigration{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       labels,
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    migapi.VeleroNamespace,
		},
		Spec: migapi.DirectVolumeMigrationSpec{
			SrcMigClusterRef:            t.PlanResources.SrcMigCluster.GetObjectReference(),
			DestMigClusterRef:           t.PlanResources.DestMigCluster.GetObjectReference(),
			PersistentVolumeClaims:      pvcList,
			CreateDestinationNamespaces: true,
			LiveMigrate:                 t.PlanResources.MigPlan.Spec.LiveMigrate,
			MigrationType:               &migrationType,
		},
	}
	migapi.SetOwnerReference(t.Owner, t.Owner, dvm)
	return dvm
}

func (t *Task) getDirectVolumeMigration() (*migapi.DirectVolumeMigration, error) {
	// Get correlation labels
	labels := t.Owner.GetCorrelationLabels()
	labels[migapi.DirectVolumeMigrationLabel] = t.UID()
	// Get DVM with label
	list := migapi.DirectVolumeMigrationList{}
	err := t.Client.List(
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

// Check if the DVM has completed.
// Returns if it has completed, why it failed, and it's progress results
func (t *Task) hasDirectVolumeMigrationCompleted(dvm *migapi.DirectVolumeMigration) (completed bool, failureReasons, progress []string) {
	volumeProgress := fmt.Sprintf("%v total volumes; %v successful; %v running; %v failed",
		len(dvm.Spec.PersistentVolumeClaims),
		len(dvm.Status.SuccessfulPods)+len(dvm.Status.SuccessfulLiveMigrations),
		len(dvm.Status.RunningPods)+len(dvm.Status.FailedLiveMigrations),
		len(dvm.Status.FailedPods)+len(dvm.Status.RunningLiveMigrations))
	switch {
	// case dvm.Status.Phase != "" && dvm.Status.Phase != dvmc.Completed:
	// TODO: Update this to check on the associated dvmp resources and build up a progress indicator back to
	case dvm.Status.Phase == dvmc.Completed && (dvm.Status.Itinerary == dvmc.VolumeMigrationItinerary || dvm.Status.Itinerary == dvmc.VolumeMigrationRollbackItinerary) && dvm.Status.HasCondition(dvmc.Succeeded):
		// completed successfully
		completed = true
	case (dvm.Status.Phase == dvmc.MigrationFailed || dvm.Status.Phase == dvmc.Completed) && dvm.Status.HasCondition(dvmc.Failed):
		failureReasons = append(failureReasons, fmt.Sprintf("direct volume migration failed. %s", volumeProgress))
		completed = true
	default:
		progress = append(progress, volumeProgress)
	}
	progress = append(progress, t.getDVMProgress(dvm)...)

	// sort the progress report so we dont have flapping for the same progress info
	sort.Strings(progress)
	return completed, failureReasons, progress
}

func (t *Task) getWarningForDVM(dvm *migapi.DirectVolumeMigration) *migapi.Condition {
	conditions := dvm.Status.Conditions.FindConditionByCategory(dvmc.Warn)
	if len(conditions) > 0 {
		return &migapi.Condition{
			Type:     DirectVolumeMigrationBlocked,
			Status:   True,
			Reason:   migapi.NotReady,
			Category: migapi.Warn,
			Message:  joinConditionMessages(conditions),
		}
	}

	return nil
}

func joinConditionMessages(conditions []*migapi.Condition) string {
	messages := []string{}
	for _, c := range conditions {
		messages = append(messages, c.Message)
	}
	return strings.Join(messages, ", ")
}

// Set warning conditions on migmigration if there were any partial failures
func (t *Task) setDirectVolumeMigrationFailureWarning(dvm *migapi.DirectVolumeMigration) {
	message := fmt.Sprintf(
		"DirectVolumeMigration (dvm): %s/%s failed. See in dvm status.Errors", dvm.GetNamespace(), dvm.GetName())
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     DirectVolumeMigrationFailed,
		Status:   True,
		Category: migapi.Warn,
		Message:  message,
		Durable:  true,
	})
}

func (t *Task) getDVMProgress(dvm *migapi.DirectVolumeMigration) []string {
	progress := getDVMPodProgress(dvm)
	return append(progress, getDVMLiveMigrationProgress(dvm)...)
}

func getDVMPodProgress(dvm *migapi.DirectVolumeMigration) []string {
	progress := []string{}
	podProgressIterator := map[string][]*migapi.PodProgress{
		"Pending":   dvm.Status.PendingPods,
		"Running":   dvm.Status.RunningPods,
		"Completed": dvm.Status.SuccessfulPods,
		"Failed":    dvm.Status.FailedPods,
	}
	for state, pods := range podProgressIterator {
		for _, pod := range pods {
			p := ""
			if pod.PVCReference != nil {
				operation := dvm.Status.GetRsyncOperationStatusForPVC(pod.PVCReference)
				p = fmt.Sprintf(
					"[%s] %s: %s",
					pod.PVCReference.Name,
					path.Join(pod.Namespace, pod.Name),
					state)
				if operation.CurrentAttempt > 1 {
					p += fmt.Sprintf(
						" - Attempt %d of %d",
						operation.CurrentAttempt,
						dvmc.GetRsyncPodBackOffLimit(dvm))
				}
			} else {
				p = fmt.Sprintf("Rsync Pod %s: %s", path.Join(pod.Namespace, pod.Name), state)
			}
			if pod.LastObservedProgressPercent != "" {
				p += fmt.Sprintf(" %s", pod.LastObservedProgressPercent)
			}
			if pod.LastObservedTransferRate != "" {
				p += fmt.Sprintf(" (Transfer rate %s)", pod.LastObservedTransferRate)
			}
			if pod.TotalElapsedTime != nil {
				p += fmt.Sprintf(" (%s)", pod.TotalElapsedTime.Duration.Round(time.Second))
			}
			progress = append(progress, p)
		}
	}
	return progress
}

// Live migration progress
func getDVMLiveMigrationProgress(dvm *migapi.DirectVolumeMigration) []string {
	progress := []string{}
	if dvm == nil {
		return progress
	}
	liveMigrationProgressIterator := map[string][]*migapi.LiveMigrationProgress{
		"Running":   dvm.Status.RunningLiveMigrations,
		"Completed": dvm.Status.SuccessfulLiveMigrations,
		"Failed":    dvm.Status.FailedLiveMigrations,
		"Pending":   dvm.Status.PendingLiveMigrations,
	}
	for state, liveMigrations := range liveMigrationProgressIterator {
		for _, liveMigration := range liveMigrations {
			p := fmt.Sprintf(
				"[%s] Live Migration %s: %s",
				liveMigration.PVCReference.Name,
				path.Join(liveMigration.VMNamespace, liveMigration.VMName),
				state)
			if liveMigration.Message != "" {
				p += fmt.Sprintf(" %s", liveMigration.Message)
			}
			if liveMigration.LastObservedProgressPercent != "" {
				p += fmt.Sprintf(" %s", liveMigration.LastObservedProgressPercent)
			}
			if liveMigration.LastObservedTransferRate != "" {
				p += fmt.Sprintf(" (Transfer rate %s)", liveMigration.LastObservedTransferRate)
			}
			if liveMigration.TotalElapsedTime != nil {
				p += fmt.Sprintf(" (%s)", liveMigration.TotalElapsedTime.Duration.Round(time.Second))
			}
			progress = append(progress, p)
		}

	}
	return progress
}

func (t *Task) getDirectVolumeClaimList() []migapi.PVCToMigrate {
	nsMapping := t.PlanResources.MigPlan.GetNamespaceMapping()
	var pvcList []migapi.PVCToMigrate
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action != migapi.PvCopyAction || (pv.Selection.CopyMethod != migapi.PvFilesystemCopyMethod && pv.Selection.CopyMethod != migapi.PvBlockCopyMethod) {
			continue
		}
		accessModes := pv.PVC.AccessModes
		volumeMode := pv.PVC.VolumeMode
		// if the user overrides access modes, set up the destination PVC with user-defined
		// access mode
		if pv.Selection.AccessMode != "" {
			accessModes = []kapi.PersistentVolumeAccessMode{pv.Selection.AccessMode}
		}
		pvcList = append(pvcList, migapi.PVCToMigrate{
			ObjectReference: &kapi.ObjectReference{
				Name:      pv.PVC.GetSourceName(),
				Namespace: pv.PVC.Namespace,
			},
			TargetStorageClass: pv.Selection.StorageClass,
			TargetAccessModes:  accessModes,
			TargetVolumeMode:   &volumeMode,
			TargetNamespace:    nsMapping[pv.PVC.Namespace],
			TargetName:         pv.PVC.GetTargetName(),
			Verify:             pv.Selection.Verify,
		})
	}
	return pvcList
}

func (t *Task) deleteDirectVolumeMigrationResources() error {

	// fetch the DVM
	dvm, err := t.getDirectVolumeMigration()
	if err != nil {
		return liberr.Wrap(err)
	}

	if dvm != nil {
		// delete the DVM instance
		t.Log.Info("Deleting DirectVolumeMigration on host cluster "+
			"due to correlation with MigPlan",
			"directVolumeMigration", path.Join(dvm.Namespace, dvm.Name))
		err = t.Client.Delete(context.TODO(), dvm)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}
