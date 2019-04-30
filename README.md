# mig-controller

## Prerequisites

 - golang compiler (tested @ 1.11.5)
 - kubebuilder (tested @ 1.0.7)
 - dep (tested @ v0.5.0)

---

## Quick-start

__1. Create required CRDs (MigMigration, MigPlan, MigCluster, Cluster...)__

Do this on the cluster where you'll be running the controller.

```
# Create 'Mig' CRDs
$ oc apply -f config/crds

# Create 'Cluster' CRD
$ oc apply -f https://raw.githubusercontent.com/kubernetes/cluster-registry/master/cluster-registry-crd.yaml
```

---

__2.  Use `make run` to run the controller from your terminal.__ 

The controller will connect to OpenShift using your currently active kubeconfig. You may need to run `oc login` first.

```
$ make run

go generate ./pkg/... ./cmd/...
go fmt ./pkg/... ./cmd/...
go vet ./pkg/... ./cmd/...
go run ./cmd/manager/main.go
{"level":"info","ts":1555619492,"logger":"entrypoint","msg":"setting up client for manager"}
{"level":"info","ts":1555619492,"logger":"entrypoint","msg":"setting up manager"}
{"level":"info","ts":1555619493,"logger":"entrypoint","msg":"Registering Components."}

[...]
```

---

__3. Create Mig CRs to describe the Migration that will be performed__

Before mig-controller can run a Migration, you'll need to provide:
 - Coordinates & auth info for 2 OpenShift clusters (source + destination)
 - A list of namespaces to be migrated
 - Storage to use for the migration

 These items can be specified by "Mig" CRs. For the source of truth on what will be accepted in CR fields, see the appropriate _types.go_ file.

- [MigPlan](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migplan_types.go)
- [MigCluster](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migcluster_types.go)
- [MigStorage](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migstorage_types.go)
- [MigAssetCollection](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migassetcollection_types.go)
- [MigStage](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migstage_types.go)
- [MigMigration](https://github.com/fusor/mig-controller/blob/master/pkg/apis/migration/v1alpha1/migmigration_types.go)
- [Cluster](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go)


```
make samples
# [... sample CR content will be copied to 'migsamples' dir]
```

Inspect and edit each of the files in the 'migsamples' directory, making changes as needed.

After modifying resource yaml, create the resources on the OpenShift cluster where the controller is running.

```
# Option 1: Create everything in a single command
oc apply -f migsamples

# ------------------------------------------------

# Option 2: Create resources individually
cd migsamples

# Source cluster definition 
# Note: no coordinates/auth needed when MigCluster has 'isHostCluster: true'
oc apply -f mig-cluster-local.yaml

# Destination cluster definition, coordinates, auth details
oc apply -f cluster-aws.yaml
oc apply -f sa-secret-aws.yaml
oc apply -f mig-cluster-aws.yaml

# Describes where to store data during Migration
oc apply -f mig-storage.yaml

# Describes which resources should be Migrated
oc apply -f mig-assets.yaml

# Describes which clusters, storage, and namespaces should be to run a Migration
oc apply -f mig-plan.yaml

# Declares that a Stage operation should be run
oc apply -f mig-stage.yaml

# Declares that a Migration operation should be run 
oc apply -f mig-migration.yaml
```

- See [config/samples](https://github.com/fusor/mig-controller/tree/master/config/samples) CR samples. It is _highly_ recommended to run `make samples` to copy these to the .gitignore'd 'migsamples' before filling out cluster details (URLs + SA tokens).

### Creating a Service Account (SA) token

For mig-controller to perform migration actions on a remote cluster, you'll need to provide:
- The remote OpenShift cluster URL
- A valid Service Account (SA) token granting 'cluster-admin' access to the remote cluster


To configure the SA token, run the following on the remote cluster:
```bash
# Create a new service account in the mig ns
oc create namespace mig
oc create sa -n mig mig
# Grant the 'mig' service account cluster-admin (cluster level root privileges, use with caution!)
oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:mig:mig
# Get the ServiceAccount token in a base64-encoded format to put in the remote MigCluster spec
oc sa get-token -n mig mig|base64 -w 0

```
Use the base64-encoded SA token from the last command output to fill in `migsamples/sa-secret-aws.yaml`