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
$ oc create -f config/crds

# Create 'Cluster' CRD
$ oc create -f https://raw.githubusercontent.com/kubernetes/cluster-registry/master/pkg/apis/clusterregistry/v1alpha1/types.go
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
# Source cluster coordinates & auth details
oc create -f cluster-src.yaml
oc create -f cluster-src-sa-secret.yaml
oc create -f mig-cluster-src.yaml

# Destination cluster coordinates & auth details
oc create -f cluster-dest.yaml
oc create -f cluster-dest-sa-secret.yaml
oc create -f mig-cluster-dest.yaml

# Describes where to store data during Migration
oc create -f mig-storage.yaml

# Describes which resources should be Migrated
oc create -f mig-assets.yaml

# Describes which clusters, storage, and namespaces should be to run a Migration
oc create -f mig-plan.yaml

# Declares that a Stage operation should be run
oc create -f mig-stage.yaml

# Declares that a Migration operation should be run 
oc create -f mig-migration.yaml
```

- See [config/samples](https://github.com/fusor/mig-controller/tree/master/config/samples) for blank CR samples
