# Supporting StorageClass change on migration

The Migration Controller needs to support changing StorageClass on the
destination cluster. The target cluster may not have all of the same
StorageClasses available as the source, which would require a
change. In addition, if the destination cluster has OCS4 Ceph
installed, we want to default to OCS4 StorageClasses for certain
source volume types.

## Controller design

On the back end, the StorageClass change is integrated into the
existing PV discovery and action selection functionality. When we
discover PVs mounted by running pods, in addition to identifying which
PV actions are available ("move" or "copy" or both), we also look at
the StorageClass of the volume (if available) and PV characteristics,
along with the list of available StorageClasses in the destination
cluster to determine the suggested choice for the target StorageClass
selection.

For certain source volume types, specifically NFS and Gluster volumes,
we want to recommend cephfs or rbd as the target StorageClass, if
available. If the source volume is configured for ReadWriteMany (RWX),
the controller selects cephfs. Otherwise, the controller selects
rbd. If the desired StorageClass is not available in the
destination cluster, then the controller chooses the default
StorageClass. For Gluster source volumes, if the desired OCS4
StorageClass is not available, a warning condition is raised to
recommend to the user that OCS4 be installed, but the user can choose
to ignore the warning and use the defaulted value.

For other storage types, the controller determines whether there is a
StorageClass with the same provisioner on the destination cluster and
selects this one if it is available. If it is not available, or the
source volume has no StorageClass, the default StorageClass is
selected.

If there is no default StorageClass for the cluster, then any volume
for which the controller would have selected the default will have an
empty StorageClass selection, which will be shown as an error
condition in the UI. The user must choose a StorageClass for these
volumes before running a migration.

## User Inferface design

The Migration Plan wizard includes a StorageClass selection step after
the PV discovery on the plan is complete. This step shows the user the
StorageClass for the source volume (if available) and the
controller-selected initial value for the target StorageClass in a
dropdown menu. This allows the user a chance to choose among the other
available StorageClasses in the destination cluster if desired.

## Access Mode selection

For the majority of use cases, selecting Access Modes will not be
needed. In certain cases, a user might want to migrate a volume to a
destination StorageClass which does not support the Access Mode of the
source volume. In most cases, this would mean that there is a RWX
source volume (perhaps on NFS or Gluster) which needs to be migrated
to a StorageClass that does not support RWX (such as EBS/gp2). In this
case, without changing the AccessMode on migration, the volume will
never be properly migrated to the destination because the destination
cluster will not be able to create an RWX volume for a StorageClass
such as EBS. The only way to successfully migrate this volume will be
to select a different destination/target Access Mode such as
ReadWriteOnce (RWO). The User Interface will provide the Access Mode
selection next to the StorageClass selection, and the controller API
will treat this as an optional attribute. The most common use case
will be no Access Mode selected, and the source volume's access mode
will be preserved on migration.

## Velero Plugin design

In order to actually change the StorageClass on the destination, the
controller will set annotations for target StorageClass (and Access
Mode, if it's changing) on the PersistentVolumeClaims (PVCs)
before doing the stage backup. There is a registered restore plugin
for PVCs which will modify `pvc.Spec.StorageClassName` and
`pvc.Spec.AccessModes` based on these annotations.

## Implementation concerns

Implementing this depends on the upcoming (mid to late August) Velero
1.1 release. There is a bug in handling CSI volumes in Velero 1.0, and
the Velero 1.1 beta introduced a couple of bugs related to image
backup and restore:

* On backup, if there is more than one pod containing mounted volumes
  in a namespace, only the last pod's volumes will be properly
  migrated. After working with upstream development, this has been
  fixed and merged upstream to master.
* On restore, if velero is running in a non-standard namespace (such
  as in the `mig` namespace when installed by the migration operator),
  velero currently hangs on restore. After reporting this bug
  upstream, they have reproduced this problem in their environment,
  and I expect a fix will be available shortly.

We need both of these fixes in our Velero fork before we can update
our quay images to include them and make them available for testing
with the migration operator. Other than the known velero bugs, this
feature is now fully implemented and merged.