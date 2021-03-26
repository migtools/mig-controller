# Hacking on mig-controller

## Design patterns, conventions, and practices

See [devguide.md](https://github.com/konveyor/mig-controller/blob/master/docs/devguide.md)

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
***Note***: controller-runtime version 0.3.0 is desired for development.   

## Invoking CI on pull requests

You can invoke CI via webhook with an appropriate pull request comment command. CI will run a stateless migration and return results as a comment on the relevant PR where CI was requested.

| Command | Description |
| --- | --- |
| `\test` | Run  OCP 3 => 4 migration using the master branch of mig-operator   |
| `\test-with-operator <PR_number>` | Run  OCP 3 => 4 migration using a specified PR from mig-operator  |

---

## Generating a phase performance profile

To understand phase performance and identify areas of the controller with greatest potential for improvement, mig-controller logs a "Phase completed" message at the end of each phase indicating elapsed seconds since the phase began.

```json
# Phase completion log
{"[...]", "migMigration": "my-migration", "msg":"Phase completed","Phase": "StartRefresh", "phaseElapsed":0.522302}
```

We can choose a particular MigMigration name to profile, and turn the series of phase completion messages into a performance report CSV.

### Obtaining logs for analysis

First, we need to grab the logs needed for analysis.
```sh
# Option 1) Get logs from mig-controller running in-cluster
oc get logs openshift-migration migration-controller-[...] > controller.log

# Option 2) Run mig-controller locally and output logs to file
make run-fast 2>&1 | tee controller.log
```

### Parsing logs for phase elapsed times

Once we have the logs, we can use this this one-liner to generate a CSV of phase timing information.
```
# Dependencies: jq, grep
echo "Logger, PhaseName, ElapsedSeconds, PercentOfTotal" > phase_timing.csv; total_time=$(cat controller.log | grep '\"Phase\ completed\"' | jq '([inputs | .phaseElapsed] | add)'); cat controller.log | grep '\"Phase\ completed\"' | jq -r --arg total_time "$total_time" '"\(.logger), \(.phase), \(.phaseElapsed), \(.phaseElapsed / ($total_time|tonumber))"' >> phase_timing.csv
```

We can take this CSV and plug it into a spreadsheet app to get color formatting and sort by longest phases.

![Phase Performance Table](./images/phase-perf-table.png)

---

## Using Jaeger to looks at mig-controller performance and interactions

Since mig-controller is composed of ~10 sub-controllers and also relies on Velero, it's useful to get a high-level view of interaction between all of the controllers. Jaeger tracing provides this view.

### Starting a Jaeger collector

Currently, we have support for running a Jaeger collector and mig-controller locally. The Jaeger collector will connect with mig-controller and pull traces from it.

```
docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HTTP_PORT=9411 \
  -p 5775:5775/udp \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 14250:14250 \
  -p 9411:9411 \
  jaegertracing/all-in-one:1.22
```

After you've started the collector, navigate to the Web UI at http://localhost:16686

### Starting mig-controller with Jaeger tracing enabled

To run mig-controller with Jaeger tracing enabled, set the env var `JAEGER_ENABLE=true`.

```sh
JAEGER_ENABLED=true make run-fast
```
