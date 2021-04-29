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

func (t *Task) createDirectVolumeMigration() error {
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
	t.Log.Info("Creating DirectVolumeMigration on host cluster",
		"directVolumeMigration", path.Join(dvm.Namespace, dvm.Name))
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
	dvm := &migapi.DirectVolumeMigration{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       labels,
			GenerateName: t.Owner.GetName() + "-",
			Namespace:    migapi.VeleroNamespace,
		},
		Spec: migapi.DirectVolumeMigrationSpec{
			SrcMigClusterRef:            t.PlanResources.SrcMigCluster.GetObjectReference(),
			DestMigClusterRef:           t.PlanResources.DestMigCluster.GetObjectReference(),
			PersistentVolumeClaims:      *pvcList,
			CreateDestinationNamespaces: true,
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

// Check if the DVM has completed.
// Returns if it has completed, why it failed, and it's progress results
func (t *Task) hasDirectVolumeMigrationCompleted(dvm *migapi.DirectVolumeMigration) (completed bool, failureReasons, progress []string) {
	totalVolumes := len(dvm.Spec.PersistentVolumeClaims)
	successfulPods := 0
	failedPods := 0
	runningPods := 0
	if dvm.Status.SuccessfulPods != nil {
		successfulPods = len(dvm.Status.SuccessfulPods)
	}
	if dvm.Status.FailedPods != nil {
		failedPods = len(dvm.Status.FailedPods)
	}
	if dvm.Status.RunningPods != nil {
		runningPods = len(dvm.Status.RunningPods)
	}

	volumeProgress := fmt.Sprintf("%v total volumes; %v successful; %v running; %v failed", totalVolumes, successfulPods, runningPods, failedPods)
	switch {
	//case dvm.Status.Phase != "" && dvm.Status.Phase != dvmc.Completed:
	//	// TODO: Update this to check on the associated dvmp resources and build up a progress indicator back to
	case dvm.Status.Phase == dvmc.Completed && dvm.Status.Itinerary == "VolumeMigration" && dvm.Status.HasCondition(dvmc.Succeeded):
		// completed successfully
		completed = true
	case (dvm.Status.Phase == dvmc.MigrationFailed || dvm.Status.Phase == dvmc.Completed) && dvm.Status.HasCondition(dvmc.Failed):
		failureReasons = append(failureReasons, fmt.Sprintf("direct volume migration failed. %s", volumeProgress))
		completed = true
	default:
		progress = append(progress, volumeProgress)
	}
	progress = append(progress, t.getDVMPodProgress(*dvm)...)

	// sort the progress report so we dont have flapping for the same progress info
	sort.Strings(progress)
	return completed, failureReasons, progress
}

func (t *Task) getWarningForDVM(dvm *migapi.DirectVolumeMigration) (*migapi.Condition, error) {
	conditions := dvm.Status.Conditions.FindConditionByCategory(dvmc.Warn)
	if len(conditions) > 0 {
		return &migapi.Condition{
			Type:     DirectVolumeMigrationBlocked,
			Status:   True,
			Reason:   migapi.NotReady,
			Category: migapi.Warn,
			Message:  joinConditionMessages(conditions),
		}, nil
	}

	return nil, nil
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

func (t *Task) getDVMPodProgress(dvm migapi.DirectVolumeMigration) []string {
	progress := []string{}
	progressIterator := map[string][]*migapi.PodProgress{
		"Pending":   dvm.Status.PendingPods,
		"Running":   dvm.Status.RunningPods,
		"Completed": dvm.Status.SuccessfulPods,
		"Failed":    dvm.Status.FailedPods,
	}
	for state, pods := range progressIterator {
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

func (t *Task) getDirectVolumeClaimList() *[]migapi.PVCToMigrate {
	pvcList := []migapi.PVCToMigrate{}
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action != migapi.PvCopyAction || pv.Selection.CopyMethod != migapi.PvFilesystemCopyMethod {
			continue
		}
		accessModes := pv.PVC.AccessModes
		// if the user overrides access modes, set up the destination PVC with user-defined
		// access mode
		if pv.Selection.AccessMode != "" {
			accessModes = []kapi.PersistentVolumeAccessMode{pv.Selection.AccessMode}
		}
		pvcList = append(pvcList, migapi.PVCToMigrate{
			ObjectReference: &kapi.ObjectReference{
				Name:      pv.PVC.Name,
				Namespace: pv.PVC.Namespace,
			},
			TargetStorageClass: pv.Selection.StorageClass,
			TargetAccessModes:  accessModes,
			Verify:             pv.Selection.Verify,
		})
	}
	if len(pvcList) > 0 {
		return &pvcList
	}
	return nil
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
