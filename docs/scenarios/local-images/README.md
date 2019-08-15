## Migrating OpenShift apps running *local images* with mig-controller

This scenario walks through Migration of apps using images stored in local OpenShift image registries. 

We'll cover migration of:
- DeploymentConfigs
- Deployments
- Jobs
- DaemonSets
- StatefulSets
- Standalone Pods

---

### 1. Prerequisites

Referring to the getting started [README.md](https://github.com/fusor/mig-controller/blob/master/README.md), you'll first need to deploy mig-controller and Velero (including ocp-velero-plugin), and then create the following 'Mig' resources on the cluster where mig-controller is running to prepare for Migration:

|Resource|Purpose|
|---|---|
|`MigCluster`|represents the _source_ and _destination_ clusters|
|`Cluster`|describes coordinates of any _remote_ clusters (at least one)|
|`MigStorage`|provides config for storing resource YAML in transit between clusters |
 
### 2. Deploying the sample apps

On the _source_ cluster (where you'll migrate your application _from_), create a namespace to hold some sample resources.

```bash
# Login to the migration 'source cluster'
$ oc login https://my-source-cluster:8443

# Create a namespace for the test resources
$ oc create namespace registry-example
$ oc project registry-example
```

#### 2a. Building a local image with s2i (source-to-image)

Use `oc new-app` to build a local nodejs s2i image.
```bash
# Running 'oc new-app' will result in a new image being built and pushed to our local registry
$ oc new-app https://github.com/openshift/nodejs-ex
```

Wait for the build to complete.

```bash
# When `oc get build` shows `nodejs-ex-1` with a status of `Complete`, move on to the next step.
$ oc get build
NAME          TYPE      FROM          STATUS     STARTED          DURATION
nodejs-ex-1   Source    Git@e59fe75   Complete   2 minutes ago   1m14s
```

Viewing status information will reveal that a DeploymentConfig has been created based on our locally built image.

```bash
$ oc status
In project registry-example on server [...]

svc/nodejs-ex - 172.30.91.99:8080
  dc/nodejs-ex deploys istag/nodejs-ex:latest <-
    bc/nodejs-ex source builds https://github.com/openshift/nodejs-ex on openshift/nodejs:10 
    deployment #1 deployed 3 minutes ago - 1 pod
```

Create a route in front of the nodejs-ex service to use for testing later in this scenario.

```bash
$ oc expose service nodejs-ex
route.route.openshift.io/nodejs-ex exposed
```

#### 2b. Deploying the local 'nodejs-ex' image in other ways

To illustrate mig-controller handling migration of different OpenShift resource types, we'll deploy our s2i image from _step 2a._ in these additional ways:

- DeploymentConfig (this time without a build trigger)
- Deployment
- Job
- DaemonSet
- StatefulSet
- Standalone Pod

First, set up an environment variable with the local registry hostname.
```bash
$ export REGISTRY_HOST=$(oc registry info)
```

_DeploymentConfig_
```bash
# Create a DeploymentConfig using the local registry S2I image
$ oc run test-dc --image=$REGISTRY_HOST/registry-example/nodejs-ex:latest
```

_Deployment_
```bash
# Create a Deployment using the local registry S2I image
$ ./create_deployment.sh
```

_Job_
```bash
# Create a Job using the local registry S2I image
$ oc run test-job --image=$REGISTRY_HOST/registry-example/nodejs-ex:latest -n registry-example --restart='OnFailure'
```

_DaemonSet_
```bash
# Create a DaemonSet using the local registry S2I image
$ ./create_daemonset.sh
```

_StatefulSet_
```bash
# Create a StatefulSet using the local registry S2I image
$ ./create_statefulset.sh
```

_Standalone Pod_
```bash
# Create a Pod using the local registry S2I image
$ oc run test-standalone --image=$REGISTRY_HOST/registry-example/nodejs-ex:latest -n registry-example --restart='Never'
```

#### 2c. Verifying creation of apps using local 'nodejs-ex' image

After creating all of these resources, verify that all associated Pods are running.

```bash
# Making sure the pods are running
$ oc get pods -n registry-example
```

We can check that the two pods with exposed routes are serving their starter webpage successfully before continuing.

```bash
# Get the app route host/port
$ oc get route -n registry-example
NAME               HOST/PORT								PATH      SERVICES           PORT       
nodejs-ex          nodejs-ex-registry-example.apps.my-source-cluster.example.com        	  nodejs-ex          8080-tcp   
test-statefulset   test-statefulset-registry-example.apps.my-source-cluster.example.com 	  test-statefulset   8080-tcp   


# Verify the route is accessible and serving the expected content (for both routes above)
$ curl nodejs-ex-registry-example.apps.my-source-cluster.example.com
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
  <title>Welcome to OpenShift</title>
[... snipped ...]

```

Next, let's fill out our 'MigPlan' specifying the `registry-example` namespace should be moved to our destination cluster during the migration.

---

### 3. Create a 'MigPlan' that references our 'registry-example' namespace

To migrate our `registry-example` namespace, we'll ensure that the `namespaces` field of our MigPlan includes registry-example. Starting with [config/samples/mig-plan.yaml](https://github.com/fusor/mig-controller/blob/master/config/samples/mig-plan.yaml) create a mig-plan yaml file which specifies `registry-example` in the `spec.namespaces` field.

```yaml
apiVersion: migration.openshift.io/v1alpha1
kind: MigPlan
  # [!] Change namespaces to adjust which OpenShift namespaces should be migrated from source to destination cluster
  namespaces:
  - registry-example

[... snipped, see config/samples/mig-plan.yaml for other required fields ...]
```

After changing the 'nginx-example' namespace to 'registry-example', no further changes are needed. Let's create our MigPlan and also create a MigMigration referencing that MigPlan to start moving this app over to our _destination_ cluster.

```bash
# From project root, run `make samples` to put these yaml files in a 'migsamples' directory your can safely modify.

# Creates MigPlan 'migplan-sample' in namespace 'mig'
$ oc apply -f mig-plan.yaml

# Describe our MigPlan. Assuming the controller is running, validations
# should have run against the plan, and you should be able to see 
# "The Migration Plan is ready" or a list of issues to resolve.
$ oc describe migplan migplan-sample -n mig
Name:         migplan-sample
Namespace:    openshift-migration
API Version:  migration.openshift.io/v1alpha1
Kind:         MigPlan

Status:
  Conditions:
    Category:              Required
    Last Transition Time:  2019-05-24T14:50:06Z
    Message:               The migration plan is ready.
    Status:                True
    Type:                  Ready
    [... snipped ...]


# If you see 'The migration plan is ready.' from the 'oc describe' above,
# proceed to creation of a MigMigration that will execute our MigPlan. 
# If the plan is not ready, make edits to the MigPlan resource as necessary
# using the feedback provided by 'oc describe'.

# Create MigMigration 'migmigration-sample' in namespace 'mig'
$ oc apply -f mig-migration.yaml

# Monitor progress of the migration with 'oc describe'. You should see 
# 'The migration is ready', otherwise you'll see an error condition within
# 'oc describe' output indicating what action you need to take before the 
# migration can begin.
$ oc describe migmigration -n mig migmigration-sample
Name:         migmigration-sample
Namespace:    openshift-migration
[... snipped ...]
Status:
  Completion Timestamp:    2019-05-22T21:46:09Z
  Migration Completed:     true
  Start Timestamp:         2019-05-22T21:43:27Z
  Task Phase:              Completed
  [... snipped ...]
```

Notice how the MigMigration shown above has 'Task Phase': Completed. This means that the Migration is complete.

We should be able to verify our apps existence on the destination cluster. You can continuously describe the MigMigration to see phase info, or tail the mig-controller logs with `oc logs -f <pod-name>`.

---


### 4. Verify that the Migration completed successfully

Before moving onto this step, make sure that `oc describe` on the MigMigration resource indicates that the migration is complete.

To double-check the work mig-controller did, login to our destination cluster and verify existence of the pods and route we used to inspect our stateless app previously. If the 'registry-example' namespace didn't previously exist on the destination cluster, it should have been created.

```bash
# Login to the migration 'destination cluster'
$ oc login https://my-destination-cluster:8443

# Make sure pods are running
$ oc get pods -n registry-example
NAME                               READY     STATUS    RESTARTS   AGE
nodejs-ex-1-c6chd                  1/1       Running   0          13m
test-daemonset-zxhvq               1/1       Running   0          21m
test-deployment-7597cd5c9d-dtr7j   1/1       Running   0          13m
test-job-4t8qk                     1/1       Running   0          13m
test-statefulset-0                 1/1       Running   0          11m

# Get the app route host/port
$ oc get route -n registry-example
NAME               HOST/PORT								PATH      SERVICES           PORT       
nodejs-ex          nodejs-ex-registry-example.apps.my-destination-cluster.example.com        	  nodejs-ex          8080-tcp   
test-statefulset   test-statefulset-registry-example.apps.my-destination-cluster.example.com 	  test-statefulset   8080-tcp   


# Verify the nginx route is accessible
$ curl nodejs-ex-registry-example.apps.my-destination-cluster.example.com
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
  <title>Welcome to OpenShift</title>
[... snipped ...]
```
