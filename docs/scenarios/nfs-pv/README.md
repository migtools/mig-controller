## Migrating a *stateful* OpenShift app with *NFS PVs*

This scenario walks through Migration of a stateful OpenShift app with Persistent Volume (PV) claims tied into [NFS (Network File System)](https://en.wikipedia.org/wiki/Network_File_System).

### Supported PV Actions - NFS to NFS

| Action | Supported | Description |
|-----------|------------|-------------|
| Copy | Yes | Create new PV on *destination cluster*. Restic will copy data from source PV to destination PV |
| Move | Yes  | Detach PV from *source cluster*, then re-attach to *destination cluster* without copying data |

---

### 1. Prerequisites

Referring to the getting started [README.md](https://github.com/fusor/mig-controller/blob/master/README.md), you'll first need to deploy mig-controller and Velero, and then create the following 'Mig' resources on the cluster where mig-controller is running to prepare for Migration:

- `MigCluster` resources for the _source_ and _destination_ clusters
- `Cluster` resource for any _remote_ clusters (e.g. clusters the controller will connect to remotely, there will be at least one of these)
- `MigStorage` providing information on how to store resource YAML in transit between clusters 

Before proceeding, be sure that you have at least one available NFS PV on your *source cluster* that our sample MySQL app will be able to bind to. You can set up the NFS server however you like. We used Ansible Playbooks from the [mig-ci](https://github.com/fusor/mig-ci) repo to provision the NFS PVs used in this scenario, but other NFS server + PV configurations compatible with OpenShift should work equally well.

- [nfs_server_deploy.yml](https://github.com/fusor/mig-ci/blob/master/nfs_server_deploy.yml)
- [nfs_provision_pvs.yml](https://github.com/fusor/mig-ci/blob/master/nfs_provision_pvs.yml)

Once you have at least one PV available with '10Gi' capacity (required by the mysql template we'll be using), you can proceed with the scenario.

```
$ oc get PersistentVolumes
NAME      CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS 
pv1       100Gi      RWO,ROX,RWX    Retain           Available
pv2       100Gi      RWO,ROX,RWX    Retain           Available
```

The sample app used in this scenario has been adapted from the [mysql_pvc](https://github.com/fusor/ocp-mig-test-data/tree/master/roles/pvc/mysql_pvc) Ansible role.

#### 1.1. Prerequisites - _Copy_

Copying NFS PV disk contents is currently _unsupported_ by mig-controller. Future support is planned.

#### 1.2. Prerequisites - _Move_

To take a 'move' action on NFS PVs, the `path` and `server` specified in _source cluster_ NFS PVs must be **accessible** for mounting from the _destination cluster_. 

```yaml
apiVersion: v1
kind: PersistentVolume
[...]
spec:
  # [!] NFS path and server URL in PV defined on host cluster will be 'moved' over
  #     to destination cluster as-is. The PV disk data will not be manipulated, just
  #     re-mounted to destination cluster Pods after quiescing (stopping) source cluster Pods.
  nfs:
    path: /var/lib/nfs/exports/pv2  
    server: 123.234.321.210        
[...]
```

---

### 2. Deploying a _stateful_ sample app (MySQL with NFS PVs)

After you've created a suitable PV on your *source cluster*, create the provided [MySQL template](mysql-persistent-template.yaml) provided with this scenario.

```bash
$ oc create -f mysql-persistent-template.yaml 
namespace/mysql-persistent created
secret/mysql created
service/mysql created
persistentvolumeclaim/mysql created
deploymentconfig.apps.openshift.io/mysql created
```

Note that the template deploys our MySQL instance to a new namespace `mysql-persistent`.

Verify that the MySQL deployment bound to one of your PVs successfully.

```bash
$ oc get PersistentVolumes
NAME      CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS      CLAIM               
pv1       100Gi      RWO,ROX,RWX    Retain           Available
pv2       100Gi      RWO,ROX,RWX    Retain           Bound       mysql-persistent/mysql
```

---

### 3. Populating MySQL with sample data

Before performing a migration, let's populate our MySQL database with some data. Start by getting the pod name of the MySQL instance that we deployed in the previous step.

```bash
$ oc get pods -n mysql-persistent --selector "deployment=mysql-1"
NAME            READY     STATUS    RESTARTS   AGE
mysql-1-k985t   1/1       Running   0          35m
```

Now that we have the pod name, we can copy `data.sql` to `opt/app-root/src/data.sql` in our MySQL pod. We'll populate the database with some sample data this way.
```bash
$ oc cp -n mysql-persistent data.sql mysql-1-k985t:/opt/app-root/src
```

```
$ oc exec -n mysql-persistent mysql-1-k985t -- /bin/bash -c "mysql -uMYSQL_USER -pMYSQL_PASSWORD MYSQL_DATABASE < /opt/app-root/src/data.sql"
```

Running data.sql against the database may take a while. After it exits, you can check the mounted NFS volume consumption.

```
$ oc exec -n mysql-persistent mysql-1-gpl2p -- /bin/bash -c df -h
Filesystem                                1K-blocks  Used      Available  Use%  Mounted on
123.234.321.210:/var/lib/nfs/exports/pv2  10473472   2881536   7591936    28%   /var/lib/mysql/data
[...]
```

---

### 4. Modifying MigPlan and MigMigration to migrate our NFS PV backed MySQL database

To migrate our `mysql-persistent` namespace, we'll ensure that the `namespaces` field of our MigPlan includes mysql-persistent. 

Modify the contents of [config/samples/mig-plan.yaml](https://github.com/fusor/mig-controller/blob/master/config/samples/mig-plan.yaml), adding 'mysql-persistent' to 'namespaces'.

```yaml
apiVersion: migration.openshift.io/v1alpha1
kind: MigPlan
spec:
  # [!] Change namespaces to adjust which OpenShift namespaces should be migrated 
  #     from source to destination cluster.
  namespaces:
  - mysql-persistent

[... snipped, see config/samples/mig-plan.yaml for other required fields ...]
```

We also need to create a MigMigration with `quiescePods: true` since only one MySQL Pod can hold the lock at once, and this scenario covers the 'move' PV action that re-mounts the *source cluster* NFS PVs on the *destination cluster*

Modify the contents of [config/samples/mig-migration.yaml](https://github.com/fusor/mig-controller/blob/master/config/samples/mig-migration.yaml), changing `quiescePods` to 'true'.

```yaml
apiVersion: migration.openshift.io/v1alpha1
kind: MigMigration
spec:
  # [!] Change quiescePods to 'true' to ensure that the MySQL pod on the source cluster
  #     is terminated, which will allow a new pod on the destination cluster to acquire
  #     the database lock.
  quiescePods: true

[... snipped, see config/samples/mig-migration.yaml for other required fields ...]
```

---

### 5. Create the MigPlan and MigMigration, specify PV actions

Let's create our MigPlan and also create a MigMigration referencing our MigPlan to start moving this app over to our _destination_ cluster.

```bash
# From project root, run `make samples` to put these yaml files in a 'migsamples' directory your can safely modify.

# Creates MigPlan 'migplan-sample' in namespace 'mig'
$ oc apply -f mig-plan.yaml

# Describe the MigPlan. Assuming the controller is running, validations
# should have run against the plan, and you should be able to see 
# "The 'persistentVolumes' list has been updated with discovered PVs."
$ oc describe migplan migplan-sample -n mig
[...]
Kind:         MigPlan
Metadata:
  [...]
  Persistent Volumes:
    Name:  pv2
    Supported Actions:
      copy
      move
  [...]
Status:
  Conditions:
    Category:              Error
    Last Transition Time:  2019-05-29T20:35:44Z
    Message:               PV in 'persistentVolumes' [pv2] has an unsupported 'action'.
    Reason:                NotDone
    Status:                True
    Type:                  PvInvalidAction

    Category:              Required
    Last Transition Time:  2019-05-29T21:13:25Z
    Message:               The 'persistentVolumes' list has been updated with discovered PVs.
    Reason:                Done
    Status:                True
    Type:                  PvsDiscovered
    [...]
Events:                    <none>


# We need to assign an action for each PV that has been detected. In this 
# case, there is only one PV 'pv2'. We'll assign the 'move' action like so:
$ oc edit migplan migplan-sample -n mig
[...]
  persistentVolumes:
  - name: pv2
    action: move
[... save changes to the MigPlan after making this change ...]

# After selecting an action for each PV, the 'unsupported action' error should
# be replaced by 'The migration plan is ready'.
$ oc describe migplan migplan-sample -n mig
[...]
Message:               The migration plan is ready.
Status:                True
Type:                  Ready

# If you see 'The migration plan is ready.' from the 'oc describe' above,
# proceed to creation of a MigMigration that will execute our MigPlan. 
# If the plan is not ready, make edits to the MigPlan resource as necessary
# using the feedback provided by 'oc describe'.

# Create MigMigration 'migmigration-sample' in namespace 'mig'
$ oc apply -f mig-migration.yaml
```

After creating the MigMigration resource, you can monitor its progress with `oc describe`. If the migration is running, you'll see a "Task Phase" reflecting current progress.

---

### 6. Monitor migration progress

```
$ oc describe migmigration migmigration-sample -n mig
Name:         migmigration-sample
Namespace:    mig
Status:
  Migration Started:       true
  Start Timestamp:         2019-05-31T14:22:08Z
  Task Phase:              BackupStarted
```

The first phase that takes some time to complete is `BackupStarted`, as shown above. 

Recall that we set `quiescePods: true` on the MigMigration resource in an earlier step. Due to this setting, the migrated Pods on the *source cluster* will be scaled down when the `Task Phase` progresses beyond BackupStarted.

```
# Since we are at 'WaitOnBackupReplication', the Pods have already started scaling down.
$ oc describe migmigration migmigration-sample -n mig
Name:         migmigration-sample
Namespace:    mig
  Migration Started:       true
  Start Timestamp:         2019-05-31T14:22:08Z
  Task Phase:              WaitOnBackupReplication

# Note the mysql-1-k985t Pod is scaling down on the source cluster.
$ oc get pods -n mysql-persistent 
NAME             READY     STATUS        RESTARTS   AGE
mysql-1-k985t    1/1       Terminating   1          2h
mysql-1-deploy   0/1       Completed     0          2h
```

Assuming no errors are encountered, describing the MigMigration will eventually show `Migration Completed: true`.

```
$ oc describe migmigration migmigration-sample -n mig
Name:         migmigration-sample
Namespace:    mig
[...]
  Migration Completed:     true
  Start Timestamp:         2019-05-31T14:22:08Z
  Task Phase:              Completed
```

---

### 7. Inspect migration results on *destination cluster*

Logging into the *destination cluster*, we should now see a running mysql pod in the `mysql-persistent` namespace, matching what existed on the *source cluster*.

```
$ oc get pods -n mysql-persistent
NAME            READY     STATUS    RESTARTS   AGE
mysql-1-vs9b6   1/1       Running   1          3m
```

We can inspect the PersistentVolumes on the *destination cluster* and notice `pv2` has been created which matches the contents of `pv2` on the *source cluster*.

```
$ oc get PersistentVolumes
NAME      CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS    CLAIM                    STORAGECLASS   REASON    AGE
pv2       100Gi      RWO,ROX,RWX    Retain           Bound     mysql-persistent/mysql                            3m

$ oc describe pv pv2
Name:            pv2
[...]
Status:          Bound
Claim:           mysql-persistent/mysql
Source:
    Type:      NFS (an NFS mount that lasts the lifetime of a pod)
    Server:    123.234.321.210
    Path:      /var/lib/nfs/exports/pv2
[...]

```

To verify that the MySQL database is running, we can check the Pod logs. If you forgot to set `quiescePods: true` and selected a PV action `move`, you should expect to see error messages regarding failure to acquire the database lock until you manually scale down MySQL on the *source cluster*. This is because each MySQL instance would be connected to the same NFS mount.

```
# Assuming we set 'quiescePods: true' using PV action 'move', we shouldn't see any errors acquiring the lock
$ oc logs mysql-1-vs9b6 | tail 

2019-05-30T17:52:39.696393Z 0 [Warning] 'user' entry 'mysql.sys@localhost' ignored in --skip-name-resolve mode.
2019-05-30T17:52:39.696423Z 0 [Warning] 'db' entry 'performance_schema mysql.session@localhost' ignored in --skip-name-resolve mode.
2019-05-30T17:52:39.696430Z 0 [Warning] 'db' entry 'sys mysql.sys@localhost' ignored in --skip-name-resolve mode.
2019-05-30T17:52:39.696443Z 0 [Warning] 'proxies_priv' entry '@ root@localhost' ignored in --skip-name-resolve mode.
2019-05-30T17:52:39.735245Z 0 [Note] InnoDB: Buffer pool(s) load completed at 190530 17:52:39
2019-05-30T17:52:39.739264Z 0 [Warning] 'tables_priv' entry 'user mysql.session@localhost' ignored in --skip-name-resolve mode.
2019-05-30T17:52:39.739282Z 0 [Warning] 'tables_priv' entry 'sys_config mysql.sys@localhost' ignored in --skip-name-resolve mode.
2019-05-30T17:52:39.766919Z 0 [Note] Event Scheduler: Loaded 0 events
2019-05-30T17:52:39.767119Z 0 [Note] /opt/rh/rh-mysql57/root/usr/libexec/mysqld: ready for connections.
Version: '5.7.24'  socket: '/var/lib/mysql/mysql.sock'  port: 3306  MySQL Community Server (GPL)
```

Finally, checking the mounted NFS volume on the MySQL Pod. Running `df -h` on the destination MySQL pod, we can expect to see roughly the same amount of `Use%` on each side, since the same NFS export from the *source cluster* is mounted to the new Pod.

```
$ oc exec -n mysql-persistent mysql-1-k985t -- /bin/bash -c df -h
bash-4.2$ df -h
Filesystem                                1K-blocks  Used      Available  Use%  Mounted on
123.234.321.210:/var/lib/nfs/exports/pv2  10473472   2881536   7591936    28%   /var/lib/mysql/data
[...]
```
