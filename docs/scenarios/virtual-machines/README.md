## Virtual machine storage migration

Migration toolkit for containers now supports storage migration for KubeVirt virtual machines. Migration can happen offline (virtual machine is turned off) as well as online (virtual machine is running).

Main use cases for this functionality is migrating from one storage class to another. For instance if one is rebalancing across different storage providers, or some new storage is available that is better suited for the workload running inside the VM.

### Supported PV Actions

| Action | Supported | Description |
|-----------|------------|-------------|
| Copy | Yes | Create new PV in *same namespace*. Copy data from source PV to target PV, and modify Virtual Machine definition to point to the new PV. If liveMigrate flag is set, the VM will live migrate. If not set, the VM is shutdown, the source PV contents are copied to the target PV and the VM is started again |
| Move | No  | |

### Prerequisites

Referring to the getting started [README.md](https://github.com/konveyor/mig-controller/blob/master/README.md), you'll first need to deploy mig-controller and Velero, and then create the following 'Mig' resources on the cluster where mig-controller is running to prepare for Migration:

Also deploy [KubeVirt](https://github.com/kubevirt/kubevirt/) and the Containerized Data Importer [CDI](https://github.com/kubevirt/containerized-data-importer/?tab=readme-ov-file#deploy-it) according to the documentation provided there. This will add the appropriate Custom Resource Definitions (CRD) used by mig-controller to manipulate the Virtual Machines.

**_NOTE:_** If the mig-controller pod was started before KubeVirt and CDI was installed, the mig-controller will not automatically see the CRDs were installed. You have to (re)start the mig-controller pod after installing KubeVirt and CDI.

**_NOTE:_** In order to support storage live migration, you need at least KubeVirt version 1.3.0. Earlier versions do not support storage live migration. You also need to configure KubeVirt to enable storage live migration according to the [documentation](https://kubevirt.io/user-guide/storage/volume_migration/)

|Resource|Purpose|
|---|---|
|`MigCluster`|Represents the cluster to use when doing the storage migration|
|`StorageClass`|The storage class, there should be at least two|
|`VirtualMachine`|A virtual machine definition, installed by KubeVirt|
|`VirtualMachineInstance`|A running virtual machine, installed by KubeVirt|
|`DataVolume`|A definition on how to populate a Persistent Volume with a virtual machine disk, installed by CDI|


### Deploying a Virtual Machine

Once KubeVirt and CDI are installed and active, create a namespace. For this example we use the namespace `mig-vm`. And create the following yaml to create a Fedora Virtual Machine.

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  labels:
    kubevirt.io/vm: vm-fedora
  name: vm-fedora
  namespace: mig-vm
spec:
  dataVolumeTemplates:
  - metadata:
      name: fedora-40-dv-template
      namespace: mig-vm
    spec:
      storage:
        resources:
          requests:
            storage: 8Gi
      source:
        http:
          url: "https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-AmazonEC2.x86_64-40-1.14.raw.xz"
  running: true
  template:
    metadata:
      labels:
        kubevirt.io/vm: vm-fedora
    spec:
      networks:
      - name: default
        pod: {}
      domain:
        ioThreadsPolicy: auto
        devices:
          disks:
          - disk:
              bus: virtio
            name: datavolumedisk1
          - disk:
             bus: virtio
            name: cloudinitdisk
          interfaces:
          - masquerade: {}
            name: default
        resources:
          requests:
            memory: 1024Mi
      terminationGracePeriodSeconds: 0
      volumes:
      - dataVolume:
          name: fedora-40-dv-template
        name: datavolumedisk1
      - cloudInitNoCloud:
          userData: |-
            #cloud-config
            password: fedora
            chpasswd: { expire: False }
        name: cloudinitdisk
```

This will create both a virtual machine definition, and a datavolume containing the Fedora operating system. At the time of this writing Fedora 40. `running: true` indicates the virtual machine should be started after creation. The datavolume will also create a `PersistentVolumeClaim` called `fedora-40-dv-template` which is the same name as the datavolume. After a while the persistent volume will be populated with the operating system, and the virtual machine is started.

### Creating a migration plan
To migrate our `mig-vm` namespace, we'll ensure that the `namespaces` field of our MigPlan includes mig-vm. 

Modify the contents of [config/samples/mig-plan.yaml](https://github.com/konveyor/mig-controller/blob/master/config/samples/mig-plan.yaml), adding 'mig-vm' to 'namespaces'.

```yaml
apiVersion: migration.openshift.io/v1alpha1
kind: MigPlan
metadata:
  name: live-migrate-plan
  namespace: openshift-migration
spec:
  # [!] Change namespaces to adjust which OpenShift namespaces should be migrated 
  #     from source to destination cluster.
  namespaces:
  - mig-vm

[... snipped, see config/samples/mig-plan.yaml for other required fields ...]
```

In order to attempt a storage live migration the `liveMigrate` field in the spec must be set to true (and KubeVirt must be configred and able to do storage live migratio, see pre-requisites)

```yaml
apiVersion: migration.openshift.io/v1alpha1
kind: MigPlan
metadata:
  name: live-migrate-plan
  namespace: openshift-migration
spec:
  liveMigrate: true
  namespaces:
[... snipped, see config/samples/mig-plan.yaml for other required fields ...]
```

Live Migration only happens during cutover of a migration plan. Staging the migration plan will skip any running virtual machines and not sync the data. Any stopped virtual machine disks will be synced.

During the migration a `MigMigration` resource is created indicating what type of migration is happening:

* stage
* rollback
* cutover

The status of the `MigMigration` will contain progress information about any storage live migrations. Any offline migrations will have a `DirectVolumeMigrationProgress` that shows the progress of the offline migration.

Each MigMigration will create a `DirectVolumeMigration` if the migration plan is a direct volume migration plan. To do storage live migration a direct volume migration is required. The `DirectVolumeMigration` resource status will indicate progress and status of any ongoing migrations either live migration or offline migration.

### Current known limitations
* Cannot switch Persistent Volume volume mode.
* Can only storage migrate in the same namespace.

#### Online migration limitations
* Virtual machine must be running
* The volume housing the disk must not have any of the following properties:
  * Shareable (shared disks cannot be live migrated)
  * Hotplugged 
  * Virtio-fs (filesystem shared inside the VM, since virtio-fs volume are not live migrateable)
  * LUNs LUN -> Disk or Disk -> LUN is not supported in libvirt
  * LUNs with persistent reservation
  * Target Persistent Volume size must match the source Persistent Volume size
* Virtual machine must be migrated to a different node.