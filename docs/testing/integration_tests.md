# Testing Manual

This manual describes the Gardener integration test directory structure,

## Structure

Gardener integration tests can be found under `gardener/test/integration`, the testing directory
is divided into the following major subdirectories:

```console
├── framework
├── resources
│   ├── charts
│   ├── repository
│   └── templates
└── seeds
    ├── logging
    ├── networkpolicies
└── shoots
    ├── applications
    └── operations
```

In the following sections we will briefly describe the contents of each subdirectory.

### Framework

The framework directory contains all the necessary functions / utilities for running the integration test suite. For example, there are methods for creation/deletion of shoots, waiting for shoot deletion/creation, downloading/installing/deploying helm charts, logging, etc.

```console
framework
├── common.go
├── errors.go
├── framework_test.go
├── helm_utils.go
├── networkpolicies
├── plant_operations.go
├── shoot_app.go
├── shoot_operations.go
├── types.go
└── utils.go
```

### NetworkPolicies

The networkpolicies directory contains all information for generation of Network policies E2E test suites.

```console
networkpolicies
├── agnostic.go
├── alicloud.go
├── aws.go
├── azure.go
├── doc.go
├── gcp.go
├── netpol-gen
│   ├── LICENSE_BOILERPLATE.txt
│   ├── generators
│   │   ├── networkpolicies.go
│   │   ├── registry.go
│   │   └── tags.go
│   └── netpol-gen.go
├── networkpolicies_suite_test.go
├── openstack.go
├── rule_builder.go
├── rule_builder_test.go
├── shared_resources.go
├── types.go
└── types_test.go
```

If a new provider is added, it must implement the `CloudAwarePodInfo` interface from `pod_info.go`, have the correct tags like

```
// AWSNetworkPolicy holds aws-specific network policy settings.
// +gen-netpoltests=true
// +gen-packagename=aws
type AWSNetworkPolicy struct {
}
```

and have a instance of that struct added to `defaultRegistry()` function in `netpol-gen/generators/registry.go`.

After this code generation should be ran with

```console
go generate ./test/integration/framework/networkpolicies
```

producing a generated test suite like

```console
test/integration/seeds/networkpolicies
├── alicloud
│   ├── doc.go
│   ├── networkpolicies_suite_test.go
│   └── networkpolicy_alicloud_test.go
├── aws
│   ├── doc.go
│   ├── networkpolicies_suite_test.go
│   └── networkpolicy_aws_test.go
├── azure
│   ├── doc.go
│   ├── networkpolicies_suite_test.go
│   └── networkpolicy_azure_test.go
├── gcp
│   ├── doc.go
│   ├── networkpolicies_suite_test.go
│   └── networkpolicy_gcp_test.go
└── openstack
    ├── doc.go
    ├── networkpolicies_suite_test.go
    └── networkpolicy_openstack_test.go
```

### Resources

The resources directory contains all the templates, helm config files (e.g., repositories.yaml, charts, and cache index which are downloaded upon the start of the test), shoot configs, etc.

```console
resources
├── charts
│   └── redis
├── repository
│   ├── cache
│   └── repositories.yaml
└── templates
    └── guestbook-app.yaml.tp
    └── (...)
```

There are two special directories that are dynamically filled with the correct test files:

- **Charts:** the charts will be downloaded and saved in this directory
- **Repository** contains the repository.yaml file that the target helm repos will be read from and the cache where the `stable-index.yaml` file will be created

### Shoots

This directory contains the actual tests. Currently the testing framework supports two kinds of tests:

- Shoot operation
- Shoot application

### Shoot operation

Shoot operation tests are meant to test shoot creation / deletion as standalone tests or as coupled tests.

The test takes the following options

```go
// Required kubeconfig for the garden cluster
kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")

// Needed for the standalone shoot creation test and the coupled creation / deletion test
shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")

// Needed for the standalone deletion test and the coupled creation / deletion test
shootNamespace    = flag.String("shootNamespace", "", "shootNamespace is used for creation and deletion of shoots")
logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")

// Prefix for the names of the test shoots
testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")
```

#### Example Run

```console
go test -kubeconfig $HOME/.kube/config -shootpath shoot.yaml -shootNamespace garden-dev -ginkgo.v
```

> NOTE: please remove the shoot name `metadata.name` and the DNS domain `spec.dns.domain` from your shoot YAML as the testing framework will be creating those for you.

The above command will read the garden kubeconfig from `$HOME/.kube/config`, the shoot yaml from `shoot.yaml`, the project namespace from `shootNamespace` and finally, it also request `cleanup` of resources after the test is run.

### Shoot application

Shoot applications tests are tests that are executed on a shoot. There are two possiblities for running these tests:

- execute against an existing shoot by specifying the shootName
- create a shoot from a yaml spec and execute the tests against this shoot

Below are the flags used for running the application tests:

```go
// Required kubeconfig for the garden cluster
kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")

// Shoot options if running tests against an existing shoot
shootName         = flag.String("shootName", "", "the name of the shoot we want to test")
shootNamespace    = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")

// Prefix for the names of the test shoots
testShootsPrefix  = flag.String("prefix", "", "prefix to use for test shoots")

// If it is preferrable to create a test shoot for running the application test, the shoot yaml path needs to be specified
shootTestYamlPath = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
logLevel          = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")

// Optional parameter that can be used to customize the location where the shoot kubeconfig is downloaded
downloadPath      = flag.String("downloadPath", "/tmp/test", "the path to which you download the kubeconfig")

// If specified the test will clean up the created / existing shoot
cleanup           = flag.Bool("cleanup", false, "deletes the newly created / existing test shoot after the test suite is done")
```

#### Example Run

To create a test shoot and run the tests on it, use the following command:

```console
go test -kubeconfig $HOME/.kube/config  -shootpath shoot.yaml -cleanup -ginkgo.v
```

> NOTE: please remove the shoot name `metadata.name` and the DNS domain `spec.dns.domain` from your shoot YAML as the testing framework will be creating those for you.

The command requires a garden kubeconfig and the shoot yaml path. Additionally, the cleanup option was specified to delete the created shoots after tests are complete.

To use an existing shoot, the following command can be used:

```console
go test -kubeconfig $HOME/.kube/config -shootName "test-zefc8aswue" -shootNamespace "garden-dev" -ginkgo.v -ginkgo.progress
```

The above command requires a garden kubeconfig, the seed name, the shoot name, and the project namespace.

### Seed Logging

Seed logging tests are meant to test the logging functionality for seed clusters. There are two possiblities for running these tests:

- execute against an existing shoot by specifying the shootName
- create a shoot from a yaml spec and execute the tests against this shoot

Below are the flags used for running the logging tests:

```go
// Required kubeconfig for the garden cluster
kubeconfig           = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")

// Shoot options if running tests against an existing shoot
shootName            = flag.String("shootName", "", "the name of the shoot we want to test")
shootNamespace       = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")

// Prefix for the names of the test shoots
testShootsPrefix     = flag.String("prefix", "", "prefix to use for test shoots")

// If it is preferrable to create a test shoot for running the application test, the shoot yaml path needs to be specified
shootTestYamlPath    = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
logLevel             = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
logsCount            = flag.Uint64("logsCount", 10000, "the logs count to be logged by the logger application")

// If specified the test will clean up the created shoot
cleanup              = flag.Bool("cleanup", false, "deletes the newly created test shoot after the test suite is done")
```

#### Example Run

To create a test shoot and run the tests on it, use the following command:

```console
cd test/integration/seeds/logging
go test -kubeconfig $HOME/.kube/config -shootpath shoot.yaml -timeout 20m -cleanup -ginkgo.v
```

To use an existing shoot, the following command can be used:

```console
cd test/integration/seeds/logging
go test -kubeconfig $HOME/.kube/config -shootName "test-zefc8aswue" -shootNamespace "garden-dev" -ginkgo.v -ginkgo.progress
```

### Seed NetworkPolicies

Seed Network Policies tests are meant to test the Network Poilicies for seed clusters. It's possible run only againts an existing shoot by specifying the `shootName` and `shootNamespace`.

Below are the flags used for running the logging tests:

```go
// Required kubeconfig for the garden cluster
kubeconfig           = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")

// Shoot options if running tests against an existing shoot
shootName            = flag.String("shootName", "", "the name of the shoot we want to test")
shootNamespace       = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")

// If specified the test will clean up the created shoot
cleanup              = flag.Bool("cleanup", false, "deletes all created e2e resources after the test suite is done")
```

#### Example Run

To use an existing shoot, running on AWS, the following command can be used:

```console
cd test/integration/seeds/networkpolicies
ginkgo \
    --progress \
    -v \
    --noColor \
    --nodes=25 \
    --randomizeAllSpecs \
    --randomizeSuites \
    --failOnPending \
    --trace \
    --race \
    aws -- \
    --kubeconfig=$HOME/.kube/config \
    --shootName="netpolicy" \
    --shootNamespace="garden-dev" \
    --cleanup=true
```

## Gardener

Currently the gardener tests consists of:

- RBAC test
- shoots reconcile test

### Gardener RBAC test

The gardener RBAC test is meant to test if RBAC is enabled on the gardener cluster.
This is tested by:
1. Check if the RBAC API-Resource is available
2. Check if a service account in a project namespace can access the `garden` project.

#### Example Run
```console
cd test/integration/gardener/rbac
ginkgo \
    --progress \
    -v \
    --noColor \
    -kubeconfig=$HOME/.kube/config \
    -project-namespace=garden-core
```

### Gardener reconcile test

The gardener reconcile test checks if all shoots of a gardener cluster are successfully reconciled after the gardener version was updated.

#### Example Run
```console
cd test/integration/gardener/rbac
ginkgo \
    --progress \
    -v \
    --noColor \
    -kubeconfig=$HOME/.kube/config \
    -version=0.26.4
```