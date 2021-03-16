# mig-controller

## Installing
mig-controller is installed by [mig-operator](https://github.com/konveyor/mig-operator).

## Demo Video 

This [video](https://www.youtube.com/watch?v=OaRp4_j9F_A) shows a CLI driven migration of a [rocket-chat](https://github.com/konveyor/mig-demo-apps/tree/master/apps/rocket-chat) demo app and associated PersistentVolume data.

[![Watch the demo](https://user-images.githubusercontent.com/7576968/111339370-16061300-864e-11eb-8ae5-37a250c65f08.png)](https://www.youtube.com/watch?v=OaRp4_j9F_A)

## Development

[HACKING.md](./HACKING.md) has instructions for:
 - Building mig-controller
 - Running mig-controller locally
 - Invoking CI on pull requests

 [DEVGUIDE.md](./docs/devguide.md) has guidelines for:
 - Design patterns
 - Conventions
 - Dev practices

## Quick-start: CLI based migration

__1. Identify a pair of running OpenShift clusters to migrate workloads between__

mig-controller can help you move OpenShift application workloads from a _source_ to a _destination_ cluster. You'll need cluster-admin permissions on both OpenShift clusters. 

- **velero** will need to be installed on both clusters, and will be driven by mig-controller
- **mig-controller** should only be installed on _one of the two_ clusters. You can decide which cluster will host this component. 

---

__2. Use mig-operator to deploy Migration Tools to both the _source_ and _destination_ OpenShift clusters__

Use mig-operator to install selected components of Migration Tooling (mig-controller, mig-ui, velero) onto your source and destination OpenShift clusters.

After installing mig-operator, you can select which components should be installed by creating a [MigrationController CR](https://github.com/konveyor/mig-operator#migration-controller-installation):

```
apiVersion: migration.openshift.io/v1alpha1
kind: MigrationController
[...]
spec:
  migration_velero: true
  migration_controller: true
  migration_ui: true
 
[...]
```

See mig-operator docs for more details: https://github.com/konveyor/mig-operator

---

__3. Create Mig CRs to describe the Migration that will be performed__

Before mig-controller can run a Migration, you'll need to provide it with:
 - Coordinates & auth info for 2 OpenShift clusters (source + destination)
 - A list of namespaces to be migrated
 - Storage to use for the migration

 These details are specified by "Mig" CRs. For the source of truth on what will be accepted in CR fields, see the appropriate _types.go_ file.

- [MigPlan](https://github.com/konveyor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migplan_types.go)
- [MigCluster](https://github.com/konveyor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migcluster_types.go)
- [MigStorage](https://github.com/konveyor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migstorage_types.go)
- [MigMigration](https://github.com/konveyor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migmigration_types.go)

---

*__To make it easier to run your first Migration with mig-controller__*, we've published a set of annotated sample CRs in [config/samples](https://github.com/konveyor/mig-controller/tree/master/config/samples) that you can walk through and fill out values on. The first step will be to run `make samples`, which will copy these to `migsamples`.

```
make samples
# [... sample CR content will be copied to 'migsamples' dir, which is .gitignored]
```

These sample resources describe a migration where the _source_ cluster is running the controller, so a Service Account (SA) token and cluster URL must be provided for the _destination_ cluster only.

---

**_Inspect and edit each of the files in the 'migsamples' directory, making changes as needed._** Much of the content in these sample files can stay unchanged. 

As an example, you'll need to provide the following parameters to perform a Migration using an AWS S3 bucket as temporary migration storage:

| Parameter | Purpose | Sample CR File |
| --- | --- | --- |
| `namespaces` | List of namespaces to migrate from source to destination cluster | `mig-plan.yaml` |
| `url` | Endpoint of remote cluster mig-controller will connect to | `cluster-remote.yaml` |
| `saToken` | Base64 encoded SA token used to authenticate with remote cluster | `sa-secret-remote.yaml` | 
| `awsBucketName` | Name of the S3 bucket to be used for temporary Migration storage | `mig-storage.yaml` |
| `awsRegion` | Region of S3 bucket to be used for temporary Migration storage | `mig-storage.yaml` |
| `aws-access-key-id` | AWS access key to auth with AWS services | `mig-storage-creds.yaml` |
| `aws-secret-access-key` | AWS secret access key to auth with AWS services | `mig-storage-creds.yaml` |


After modifying resource yaml, create the resources on the OpenShift cluster where the controller is running.

```bash
# Option 1: Create everything in a single command
oc apply -f migsamples

# ------------------------------------------------

# Option 2: Create resources individually
cd migsamples

# Source cluster definition 
# Note: no coordinates/auth needed when MigCluster has 'isHostCluster: true'
oc apply -f mig-cluster-local.yaml

# Destination cluster definition, coordinates, auth details
oc apply -f sa-secret-remote.yaml
oc apply -f mig-cluster-remote.yaml

# Describes where to store data during Migration, storage auth details
oc apply -f mig-storage-creds.yaml
oc apply -f mig-storage.yaml

# Describes which clusters, storage, and namespaces should be to run a Migration
oc apply -f mig-plan.yaml

# Declares that a Migration operation should be run 
oc apply -f mig-migration.yaml
```

---

### Getting a migration-controller Service Account (SA) token

For mig-controller to perform migration actions on a remote cluster, you'll need to provide a valid Service Account (SA) token granting 'cluster-admin' access to the remote cluster.


To get the SA token created by mig-operator, run the following on the remote cluster:
```bash
# Get the ServiceAccount token in a base64-encoded format to put in the remote MigCluster spec
# If you are providing this token to mig-ui, skip the base64 encoding step
oc sa get-token -n openshift-migration migration-controller | base64 -w 0
```
Use the base64-encoded SA token from the last command output to fill in `migsamples/sa-secret-remote.yaml`
