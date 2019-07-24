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
		pl := PlanStorage{
			Client:       r,
			targetClient: client,
			storage:      storage,
			plan:         plan,
		}

		// BSL
		err := pl.ensureBSL()
		if err != nil {
			log.Trace(err)
			return err
		}

		// VSL
		err = pl.ensureVSL()
		if err != nil {
			log.Trace(err)
			return err
		}

		// Cloud Secret
		err = pl.ensureCloudSecret()
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

//
// PlanStorage
// Client: The controller client.
// targetClient: A client for a cluster.
// plan: A plan resource.
// storage: A storage resource.
//
type PlanStorage struct {
	k8sclient.Client
	targetClient k8sclient.Client
	plan         *migapi.MigPlan
	storage      *migapi.MigStorage
}

// Create the velero BackupStorageLocation has been created.
func (r PlanStorage) ensureBSL() error {
	newBSL := r.storage.BuildBSL()
	newBSL.Labels = r.plan.GetCorrelationLabels()
	foundBSL, err := r.plan.GetBSL(r.targetClient)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundBSL == nil {
		err = r.targetClient.Create(context.TODO(), newBSL)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	if r.storage.EqualsBSL(foundBSL, newBSL) {
		return nil
	}
	r.storage.UpdateBSL(foundBSL)
	err = r.targetClient.Update(context.TODO(), foundBSL)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Create the velero VolumeSnapshotLocation has been created.
func (r PlanStorage) ensureVSL() error {
	newVSL := r.storage.BuildVSL()
	newVSL.Labels = r.plan.GetCorrelationLabels()
	foundVSL, err := r.plan.GetVSL(r.targetClient)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundVSL == nil {
		err = r.targetClient.Create(context.TODO(), newVSL)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	if r.storage.EqualsVSL(foundVSL, newVSL) {
		return nil
	}
	r.storage.UpdateVSL(foundVSL)
	err = r.targetClient.Update(context.TODO(), foundVSL)
	if err != nil {
		log.Trace(err)
		return err
	}

	return nil
}

// Create the velero BSL cloud secret has been created.
func (r PlanStorage) ensureCloudSecret() error {
	newSecret, err := r.storage.BuildBSLCloudSecret(r.Client)
	if err != nil {
		log.Trace(err)
		return err
	}
	newSecret.Labels = r.plan.GetCorrelationLabels()
	foundSecret, err := r.plan.GetCloudSecret(r.targetClient)
	if err != nil {
		log.Trace(err)
		return err
	}
	if foundSecret == nil {
		err = r.targetClient.Create(context.TODO(), newSecret)
		if err != nil {
			log.Trace(err)
			return err
		}
		return nil
	}
	if r.storage.EqualsCloudSecret(foundSecret, newSecret) {
		return nil
	}
	r.storage.UpdateBSLCloudSecret(r.Client, foundSecret)
	err = r.targetClient.Update(context.TODO(), foundSecret)
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
