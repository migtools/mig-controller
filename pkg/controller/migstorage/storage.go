package migstorage

import (
	"context"
	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Delete velero CRs on all MigClusters owned by the storage.
// Should be called when the storage is deleted.
func (r ReconcileMigStorage) deleteVeleroResources(storage *migapi.MigStorage) error {
	err := r.deleteBSLs(storage)
	if err != nil {
		log.Trace(err)
		return err
	}
	err = r.deleteVSLs(storage)
	if err != nil {
		log.Trace(err)
		return err
	}
	return nil
}

// Delete the velero BackupStorageLocation(s) owned by the storage on all clusters.
func (r ReconcileMigStorage) deleteBSLs(storage *migapi.MigStorage) error {
	var client k8sclient.Client
	clusters, err := migapi.ListClusters(r, storage.Namespace)
	if err != nil {
		log.Trace(err)
		return err
	}
	for _, cluster := range clusters {
		if !cluster.Spec.IsHostCluster {
			client, err = cluster.GetClient(r)
			if err != nil {
				log.Trace(err)
				return err
			}
		} else {
			client = r
		}
		newLocation := storage.BuildBSL()
		location, err := storage.GetBSL(client)
		if err != nil {
			log.Trace(err)
			return err
		}
		if location != nil {
			err = client.Delete(context.TODO(), newLocation)
			if err != nil {
				log.Trace(err)
				return err
			}
		}
	}

	return err
}

// Delete the velero VolumeSnapshotLocation(s) related through the MigPlan.
func (r ReconcileMigStorage) deleteVSLs(storage *migapi.MigStorage) error {
	// TODO:
	return nil
}
