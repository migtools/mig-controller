package migmigration

import (
	"context"
	"errors"
	"fmt"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	dvmc "github.com/konveyor/mig-controller/pkg/controller/directvolumemigration"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	path "path"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
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
	dvm := t.buildDirectVolumeMigration()
	if dvm == nil {
		return errors.New("failed to build directvolumeclaim list")
	}
	client, err := t.getDestinationClient()
	if err != nil {
		return err
	}
	err = client.Create(context.TODO(), dvm)
	return err

}

func (t *Task) buildDirectVolumeMigration() *migapi.DirectVolumeMigration {
	// Set correlation labels
	labels := t.Owner.GetCorrelationLabels()
	labels[DirectVolumeMigrationLabel] = t.UID()
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
	labels[DirectVolumeMigrationLabel] = t.UID()
	// Get DVM with label
	client, err := t.getDestinationClient()
	if err != nil {
		return nil, err
	}
	list := migapi.DirectVolumeMigrationList{}
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
	case dvm.Status.Phase != "" && dvm.Status.Phase != dvmc.Completed:
		// TODO: Update this to check on the associated dvmp resources and build up a progress indicator back to
		progress = append(progress, t.getDVMProgressForRunningPods(dvm.Status.RunningPods)...)
	case dvm.Status.Phase == dvmc.Completed && dvm.Status.Itinerary == "VolumeMigration":
		progress = append(progress, t.getDVMProgressForSucceededPods(dvm.Status.SuccessfulPods)...)
		completed = true
	case (dvm.Status.Phase == dvmc.MigrationFailed || dvm.Status.Phase == dvmc.Completed) && dvm.Status.Itinerary == "VolumeMigrationFailed":
		failureReasons = append(failureReasons, fmt.Sprintf("direct volume migration failed. %s", volumeProgress))
		progress = append(progress, t.getDVMProgressForFailedPods(dvm.Status.FailedPods)...)
		completed = true
	default:
		progress = append(progress, volumeProgress)
	}
	// sort the progress report so we dont have flapping for the same progress info
	sort.Sort(sort.StringSlice(progress))
	return completed, failureReasons, progress
}

// Set warning conditions on migmigration if there were any partial failures
func (t *Task) setDirectVolumeMigrationFailureWarning(dvm *migapi.DirectVolumeMigration) {
	message := fmt.Sprintf(
		"Direct Volume Migration: one or more volumes failed in dvm %s/%s", dvm.GetNamespace(), dvm.GetName())
	t.Owner.Status.SetCondition(migapi.Condition{
		Type:     DirectVolumeMigrationsFailed,
		Status:   True,
		Category: migapi.Warn,
		Message:  message,
		Durable:  true,
	})
}

func (t *Task) getDVMProgressForRunningPods(runningPods []*migapi.RunningPod) []string {
	progress := []string{}
	for _, pod := range runningPods {
		p := fmt.Sprintf("Rsync Client Pod %s: Running", path.Join(pod.Namespace, pod.Name))
		if pod.LastObservedProgressPercent != "" {
			p += fmt.Sprintf(", progress percent %s", pod.LastObservedProgressPercent)
		}
		if pod.LastObservedTransferRate != "" {
			p += fmt.Sprintf(", transfer rate %s", pod.LastObservedTransferRate)
		}
		progress = append(progress, p)
	}
	return progress
}

func (t *Task) getDVMProgressForSucceededPods(pods []*kapi.ObjectReference) []string {
	// TODO: need to add the over all transfer rate for each PV
	progress := []string{}
	for _, pod := range pods {
		progress = append(progress, fmt.Sprintf("Rsync Client Pod %s: Succeeded", path.Join(pod.Namespace, pod.Name)))
	}
	return progress
}

func (t *Task) getDVMProgressForFailedPods(pods []*kapi.ObjectReference) []string {
	progress := []string{}
	for _, pod := range pods {
		progress = append(progress, fmt.Sprintf("Rsync Client Pod %s: Failed", path.Join(pod.Namespace, pod.Name)))
	}
	return progress
}

func (t *Task) getDirectVolumeClaimList() *[]migapi.PVCToMigrate {
	pvcList := []migapi.PVCToMigrate{}
	for _, pv := range t.PlanResources.MigPlan.Spec.PersistentVolumes.List {
		if pv.Selection.Action != migapi.PvCopyAction && pv.Selection.CopyMethod != migapi.PvFilesystemCopyMethod {
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
		})
	}
	if len(pvcList) > 0 {
		return &pvcList
	}
	return nil
}
