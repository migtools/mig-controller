package directvolumemigration

import (
	"context"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (t *Task) areSourcePVCsUnattached() error {
	// This function provides state checking on source PVCs, make sure app is
	// quiesced
	return nil
}

func (t *Task) createDestinationPVCs() error {
	// Get client for destination
	destClient, err := t.getDestinationClient()
	if err != nil {
		return err
	}

	// Get client for source
	srcClient, err := t.getSourceClient()
	if err != nil {
		return err
	}

	for _, pvc := range t.Owner.Spec.PersistentVolumeClaims {
		// Get pvc definition from source cluster

		srcPVC := corev1.PersistentVolumeClaim{}
		key := types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}
		err = srcClient.Get(context.TODO(), key, &srcPVC)
		if err != nil {
			return err
		}

		plan := t.PlanResources.MigPlan
		matchingPV := t.findMatchingPV(plan, pvc.Name, pvc.Namespace)
		srcPVCRequest := srcPVC.Spec.Resources.Requests[corev1.ResourceStorage]

		newSpec := srcPVC.Spec
		newSpec.StorageClassName = &pvc.TargetStorageClass
		newSpec.AccessModes = pvc.TargetAccessModes
		newSpec.VolumeName = ""

		// Adjusting destination PVC storage size request
		if matchingPV != nil {
			// update src PVC capacity if matching PV's capacity is the maximum
			if matchingPV.Capacity.Cmp(srcPVCRequest) > 0 && matchingPV.Capacity.Cmp(matchingPV.ProposedCapacity) > 0 {
				newSpec.Resources.Requests[corev1.ResourceStorage] = matchingPV.Capacity
			}

			// update src PVC capacity if matching PV's proposed capacity is the maximum
			if matchingPV.ProposedCapacity.Cmp(srcPVCRequest) > 0 && matchingPV.ProposedCapacity.Cmp(matchingPV.Capacity) > 0 {
				newSpec.Resources.Requests[corev1.ResourceStorage] = matchingPV.ProposedCapacity
			}
		}

		//Add src labels and rollback labels
		pvcLabels := srcPVC.Labels
		if pvcLabels == nil {
			pvcLabels = make(map[string]string)
		}

		if t.MigrationUID != "" && t.PlanResources.MigPlan != nil {
			pvcLabels[MigratedByMigrationLabel] = t.MigrationUID
			pvcLabels[MigratedByPlanLabel] = string(t.PlanResources.MigPlan.UID)
		}

		// Create pvc on destination with same metadata + spec
		destPVC := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
				Labels:    pvcLabels,
			},
			Spec: newSpec,
		}
		err = destClient.Create(context.TODO(), &destPVC)
		if k8serror.IsAlreadyExists(err) {
			t.Log.Info("PVC already exists on destination", "name", pvc.Name)
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (t *Task) getDestinationPVCs() error {
	// Ensure PVCs are bound and not in pending state
	return nil
}

func (t *Task) findMatchingPV(plan *migapi.MigPlan, pvcName string, pvcNamespace string) *migapi.PV {
	for _, planVol := range plan.Spec.PersistentVolumes.List {
		if planVol.PVC.Name == pvcName && planVol.PVC.Namespace == pvcNamespace {
			return &planVol
		}
	}
	return nil
}
