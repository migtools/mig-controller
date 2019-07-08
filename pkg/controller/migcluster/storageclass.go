package migcluster

import (
	"strconv"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Sets the list of storage classes from the cluster
func (r *ReconcileMigCluster) setStorageClasses(cluster *migapi.MigCluster) error {
	var client k8sclient.Client
	client, err := cluster.GetClient(r)
	if err != nil {
		log.Trace(err)
		return err
	}
	clusterStorageClasses, err := cluster.GetStorageClasses(client)
	if err != nil {
		log.Trace(err)
		return err
	}
	var storageClasses []migapi.StorageClass
	for _, clusterStorageClass := range clusterStorageClasses {
		storageClass := migapi.StorageClass{
			Name:        clusterStorageClass.Name,
			Provisioner: clusterStorageClass.Provisioner,
		}
		if clusterStorageClass.Annotations != nil {
			storageClass.Default, _ = strconv.ParseBool(clusterStorageClass.Annotations["storageclass.kubernetes.io/is-default-class"])
		}
		storageClasses = append(storageClasses, storageClass)
	}
	cluster.Spec.StorageClasses = storageClasses
	return nil
}
