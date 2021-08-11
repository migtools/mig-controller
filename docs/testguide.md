# Testing guide

The purpose of this document is to provide start guide to set up, run and extend e2e test suite.

### Prerequisites/Dependencies

- Install Ginkgo and Gomega 

      go get github.com/onsi/ginkgo/ginkgo
      go get github.com/onsi/gomega/...
    <em>Note:</em> This might already be installed.
   
- Define env variables
    - Required environment variables
    
      | Name      | Description |
      | ----------- | ----------- |
      | AWSBUCKETNAME | Name of the aws bucket |
      | AWSACCESSKEY  | Aws access key  |
      | AWSSECRETKEY | Aws secret key |
      | AWSREGION | Region of the aws bucket |
      | EXPOSEDREGISTRYPATH | Exposed registry URL of sourtce cluster |
      | SOURCEURL | Url of source cluster |
      | BACKUPSTORAGEPROVIDER | Backup storage provider name |
      | SOURCECONFIG | Path to kubeconfig of the source cluster |
      | HOSTCONFIG | Path to kubecinfig of the host cluster |
            
    - Optional environment variables
        
         | Name      | Description | Type | Default value |
         | --------- | ----------- | ---- | ------------- |
         | VELERO_PLUGIN_FQIN | Openshift velero plugin image | String |  "quay.io/konveyor/openshift-velero-plugin:latest" |
         | MIG_CONTROLLER_IMAGE_FQIN | Mig controller image | String |  "quay.io/konveyor/mig-controller:latest" |
         | RSYNC_TRANSFER_IMAGE_FQIN | Rsync trandfer image | String |  "quay.io/konveyor/rsync-transfer:latest" |
         | MIG_POD_LIMIT | Migration pod limit | Integer | 100 |
         | CLUSTER_NAME | Name of the host cluster | String | "host" |
         | RESTIC_TIMEOUT | Restic timeout duration | String | "1h" |
         | MIGRATION_VELERO | Flag to use velero | Bool |  true |
         | MIG_NAMESPACE_LIMIT | Limit of namespaces that can be migrated at once | Integer |  10 |
         | MIG_PV_LIMIT | Migration pv limit | Integer |  100 |
         | VERSION | Version | String | "latest" |


### Command to run the test suite

```
# this command should be run from root of the mig-controller repo
make e2e-test 
```

### Structure of the current suite

#### Set-up for the test suite
`BeforeSuite` of `test_suite_test.go` is responsible for setting up E2E test dependencies needed prior to creating a plan and running a migration. This ensures the following are present and in the desired state:
- `MigrationController`
- `MigStorage` 
- `MigCluster`

Currently `MigStorage` is limited to `aws s3` bucket. Support for other types of `MigStorage` creation will be added in future.      
#### Tests pertaining to features/scenarios
`Describe` in `tests_test.go` should pertain to one feature/scenario. As of now, one test case to test BZ-1965421 is included in the e2e suite.

Each feature might need some set up of their own, such as a `migPlan`, `migMigration`.  These dependencies can be fulfilled with helpers `BeforeEach`, `JustBeforeEach` and clean up of these dependencies should be handled in `AfterEach`, `JustAfterEach`. `It` is the lowest level block where the assertion of the desired properties or behavior should be made (<em><b>Note: </b>`BeforeEach/AfterEach` gets invoked for every `It`</em>). Detailed explanation on how these blocks are used can be found [here](https://onsi.github.io/ginkgo/#structuring-your-specs). Explanation on how to assert for different errors/values can be found [here](https://onsi.github.io/gomega/#making-assertions) and [here](https://onsi.github.io/gomega/#making-assertions-in-helper-functions).

<em><b>Note:</b></em> While adding new tests, making sure to not create objects that might be in conflict with other tests and cleaning up after the test is executed is of utmost importance.

#### Clean-up for the test suite
`AfterSuite` of `test_suite_test.go` is responsible for cleaning up the environment. It deletes `migCluster`, `migStorage` and deletes the installer if it was created by the suite.
 
### How to extend the test-suite

Each test case for a feature/scenario should typically be of the following structure.
```
Describe()
    BeforeEach()
    AfterEach()
    JustBeforeEach()
    Context()
        It()
    
    Context()
        BeforeEach() // specific to this context
        It()        
```

To keep the code readable, it is advisable to define the objects needed for the tests within a function in `test_helper.go` and use them while creating within the test.

### Limitations
- Currently `aws s3` is configured for `migStorage`.

<em>This document is subject to change.</em>