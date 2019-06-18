package migplan

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Create the velero BackupStorageLocation(s) and VolumeSnapshotLocation(s)
// have been created on both the source and destination clusters associated
// with the migration plan.
func (r ReconcileMigPlan) ensureStorage(plan *migapi.MigPlan) error {
	var client k8sclient.Client
	nEnsured := 0

	storage, err := plan.GetStorage(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	if storage == nil {
		return nil
	}
	clusters, err := r.planClusters(plan)
	if err != nil {
		log.Trace(err)
		return err
	}

	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err = cluster.GetClient(r)
		if err != nil {
			log.Trace(err)
			return err
		}
		// BSL
		err := r.ensureBSL(client, storage)
		if err != nil {
			log.Trace(err)
			return err
		}

		// VSL
		err = r.ensureVSL(client, storage)
		if err != nil {
			log.Trace(err)
			return err
		}

		// Cloud Secret
		err = r.ensureCloudSecret(client, storage)
		if err != nil {
			log.Trace(err)
			return err
		}

		nEnsured++
	}

	// Condition
	ensured := nEnsured == 2 // Both clusters.
	if ensured {
		plan.Status.SetCondition(migapi.Condition{
			Type:     StorageEnsured,
			Status:   True,
			Category: migapi.Required,
			Message:  StorageEnsuredMessage,
		})
	}

	return err
}

// Create the velero BackupStorageLocation has been created.
func (r ReconcileMigPlan) ensureBSL(client k8sclient.Client, storage *migapi.MigStorage) error {
	newBSL := storage.BuildBSL()
	foundBSL, err := storage.GetBSL(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundBSL == nil {
		err = client.Create(context.TODO(), newBSL)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	if storage.EqualsBSL(foundBSL, newBSL) {
		return nil
	}
	storage.UpdateBSL(foundBSL)
	err = client.Update(context.TODO(), foundBSL)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Create the velero VolumeSnapshotLocation has been created.
func (r ReconcileMigPlan) ensureVSL(client k8sclient.Client, storage *migapi.MigStorage) error {
	storage.DefaultVSLSettings()
	newVSL := storage.BuildVSL()
	foundVSL, err := storage.GetVSL(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundVSL == nil {
		err = client.Create(context.TODO(), newVSL)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	if storage.EqualsVSL(foundVSL, newVSL) {
		return nil
	}
	storage.UpdateVSL(foundVSL)
	err = client.Update(context.TODO(), foundVSL)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Create the velero BSL cloud secret has been created.
func (r ReconcileMigPlan) ensureCloudSecret(client k8sclient.Client, storage *migapi.MigStorage) error {
	newSecret, err := storage.BuildCloudSecret(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	foundSecret, err := storage.GetCloudSecret(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundSecret == nil {
		err = client.Create(context.TODO(), newSecret)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	if storage.EqualsCloudSecret(foundSecret, newSecret) {
		return nil
	}
	storage.UpdateCloudSecret(r, foundSecret)
	err = client.Update(context.TODO(), foundSecret)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
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
