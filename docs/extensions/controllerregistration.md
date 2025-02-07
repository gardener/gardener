---
title: Registering Extension Controllers
---

# Registering Extension Controllers

Extensions are registered in the garden cluster via [`ControllerRegistration`](../../example/25-controllerregistration.yaml) resources.
Deployment for respective extensions are specified via [`ControllerDeployment`](../../example/25-controllerdeployment.yaml) resources.
Gardener evaluates the registrations and deployments and creates [`ControllerInstallation`](../../example/25-controllerinstallation.yaml) resources which describe the request "please install this controller `X` to this seed `Y`".

Similar to how `CloudProfile` or `Seed` resources get into the system, the Gardener administrator must deploy the `ControllerRegistration` and `ControllerDeployment` resources (this does not happen automatically in any way - the administrator decides which extensions shall be enabled).

The specification mainly describes which of Gardener's extension CRDs are managed, for example:

```yaml
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: os-gardenlinux
helm:
  ociRepository:
    ref: registry.example.com/os-gardenlinux/charts/os-gardenlinux:1.0.0
  # or a base64-encoded, gzip'ed, tar'ed extension controller chart
  # rawChart: H4sIFAAAAAAA/yk...
  values:
    foo: bar
---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: os-gardenlinux
spec:
  deployment:
    deploymentRefs:
    - name: os-gardenlinux
  resources:
  - kind: OperatingSystemConfig
    type: gardenlinux
    primary: true
```

This information tells Gardener that there is an extension controller that can handle `OperatingSystemConfig` resources of type `gardenlinux`.
A reference to the shown `ControllerDeployment` specifies how the deployment of the extension controller is accomplished.

Also, it specifies that this controller is the primary one responsible for the lifecycle of the `OperatingSystemConfig` resource.
Setting `primary` to `false` would allow to register additional, secondary controllers that may also watch/react on the `OperatingSystemConfig/coreos` resources, however, only the primary controller may change/update the main `status` of the extension object (that are used to "communicate" with the gardenlet).
Particularly, only the primary controller may set `.status.lastOperation`, `.status.lastError`, `.status.observedGeneration`, and `.status.state`.
Secondary controllers may contribute to the `.status.conditions[]` if they like, of course.

Secondary controllers might be helpful in scenarios where additional tasks need to be completed which are not part of the reconciliation logic of the primary controller but separated out into a dedicated extension.

⚠️ There must be exactly one primary controller for every registered kind/type combination.
Also, please note that the `primary` field cannot be changed after creation of the `ControllerRegistration`.

## Deploying Extension Controllers

Submitting the above `ControllerDeployment` and `ControllerRegistration` will create a `ControllerInstallation` resource:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerInstallation
metadata:
  name: os-gardenlinux
spec:
  deploymentRef:
    name: os-gardenlinux
  registrationRef:
    name: os-gardenlinux
  seedRef:
    name: aws-eu1
```

This resource expresses that Gardener requires the `os-gardenlinux` extension controller to run on the `aws-eu1` seed cluster.

gardener-controller-manager automatically determines which extension is required on which seed cluster and will only create `ControllerInstallation` objects for those.
Also, it will automatically delete `ControllerInstallation`s referencing extension controllers that are no longer required on a seed (e.g., because all shoots on it have been deleted).
There are additional configuration options, please see the [Deployment Configuration Options section](#deployment-configuration-options).
After gardener-controller-manager has written the `ControllerInstallation` resource, gardenlet picks it up and installs the controller on the respective `Seed` using the referenced `ControllerDeployment`.

It is sufficient to create a Helm chart and deploy it together with some static configuration values.
For this, operators have to provide the deployment information in the `ControllerDeployment.helm` section:

```yaml
...
helm:
  rawChart: H4sIFAAAAAAA/yk...
  values:
    foo: bar
```

You can check out [`hack/generate-controller-registration.yaml`](../../hack/generate-controller-registration.sh) for generating a `ControllerDeployment` including a controller helm chart.

If `ControllerDeployment.helm` is specified, gardenlet either decodes the provided Helm chart (`.helm.rawChart`) or pulls the chart from the referenced OCI Repository (`.helm.ociRepository`).
When referencing an OCI Repository, you have several options in how to specify where to pull the chart:

```yaml
helm:
  ociRepository:
    # full ref with either tag or digest, or both
    ref: registry.example.com/foo:1.0.0@sha256:abc
---
helm:
  ociRepository:
    # repository and tag
    repository: registry.example.com
    tag: 1.0.0
---
helm:
  ociRepository:
    # repository and digest
    repository: registry.example.com
    digest: sha256:abc
---
helm:
  ociRepository:
    # when specifying both tag and digest, the tag is ignored.
    repository: registry.example.com
    tag: 1.0.0
    digest: sha256:abc
```

If needed, a pull secret can be referenced in the `ControllerDeployment.helm.ociRepository.pullSecretRef` field.
The pull secret must be available in the `garden` namespace of the cluster where the `ControllerDeployment` is created and must contain the data key `.dockerconfigjson` with the base64-encoded Docker configuration JSON.  It should be of type `kubernetes.io/dockerconfigjson`.

```yaml
helm:
  ociRepository:
    repository: registry.example.com
    tag: 1.0.0
    pullSecretRef:
      name: my-pull-secret
```

Gardenlet caches the downloaded chart in memory. It is recommended to always specify a digest, because if it is not specified, gardenlet needs to fetch the manifest in every reconciliation to compare the digest with the local cache.

No matter where the chart originates from, gardenlet deploys it with the provided static configuration (`.helm.values`).
The chart and the values can be updated at any time - Gardener will recognize it and re-trigger the deployment process.
In order to allow extensions to get information about the garden and the seed cluster, gardenlet mixes in certain properties into the values (root level) of every deployed Helm chart:

```yaml
gardener:
  version: <gardener-version>
  garden:
    clusterIdentity: <uuid-of-gardener-installation>
    genericKubeconfigSecretName: <generic-garden-kubeconfig-secret-name>
  seed:
    name:             <seed-name>
    clusterIdentity:  <seed-cluster-identity>
    annotations:      <seed-annotations>
    labels:           <seed-labels>
    provider:         <seed-provider-type>
    region:           <seed-region>
    volumeProvider:   <seed-first-volume-provider>
    volumeProviders:  <seed-volume-providers>
    ingressDomain:    <seed-ingress-domain>
    protected:        <seed-protected-taint>
    visible:          <seed-visible-setting>
    taints:           <seed-taints>
    networks:         <seed-networks>
    blockCIDRs:       <seed-networks-blockCIDRs>
    spec:             <seed-spec>
  gardenlet:
    featureGates: <gardenlet-feature-gates>
```

Extensions can use this information in their Helm chart in case they require knowledge about the garden and the seed environment.
The list might be extended in the future.

gardenlet reports whether the extension controller has been installed successfully and running in the `ControllerInstallation` status:

```yaml
status:
  conditions:
  - lastTransitionTime: "2024-05-16T13:04:16Z"
    lastUpdateTime: "2024-05-16T13:04:16Z"
    message: The controller running in the seed cluster is healthy.
    reason: ControllerHealthy
    status: "True"
    type: Healthy
  - lastTransitionTime: "2024-05-16T13:04:06Z"
    lastUpdateTime: "2024-05-16T13:04:06Z"
    message: The controller was successfully installed in the seed cluster.
    reason: InstallationSuccessful
    status: "True"
    type: Installed
  - lastTransitionTime: "2024-05-16T13:04:16Z"
    lastUpdateTime: "2024-05-16T13:04:16Z"
    message: The controller has been rolled out successfully.
    reason: ControllerRolledOut
    status: "False"
    type: Progressing
  - lastTransitionTime: "2024-05-16T13:03:39Z"
    lastUpdateTime: "2024-05-16T13:03:39Z"
    message: chart could be rendered successfully.
    reason: RegistrationValid
    status: "True"
    type: Valid
```

### Deployment Configuration Options

The `.spec.deployment` resource allows to configure a deployment `policy`.
There are the following policies:

* `OnDemand` (default): Gardener will demand the deployment and deletion of the extension controller to/from seed clusters dynamically. It will automatically determine (based on other resources like `Shoot`s) whether it is required and decide accordingly.
* `Always`: Gardener will demand the deployment of the extension controller to seed clusters independent of whether it is actually required or not. This might be helpful if you want to add a new component/controller to all seed clusters by default. Another use-case is to minimize the durations until extension controllers get deployed and ready in case you have highly fluctuating seed clusters.
* `AlwaysExceptNoShoots`: Similar to `Always`, but if the seed does not have any shoots, then the extension is not being deployed. It will be deleted from a seed after the last shoot has been removed from it.

Also, the `.spec.deployment.seedSelector` allows to specify a label selector for seed clusters.
Only if it matches the labels of a seed, then it will be deployed to it.
Please note that a seed selector can only be specified for secondary controllers (`primary=false` for all `.spec.resources[]`).

### `Extension` Resource Configurations

The `Extension` resource allows injecting arbitrary steps into the shoot reconciliation flow that are unknown to Gardener.
Hence, it is slightly special and allows further configuration when registering it:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: extension-foo
spec:
  resources:
  - kind: Extension
    type: foo
    primary: true
    globallyEnabled: true
    reconcileTimeout: 30s
    lifecycle:
      reconcile: AfterKubeAPIServer
      delete: BeforeKubeAPIServer
      migrate: BeforeKubeAPIServer
```

The `globallyEnabled=true` option specifies that the `Extension/foo` object shall be created by default for all shoots (unless they opted out by setting `.spec.extensions[].enabled=false` in the `Shoot` spec).

The `reconcileTimeout` tells Gardener how long it should wait during its shoot reconciliation flow for the `Extension/foo`'s reconciliation to finish.

#### `Extension` Lifecycle

The `lifecycle` field tells Gardener when to perform a certain action on the `Extension` resource during the reconciliation flows. If omitted, then the default behaviour will be applied. Please find more information on the defaults in the explanation below. Possible values for each control flow are `AfterKubeAPIServer`, `BeforeKubeAPIServer`, and `AfterWorker`. Let's take the following configuration and explain it.

```yaml
    ...
    lifecycle:
      reconcile: AfterKubeAPIServer
      delete: BeforeKubeAPIServer
      migrate: BeforeKubeAPIServer
```

* `reconcile: AfterKubeAPIServer` means that the extension resource will be reconciled after the successful reconciliation of the `kube-apiserver` during shoot reconciliation. This is also the default behaviour if this value is not specified. During shoot hibernation, the opposite rule is applied, meaning that in this case the reconciliation of the extension will happen before the `kube-apiserver` is scaled to 0 replicas. On the other hand, if the extension needs to be reconciled before the `kube-apiserver` and scaled down after it, then the value `BeforeKubeAPIServer` should be used.
* `delete: BeforeKubeAPIServer` means that the extension resource will be deleted before the `kube-apiserver` is destroyed during shoot deletion. This is the default behaviour if this value is not specified.
* `migrate: BeforeKubeAPIServer` means that the extension resource will be migrated before the `kube-apiserver` is destroyed in the source cluster during [control plane migration](../operations/control_plane_migration.md). This is the default behaviour if this value is not specified. The restoration of the control plane follows the reconciliation control flow.

The lifecycle value `AfterWorker` is only available during `reconcile`. When specified, the extension resource will be reconciled after the workers are deployed. This is useful for extensions that want to deploy a workload in the shoot control plane and want to wait for the workload to run and get ready on a node. During shoot creation the extension will start its reconciliation before the first workers have joined the cluster, they will become available at some later point.
