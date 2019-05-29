## Migrating a *stateless* OpenShift app with mig-controller

This scenario walks through Migration of a stateless OpenShift app, meaning that the app has no Persistent Volumes (PVs) attached.

---

### 1. Prerequisites

Referring to the getting started [README.md](https://github.com/fusor/mig-controller/blob/master/README.md), you'll first need to deploy mig-controller and Velero, and then create the following 'Mig' resources on the cluster where mig-controller is running to prepare for Migration:

- `MigCluster` resources for the _source_ and _destination_ clusters
- `Cluster` resource for any _remote_ clusters (e.g. clusters the controller will connect to remotely, there will be at least one of these)
- `MigStorage` providing information on how to store resource YAML in transit between clusters 
 
---

### 2. Deploying a stateless sample app (nginx)

On the _source_ cluster (where you will migrate your application _from_), create the contents of the `nginx-deployment.yaml` manifest. 

```bash
# Login to the migration 'source cluster'
$ oc login https://my-source-cluster:8443

# Create resources defined in 'nginx-deployment.yaml'
$ oc create -f nginx-deployment.yaml 

namespace/nginx-example created
deployment.apps/nginx-deployment created
service/my-nginx created
route.route.openshift.io/my-nginx created
```

In a few seconds, you should see nginx pods start running.

```bash
# Making sure our stateless app is running
$ oc get pods -n nginx-example
NAME                                READY     STATUS    RESTARTS   AGE
nginx-deployment-55b5c6f96c-bs5qb   1/1       Running   0          11s
nginx-deployment-55b5c6f96c-rp4p2   1/1       Running   0          11s
```

We can check that our nginx pods are serving their starter webpage successfully before continuing.

```bash
# Get the stateless app route host/port
$ oc get route -n nginx-example
NAME       HOST/PORT
my-nginx   my-nginx-nginx-example.apps.my-source-cluster.example.com 

# Verify the nginx route is accessible
$ curl my-nginx-nginx-example.apps.my-source-cluster.example.com 
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
[... snipped ...]

```

Next, let's fill out our 'MigPlan' specifying the `nginx-example` namespace should be moved to our destination cluster during the migration.

---

### 3. Create a 'MigPlan' that references our 'nginx-example' namespace

To migrate our `nginx-example` namespace, we'll ensure that the `namespaces` field of our MigPlan includes nginx-example. Luckily [config/samples/mig-plan.yaml](https://github.com/fusor/mig-controller/blob/master/config/samples/mig-plan.yaml) does exactly this.

```yaml
apiVersion: migration.openshift.io/v1alpha1
kind: MigPlan
spec:
  # [!] Change namespaces to adjust which OpenShift namespaces should be migrated from source to destination cluster
  namespaces:
  - nginx-example

[... snipped, see config/samples/mig-plan.yaml for other required fields ...]
```

With the 'nginx-example' namespace listed, no further changes are needed. Let's create our MigPlan and also create a MigMigration referencing that MigPlan to start moving this app over to our _destination_ cluster.

```bash
# From project root, run `make samples` to put these yaml files in a 'migsamples' directory your can safely modify.

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

To double-check the work mig-controller did, login to our destination cluster and verify existence of the pods and route we used to inspect our stateless app previously. If the 'nginx-example' namespace didn't previously exist on the destination cluster, it should have been created.

```bash
# Login to the migration 'destination cluster'
$ oc login https://my-destination-cluster:8443

# Make sure nginx pods are running
$ oc get pods -n nginx-example
NAME                                READY     STATUS    RESTARTS   AGE
nginx-deployment-55b5c6f96c-bs5qb   1/1       Running   0          11s
nginx-deployment-55b5c6f96c-rp4p2   1/1       Running   0          11s

# Get the stateless app route host/port
$ oc get route -n nginx-example
NAME       HOST/PORT
my-nginx   my-nginx-nginx-example.apps.my-destination-cluster.example.com 

# Verify the nginx route is accessible
$ curl my-nginx-nginx-example.apps.my-destination-cluster.example.com 
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
[... snipped ...]

```
