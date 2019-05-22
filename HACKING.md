# Hacking on mig-controller

## Building and running mig-controller with `make run`

__1. Install prerequisites__

 - golang compiler (tested @ 1.11.5)
 - kubebuilder (tested @ 1.0.7)
 - dep (tested @ v0.5.0)
 - velero (tested @ v0.11.0) installed on both clusters involved in migration

__2. Clone the project to your `$GOPATH`__

Clone mig-controller to your $GOPATH so that dependencies in `vendor` will be found at build time.

```
# Sample of setting $GOPATH
$ mkdir -p $HOME/code/go
$ export GOPATH="$HOME/code/go"

# Running 'go get -d' will clone the mig-controller repo into the proper location on your $GOPATH
$ go get -d github.com/fusor/mig-controller

# Take a peek at the newly cloned files
$ ls -al $GOPATH/src/github.com/fusor/mig-controller
```

__3. Create required CRDs (MigMigration, MigPlan, MigCluster, Cluster...)__

Do this on the cluster where you'll be running the controller.

```
# Create 'Mig' CRDs
$ oc apply -f config/crds

# Create 'Cluster' CRD
$ oc apply -f https://raw.githubusercontent.com/kubernetes/cluster-registry/master/cluster-registry-crd.yaml
```

---

__4.  Use `make run` to run the controller from your terminal.__

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

## Familiarizing with `make` targets

There are several useful Makefile targets for mig-controller that developers should be aware of.

| Command | Description |
| --- | --- |
| `make run` | Build a controller manager binary and run the controller against the active cluster |
| `make manager` | Build a controller manager binary |
| `make install` | Install generated CRDs onto the active cluster |
| `make manifests` | Generate updated CRDs from types.go files, RBAC from annotations in controller, deploy manifest YAML |
| `make samples` | Copy annotated sample CR contents from config/samples to migsamples, which is .gitignored and safe to keep data in |
| `make docker-build` | Build the controller-manager into a container image. Requires support for multi-stage builds, which may require moby-engine |
