package migplan

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Create velero CRs on referenced MigClusters.
func (r ReconcileMigPlan) createStorage(plan *migapi.MigPlan) (int, error) {
	nSet := 0
	err := r.createBSLs(plan)
	if err != nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    CreateBSLFailed,
			Status:  True,
			Message: CreateBSLFailedMessage,
		})
	} else {
		plan.Status.DeleteCondition(CreateBSLFailed)
	}
	err = r.createVSLs(plan)
	if err != nil {
		plan.Status.SetCondition(migapi.Condition{
			Type:    CreateVSLFailed,
			Status:  True,
			Message: CreateVSLFailedMessage,
		})
	} else {
		plan.Status.DeleteCondition(CreateBSLFailed)
	}

	return nSet, nil
}

// Create the velero BackupStorageLocation(s).
func (r ReconcileMigPlan) createBSLs(plan *migapi.MigPlan) error {
	var client k8sclient.Client
	storage, err := plan.GetStorage(r)
	if err != nil {
		return err
	}
	if storage == nil {
		return nil
	}
	clusters, err := r.planClusters(plan)
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		if !cluster.Status.IsReady() {
			continue
		}
		client, err = cluster.GetClient(r)
		newLocation := storage.BuildBSL()
		location, err := storage.GetBSL(client)
		if err != nil {
			return err
		}
		if location == nil {
			err = client.Create(context.TODO(), newLocation)
			if err != nil {
				return err
			}
			continue
		}
		if storage.Equals(location, newLocation) {
			continue
		}
		storage.UpdateBSL(location)
		err = client.Update(context.TODO(), location)
		if err != nil {
			return err
		}
	}

	return err
}

// Create the velero VolumeSnapshotLocation(s).
func (r ReconcileMigPlan) createVSLs(plan *migapi.MigPlan) error {
	// TODO:
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
