# Integration Testing Manual

This manual gives an overview about existing integration tests of Gardener.

- [Add a new test](#add-a-new-test)
- [Available test labels](#test-labels)
- [Framework](#framework)

## Structure

Gardener integration test are split into 2 big test suites that can be found under [gardener/test/suites](../../test/suites):

* The **Gardener Test Suite** contains all tests that only require a running gardener instance.
* The **Shoot Test Suite** contains all tests that require a predefined running shoot cluster.

The corresponding tests of a test suite are defined in the import statement of the suite definition see [gardener/test/suites/shoot/run_suite_test.go](../../test/suites/shoot/run_suite_test.go)
and their source code can be found under [gardener/test/integration](../../test/integration)

The testing directory
is divided into the following major subdirectories:

```console
├── framework
│   ├── applications
│   ├── config
│   ├── reporter
│   ├── resources
├── integration
│   ├── gardener
│   │   ├── scheduler
│   │   └── security
│   ├── plants
│   └── shoots
│       ├── applications
│       ├── logging
│       ├── maintenance
│       └── operations
├── suites
│   ├── gardener
│   └── shoot
└── system
    ├── complete_reconcile
    ├── shoot_creation
    ├── shoot_deletion
    └── shoot_update
```

A suite can be executed by running the suite definition with ginkgo's `focus` and `skip` flags 
to control the execution of specific labeled test. See example below:
```console
go test -timeout=0 -mod=vendor ./test/suites/shoot \
      --v -ginkgo.v -ginkgo.progress -ginkgo.noColor \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      --disable-dump=false \                               # disables dumping of teh current state if a test fails
      -kubecfg=/path/to/gardener/kubeconfig \
      -shoot-name=<shoot-name> \                           # Name of the shoot to test
      -project-namespace=<gardener project namespace> \    # Name of the gardener project the test shoot resides
      -ginkgo.focus="\[RELEASE\]" \                        # Run all tests that are tagged as release
      -ginkgo.skip="\[SERIAL\]|\[DISRUPTIVE\]"             # Exclude all tests that are tagged SERIAL or DISRUPTIVE
```

## Add a new test

To add a new test the framework requires the following steps:

(The step 1. and 2. can be skipped if the test is added to an already existing package)
1. Create a new test file e.g. `test/integration/shoot/security/my-sec-test.go`
2. Import the test into the appropriate framework you want use (gardener or shoot): `import _ "github.com/gardener/gardener/test/integration/shoot/security"`
3. Define your test with the testframework. The framework will automatically add its initialization, cleanup and dump functions.
```golang
var _ = ginkgo.Describe("my suite", func(){
  f := framework.NewShootFramework(nil)

  f.Beta().CIt("my first test", func(ctx context.Context) {
    f.ShootClient.Get(xx)
    // testing ...
  })
})
```

The newly created test can be tested by focusing the test with the default ginkgo focus `f.Beta().FCIt("my first test", func(ctx context.Context)`
and run the shoot test suite with:
```
go test -timeout=0 -mod=vendor ./test/suites/shoot \
      --v -ginkgo.v -ginkgo.progress -ginkgo.noColor \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      --disable-dump=false \                               # disables dumping of the current state if a test fails
      -kubecfg=/path/to/gardener/kubeconfig \
      -shoot-name=<shoot-name> \                           # Name of the shoot to test
      -project-namespace=<gardener project namespace> \
      -fenced=<true|false>                                 # Tested shoot is running in a fenced environment and cannot be reached by gardener
```
or for the gardener suite with:
```
go test -timeout=0 -mod=vendor ./test/suites/gardener \
      --v -ginkgo.v -ginkgo.progress -ginkgo.noColor \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      --disable-dump=false \                               # disables dumping of the current state if a test fails
      -kubecfg=/path/to/gardener/kubeconfig \
      -project-namespace=<gardener project namespace>
```

:warning: Make sure that you do not commit any code focused test as this feature is only intended for local development!

Alternatively, a test can be triggered by specifying a ginkgo focus regex with the name of the test e.g.
```
go test -timeout=0 -mod=vendor ./test/suites/gardener \
      --v -ginkgo.v -ginkgo.progress -ginkgo.noColor \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      -kubecfg=/path/to/gardener/kubeconfig \
      -project-namespace=<gardener project namespace> \
      -ginkgo.focus="my first test"                        # regex to match test cases
```


## Test Labels

Every test should be labeled by using the predefined labels available with every framework to have consistent labeling across 
all gardener integration tests. 
The labels are applied to every new `It()/CIt()` definition by:
```golang
f := framework.NewCommonFramework()
f.Default().Serial().It("my test") => "[DEFAULT] [SERIAL] my test"

f := framework.NewShootFramework()
f.Default().Serial().It("my test") => "[DEFAULT] [SERIAL] [SHOOT] my test"

f := framework.NewGardenerFramework()
f.Default().Serial().It("my test") => "[DEFAULT] [GARDENER] [SERIAL] my test"
```

Labels:
- _Beta_: Newly created tests with no experience on stableness should be first labeled as beta tests.
They should be watched (and probably improved) until stable enough to be promoted to _Default_.
- _Default_: Tests that were _Beta_ before and proved to be stable are promoted to _Default_ eventually.
 _Default_ tests run more often, produce alerts and are _considered_ during the release decision although they don't necessarily block a release.
- _Release_: Test are release relevant. A failing _Release_ test blocks the release pipeline.
Therefore these tests need to be stable. Only tests proven to be stable will eventually be promoted to _Release_.

Behavior Labels: 
- _Serial_: The test should always be executed in serial with no other tests running as it may impact other tests.
- _Destructive_: The test is destructive. Which means that is runs with no other tests and may break gardener or the shoot.
Only create such tests if really necessary as the execution will be expensive (neither gardener nor the shoot can be reused in this case for other tests).

## Framework

The framework directory contains all the necessary functions / utilities for running the integration test suite. 
For example, there are methods for creation/deletion of shoots, waiting for shoot deletion/creation, downloading/installing/deploying helm charts, logging, etc.

The framework itself consists of 3 different framework that expect different prerequisites and offer context specific functionality.
- **CommonFramework**: The common framework is the base framework that handles logging and setup of commonly needed resources like helm.
It also contains common functions for interacting with kubernetes clusters like `Waiting for resources to be ready` or `Exec into a running pod`.
- **GardenerFramework** contains all functions of the common framework and expects a running gardener instance with the provided gardener kubeconfig and a project namespace.
It also contains functions to interact with gardener like `Waiting for a shoot to be reconciled` or `Patch a shoot` or `Get a seed`.
- **ShootFramework**: contains all functions of the common and the gardener framework. 
It expects a running shoot cluster defined by the shoot's name and namespace(project namespace).
This framework contains functions to directly interact with the specific shoot.

The whole framework also includes commonly used checks, ginkgo wrapper, etc. as well as commonly used tests.
Theses common application tests (like the guestbook test) can be used within multiple tests to have a default application (with ingress, deployment, stateful backend) to test external factors.


**Config**

Every framework commandline flag can also be defined by a configuration file (the value of the configuration file is only used if flag is not specified by commandline).
The test suite searches for a configuration file (yaml is preferred) if the command line flag `--config=/path/to/config/file` is provided.
A framework can be defined in the configuration file by just using the flag name as root key e.g.
```yaml
verbose: debug
kubecfg: /kubeconfig/path
project-namespace: garden-it
```

**Report**

The framework automatically writes the default ginkgo default report to stdout and a specifically structured elastichsearch bulk report file to a specified location.
The elastichsearch bulk report will write one json document per testcase and injects metadata of the whole testsuite.
An example document for one test case would look like the following document:
```
{
    "suite": {
        "name": "Shoot Test Suite",
        "phase": "Succeeded",
        "tests": 3,
        "failures": 1,
        "errors": 0,
        "time": 87.427
    },
    "name": "Shoot application testing  [DEFAULT] [RELEASE] [SHOOT] should download shoot kubeconfig successfully",
    "shortName": "should download shoot kubeconfig successfully",
    "labels": [
        "DEFAULT",
        "RELEASE",
        "SHOOT"
    ],
    "phase": "Succeeded",
    "time": 0.724512057
}
```

**Resources**

The resources directory contains all the templates, helm config files (e.g., repositories.yaml, charts, and cache index which are downloaded upon the start of the test), shoot configs, etc.

```console
resources
├── charts
├── repository
│   └── repositories.yaml
└── templates
    ├── guestbook-app.yaml.tpl
    └── logger-app.yaml.tpl
```

There are two special directories that are dynamically filled with the correct test files:

- **charts:** the charts will be downloaded and saved in this directory
- **repository** contains the repository.yaml file that the target helm repos will be read from and the cache where the `stable-index.yaml` file will be created

### System Tests

This directory contains the system tests that have a special meaning for the testmachinery with their own Test Definition.
Currently these system tests consists of:

- Shoot creation
- Shoot deletion
- Shoot Kubernetes update
- Gardener Full reconcile check

#### Shoot Creation test

Create Shoot test is meant to test shoot creation.

**Example Run**

```console
go test -mod=vendor -timeout=0 ./test/system/shoot_creation \
  --v -ginkgo.v -ginkgo.progress \
  -kubecfg=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME \
  -cloud-profile=$CLOUDPROFILE \
  -seed=$SEED \
  -secret-binding=$SECRET_BINDING \
  -provider-type=$PROVIDER_TYPE \
  -region=$REGION \
  -k8s-version=$K8S_VERSION \
  -project-namespace=$PROJECT_NAMESPACE \
  -annotations=$SHOOT_ANNOTATIONS \
  -infrastructure-provider-config-filepath=$INFRASTRUCTURE_PROVIDER_CONFIG_FILEPATH \
  -controlplane-provider-config-filepath=$CONTROLPLANE_PROVIDER_CONFIG_FILEPATH \
  -workers-config-filepath=$$WORKERS_CONFIG_FILEPATH \
  -worker-zone=$ZONE \
  -networking-pods=$NETWORKING_PODS \
  -networking-services=$NETWORKING_SERVICES \
  -networking-nodes=$NETWORKING_NODES \
  -start-hibernated=$START_HIBERNATED
```

#### Shoot Deletion test

Delete Shoot test is meant to test the deletion of a shoot.

**Example Run**

```console
go test -mod=vendor -timeout=0 -ginkgo.v -ginkgo.progress \
  ./test/system/shoot_deletion \
  -kubecfg=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME \
  -project-namespace=$PROJECT_NAMESPACE
```

#### Shoot Update test

The Update Shoot test is meant to test the kubernetes version update of a existing shoot.
If no specific version is provided the next patch version is automatically selected.
If there is no available newer version this test is a noop.

**Example Run**

```console
go test -mod=vendor -timeout=0 ./test/system/shoot_update \
  --v -ginkgo.v -ginkgo.progress \
  -kubecfg=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME \
  -project-namespace=$PROJECT_NAMESPACE \
  -version=$K8S_VERSION
```

#### Gardener Full Reconcile test

The Gardener Full Reconcile test is meant to test if all shoots of a gardener instance are successfully reconciled.

**Example Run**

```console
go test -mod=vendor -timeout=0 ./test/system/complete_reconcile \
  --v -ginkgo.v -ginkgo.progress \
  -kubecfg=$HOME/.kube/config \
  -project-namespace=$PROJECT_NAMESPACE \
  -gardenerVersion=$GARDENER_VERSION # needed to validate the last acted gardener version of a shoot
```
