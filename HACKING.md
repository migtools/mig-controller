# Hacking on mig-controller

## Building and running mig-controller with `make run`

__1. Install prerequisites__

 - golang compiler (tested @ 1.14.4)
 - kubebuilder (tested @ 1.0.7)
 - mig-operator (tested @ latest) installed on both clusters involved in migration

__2. Clone mig-controller project to your `$GOPATH`__

Clone mig-controller to your $GOPATH so that dependencies in `vendor` will be found at build time.

```
# Sample of setting $GOPATH
mkdir -p $HOME/code/go
export GOPATH="$HOME/code/go"

# Running 'go get -d' will clone the mig-controller repo into the proper location on your $GOPATH
go get -d github.com/konveyor/mig-controller

# Take a peek at the newly cloned files
ls -al $GOPATH/src/github.com/konveyor/mig-controller
```

__3. Install migration components__

Follow [mig-operator](https://github.com/konveyor/mig-operator) instructions to install migration components. 


__4. Stop any running instances of mig-controller connected to the active cluster__

After installing migration components, make sure mig-controller is stopped so that we can connect a locally built copy to the cluster. 

```bash
# Open the `MigrationController` CR for editing
oc edit migrationcontroller -n openshift-migration
```

```yaml
# Set `migration_controller: false`
kind: MigrationController
metadata:
  name: migration-controller
  namespace: openshift-migration
spec:
[...]
  migration_controller: false
[...]
```

After making this edit, wait for mig-operator to remove the mig-controller Pod.

__5. Enable local mig-controller integration with mig-ui__

If desired, reconfigure mig-ui to communicate with your locally running mig-controller.

```bash
# Patches MigrationController CR to use local mig-controller + discovery service.
# Will set `discovery_api_url: http://localhost:8080` in mig-ui-config configmap.
make use-local-controller
```

__6.  Use `make run` to run the controller from your terminal.__

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

## Familiarizing with `make` targets

There are several useful Makefile targets for mig-controller that developers should be aware of.

| Command | Description |
| --- | --- |
| `make run` | Build a controller manager binary and run the controller against the active cluster |
| `make run-fast` | Same as make run, but skips generate, fmt, vet. Useful for quick iteration |
| `make use-local-controller` | Patches MigrationController CR to use local mig-controller + discovery service |
| `make use-oncluster-controller` | Patches MigrationController CR to use on-cluster mig-controller + discovery service |
| `make manager` | Build a controller manager binary |
| `make install` | Install generated CRDs onto the active cluster |
| `make manifests` | Generate updated CRDs from types.go files, RBAC from annotations in controller, deploy manifest YAML |
| `make samples` | Copy annotated sample CR contents from config/samples to migsamples, which is .gitignored and safe to keep data in |
| `make docker-build` | Build the controller-manager into a container image. Requires support for multi-stage builds. |

---
## Invoking CI on pull requests

You can invoke CI via webhook with an appropriate pull request comment command. CI will run a stateless migration and return results as a comment on the relevant PR where CI was requested.

| Command | Description |
| --- | --- |
| `\test` | Run  OCP 3 => 4 migration using the master branch of mig-operator   |
| `\test-with-operator <PR_number>` | Run  OCP 3 => 4 migration using a specified PR from mig-operator  |

---
## Design patterns, conventions, and practices

See [devguide.md](https://github.com/konveyor/mig-controller/blob/master/docs/devguide.md)
