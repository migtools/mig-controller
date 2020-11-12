package directvolumemigration

import (
	"context"
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

		newSpec := srcPVC.Spec
		newSpec.StorageClassName = &pvc.TargetStorageClass
		newSpec.AccessModes = pvc.TargetAccessModes
		newSpec.VolumeName = ""

		// Create pvc on destination with same metadata + spec
		destPVC := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
				Labels:    srcPVC.Labels,
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
