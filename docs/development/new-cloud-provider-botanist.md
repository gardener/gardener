# Botanist Implementation for New Cloud Provider

This document describes how to extend Botanist to support a new cloud provider. There is a significant number of steps necessary.

## Steps

The implementation consists of several parts:

1. Update the helm chart configurations
2. Update the Gardener golang code to support the cloud provider.

### Helm charts

1. If your implementation is not in-tree to Kubernetes as a cloud provider, add the cloud-controller-manager (CCM) and cloud-storage-interface (CSI) images to [charts/images.yaml](../../charts/images.yaml). This is _just_ a manifest of all _potential_ images to be used in Gardener. The images are referenced in Gardener by the unique `name` key in the file, and can be used elsewhere in Gardener.
    * Add the provider-specific CCM with a unique name across Gardener
    * Add the provider-specific CSI with a unique name across Gardener
2. Update config files for the seed control plane charts in [charts/seed-controlplane/charts/](../../charts/seed-controlplane/charts/):
    1. If necessary, update the seed control plane chart for the cloud controller manager in `cloud-controller-manager/templates/cloud-controller-manager.yaml`
    2. If necessary, update the default values for the cloud controller manager in `cloud-controller-manager/values.yaml`
    3. Ensure the cloud provider is listed as an option for the cloud volume plugin in the relevant `if contains` statement in `kube-controller-manager/templates/_helpers.tpl`
3. Create config files for the seed machines charts in [charts/seed-machines/charts](../../charts/seed-machines/charts):
    1. Create a directory for the cloud provider machine class as `<provider>-machineclass/Chart.yaml`
    2. Create a machine class template for the cloud provider as `<provider>-machineclass/templates/<provider>-machineclass.yaml`
    3. Create default values for the cloud provider as `<provider>-machineclass/values.yaml`
4. Create config files for the seed terraformer charts in [charts/seed-terraformer/charts/](../../charts/seed-terraformer/charts/):
    1. Create common infrastructure chart as `<provider>-infra/Chart.yaml`
    2. Create the main infrastructure template as `<provider>-infra/templates/_main.tf`
    3. Create terraform values template as `<provider>-infra/templates/_terraform.tfvars`
    4. Create the terraform variables template as `<provider>-infra/templates/_variables.tf`
    5. Create the `config.yaml` to include the common config as `<provider>-infra/templates/config.yaml`
    6. Create the default values as `<provider>-infra/values.yaml`
    7. Add a symlink from `<provider>-infra/charts/terraformer-common` to `../../terraformer-common`
5. If your cloud provider's volume provisioner is not in-tree for Kubernetes, and thus requires usage of a container storage interface (CSI) provider, perform this step:
    1. Create a CSI chart for your cloud provider in [charts/shoot-core/charts/](../../charts/shoot-core/charts/) named `csi-<provider>`
    2. Populate the chart with the `yaml` files necessary to use CSI for your cloud provider. See existing examples. You must provide the _entire_ CSI uplift, including the common attacher and provisioner, as well as your cloud provider's specific plugin.
    3. Add a section for `csi-<provider>` with the images and credentials, and `enabled: false`, to [charts/shoot-core/values.yaml](../../charts/shoot-core/values.yaml). See existing examples.
6. Add your cloud provider and supported versions to [docs/usage/supported_k8s_versions.md](../../docs/usage/supported_k8s_versions.md)
7. Update the config files in [charts/seed-bootstrap/templates](../../charts/seed-bootstrap/templates/) as follows:
    1. Update `clusterrole-machine-controller-manager.yaml` to include your new provider as `<provider>machineclasses` in the list of `resources` in the `ClusterRole`.
    2. In `crd-machines.yaml`, create a `CustomResourceDefinition` for your provider named `<provider>MachineClass`. See the existing definitions.
8. If you have updated the machine controller manager (MCM) in a previous step to include your cloud provider, update [charts/values.yaml](../../charts/values.yaml) to increment the `images:` entry for `name: machine-controller-manager` with the new tag version.
9. Add examples to [hack/templates/resources/](../../hack/templates/resources//):
    1. Create a file `30-cloudprofile-<provider>.yaml.tpl`. See existing examples.
    2. Create a file `50-seed-<provider>.yaml.tpl`. See existing examples.
    3. Run `hack/generate-examples` to seed the `example/` directory.

### Golang code

1. If necessary, add support for your cloud provider libraries to [Gopkg.toml](../../Gopkg.toml), and run `make revendor`. 
2. Update [pkg/apis/garden/v1beta1/helper/helpers.go](../../pkg/apis/garden/v1beta1/helper/helpers.go) as follows:
    * Add a `case` for your cloud provider as `case gardenv1beta1.CloudProvider<Provider>` to `GetShootCloudProviderWorkers()`
    * Add a `case` for your cloud provider as `case gardenv1beta1.CloudProvider<Provider>` to `DetermineMachineImage()`
3. If your cloud provider SDK uses a persistent client object, Create a client implementation for the provider at [pkg/client/](../../pkg/client/) named `pkg/client/<provider>/`. This client will only be used by code that you will implement in the following steps, so it can follow any convention you want. However, by normal convention, it uses the following standard:
    * Main entry file `client.go`
    * Any types defined in `types.go`
4. Create a cloud botanist implementation for the cloud provider at [pkg/operation/cloudbotanist/](../../pkg/operation/cloudbotanist/) named `<provider>botanist/`. The botanist will be used for primary interfacing with the cloud provider. It should follow these conventions:
    * The primary entrypoint to create a new instance is `<provider>botanist.New(o *operation.Operation, purpose string)`, and will be called by `cloudbotanist.go`
    * `New()` is expected to return a `struct` that implements the `CloudBotanist interface` defined in [pkg/operation/cloudbotanist/types.go](../../pkg/operation/cloudbotanist/types.go)
5. Consume the cloud botanist implementation you created in the previous step by adding a `case gardenv1beta1.CloudProvider<Provider>` to the `func New()` in [pkg/operation/cloudbotanist/cloudbotanist.go](../../pkg/operation/cloudbotanist/cloudbotanist.go)
6. Update [pkg/operation/shoot/shoot.go](../../pkg/operation/shoot/shoot.go) to include the following:
    * Add a `case gardenv1beta1.CloudProvider<Provider>` to `GetK8SNetworks()` to return a proper `K8SNetworks` reference for the provider

