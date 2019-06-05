## Deploying the OpenShift Migration Controller + UI

### __Migration controller__ - deploy to _one_ cluster
Run `deploy_mig.sh` to deploy the mig-controller to only _one_ of the involved clusters.

### __Velero__ - deploy to _source_ and _destination_ clusters
Run `deploy_velero.sh` on both _source_ and _destination_ clusters which will be part of a migration. 

## Performing a Migration
To perform a Migration, Create mig* resources manually ([annotated sample CRs](https://github.com/fusor/mig-controller/tree/master/config/samples)).

After familiarizing with sample CR content, read some [Migration Scenarios](https://github.com/fusor/mig-controller/tree/master/docs/scenarios) to learn how to run Migrations for a variety of use-cases.