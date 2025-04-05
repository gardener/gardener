---
title: Registering Extension Controllers
---

# Registering Extension Controllers

Before Gardener can manage shoot clusters, it needs to know which required and optional extensions are available in the landscape.
The following sections explain the general registration process for extensions.

## `Extension`s

The registration starts by creating [`Extension`](../../example/operator/15-extension.yaml) resources in the garden runtime cluster. 
They represent the single source for all deployment aspects an extension may offer (garden, seed, shoot, admission).
The `gardener-operator` takes these resources to deploy the extension to the garden runtime cluster, as well as creating corresponding [`ControllerRegistration`](../../example/25-controllerregistration.yaml) and [`ControllerDeployment`](../../example/25-controllerdeployment.yaml) resources in the virtual garden cluster.

Please see the following example of an `Extension` resource which mainly configures:
- The resource kinds and types the extension is responsible for
- A Reference the OCI Helm chart for the extension
- Values for the extension in the garden runtime cluster
- Values for the deployment in the seed clusters
- A Reference the OCI Helm chart(s) for the extension admission

```yaml
apiVersion: operator.gardener.cloud/v1alpha1
kind: Extension
metadata:
  name: provider-local
spec:
  resources:
  - kind: BackupBucket
    type: local
  - kind: BackupEntry
    type: local
  - kind: DNSRecord
    type: local
  - kind: Infrastructure
    type: local
  - kind: ControlPlane
    type: local
  - kind: Worker
    type: local
  deployment:
    admission:
      values: {}
      runtimeCluster:
        helm:
          ociRepository:
            ref: registry.example.com/gardener/extensions/local/admission-runtime:v1.0.0
      virtualCluster:
        helm:
          ociRepository:
            ref: registry.example.com/gardener/extensions/local/adission-application:v1.0.0
    extension:
      values:
         controllers:
           dnsrecord:
             concurrentSyncs: 20
      runtimeClusterValues:
        controllers:
          dnsrecord:
            concurrentSyncs: 1
      helm:
          ociRepository:
            ref: registry.example.com/gardener/extensions/local/extension:v1.0.0
```

Operators may use `Extension`s to observe their status conditions, regularly updated by `gardener-operator`.
They provide more information about whether an extension is currently in use and if their installation was successful.

```yaml
status:
  conditions:
  - lastTransitionTime: "2025-03-12T13:46:51Z"
    lastUpdateTime: "2025-03-12T13:46:51Z"
    message: Extension required for kinds [DNSRecord]
    reason: ExtensionRequired
    status: "True"
    type: RequiredRuntime
  - lastTransitionTime: "2025-01-20T10:39:47Z"
    lastUpdateTime: "2025-01-20T10:39:47Z"
    message: Extension has required ControllerInstallations for seed clusters
    reason: RequiredControllerInstallation
    status: "True"
    type: RequiredVirtual
  - lastTransitionTime: "2025-04-03T06:42:37Z"
    lastUpdateTime: "2025-04-03T06:42:37Z"
    message: Extension has been reconciled successfully
    reason: ReconcileSuccessful
    status: "True"
    type: Installed
```

## `ControllerRegistration`s

In the virtual garden cluster, the native extension registration resource kinds are `ControllerRegistration` and `ControllerDeployment`.
These resources are usually created by the `gardener-operator` based on `Extension`s in the runtime cluster.
They provide the `gardenlet`s in the seed clusters with information about which extensions are available and how to deploy them.

> [!NOTE]
> Before [gardener/gardener#9635](https://github.com/gardener/gardener/issues/9635), the only option to register extensions was via `ControllerRegistration`/`ControllerDeployment` resources.
> In the meantime, they became an implementation detail of the extension registration and should be treated as a gardener internal object.
> While it's still possible to create them manually (without `Extension`s), operators should only consider this option for advanced use cases.

Once created, gardener evaluates the registrations and deployments and creates [`ControllerInstallation`](../../example/25-controllerinstallation.yaml) resources which describe the request "please install this controller `X` to this seed `Y`".

The specification mainly describes which of Gardener's extension CRDs are managed, for example:

```yaml
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: provider-local
helm:
  ociRepository:
    ref: registry.example.com/gardener/extensions/local/extension:v1.0.0
  values:
    controllers:
      dnsrecord:
        concurrentSyncs: 20
---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: provider-local
spec:
  deployment:
    deploymentRefs:
    - name: provider-local
  resources:
  - kind: BackupBucket
    type: local
  - kind: BackupEntry
    type: local
  - kind: DNSRecord
    type: local
  - kind: Infrastructure
    type: local
  - kind: ControlPlane
    type: local
  - kind: Worker
    type: local
```

This information tells Gardener that there is an extension controller that can handle `BackupBucket`, `BackupEntry`, `DNSRecord`, `Infrastructure`, `ControlPlane` and `Worker` resources of type `local`.
A reference to the shown `ControllerDeployment` specifies how the deployment of the extension controller is accomplished.

## Deploying Extension Controllers

In the garden runtime cluster `gardener-operator` deploys the extension directly, as soon as it is considered as required.
Deployments in the seed clusters are represented by another resource called `ControllerInstallation`.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerInstallation
metadata:
  name: provider-local
spec:
  deploymentRef:
    name: provider-local
  registrationRef:
    name: provider-local
  seedRef:
    name: local-1
```

This resource expresses that Gardener requires the `provider-local` extension controller to run on the `local-1` seed cluster.

`gardener-controller-manager` automatically determines which extension is required on which seed cluster and will only create `ControllerInstallation` objects for those.
Also, it will automatically delete `ControllerInstallation`s referencing extension controllers that are no longer required on a seed (e.g., because all shoots on it have been deleted).
There are additional configuration options, please see the [Deployment Configuration Options section](#deployment-configuration-options).
After `gardener-controller-manager` has written the `ControllerInstallation` resource, gardenlet picks it up and installs the controller on the respective `Seed` using the referenced `ControllerDeployment`.

### Helm Charts

`Extension`s and `ControllerDeployment`s both need to specify a reference to an OCI Helm chart that contains the extension controller.
Those charts are usually provided by the extension and allow their deployment to the garden runtime or seed clusters.

> [!Note]
> Due to legacy reasons, a `ControllerDeployment` can work with a `rawChart` instead of an OCI image reference.
> If your extension does not yet offer an OCI image, you may consider using this option as a temporary workaround.
> Please note, that `rawChart` is not supported in `Extension`s and thus cannot be used for a deployment in the garden runtime cluster.

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

```yaml
helm:
  ociRepository:
    repository: registry.example.com
    tag: 1.0.0
    pullSecretRef:
      name: my-pull-secret
```

The pull secret must be available in the `garden` namespace of the cluster where the `ControllerDeployment` is created and must contain the data key `.dockerconfigjson` with the base64-encoded Docker configuration JSON.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-pull-secret
  namespace: garden
  labels:
    gardener.cloud/role: helm-pull-secret
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64-encoded-docker-config-json>
```

The downloaded chart is cached in memory. It is recommended to always specify a digest, because if it is not specified, the manifest is fetched in every reconciliation to compare the digest with the local cache.

### Helm Values

No matter where the chart originates from, `gardener-operator` and `gardenlet` deploy it with the provided Helm values.
The chart and the values can be updated at any time - Gardener will recognize it and re-trigger the deployment process.
In order to allow extensions to get information about the garden and the seed cluster, additional properties are mixed into the values (root level) of every deployed Helm chart:

- Additional properties for garden deployment
```yaml
  gardener:
    runtimeCluster:
      enabled: true
      priorityClassName: <priority-class-name-for-extension>
```

- Additional properties for seed deployment
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

### Deployment Configuration Options

The `.spec.extension` structure allows to configure a deployment `policy`.
There are the following policies:

* `OnDemand` (default): Gardener will demand the deployment and deletion of the extension controller to/from seed clusters dynamically. It will automatically determine (based on other resources like `Shoot`s) whether it is required and decide accordingly.
* `Always`: Gardener will demand the deployment of the extension controller to seed clusters independent of whether it is actually required or not. This might be helpful if you want to add a new component/controller to all seed clusters by default. Another use-case is to minimize the durations until extension controllers get deployed and ready in case you have highly fluctuating seed clusters.
* `AlwaysExceptNoShoots`: Similar to `Always`, but if the seed does not have any shoots, then the extension is not being deployed. It will be deleted from a seed after the last shoot has been removed from it.

Also, the `.spec.extension.seedSelector` allows to specify a label selector for seed clusters.
Only if it matches the labels of a seed, then it will be deployed to it.
Please note that a seed selector can only be specified for secondary controllers (`primary=false` for all `.spec.resources[]`).

### `Extension` Resource Configurations

The `Extension` resource allows injecting arbitrary steps into the garden, seed and shoot reconciliation flow that are unknown to Gardener.
Hence, it is slightly special and allows further configuration when registering it:

```yaml
apiVersion: operator.gardener.cloud/v1alpha1
kind: Extension
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

The `reconcileTimeout` tells Gardener how long it should wait during its reconciliation flow for the `Extension/foo`'s reconciliation to finish.

`primary` specifies whether the extension controller is the main one responsible for the lifecycle of the `Extension` resource.
Setting `primary` to `false` would allow to register additional, secondary controllers that may also watch/react on the `Extension/foo` resources, however, only the primary controller may change/update the main `status` of the extension object.
Particularly, only the primary controller may set `.status.lastOperation`, `.status.lastError`, `.status.observedGeneration`, and `.status.state`.
Secondary controllers may contribute to the `.status.conditions[]` if they like, of course.

Secondary controllers might be helpful in scenarios where additional tasks need to be completed which are not part of the reconciliation logic of the primary controller but separated out into a dedicated extension.

⚠️ There must be exactly one primary controller for every registered kind/type combination.
Also, please note that the `primary` field cannot be changed after creation of the `Extension`.

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

Due to technical reasons, exceptions apply for different reconcile flows, for example:
- The garden reconciliation doesn't distinguish between `AfterKubeAPIServer` and `AfterWorker`.
- The seed reconciliation completely ignores the `lifecycle` field.
- The lifecycle value `AfterWorker` is only available during `reconcile`. When specified, the extension resource will be reconciled after the workers are deployed. This is useful for extensions that want to deploy a workload in the shoot control plane and want to wait for the workload to run and get ready on a node. During shoot creation the extension will start its reconciliation before the first workers have joined the cluster, they will become available at some later point.
