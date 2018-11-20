# Testing Manual

This manual describes the Gardener integration test directory structure,

## Structure

Gardener integration tests can be found under `gardener/test/integration`, the testing directory
is divided into the following major subdirectories:

```console
├── framework
│   └── testlogger
├── resources
│   ├── charts
│   ├── repository
│   └── templates
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
├── helm_utils.go
├── shoot_app.go
├── shoot_operations.go
├── testlogger
│   ├── testlogger.go
│   ├── testlogger_suite_test.go
│   └── testlogger_test.go
├── tiller.go
├── types.go
└── utils.go
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
kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig  of Garden the cluster that will be used for integration tests")

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
go test -kubeconfig $HOME/.kube/config  -shootpath shoot.yaml -shootNamespace garden-dev -ginkgo.v
```

> NOTE: please remove the shoot name `metadata.name` and the DNS domain `spec.dns.domain` from your shoot YAML as the testing framework will be creating those for you.

The above command will read the garden kubeconfig from `$HOME/.kube/config`, the shoot yaml from `shoot.yaml`, the project namespace from `shootNamespace` and finally, it also request `cleanup` of resources after the test is run.

### Shoot application

Shoot applications tests are tests that are executed on a shoot. There are two possiblities for running these tests:

- execute against an existing shoot by specifying the shoot-name
- create a shoot from a yaml spec and execute the tests against this shoot

Below are the flags used for running the application tests:

```go
// Required kubeconfig for the garden cluster
kubeconfig        = flag.String("kubeconfig", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")

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
go test  -kubeconfig $Home/.kube/config  -shootName "test-zefc8aswue" -shootNamespace "garden-dev"  -ginkgo.v -ginkgo.progress
```

The above command requires a garden kubeconfig, the seed name, the shoot name, and the project namespace.

