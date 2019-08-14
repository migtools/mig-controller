package migcluster

import (
	"strconv"
	"strings"

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

type provisionerAccessModes struct {
	Provisioner   string
	MatchBySuffix bool
	AccessModes   []kapi.PersistentVolumeAccessMode
}

// Since the StorageClass API doesn't provide this information, the support list has been
// compiled from Kubernetes API docs. Most of the below comes from:
// https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes
// https://kubernetes.io/docs/concepts/storage/storage-classes/#provisioner
var accessModeList = []provisionerAccessModes{
	provisionerAccessModes{
		Provisioner: "kubernetes.io/aws-ebs",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/azure-file",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/azure-disk",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/cinder",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
	},
	// FC : {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	// Flexvolume : {kapi.ReadWriteOnce, kapi.ReadOnlyMany}, RWX?
	// Flocker . : {kapi.ReadWriteOnce},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/gce-pd",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/glusterfs",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	},
	provisionerAccessModes{
		Provisioner: "gluster.org/glusterblock",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	}, // verify glusterblock ROX
	// ISCSI : {kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/quobyte",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	},
	// NFS : {kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/rbd",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/vsphere-volume",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/portworx-volume",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadWriteMany},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/scaleio",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany},
	},
	provisionerAccessModes{
		Provisioner: "kubernetes.io/storageos",
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
	},
	// other CSI?
	// other OCP4?
	provisionerAccessModes{
		Provisioner:   "rbd.csi.ceph.com",
		MatchBySuffix: true,
		AccessModes:   []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce},
	},
	provisionerAccessModes{
		Provisioner:   "cephfs.csi.ceph.com",
		MatchBySuffix: true,
		AccessModes:   []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	},
	provisionerAccessModes{
		Provisioner: "netapp.io/trident",
		// Note: some backends won't support RWX
		AccessModes: []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce, kapi.ReadOnlyMany, kapi.ReadWriteMany},
	},
}

// Gets the list of supported access modes for a provisioner
// TODO: allow the in-file mapping to be overridden by a configmap
func (r *ReconcileMigCluster) accessModesForProvisioner(provisioner string) []kapi.PersistentVolumeAccessMode {
	for _, pModes := range accessModeList {
		if !pModes.MatchBySuffix {
			if pModes.Provisioner == provisioner {
				return pModes.AccessModes
			}
		} else {
			if strings.HasSuffix(provisioner, pModes.Provisioner) {
				return pModes.AccessModes
			}
		}
	}

	// default value
	return []kapi.PersistentVolumeAccessMode{kapi.ReadWriteOnce}
}
