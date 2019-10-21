# Integration Testing Manual

This manual gives an overview about existing integration tests of Gardener.

## Structure

Gardener integration tests can be found under `gardener/test/integration`, the testing directory
is divided into the following major subdirectories:

```console
├── framework
├── gardener
│   ├── rbac
│   └── reconcile
├── plants
├── resources
│   ├── charts
│   ├── repository
│   └── templates
├── scheduler
├── seeds
│   └── logging
└── shoots
    ├── applications
    ├── creation
    ├── deletion
    ├── hibernation
    ├── maintenance
    ├── networking
    ├── reconcile
    ├── update
    └── worker
```

In the following sections we will briefly describe the contents of each subdirectory.

### Framework

The framework directory contains all the necessary functions / utilities for running the integration test suite. For example, there are methods for creation/deletion of shoots, waiting for shoot deletion/creation, downloading/installing/deploying helm charts, logging, etc.

```console
framework
├── common.go
├── dump.go
├── errors.go
├── garden_operation.go
├── helm_utils.go
├── plant_operations.go
├── scheduler_operations.go
├── shoot_maintenance_operations.go
├── shoot_operations.go
├── types.go
├── utils.go
└── worker_operations.go
```

### Resources

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

- **Charts:** the charts will be downloaded and saved in this directory
- **Repository** contains the repository.yaml file that the target helm repos will be read from and the cache where the `stable-index.yaml` file will be created

### Shoots

This directory contains the actual tests. Currently the testing framework supports several kinds of tests:

- Shoot creation
- Shoot application

### Shoot Creation test

Create Shoot test is meant to test shoot creation.

#### Example Run

```console
go test \
  -mod=vendor \
  -timeout=20m \
  ./test/integration/shoots/creation \
  -gardener-kubecfg-path=$HOME/.kube/config \
  -shoot-name=$SHOOT_NAME
  -cloud-profile=$CLOUDPROFILE \
  -seed=$SEED \
  -secret-binding=$SECRET_BINDING \
  -provider-type=$PROVIDER_TYPE \
  -region=$REGION \
  -k8s-version=$K8S_VERSION \
  -project-namespace=$PROJECT_NAMESPACE \
  -infrastructure-provider-config-filepath=$INFRASTRUCTURE_PROVIDER_CONFIG_FILEPATH \
  -controlplane-provider-config-filepath=$CONTROLPLANE_PROVIDER_CONFIG_FILEPATH \
  -workers-config-filepath=$$WORKERS_CONFIG_FILEPATH \
  -networking-pods=$NETWORKING_PODS \
  -networking-services=$NETWORKING_SERVICES \
  -networking-nodes=$NETWORKING_NODES \
  -ginkgo.v \
  -ginkgo.progress
```

### Shoot application

Shoot applications tests are tests that are executed on a Shoot. The test is executed against an existing Shoot by specifying the `-shoot-name` and `shoot-namespace` flags.

Below are the flags used for running the application tests:

```go
kubeconfig     = flag.String("kubecfg", "", "the path to the kubeconfig of the Garden cluster that will be used for integration tests")
shootName      = flag.String("shoot-name", "", "the name of the shoot we want to test")
shootNamespace = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
downloadPath   = flag.String("download-path", "/tmp/test", "the path to which you download the kubeconfig")
```

#### Example Run

The command requires a garden kubeconfig, Shoot name and Shoot namespace.

```console
go test \
  -mod=vendor \
  ./test/integration/shoots/applciation \
  -kubecfg $HOME/.kube/config \
  -shoot-name $SHOOT_NAME \
  -shoot-namespace "garden-dev" \
  -ginkgo.v \
  -ginkgo.progress
```

### Seed Logging

Seed logging tests are meant to test the logging functionality for Seed clusters. The test is executed against an existing Shoot by specifying the `-shoot-name` and `shoot-namespace` flags.

Below are the flags used for running the logging tests:

```go
kubeconfig       = flag.String("kubecfg", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
shootName        = flag.String("shoot-name", "", "the name of the shoot we want to test")
shootNamespace   = flag.String("shoot-namespace", "", "the namespace name that the shoot resides in")
testShootsPrefix = flag.String("prefix", "", "prefix to use for test shoots")
logLevel         = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
logsCount        = flag.Uint64("logs-count", 10000, "the logs count to be logged by the logger application")
```

#### Example Run

The command requires a garden kubeconfig, shoot name and shoot namespace.

```console
go test \
  -mod=vendor \
  ./test/integration/seeds/logging \
  -kubecfg $HOME/.kube/config \
  -shoot-name $SHOOT_NAME \
  -shoot-namespace $SHOOT_NAMESPACE \
  -ginkgo.v \
  -ginkgo.progress
```

## Gardener

Currently the gardener tests consists of:

- RBAC test
- shoots reconcile test

### Gardener RBAC test

The gardener RBAC test is meant to test if RBAC is enabled on the gardener cluster.
This is tested by:

1. Check if the RBAC API-Resource is available
1. Check if a service account in a project namespace can access the `garden` project.

#### Example Run

```console
go test \
  -mod=vendor \
  ./test/integration/gardener/rbac \
  -kubecfg=$HOME/.kube/config \
  -project-namespace=$PROJECT_NAMESPACE \
  -ginkgo.v \
  -ginkgo.progress
```

### Gardener reconcile test

The gardener reconcile test checks if all shoots of a gardener cluster are successfully reconciled after the gardener version was updated.

#### Example Run

```console
go test \
  -mod=vendor \
  ./test/integration/gardener/rbac \
  -kubecfg=$HOME/.kube/config \
  -version=0.26.4 \
  -ginkgo.v \
  -ginkgo.progress
```
