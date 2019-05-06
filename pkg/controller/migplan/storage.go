package migplan

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Create the velero BackupStorageLocation(s) and VolumeSnapshotLocation(s)
// have been created on both the source and destination clusters associated
// with the migration plan.
// Returns `true` when ensured.
func (r ReconcileMigPlan) ensureStorage(plan *migapi.MigPlan) (bool, error) {
	var client k8sclient.Client
	nEnsured := 0
	storage, err := plan.GetStorage(r)
	if err != nil {
		return false, err
	}
	if storage == nil {
		return false, nil
	}
	clusters, err := r.planClusters(plan)
	if err != nil {
		return false, err
	}

	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err = cluster.GetClient(r)
		// BSL
		ensured, err := r.ensureBSL(client, storage)
		if err != nil {
			return false, err
		}
		if ensured {
			nEnsured += 1
		}
		// VSL
		ensured, err = r.ensureVSL(client, storage)
		if err != nil {
			return false, err
		}
		if ensured {
			nEnsured += 1
		}
		// Cloud Secret
		ensured, err = r.ensureCloudSecret(client, storage)
		if err != nil {
			return false, err
		}
		if ensured {
			nEnsured += 1
		}
	}

	// Condition
	ensured := nEnsured == 6 // BSL,VSL,cloud-secret x2 clusters
	if !ensured {
		plan.Status.SetCondition(migapi.Condition{
			Type:    EnsureStorageFailed,
			Status:  True,
			Message: EnsureStorageFailedMessage,
		})
	} else {
		plan.Status.DeleteCondition(EnsureStorageFailed)
	}
	err = r.Update(context.TODO(), plan)
	if err != nil {
		return false, err
	}

	return ensured, err
}

// Create the velero BackupStorageLocation has been created.
// Returns `true` when ensured.
func (r ReconcileMigPlan) ensureBSL(client k8sclient.Client, storage *migapi.MigStorage) (bool, error) {
	newBSL := storage.BuildBSL()
	foundBSL, err := storage.GetBSL(client)
	if err != nil {
		return false, err
	}
	if foundBSL == nil {
		err = client.Create(context.TODO(), newBSL)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	if storage.EqualsBSL(foundBSL, newBSL) {
		return true, nil
	}
	storage.UpdateBSL(foundBSL)
	err = client.Update(context.TODO(), foundBSL)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Create the velero VolumeSnapshotLocation has been created.
// Returns `true` when ensured.
func (r ReconcileMigPlan) ensureVSL(client k8sclient.Client, storage *migapi.MigStorage) (bool, error) {
	if storage.Spec.VolumeSnapshotProvider == "" {
		return true, nil
	}
	newVSL := storage.BuildVSL()
	foundVSL, err := storage.GetVSL(client)
	if err != nil {
		return false, err
	}
	if foundVSL == nil {
		err = client.Create(context.TODO(), newVSL)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	if storage.EqualsVSL(foundVSL, newVSL) {
		return true, nil
	}
	storage.UpdateVSL(foundVSL)
	err = client.Update(context.TODO(), foundVSL)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Create the velero BSL cloud secret has been created.
// Returns `true` when ensured.
func (r ReconcileMigPlan) ensureCloudSecret(client k8sclient.Client, storage *migapi.MigStorage) (bool, error) {
	newSecret, err := storage.BuildCloudSecret(r)
	if err != nil {
		return false, err
	}
	foundSecret, err := storage.GetCloudSecret(client)
	if err != nil {
		return false, err
	}
	if foundSecret == nil {
		err = client.Create(context.TODO(), newSecret)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	if storage.EqualsCloudSecret(foundSecret, newSecret) {
		return true, nil
	}
	storage.UpdateCloudSecret(r, foundSecret)
	err = client.Update(context.TODO(), foundSecret)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Get clusters referenced by the plan.
func (r ReconcileMigPlan) planClusters(plan *migapi.MigPlan) ([]migapi.MigCluster, error) {
	list := []migapi.MigCluster{}
	// Source
	cluster, err := plan.GetSourceCluster(r)
	if err != nil {
		return nil, err
	}
	if cluster != nil {
		list = append(list, *cluster)
	}
	// Destination
	cluster, err = plan.GetDestinationCluster(r)
	if err != nil {
		return nil, err
	}
	if cluster != nil {
		list = append(list, *cluster)
	}
	return list, nil
}
