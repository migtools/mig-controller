package directvolumemigration

import (
	"context"
	"path"

	liberr "github.com/konveyor/controller/pkg/error"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
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

	migration, err := t.Owner.GetMigrationForDVM(t.Client)
	if err != nil {
		return liberr.Wrap(err)
	}
	migrationUID := ""
	if migration != nil {
		migrationUID = string(migration.UID)
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
		matchingMigPlanPV := t.findMatchingPV(plan, pvc.Name, pvc.Namespace)
		pvcRequestedCapacity := srcPVC.Spec.Resources.Requests[corev1.ResourceStorage]

		newSpec := srcPVC.Spec
		newSpec.StorageClassName = &pvc.TargetStorageClass
		newSpec.AccessModes = pvc.TargetAccessModes
		newSpec.VolumeName = ""
		// Remove DataSource and DataSourceRef from PVC spec so any populators or sources are not
		// copied over to the destination PVC
		newSpec.DataSource = nil
		newSpec.DataSourceRef = nil

		// Adjusting destination PVC storage size request
		// max(requested capacity on source, capacity reported in migplan, proposed capacity in migplan)
		if matchingMigPlanPV != nil && settings.Settings.DvmOpts.EnablePVResizing {
			maxCapacity := pvcRequestedCapacity
			// update maxCapacity if matching PV's capacity is greater than current maxCapacity
			if matchingMigPlanPV.Capacity.Cmp(maxCapacity) > 0 {
				maxCapacity = matchingMigPlanPV.Capacity
			}

			// update maxcapacity if matching PV's proposed capacity is greater than current maxCapacity
			if matchingMigPlanPV.ProposedCapacity.Cmp(maxCapacity) > 0 {
				maxCapacity = matchingMigPlanPV.ProposedCapacity
			}
			newSpec.Resources.Requests[corev1.ResourceStorage] = maxCapacity
		}

		//Add src labels and rollback labels
		pvcLabels := srcPVC.Labels
		if pvcLabels == nil {
			pvcLabels = make(map[string]string)
		}
		// Merge DVM correlation labels into PVC labels for debug view
		corrLabels := t.Owner.GetCorrelationLabels()
		for k, v := range corrLabels {
			pvcLabels[k] = v
		}

		if migrationUID != "" && t.PlanResources != nil && t.PlanResources.MigPlan != nil {
			pvcLabels[migapi.MigMigrationLabel] = migrationUID
			pvcLabels[migapi.MigPlanLabel] = string(t.PlanResources.MigPlan.UID)
		} else if t.Owner.UID != "" {
			pvcLabels[MigratedByDirectVolumeMigration] = string(t.Owner.UID)
		}

		destNs := pvc.Namespace
		if pvc.TargetNamespace != "" {
			destNs = pvc.TargetNamespace
		}
		destName := pvc.Name
		if pvc.TargetName != "" {
			destName = pvc.TargetName
		}

		annotations := map[string]string{}
		// If a kubevirt disk PVC, copy annotations to destination PVC
		if srcPVC.Annotations != nil && srcPVC.Annotations["cdi.kubevirt.io/storage.contentType"] == "kubevirt" {
			annotations = srcPVC.Annotations
			// Ensure that when we create a matching DataVolume, it will adopt this PVC
			annotations["cdi.kubevirt.io/storage.populatedFor"] = destName
			// Remove annotations indicating the PVC is bound or provisioned
			delete(annotations, "pv.kubernetes.io/bind-completed")
			delete(annotations, "volume.beta.kubernetes.io/storage-provisioner")
			delete(annotations, "pv.kubernetes.io/bound-by-controller")
			delete(annotations, "volume.kubernetes.io/storage-provisioner")
		}

		// Create pvc on destination with same metadata + spec
		destPVC := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        destName,
				Namespace:   destNs,
				Labels:      pvcLabels,
				Annotations: annotations,
			},
			Spec: newSpec,
		}
		t.Log.Info("Creating PVC on destination MigCluster",
			"persistentVolumeClaim", path.Join(pvc.Namespace, pvc.Name),
			"destPersistentVolumeClaim", path.Join(destNs, pvc.Name),
			"pvcStorageClassName", destPVC.Spec.StorageClassName,
			"pvcAccessModes", destPVC.Spec.AccessModes,
			"pvcRequests", destPVC.Spec.Resources.Requests,
			"pvcDataSource", destPVC.Spec.DataSource,
			"pvcDataSourceRef", destPVC.Spec.DataSourceRef)
		destPVCCheck := corev1.PersistentVolumeClaim{}
		err = destClient.Get(context.TODO(), types.NamespacedName{
			Namespace: destNs,
			Name:      destName,
		}, &destPVCCheck)
		if k8serror.IsNotFound(err) {
			err = destClient.Create(context.TODO(), &destPVC)
			if err != nil {
				return err
			}
		} else if err == nil {
			t.Log.Info("PVC already exists on destination", "namespace", pvc.Namespace, "name", pvc.Name)
		} else {
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
	if plan != nil {
		for i := range plan.Spec.PersistentVolumes.List {
			planVol := &plan.Spec.PersistentVolumes.List[i]
			if planVol.PVC.GetSourceName() == pvcName && planVol.PVC.Namespace == pvcNamespace {
				return planVol
			}
		}
	}
	return nil
}
