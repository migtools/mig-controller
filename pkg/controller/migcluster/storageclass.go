package migcluster

import (
	"strconv"

	migapi "github.com/fusor/mig-controller/pkg/apis/migration/v1alpha1"
	kapi "k8s.io/api/core/v1"
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
			AccessModes: r.accessModesForProvisioner(clusterStorageClass.Provisioner),
		}
		if clusterStorageClass.Annotations != nil {
			storageClass.Default, _ = strconv.ParseBool(clusterStorageClass.Annotations["storageclass.kubernetes.io/is-default-class"])
		}
		storageClasses = append(storageClasses, storageClass)
	}
	cluster.Spec.StorageClasses = storageClasses
	return nil
}

var accessModeList = map[string][]kapi.PersistentVolumeAccessMode{
	"kubernetes.io/aws-ebs":    {kapi.ReadWriteOnce},
	"kubernetes.io/azure-file": {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	"kubernetes.io/azure-disk": {kapi.ReadWriteOnce},
	"kubernetes.io/cinder":     {kapi.ReadWriteOnce},
	// FC : {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	// Flexvolume : {kapi.ReadWriteOnce, kapi.ReadOnlyMany}, RWX?
	// Flocker . : {kapi.ReadWriteOnce},
	"kubernetes.io/gce-pd":     {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	"kubernetes.io/glusterfs":  {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	"gluster.org/glusterblock": {kapi.ReadWriteOnce, kapi.ReadOnlyMany}, // verify glusterblock ROX
	// ISCSI : {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	"kubernetes.io/quobyte": {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	// NFS : {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	"kubernetes.io/rbd":             {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	"kubernetes.io/vsphere-volume":  {kapi.ReadWriteOnce},
	"kubernetes.io/portworx-volume": {kapi.ReadWriteOnce, kapi.ReadWriteMany},
	"kubernetes.io/scaleio":         {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	"kubernetes.io/storageos":       {kapi.ReadWriteOnce},
	// other CSI?
	// other OCP4?
	"rbd.csi.ceph.com":    {kapi.ReadWriteOnce},
	"cephfs.csi.ceph.com": {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	"netapp.io/trident":   {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany}, // Note: some backends won't support RWX
}

// Gets the list of supported access modes for a provisioner
// TODO: allow the in-file mapping to be overridden by a configmap
func (r *ReconcileMigCluster) accessModesForProvisioner(provisioner string) []kapi.PersistentVolumeAccessMode {
	accessModes, ok := accessModeList[provisioner]
	if !ok {
		accessModes = []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce}
	}
	return accessModes
}
