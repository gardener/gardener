# Test Machinery Tests

In order to automatically qualify Gardener releases, we execute a set of end-to-end tests using [Test Machinery](https://github.com/gardener/test-infra).
This requires a full Gardener installation including infrastructure extensions, as well as a setup of Test Machinery itself.
These tests operate on Shoot clusters across different Cloud Providers, using different supported Kubernetes versions and various configuration options (huge test matrix).

This manual gives an overview about test machinery tests in Gardener.

- [Structure](#structure)
- [Add a new test](#add-a-new-test)
- [Test Labels](#test-labels)
- [Framework](#framework)
- [Container Images](#container-images)

## Structure

Gardener test machinery tests are split into two test suites that can be found under [`test/testmachinery/suites`](../../test/testmachinery/suites):

* The **Gardener Test Suite** contains all tests that only require a running gardener instance.
* The **Shoot Test Suite** contains all tests that require a predefined running shoot cluster.

The corresponding tests of a test suite are defined in the import statement of the suite definition (see [`shoot/run_suite_test.go`](../../test/testmachinery/suites/shoot/run_suite_test.go))
and their source code can be found under [`test/testmachinery`](../../test/testmachinery).

The `test` directory is structured as follows:

```console
test
├── e2e           # end-to-end tests (using provider-local)
│  ├── gardener
│  │  ├── seed
│  │  ├── shoot
|  |  └── ...
|  └──operator
├── framework     # helper code shared across integration, e2e and testmachinery tests
├── integration   # integration tests (envtests)
│  ├── controllermanager
│  ├── envtest
│  ├── resourcemanager
│  ├── scheduler
│  └── ...
└── testmachinery # test machinery tests
   ├── gardener   # actual test cases imported by suites/gardener
   │  └── security
   ├── shoots     # actual test cases imported by suites/shoot
   │  ├── applications
   │  ├── care
   │  ├── logging
   │  ├── operatingsystem
   │  ├── operations
   │  └── vpntunnel
   ├── suites     # suites that run agains a running garden or shoot cluster
   │  ├── gardener
   │  └── shoot
   └── system     # suites that are used for building a full test flow
      ├── complete_reconcile
      ├── managed_seed_creation
      ├── managed_seed_deletion
      ├── shoot_cp_migration
      ├── shoot_creation
      ├── shoot_deletion
      ├── shoot_hibernation
      ├── shoot_hibernation_wakeup
      └── shoot_update
```

A suite can be executed by running the suite definition with ginkgo's `focus` and `skip` flags 
to control the execution of specific labeled test. See the example below:
```console
go test -timeout=0 ./test/testmachinery/suites/shoot \
      --v -ginkgo.v -ginkgo.show-node-events -ginkgo.no-color \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      --disable-dump=false \                               # disables dumping of teh current state if a test fails
      -kubecfg=/path/to/gardener/kubeconfig \
      -shoot-name=<shoot-name> \                           # Name of the shoot to test
      -project-namespace=<gardener project namespace> \    # Name of the gardener project the test shoot resides
      -ginkgo.focus="\[RELEASE\]" \                        # Run all tests that are tagged as release
      -ginkgo.skip="\[SERIAL\]|\[DISRUPTIVE\]"             # Exclude all tests that are tagged SERIAL or DISRUPTIVE
```

## Add a New Test

To add a new test the framework requires the following steps (step 1. and 2. can be skipped if the test is added to an existing package):

1. Create a new test file e.g. `test/testmachinery/shoot/security/my-sec-test.go`
2. Import the test into the appropriate test suite (gardener or shoot): `import _ "github.com/gardener/gardener/test/testmachinery/shoot/security"`
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
and running the shoot test suite with:
```
go test -timeout=0 ./test/testmachinery/suites/shoot \
      --v -ginkgo.v -ginkgo.show-node-events -ginkgo.no-color \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      --disable-dump=false \                               # disables dumping of the current state if a test fails
      -kubecfg=/path/to/gardener/kubeconfig \
      -shoot-name=<shoot-name> \                           # Name of the shoot to test
      -project-namespace=<gardener project namespace> \
      -fenced=<true|false>                                 # Tested shoot is running in a fenced environment and cannot be reached by gardener
```
or for the gardener suite with:
```
go test -timeout=0 ./test/testmachinery/suites/gardener \
      --v -ginkgo.v -ginkgo.show-node-events -ginkgo.no-color \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      --disable-dump=false \                               # disables dumping of the current state if a test fails
      -kubecfg=/path/to/gardener/kubeconfig \
      -project-namespace=<gardener project namespace>
```

:warning: Make sure that you do not commit any focused specs as this feature is only intended for local development! Ginkgo will fail the test suite if there are any focused specs.

Alternatively, a test can be triggered by specifying a ginkgo focus regex with the name of the test e.g.
```
go test -timeout=0 ./test/testmachinery/suites/gardener \
      --v -ginkgo.v -ginkgo.show-node-events -ginkgo.no-color \
      --report-file=/tmp/report.json \                     # write elasticsearch formatted output to a file
      -kubecfg=/path/to/gardener/kubeconfig \
      -project-namespace=<gardener project namespace> \
      -ginkgo.focus="my first test"                        # regex to match test cases
```


## Test Labels

Every test should be labeled by using the predefined labels available with every framework to have consistent labeling across 
all test machinery tests. 

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
Therefore, these tests need to be stable. Only tests proven to be stable will eventually be promoted to _Release_.

Behavior Labels: 
- _Serial_: The test should always be executed in serial with no other tests running, as it may impact other tests.
- _Destructive_: The test is destructive. Which means that is runs with no other tests and may break Gardener or the shoot.
Only create such tests if really necessary, as the execution will be expensive (neither Gardener nor the shoot can be reused in this case for other tests).

## Framework

The framework directory contains all the necessary functions / utilities for running test machinery tests. 
For example, there are methods for creation/deletion of shoots, waiting for shoot deletion/creation, downloading/installing/deploying helm charts, logging, etc.

The framework itself consists of 3 different frameworks that expect different prerequisites and offer context specific functionality.
- **CommonFramework**: The common framework is the base framework that handles logging and setup of commonly needed resources like helm.
It also contains common functions for interacting with Kubernetes clusters like `Waiting for resources to be ready` or `Exec into a running pod`.
- **GardenerFramework** contains all functions of the common framework and expects a running Gardener instance with the provided Gardener kubeconfig and a project namespace.
It also contains functions to interact with gardener like `Waiting for a shoot to be reconciled` or `Patch a shoot` or `Get a seed`.
- **ShootFramework**: contains all functions of the common and the gardener framework. 
It expects a running shoot cluster defined by the shoot's name and namespace (project namespace).
This framework contains functions to directly interact with the specific shoot.

The whole framework also includes commonly used checks, ginkgo wrapper, etc., as well as commonly used tests.
Theses common application tests (like the guestbook test) can be used within multiple tests to have a default application (with ingress, deployment, stateful backend) to test external factors.


**Config**

Every framework commandline flag can also be defined by a configuration file (the value of the configuration file is only used if a flag is not specified by commandline).
The test suite searches for a configuration file (yaml is preferred) if the command line flag `--config=/path/to/config/file` is provided.
A framework can be defined in the configuration file by just using the flag name as root key e.g.
```yaml
verbose: debug
kubecfg: /kubeconfig/path
project-namespace: garden-it
```

**Report**

The framework automatically writes the ginkgo default report to stdout and a specifically structured elastichsearch bulk report file to a specified location.
The elastichsearch bulk report will write one json document per testcase and injects the metadata of the whole testsuite.
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

The resources directory contains templates used by the tests.

```console
resources
└── templates
    ├── guestbook-app.yaml.tpl
    └── logger-app.yaml.tpl
```

### System Tests

This directory contains the system tests that have a special meaning for the testmachinery with their own Test Definition.
Currently, these system tests consist of:

- Shoot creation
- Shoot deletion
- Shoot Kubernetes update
- Gardener Full reconcile check

#### Shoot Creation Test

Create Shoot test is meant to test shoot creation.

**Example Run**

```console
go test  -timeout=0 ./test/testmachinery/system/shoot_creation \
  --v -ginkgo.v -ginkgo.show-node-events \
  -kubecfg=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME \
  -cloud-profile-name=$CLOUDPROFILE \
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

#### Shoot Deletion Test

Delete Shoot test is meant to test the deletion of a shoot.

**Example Run**

```console
go test  -timeout=0 -ginkgo.v -ginkgo.show-node-events \
  ./test/testmachinery/system/shoot_deletion \
  -kubecfg=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME \
  -project-namespace=$PROJECT_NAMESPACE
```

#### Shoot Update Test

The Update Shoot test is meant to test the Kubernetes version update of a existing shoot.
If no specific version is provided, the next patch version is automatically selected.
If there is no available newer version, this test is a noop.

**Example Run**

```console
go test  -timeout=0 ./test/testmachinery/system/shoot_update \
  --v -ginkgo.v -ginkgo.show-node-events \
  -kubecfg=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME \
  -project-namespace=$PROJECT_NAMESPACE \
  -version=$K8S_VERSION
```

#### Gardener Full Reconcile Test

The Gardener Full Reconcile test is meant to test if all shoots of a Gardener instance are successfully reconciled.

**Example Run**

```console
go test  -timeout=0 ./test/testmachinery/system/complete_reconcile \
  --v -ginkgo.v -ginkgo.show-node-events \
  -kubecfg=$HOME/.kube/config \
  -project-namespace=$PROJECT_NAMESPACE \
  -gardenerVersion=$GARDENER_VERSION # needed to validate the last acted gardener version of a shoot
```

## Container Images

Test machinery tests usually deploy a workload to the Shoot cluster as part of the test execution. When introducing a new container image, consider the following:

- Make sure the container image is multi-arch.
  - Tests are executed against `amd64` and `arm64` based worker Nodes.
- Do not use container images from Docker Hub.
  - Docker Hub has rate limiting (see [Download rate limit](https://docs.docker.com/docker-hub/download-rate-limit/)). For anonymous users, the rate limit is set to 100 pulls per 6 hours per IP address. In some fenced environments the network setup can be such that all egress connections are issued from single IP (or set of IPs). In such scenarios the allowed rate limit can be exhausted too fast. See https://github.com/gardener/gardener/issues/4160.
  - Docker Hub registry doesn't support pulling images over IPv6 (see [Beta IPv6 Support on Docker Hub Registry](https://www.docker.com/blog/beta-ipv6-support-on-docker-hub-registry/)).
  - Avoid manually copying Docker Hub images to Gardener GCR (`europe-docker.pkg.dev/gardener-project/releases/3rd/`). Use the existing prow job for this (see [Copy Images](https://github.com/gardener/ci-infra/tree/master/config/images)).
  - If possible, use a Kubernetes e2e image (`registry.k8s.io/e2e-test-images/<image-name>`).
    - In some cases, there is already a Kubernetes e2e image alternative of the Docker Hub image.
      - For example, use `registry.k8s.io/e2e-test-images/busybox` instead of `europe-docker.pkg.dev/gardener-project/releases/3rd/busybox` or `docker.io/busybox`.
    - Kubernetes has multiple test images - see https://github.com/kubernetes/kubernetes/tree/v1.27.0/test/images. `agnhost` is the most widely used image in Kubernetes e2e tests. It contains multiple testing related binaries inside such as `pause`, `logs-generator`, `serve-hostname`, `webhook` and others. See all of them in the [agnhost's README.md](https://github.com/kubernetes/kubernetes/blob/v1.27.0/test/images/agnhost/README.md).
    - The list of available Kubernetes e2e images and tags can be checked in [this page](https://github.com/kubernetes/k8s.io/blob/main/registry.k8s.io/images/k8s-staging-e2e-test-images/images.yaml).
