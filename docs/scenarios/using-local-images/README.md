## Migrating OpenShift apps running against local images with mig-controller

This scenario walks through Migration of several types of OpenShift applications, all of which are running against local images. This includes sample DeploymentConfigs, as well as a Deployment, a Job, a DaemonSet, a StatefulSet, and a standalone Pod.

---

### 1. Prerequisites

Referring to the getting started [README.md](https://github.com/fusor/mig-controller/blob/master/README.md), you'll first need to deploy mig-controller and Velero (including ocp-velero-plugin), and then create the following 'Mig' resources on the cluster where mig-controller is running to prepare for Migration:

- `MigCluster` resources for the _source_ and _destination_ clusters
- `Cluster` resource for any _remote_ clusters (e.g. clusters the controller will connect to remotely, there will be at least one of these)
- `MigStorage` providing information on how to store resource YAML in transit between clusters 
 
### 2. Deploying the sample apps

On the _source_ cluster (where you will migrate your application _from_), run the following.

```bash
# Login to the migration 'source cluster'
$ oc login https://my-source-cluster:8443

# create a namespace for the test resources
oc create namespace registry-example
oc project registry-example

# create a new s2i app with a locally-built image
oc new-app https://github.com/openshift/nodejs-ex
```
At this point wait for the build to complete so that the image is available for other apps.
When `oc get build` shows `nodejs-ex-1` with a status of `Complete`, move on to the next step.

```
oc expose service nodejs-ex

# create a deploymentconfig using the same image
export REGISTRY_HOST=$(oc registry info)
oc run test-dc --image=$REGISTRY_HOST/registry-example/nodejs-ex:latest

# create a deployment using the same image:
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  name: test-deployment
  labels:
    run: test-deployment
spec:
  selector:
    matchLabels:
      run: test-deployment
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: test-deployment
    spec:
      containers:
      - image: $REGISTRY_HOST/registry-example/nodejs-ex:latest
        name: test-deployment
        resources: {}
EOF

# create a job using the same image:
oc run test-job --image=$REGISTRY_HOST/registry-example/nodejs-ex:latest -n registry-example --restart='OnFailure'

# create a daemonset using the same image:
cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  name: test-daemonset
  labels:
    run: test-daemonset
spec:
  selector:
    matchLabels:
      run: test-daemonset
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: test-daemonset
    spec:
      containers:
      - image: $REGISTRY_HOST/registry-example/nodejs-ex:latest
        name: test-daemonset
        resources: {}
EOF

# create a statefulset using the same image:
cat <<EOF | kubectl create -f -
apiVersion: v1     
kind: Service
metadata:
  name: test-statefulset
  labels:                    
    run: test-statefulset
spec:                         
  ports:
  - name: 8080-tcp
    port: 8080
    protocol: TCP
    targetPort: 8080
  clusterIP: None
  selector:
    run: test-statefulset
EOF

cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: StatefulSet
metadata:
  creationTimestamp: null
  name: test-statefulset
  labels:
    run: test-statefulset
spec:
  selector:
    matchLabels:
      run: test-statefulset
  serviceName: "test-statefulset"
  template:
    metadata:
      creationTimestamp: null
      labels:
        run: test-statefulset
    spec:
      containers:
      - image: $REGISTRY_HOST/registry-example/nodejs-ex:latest
        name: test-statefulset
        resources: {}
EOF

oc expose service/test-statefulset

# create a standalone pod using the same image
oc run test-standalone --image=$REGISTRY_HOST/registry-example/nodejs-ex:latest -n registry-example --restart='Never'

```
Once this is done, verify that the pods are running. There should be a running pod for the `nodejs-ex` s2i app, as well as a pod for the other types that we created above: daemonset, deploymentconfig (dc), deployment, job, statefulset, and the standalone pod.

```bash
# Making sure the pods are running
oc get pods -n registry-example
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

# Before creating the migplan, modify the namespace Spec section as specified above.
# Creates MigPlan 'migplan-sample' in namespace 'mig'
$ oc apply -f mig-plan.yaml

# Describe our MigPlan. Assuming the controller is running, validations
# should have run against the plan, and you should be able to see 
# "The Migration Plan is ready" or a list of issues to resolve.
$ oc describe migplan migplan-sample -n mig
Name:         migplan-sample
Namespace:    mig
API Version:  migration.openshift.io/v1alpha1
Kind:         MigPlan

[... snipped ...]

Status:
  Conditions:
    Category:              Required
    Last Transition Time:  2019-05-24T14:50:06Z
    Message:               The persistentVolumes list has been updated with discovered PVs.
    Reason:                Done
    Status:                True
    Type:                  PvsDiscovered
    Category:              Required
    Last Transition Time:  2019-05-24T14:50:06Z
    Message:               The storage resources have been created.
    Status:                True
    Type:                  StorageEnsured
    Category:              Required
    Last Transition Time:  2019-06-03T18:43:18Z
    Message:               The migration registry resources have been created.
    Status:                True
    Type:                  RegistriesEnsured
    Category:              Required
    Last Transition Time:  2019-05-24T14:50:06Z
    Message:               The migration plan is ready.
    Status:                True
    Type:                  Ready


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
Namespace:    mig
API Version:  migration.openshift.io/v1alpha1
Kind:         MigMigration
Spec:
  Mig Plan Ref:
    Name:       migplan-sample
    Namespace:  mig
  Stage:        false
Status:
  Completion Timestamp:  2019-05-22T21:46:09Z
  Conditions:
    Category:              Required
    Last Transition Time:  2019-05-24T14:50:06Z
    Message:               The migration is ready.
    Status:                True
    Type:                  Ready
  Migration Completed:     true
  Start Timestamp:         2019-05-22T21:43:27Z
  Task Phase:              Completed
Events:                    <none>
```

Notice how the MigMigration shown above has 'Task Phase': Completed. This means that the Migration is complete, and we should be able to verify our apps existence on the destination cluster. You can continuously describe the MigMigration to see phase info, or tail the mig-controller logs with `oc logs -f <pod-name>`.

---


### 4. Verify that the MigMigration completed successfully

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
$ oc get route -n nginx-example
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
