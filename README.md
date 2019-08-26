# mig-controller

## Quick-start

__1. Identify a pair of running OpenShift clusters to migrate workloads between__

mig-controller can help you move OpenShift application workloads from a _source_ to a _destination_ cluster. You'll need cluster-admin permissions on both OpenShift clusters. 

- **velero** will need to be installed on both clusters, and will be driven by mig-controller
- **mig-controller** should only be installed on _one of the two_ clusters. You can decide which cluster will host this component. 

---

__2. Deploy Velero to both the _source_ and _destination_ OpenShift clusters__

```bash
# Download bash script to deploy Velero along with required plugins
wget https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/deploy_velero.sh

# Login to source cluster, run 'deploy_velero.sh' against it
oc login https://my-source-cluster:8443
bash deploy_velero.sh

# Login to destination cluster, run 'deploy_velero.sh' against it
oc login https://my-destination-cluster:8443
bash deploy_velero.sh
```

---

__3. Deploy _mig-controller_ to one of the two involved clusters__

```bash
# Download bash script to deploy the latest mig-controller image as a StatefulSet
wget https://raw.githubusercontent.com/fusor/mig-controller/master/hack/deploy/deploy_mig.sh

# Login to cluster where mig-controller will run, 'deploy_mig.sh' against it
oc login https://my-cluster:8443
bash deploy_mig.sh
```

---

__4. Create Mig CRs to describe the Migration that will be performed__

Before mig-controller can run a Migration, you'll need to provide it with:
 - Coordinates & auth info for 2 OpenShift clusters (source + destination)
 - A list of namespaces to be migrated
 - Storage to use for the migration

 These items can be specified by "Mig" CRs. For the source of truth on what will be accepted in CR fields, see the appropriate _types.go_ file.

- [MigPlan](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migplan_types.go)
- [MigCluster](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migcluster_types.go)
- [MigStorage](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migstorage_types.go)
- [MigMigration](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migmigration_types.go)
- [Cluster](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go)

---

*__To make it easier to run your first Migration with mig-controller__*, we've published a set of annotated sample CRs in [config/samples](https://github.com/fusor/mig-controller/tree/master/config/samples) that you can walk through and fill out values on. The first step will be to run `make samples`, which will copy these to `migsamples`.

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
| `serverAddress` | Endpoint of remote cluster mig-controller will connect to | `cluster-remote.yaml` | 
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
oc apply -f cluster-remote.yaml
oc apply -f sa-secret-remote.yaml
oc apply -f mig-cluster-remote.yaml

# Describes where to store data during Migration, storage auth details
# Note: the contents of mig-storage-creds.yaml will be used to overwrite Velero cloud-credentials
oc apply -f mig-storage-creds.yaml
oc apply -f mig-storage.yaml

# Describes which clusters, storage, and namespaces should be to run a Migration
oc apply -f mig-plan.yaml

# Declares that a Migration operation should be run 
oc apply -f mig-migration.yaml
```

---

### Creating a Service Account (SA) token

For mig-controller to perform migration actions on a remote cluster, you'll need to provide a valid Service Account (SA) token granting 'cluster-admin' access to the remote cluster.


To configure the SA token, run the following on the remote cluster:
```bash
# Create a new service account in the mig ns
oc create namespace openshift-migration
oc create namespace openshift-migration
oc create sa -n openshift-migration mig
# Grant the 'mig' service account cluster-admin (cluster level root privileges, use with caution!)
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:mig:mig
# Get the ServiceAccount token in a base64-encoded format to put in the remote MigCluster spec
oc sa get-token -n openshift-migration mig|base64 -w 0

```
Use the base64-encoded SA token from the last command output to fill in `migsamples/sa-secret-remote.yaml`
