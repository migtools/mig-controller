package migmigration

import (
	"context"
	"errors"
	"fmt"
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
	t.setDirectVolumeMigrationOwnerReferences(dvm)
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
	switch dvm.Status.Phase {
	case dvmc.Started:
		// TODO: Update this to check on the associated dvmp resources and build up a progress indicator back to
		progress = append(progress, fmt.Sprintf("direct volume migration started at %v. ", dvm.Status.StartTimestamp))
	case dvmc.Completed:
		progress = append(progress, fmt.Sprintf("%v/%v volume migrations were successful", successfulPods, totalVolumes))
		completed = true
	case dvmc.MigrationFailed:
		failureReasons = append(failureReasons, fmt.Sprintf("direct volume migration failed. %s", volumeProgress))
		completed = true
	default:
		progress = append(progress, volumeProgress)
	}
	return completed, failureReasons, progress
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

func (t *Task) setDirectVolumeMigrationOwnerReferences(dvm *migapi.DirectVolumeMigration) {
	trueVar := true
	for i := range dvm.OwnerReferences {
		ref := &dvm.OwnerReferences[i]
		if ref.Kind == t.Owner.Kind {
			ref.APIVersion = t.Owner.APIVersion
			ref.Name = t.Owner.Name
			ref.UID = t.Owner.UID
			ref.Controller = &trueVar
			return
		}
	}
	dvm.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: t.Owner.APIVersion,
			Kind:       t.Owner.Kind,
			Name:       t.Owner.Name,
			UID:        t.Owner.UID,
			Controller: &trueVar,
		},
	}
}
