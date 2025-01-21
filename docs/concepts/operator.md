---
title: Gardener Operator
description: Understand the component responsible for the garden cluster environment and its various features
---

## Overview

The `gardener-operator` is responsible for the garden cluster environment.
Without this component, users must deploy ETCD, the Gardener control plane, etc., manually and with separate mechanisms (not maintained in this repository).
This is quite unfortunate since this requires separate tooling, processes, etc.
A lot of production- and enterprise-grade features were built into Gardener for managing the seed and shoot clusters, so it makes sense to re-use them as much as possible also for the garden cluster.

## Deployment

There is a [Helm chart](../../charts/gardener/operator) which can be used to deploy the `gardener-operator`.
Once deployed and ready, you can create a `Garden` resource.
Note that there can only be one `Garden` resource per system at a time.

> ℹ️ Similar to seed clusters, garden runtime clusters require a [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), see [this section](#vertical-pod-autoscaler).
> By default, `gardener-operator` deploys the VPA components.
> However, when there already is a VPA available, then set `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=false` in the `Garden` resource.

## `Garden` Resources

Please find an exemplary `Garden` resource [here](../../example/operator/20-garden.yaml).

### Configuration For Runtime Cluster

#### Settings

The `Garden` resource offers a few settings that are used to control the behaviour of `gardener-operator` in the runtime cluster.
This section provides an overview over the available settings in `.spec.runtimeCluster.settings`:

##### Load Balancer Services

`gardener-operator` deploys Istio and relevant resources to the runtime cluster in order to expose the `virtual-garden-kube-apiserver` service (similar to how the `kube-apiservers` of shoot clusters are exposed).
In most cases, the `cloud-controller-manager` (responsible for managing these load balancers on the respective underlying infrastructure) supports certain customization and settings via annotations.
[This document](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) provides a good overview and many examples.

By setting the `.spec.runtimeCluster.settings.loadBalancerServices.annotations` field the Gardener administrator can specify a list of annotations which will be injected into the `Service`s of type `LoadBalancer`.

##### Vertical Pod Autoscaler

`gardener-operator` heavily relies on the Kubernetes [`vertical-pod-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
By default, the `Garden` controller deploys the VPA components into the `garden` namespace of the respective runtime cluster.
In case you want to manage the VPA deployment on your own or have a custom one, then you might want to disable the automatic deployment of `gardener-operator`.
Otherwise, you might end up with two VPAs which will cause erratic behaviour.
By setting the `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=false` you can disable the automatic deployment.

⚠️ In any case, there must be a VPA available for your runtime cluster.
Using a runtime cluster without VPA is not supported.

##### Topology-Aware Traffic Routing

Refer to the [Topology-Aware Traffic Routing documentation](../operations/topology_aware_routing.md) as this document contains the documentation for the topology-aware routing setting for the garden runtime cluster.

#### Volumes

It is possible to define the minimum size for `PersistentVolumeClaim`s in the runtime cluster created by `gardener-operator` via the `.spec.runtimeCluster.volume.minimumSize` field.
This can be relevant in case the runtime cluster runs on an infrastructure that does only support disks of at least a certain size.

### Configuration For Virtual Cluster

#### ETCD Encryption Config

The `spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig` field in the Garden API allows operators to customize encryption configurations for the `kube-apiserver` of the virtual cluster. It provides options to specify additional resources for encryption. Similarly `spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig` field allows operators to customize encryption configurations for the `gardener-apiserver`.

- The resources field can be used to specify resources that should be encrypted in addition to secrets. Secrets are always encrypted for the `kube-apiserver`. For the `gardener-apiserver`, the following resources are always encrypted:
  - `controllerdeployments.core.gardener.cloud`
  - `controllerregistrations.core.gardener.cloud`
  - `internalsecrets.core.gardener.cloud`
  - `shootstates.core.gardener.cloud`
- Adding an item to any of the lists will cause patch requests for all the resources of that kind to encrypt them in the etcd. See [Encrypting Confidential Data at Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data) for more details.
- Removing an item from any of these lists will cause patch requests for all the resources of that type to decrypt and rewrite the resource as plain text. See [Decrypt Confidential Data that is Already Encrypted at Rest](https://kubernetes.io/docs/tasks/administer-cluster/decrypt-data/) for more details.

> ℹ️ Note that configuring encryption for a custom resource for the `kube-apiserver` is only supported for Kubernetes versions >= 1.26.

## `Extension` Resource

A Gardener installation relies on extensions to provide support for new cloud providers or to add new capabilities.
You can find out more about Gardener extensions and how they can be used [here](../extensions/resources/extension.md#contract-extension-resource).

The `Extension` resource is intended to automate the installation and management of extensions in a Gardener landscape.
It contains configuration for the following scenarios:

- The deployment of the extension chart in the garden runtime cluster.
- The deployment of `ControllerRegistration` and `ControllerDeployment` resources in the (virtual) garden cluster.
- The deployment of [extension admissions charts](../extensions/admission.md) in runtime and virtual clusters.

With regard to the `Garden` reconciliation process, there are specific types of extensions that are of key interest, namely the `BackupBucket`, `DNSRecord`, and `Extension` types.
The `BackupBucket` extension is utilized to manage the backup bucket dedicated to the garden's main etcd.
The `DNSRecord` extension type is essential to manage the API server and ingress DNS records.
Lastly, the `Extension` type plays a crucial role in managing generic Gardener extensions which deploy various components within the runtime cluster. These extensions can be activated and configured in the `.spec.extensions` field of the `Garden` resource. These extensions can supplement functionality and provide new capabilities.

Please find an exemplary `Extension` resource [here](../../example/operator/15-extension.yaml).

### Extension Deployment

The `.spec.deployment` specifies how an extension can be installed for a Gardener landscape and consists of the following parts:

- `.spec.deployment.extension` contains the deployment specification of an extension.
- `.spec.deployment.admission` contains the deployment specification of an extension admission.

Each one is described in more details below.

#### Configuration for Extension Deployment

`.spec.deployment.extension` contains configuration for the registration of an extension in the garden cluster.
`gardener-operator` follows the same principles described by [this document](../extensions/controllerregistration.md#registering-extension-controllers):
- `.spec.deployment.extension.helm` and `.spec.deployment.extension.values` are used when creating the `ControllerDeployment` in the garden cluster.
- `.spec.deployment.extension.policy` and `.spec.deployment.extension.seedSelector` define the extension's installation policy as per the [`ControllerDeployment's` respective fields](../extensions/controllerregistration.md#deployment-configuration-options)

##### Runtime

Extensions can manage resources required by the `Garden` resource (e.g. `BackupBucket`, `DNSRecord`, `Extension`) in the runtime cluster.
Since the environment in the runtime cluster may differ from that of a `Seed`, the extension is installed in the runtime cluster with a distinct set of Helm chart values specified in `.spec.deployment.extension.runtimeValues`.
If no `runtimeValues` are provided, the extension deployment for the runtime garden is considered superfluous and the deployment is uninstalled.
The configuration allows for precise control over various extension parameters, such as requested resources, [priority classes](../development/priority-classes.md), and more.

Besides the values configured in `.spec.deployment.extension.runtimeValues`, a runtime deployment flag and a priority class are merged into the values:

```yaml
gardener:
  runtimeCluster:
    enabled: true # indicates the extension is enabled for the Garden cluster, e.g. for handling `BackupBucket`, `DNSRecord` and `Extension` objects.
    priorityClassName: gardener-garden-system-200
```

As soon as a `Garden` object is created and `runtimeValues` are configured, the extension is deployed in the runtime cluster. 

##### Extension Registration

When the virtual garden cluster is available, the `Extension` controller manages [`ControllerRegistration`/`ControllerDeployment` resources](../extensions/controllerregistration.md#registering-extension-controllers)
to register extensions for shoots.  The fields of `.spec.deployment.extension` include their configuration options. 

#### Configuration for Admission Deployment

The `.spec.deployment.admission` defines how an extension admission may be deployed by the `gardener-operator`.
This deployment is optional and may be omitted.
Typically, the admission are split in two parts:

##### Runtime

The `runtime` part contains deployment relevant manifests, required to run the admission service in the runtime cluster.
The following values are passed to the chart during reconciliation:

```yaml
gardener:
  runtimeCluster:
    priorityClassName: <Class to be used for extension admission>
```

##### Virtual

The `virtual` part includes the webhook registration ([MutatingWebhookConfiguration`/`Validatingwebhookconfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)) and RBAC configuration.
The following values are passed to the chart during reconciliation:

```yaml
gardener:
  virtualCluster:
    serviceAccount:
      name: <Name of the service account used to connect to the garden cluster>
      namespace: <Namespace of the service account>
```

Extension admissions often need to retrieve additional context from the garden cluster in order to process validating or mutating requests.

For example, the corresponding `CloudProfile` might be needed to perform a provider specific shoot validation.
Therefore, Gardener automatically injects a kubeconfig into the admission deployment to interact with the (virtual) garden cluster (see [this document](https://github.com/gardener/gardener/blob/master/docs/extensions/garden-api-access.md) for more information).

### Configuration for Extension Resources

The `.spec.resources` field refers to the extension resources as defined by Gardener in the `extensions.gardener.cloud/v1alpha1` API.
These include both well-known types such as `Infrastructure`, `Worker` etc. and [generic resources](https://github.com/gardener/gardener/blob/master/docs/extensions/controllerregistration.md#extension-resource-configurations).
The field will be used to populate the respective field in the resulting `ControllerRegistration` in the garden cluster.

## Controllers

The `gardener-operator` controllers are now described in more detail.

### [`Garden` Controller](../../pkg/operator/controller/garden)

The Garden controller in the operator reconciles Garden objects with the help of the following reconcilers.

#### [`Main` Reconciler](../../pkg/operator/controller/garden/garden)

The reconciler first generates a general CA certificate which is valid for ~`30d` and auto-rotated when 80% of its lifetime is reached.
Afterwards, it brings up the so-called "garden system components".
The [`gardener-resource-manager`](./resource-manager.md) is deployed first since its `ManagedResource` controller will be used to bring up the remainders.

Other system components are:

- runtime garden system resources ([`PriorityClass`es](../development/priority-classes.md) for the workload resources)
- virtual garden system resources (RBAC rules)
- Vertical Pod Autoscaler (if enabled via `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=true` in the `Garden`)
- ETCD Druid
- Istio

As soon as all system components are up, the reconciler deploys the virtual garden cluster.
It comprises out of two ETCDs (one "main" etcd, one "events" etcd) which are managed by ETCD Druid via `druid.gardener.cloud/v1alpha1.Etcd` custom resources.
The whole management works similar to how it works for `Shoot`s, so you can take a look at [this document](etcd.md) for more information in general.

The virtual garden control plane components are:

- `virtual-garden-etcd-main`
- `virtual-garden-etcd-events`
- `virtual-garden-kube-apiserver`
- `virtual-garden-kube-controller-manager`
- `virtual-garden-gardener-resource-manager`

If the `.spec.virtualCluster.controlPlane.highAvailability={}` is set then these components will be deployed in a "highly available" mode.
For ETCD, this means that there will be 3 replicas each.
This works similar like for `Shoot`s (see [this document](../usage/high-availability/shoot_high_availability.md)) except for the fact that there is no failure tolerance type configurability.
The `gardener-resource-manager`'s [HighAvailabilityConfig webhook](resource-manager.md#high-availability-config) makes sure that all pods with multiple replicas are spread on nodes, and if there are at least two zones in `.spec.runtimeCluster.provider.zones` then they also get spread across availability zones.

> If once set, removing `.spec.virtualCluster.controlPlane.highAvailability` again is not supported.

The `virtual-garden-kube-apiserver` `Deployment` is exposed via Istio, similar to how the `kube-apiservers` of shoot clusters are exposed.

Similar to the `Shoot` API, the version of the virtual garden cluster is controlled via `.spec.virtualCluster.kubernetes.version`.
Likewise, specific configuration for the control plane components can be provided in the same section, e.g. via `.spec.virtualCluster.kubernetes.kubeAPIServer` for the `kube-apiserver` or `.spec.virtualCluster.kubernetes.kubeControllerManager` for the `kube-controller-manager`.

The `kube-controller-manager` only runs a few controllers that are necessary in the scenario of the virtual garden.
Most prominently, **the `serviceaccount-token` controller is unconditionally disabled**.
Hence, the usage of static `ServiceAccount` secrets is not supported generally.
Instead, the [`TokenRequest` API](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/) should be used.
Third-party components that need to communicate with the virtual cluster can leverage the [`gardener-resource-manager`'s `TokenRequestor` controller](resource-manager.md#tokenrequestor-controller) and the generic kubeconfig, just like it works for `Shoot`s.
Please note, that this functionality is restricted to the `garden` namespace. The current `Secret` name of the generic kubeconfig can be found in the annotations (key: `generic-token-kubeconfig.secret.gardener.cloud/name`) of the `Garden` resource.

For the virtual cluster, it is essential to provide at least one DNS domain via `.spec.virtualCluster.dns.domains`.
**The respective DNS records are not managed by `gardener-operator` and should be created manually.
They should point to the load balancer IP of the `istio-ingressgateway` `Service` in namespace `virtual-garden-istio-ingress`.
The DNS records must be prefixed with both `gardener.` and `api.` for all domains in `.spec.virtualCluster.dns.domains`.**

The first DNS domain in this list is used for the `server` in the kubeconfig, and for configuring the `--external-hostname` flag of the API server.

Apart from the control plane components of the virtual cluster, the reconcile also deploys the control plane components of Gardener.
`gardener-apiserver` reuses the same ETCDs like the `virtual-garden-kube-apiserver`, so all data related to the "the garden cluster" is stored together and "isolated" from ETCD data related to the runtime cluster.
This drastically simplifies backup and restore capabilities (e.g., moving the virtual garden cluster from one runtime cluster to another).

The Gardener control plane components are:

- `gardener-apiserver`
- `gardener-admission-controller`
- `gardener-controller-manager`
- `gardener-scheduler`

Besides those, the `gardener-operator` is able to deploy the following optional components:
 - [Gardener Dashboard](https://github.com/gardener/dashboard) (and the [controller for web terminals](https://github.com/gardener/terminal-controller-manager)) when `.spec.virtualCluster.gardener.gardenerDashboard` (or `.spec.virtualCluster.gardener.gardenerDashboard.terminal`, respectively) is set.
 You can read more about it and its configuration in [this section](#gardener-dashboard).
 - [Gardener Discovery Server](https://github.com/gardener/gardener-discovery-server) when `.spec.virtualCluster.gardener.gardenerDiscoveryServer` is set.
 The service account issuer of shoots will be calculated in the format `https://discovery.<.spec.runtimeCluster.ingress.domains[0]>/projects/<project-name>/shoots/<shoot-uid>/issuer`.
 This configuration applies for all seeds registered with the Garden cluster. Once set it should not be modified.

The reconciler also manages a few observability-related components (more planned as part of [GEP-19](../proposals/19-migrating-observability-stack-to-operators.md)):

- `fluent-operator`
- `fluent-bit`
- `gardener-metrics-exporter`
- `kube-state-metrics`
- `plutono`
- `vali`
- `prometheus-operator`
- `alertmanager-garden` (read more [here](#alertmanager))
- `prometheus-garden` (read more [here](#garden-prometheus))
- `prometheus-longterm` (read more [here](#long-term-prometheus))
- `blackbox-exporter`

It is also mandatory to provide an IPv4 CIDR for the service network of the virtual cluster via `.spec.virtualCluster.networking.services`.
This range is used by the API server to compute the cluster IPs of `Service`s.

The controller maintains the `.status.lastOperation` which indicates the status of an operation.

##### [Gardener Dashboard](https://github.com/gardener/dashboard)

`.spec.virtualCluster.gardener.gardenerDashboard` serves a few configuration options for the dashboard.
This section highlights the most prominent fields:

- `oidcConfig`: The general OIDC configuration is part of `.spec.virtualCluster.kubernetes.kubeAPIServer.oidcConfig` (deprecated). Since Kubernetes 1.30 the general OIDC configuration happens via the Structured Authentication feature `.spec.virtualCluster.kubernetes.kubeAPIServer.structuredAuthentication`.
  This section allows you to define a few specific settings for the dashboard.
  `clientIDPublic` is the public ID of the OIDC client.
  `issuerURL` is the URL of the JWT issuer.
  `sessionLifetime` is the duration after which a session is terminated (i.e., after which a user is automatically logged out).
  `additionalScopes` allows to extend the list of scopes of the JWT token that are to be recognized.
  You must reference a `Secret` in the `garden` namespace containing the client and, if applicable, the client secret for the dashboard:
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: gardener-dashboard-oidc
    namespace: garden
  type: Opaque
  stringData:
    client_id: <client_id>
    client_secret: <optional>
  ```
  If using a public client, a client secret is not required. The dashboard can function as a public OIDC client, allowing for improved flexibility in environments where secret storage is not feasible.
- `enableTokenLogin`: This is enabled by default and allows logging into the dashboard with a JWT token.
  You can disable it in case you want to only allow OIDC-based login.
  However, at least one of the both login methods must be enabled.
- `frontendConfigMapRef`: Reference a `ConfigMap` in the `garden` namespace containing the frontend configuration in the data with key `frontend-config.yaml`, for example
  ```yaml
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: gardener-dashboard-frontend
    namespace: garden
  data:
    frontend-config.yaml: |
      helpMenuItems:
      - title: Homepage
        icon: mdi-file-document
        url: https://gardener.cloud
  ```
  Please take a look at [this file](https://github.com/gardener/dashboard/blob/64516ede9110065c24c61ab67f06c866fef10f3c/charts/gardener-dashboard/values.yaml#L154-L376) to get an idea of which values are configurable.
  This configuration can also include branding, themes, and colors.
  Read more about it [here](https://github.com/gardener/dashboard/blob/master/docs/operations/customization.md).
  Assets (logos/icons) are configured in a separate `ConfigMap`, see below.
- `assetsConfigMapRef`: Reference a `ConfigMap` in the `garden` namespace containing the assets, for example
  ```yaml
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: gardener-dashboard-assets
    namespace: garden
  binaryData:
    favicon-16x16.png: base64(favicon-16x16.png)
    favicon-32x32.png: base64(favicon-32x32.png)
    favicon-96x96.png: base64(favicon-96x96.png)
    favicon.ico: base64(favicon.ico)
    logo.svg: base64(logo.svg)
  ```
  Note that the assets must be provided base64-encoded, hence `binaryData` (instead of `data`) must be used.
  Please take a look at [this file](https://github.com/gardener/dashboard/blob/master/docs/operations/customization.md#logos-and-icons) to get more information.
- `gitHub`: You can connect a GitHub repository that can be used to create issues for shoot clusters in the cluster details page.
  You have to reference a `Secret` in the `garden` namespace that contains the GitHub credentials, for example:
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: gardener-dashboard-github
    namespace: garden
  type: Opaque
  stringData:
    # This is for GitHub token authentication:
    authentication.token: <secret>
    # Alternatively, this is for GitHub app authentication:
    authentication.appId: <secret>
    authentication.clientId: <secret>
    authentication.clientSecret: <secret>
    authentication.installationId: <secret>
    authentication.privateKey: <secret>
    # This is the webhook secret, see explanation below
    webhookSecret: <secret>
  ```
  Note that you can also set up a GitHub webhook to the dashboard such that it receives updates when somebody changes the GitHub issue.
  The `webhookSecret` field is the secret that you enter in GitHub in the [webhook configuration](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries#creating-a-secret-token).
  The dashboard uses it to verify that received traffic is indeed originated from GitHub.
  If you don't want to set up such webhook, or if the dashboard is not reachable by the GitHub webhook (e.g., in restricted environments) you can also configure `gitHub.pollInterval`.
  It is the interval of how often the GitHub API is polled for issue updates.
  This field is used as a fallback mechanism to ensure state synchronization, even when there is a GitHub webhook configuration.
  If a webhook event is missed or not successfully delivered, the polling will help catch up on any missed updates.
  If this field is not provided and there is no `webhookSecret` key in the referenced secret, it will be implicitly defaulted to `15m`.
  The dashboard will use this to regularly poll the GitHub API for updates on issues.
- `terminal`: This enables the web terminal feature, read more about it [here](https://github.com/gardener/dashboard/blob/master/docs/operations/webterminals.md).
  When set, the [`terminal-controller-manager`](https://github.com/gardener/terminal-controller-manager) will be deployed to the runtime cluster.
  The `allowedHosts` field is explained [here](https://github.com/gardener/dashboard/blob/master/docs/operations/webterminals.md#configuration).
  The `container` section allows you to specify a container image and a description that should be used for the web terminals.

##### Observability

###### Garden Prometheus

`gardener-operator` deploys a Prometheus instance in the `garden` namespace (called "Garden Prometheus") which fetches metrics and data from garden system components, cAdvisors, the virtual cluster control plane, and the Seeds' aggregate Prometheus instances.
Its purpose is to provide an entrypoint for operators when debugging issues with components running in the garden cluster.
It also serves as the top-level aggregator of metering across a Gardener landscape.

To extend the configuration of the Garden Prometheus, you can create the [`prometheus-operator`'s custom resources](https://github.com/prometheus-operator/prometheus-operator?tab=readme-ov-file#customresourcedefinitions) and label them with `prometheus=garden`, for example:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    prometheus: garden
  name: garden-my-component
  namespace: garden
spec:
  selector:
    matchLabels:
      app: my-component
  endpoints:
  - metricRelabelings:
    - action: keep
      regex: ^(metric1|metric2|...)$
      sourceLabels:
      - __name__
    port: metrics
```

###### Long-Term Prometheus

`gardener-operator` deploys another Prometheus instance in the `garden` namespace (called "Long-Term Prometheus") which federates metrics from [Garden Prometheus](#garden-prometheus).
Its purpose is to store those with a longer retention than Garden Prometheus would. It is not possible to define different retention periods for different metrics in Prometheus, hence, using another Prometheus instance is the only option.
This Long-term Prometheus also has an additional [Cortex](https://cortexmetrics.io/) sidecar container for caching some queries to achieve faster processing times.

To extend the configuration of the Long-term Prometheus, you can create the [`prometheus-operator`'s custom resources](https://github.com/prometheus-operator/prometheus-operator?tab=readme-ov-file#customresourcedefinitions) and label them with `prometheus=longterm`, for example:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    prometheus: longterm
  name: longterm-my-component
  namespace: garden
spec:
  selector:
    matchLabels:
      app: my-component
  endpoints:
  - metricRelabelings:
    - action: keep
      regex: ^(metric1|metric2|...)$
      sourceLabels:
      - __name__
    port: metrics
```

###### Alertmanager

By default, the `alertmanager-garden` deployed by `gardener-operator` does not come with any configuration.
It is the responsibility of the human operators to design and provide it.
This can be done by creating `monitoring.coreos.com/v1alpha1.AlertmanagerConfig` resources labeled with `alertmanager=garden` (read more about them [here](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/design.md#alertmanagerconfig)), for example:

```yaml
apiVersion: monitoring.coreos.com/v1alpha1
kind: AlertmanagerConfig
metadata:
  name: config
  namespace: garden
  labels:
    alertmanager: garden
spec:
  route:
    receiver: dev-null
    groupBy:
    - alertname
    - landscape
    routes:
    - continue: true
      groupWait: 3m
      groupInterval: 5m
      repeatInterval: 12h
      routes:
      - receiver: ops
        matchers:
        - name: severity
          value: warning
          matchType: =
        - name: topology
          value: garden
          matchType: =
  receivers:
  - name: dev-null
  - name: ops
    slackConfigs:
    - apiURL: https://<slack-api-url>
      channel: <channel-name>
      username: Gardener-Alertmanager
      iconEmoji: ":alert:"
      title: "[{{ .Status | toUpper }}] Gardener Alert(s)"
      text: "{{ range .Alerts }}*{{ .Annotations.summary }} ({{ .Status }})*\n{{ .Annotations.description }}\n\n{{ end }}"
      sendResolved: true
```

###### Plutono

A [Plutono](https://github.com/credativ/plutono) instance is deployed by `gardener-operator` into the `garden` namespace for visualizing monitoring metrics and logs via dashboards.
In order to provide custom dashboards, create a `ConfigMap` in the `garden` namespace labelled with `dashboard.monitoring.gardener.cloud/garden=true` that contains the respective JSON documents, for example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    dashboard.monitoring.gardener.cloud/garden: "true"
  name: my-custom-dashboard
  namespace: garden
data:
  my-custom-dashboard.json: <dashboard-JSON-document>
```

#### [`Care` Reconciler](../../pkg/operator/controller/garden/care)

This reconciler performs four "care" actions related to `Garden`s.

It maintains the following conditions:

- `VirtualGardenAPIServerAvailable`: The `/healthz` endpoint of the garden's `virtual-garden-kube-apiserver` is called and considered healthy when it responds with `200 OK`.
- `RuntimeComponentsHealthy`: The conditions of the `ManagedResource`s applied to the runtime cluster are checked (e.g., `ResourcesApplied`).
- `VirtualComponentsHealthy`: The virtual components are considered healthy when the respective `Deployment`s (for example `virtual-garden-kube-apiserver`,`virtual-garden-kube-controller-manager`), and `Etcd`s (for example `virtual-garden-etcd-main`) exist and are healthy. Additionally, the conditions of the `ManagedResource`s applied to the virtual cluster are checked (e.g., `ResourcesApplied`).
- `ObservabilityComponentsHealthy`: This condition is considered healthy when the respective `Deployment`s (for example `plutono`) and `StatefulSet`s (for example `prometheus`, `vali`) exist and are healthy.

If all checks for a certain condition are succeeded, then its `status` will be set to `True`.
Otherwise, it will be set to `False` or `Progressing`.

If at least one check fails and there is threshold configuration for the conditions (in `.controllers.gardenCare.conditionThresholds`), then the status will be set:

- to `Progressing` if it was `True` before.
- to `Progressing` if it was `Progressing` before and the `lastUpdateTime` of the condition does not exceed the configured threshold duration yet.
- to `False` if it was `Progressing` before and the `lastUpdateTime` of the condition exceeds the configured threshold duration.

The condition thresholds can be used to prevent reporting issues too early just because there is a rollout or a short disruption.
Only if the unhealthiness persists for at least the configured threshold duration, then the issues will be reported (by setting the status to `False`).

In order to compute the condition statuses, this reconciler considers `ManagedResource`s (in the `garden` and `istio-system` namespace) and their status, see [this document](resource-manager.md#conditions) for more information.
The following table explains which `ManagedResource`s are considered for which condition type:

| Condition Type                   | `ManagedResource`s are considered when                                                                               |
|----------------------------------|----------------------------------------------------------------------------------------------------------------------|
| `RuntimeComponentsHealthy`       | `.spec.class=seed` and `care.gardener.cloud/condition-type` label either unset, or set to `RuntimeComponentsHealthy` |
| `VirtualComponentsHealthy`       | `.spec.class` unset or `care.gardener.cloud/condition-type` label set to `VirtualComponentsHealthy`                  |
| `ObservabilityComponentsHealthy` | `care.gardener.cloud/condition-type` label set to `ObservabilityComponentsHealthy`                                   |

#### [`Reference` Reconciler](../../pkg/operator/controller/garden/reference)

`Garden` objects may specify references to other objects in the Garden cluster which are required for certain features.
For example, operators can configure a secret for ETCD backup via `.spec.virtualCluster.etcd.main.backup.secretRef.name` or an audit policy `ConfigMap` via `.spec.virtualCluster.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef.name`.
Such objects need a special protection against deletion requests as long as they are still being referenced by the `Garden`.

Therefore, this reconciler checks `Garden`s for referenced objects and adds the finalizer `gardener.cloud/reference-protection` to their `.metadata.finalizers` list.
The reconciled `Garden` also gets this finalizer to enable a proper garbage collection in case the `gardener-operator` is offline at the moment of an incoming deletion request.
When an object is not actively referenced anymore because the `Garden` specification has changed is in deletion, the controller will remove the added finalizer again so that the object can safely be deleted or garbage collected.

This reconciler inspects the following references:

- Admission plugin kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.admissionPlugins[].kubeconfigSecretName` and `.spec.virtualCluster.gardener.gardenerAPIServer.admissionPlugins[].kubeconfigSecretName`)
- Audit policy `ConfigMap`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef.name` and `.spec.virtualCluster.gardener.gardenerAPIServer.auditConfig.auditPolicy.configMapRef.name`)
- Audit webhook kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.auditWebhook.kubeconfigSecretName` and `.spec.virtualCluster.gardener.gardenerAPIServer.auditWebhook.kubeconfigSecretName`)
- Authentication webhook kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.authentication.webhook.kubeconfigSecretName`)
- DNS `Secret`s (`.spec.dns.providers[].secretRef`)
- ETCD backup `Secret`s (`.spec.virtualCluster.etcd.main.backup.secretRef`)
- Structured authentication `ConfigMap`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.structuredAuthentication.configMapName`)
- Structured authorization `ConfigMap`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.structuredAuthorization.configMapName`)
- Structured authorization kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.structuredAuthorization.kubeconfigs[].secretName`)
- SNI `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.sni.secretName`)

Further checks might be added in the future.

### [`Controller Registrar` Controller](../../pkg/operator/controller/controllerregistrar)

Some controllers may only be instantiated or added later, because they need the `Garden` resource to be available (e.g. network configuration) or even the entire virtual garden cluster to run:

* [`NetworkPolicy` controller](gardenlet.md#networkpolicy-controller)
* [`VPA EvictionRequirements` controller](gardenlet.md#vpaevictionrequirements-controller)
* [`Required Runtime` reconciler](#required-runtime-reconciler)
* [`Required Virtual` reconciler](#required-virtual-reconciler)
* [`Access` controller](#access-controller)
* [`Virtual-Cluster-Registrar` controller](#virtual-cluster-registrar-controller)
* [`Gardenlet` controller](#gardenlet-controller)

> [!NOTE]
> Some of the listed controllers are part of `gardenlet`, as well.
> If the garden cluster is a seed cluster at the same time, `gardenlet` will skip running the `NetworkPolicy` and `VPA EvictionRequirements` controllers to avoid interferences.

### [`Extension` Controller](../../pkg/operator/controller/extension)

Gardener relies on extensions to provide various capabilities, such as supporting cloud providers.
This controller automates the management of extensions by managing all necessary resources in the runtime and virtual garden clusters.

#### [`Main` Reconciler](../../pkg/operator/controller/extension/extension)

Currently, this logic handles the following scenarios:
- Extension deployment in the runtime cluster, based on the `RequiredRuntime` condition.
- Extension admission deployment for the virtual garden cluster.
- `ControllerDeployment` and `ControllerRegistration` reconciliation in the virtual garden cluster.

#### [`Required Runtime` Reconciler](../../pkg/operator/controller/extension/required/runtime)

This reconciler reacts on events from `BackupBucket`, `DNSRecord` and `Extension` resources.
Based on these resources and the related `Extension` specification, it is checked if the extension deployment is required in the garden runtime cluster.
The result is then put into the `RequiredRuntime` condition and added to the `Extension` status.

#### [`Required Virtual` Reconciler](../../pkg/operator/controller/extension/required/virtual)

This reconciler reacts on events from `ControllerInstallation` and `Extension` resources.
It updates the `RequiredVirtual` condition of `Extension` objects, based on the existence of related `ControllerInstallation`s and whether they are marked as required.

### [`Access` Controller](../../pkg/operator/controller/virtual/access)

This controller performs actions related to the garden access secret (`gardener` or `gardener-internal`) for the virtual garden cluster.

It extracts the included Kubeconfig, and prepares a dedicated REST config, where the inline bearer token is replaced by a bearer token file.
Any subsequent reconciliation run, mostly triggered by a token replacement, causes the content of the bearer token file to be updated with the token found in the access secret.
At the end, the prepared REST config is passed to the [`Virtual-Cluster-Registrar` controller](#virtual-cluster-registrar-controller).

Together with the adjusted config and the token file, related controllers can continuously run their operations, even after credentials rotation.

### [`Virtual-Cluster-Registrar` Controller](../../pkg/operator/controller/virtual/cluster)

The `Virtual-Cluster-Registrar` controller watches for events on a dedicated channel that is shared with the [`Access` controller](#access-controller).
Once a REST config is sent to the channel, the reconciliation loop picks up the request, creates a [Cluster](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/cluster) object and stores in memory.
This `Cluster` object points to the virtual garden cluster and is used to register further controllers, e.g. [`Gardenlet` controller](#gardenlet-controller).

### [`Gardenlet` Controller](../../pkg/operator/controller/gardenlet)

The `Gardenlet` controller reconciles a `seedmanagement.gardener.cloud/v1alpha1.Gardenlet` resource in case there is no `Seed` yet with the same name.
This is used to allow easy deployments of `gardenlet`s into unmanaged seed clusters.
For a general overview, see [this document](../deployment/deploy_gardenlet.md).

On `Gardenlet` reconciliation, the controller deploys the `gardenlet` to the cluster (either its own, or the one provided via the `.spec.kubeconfigSecretRef`) after downloading the Helm chart specified in `.spec.deployment.helm.ociRepository` and rendering it with the provided values/configuration.

On `Gardenlet` deletion, nothing happens: `gardenlet`s must always be deleted manually (by deleting the `Seed` and, once gone, then the `gardenlet` `Deployment`).

> [!NOTE]
> This controller only takes care of the very first `gardenlet` deployment (since it only reacts when there is no `Seed` resource yet).
> After the `gardenlet` is running, it uses the [self-upgrade mechanism](../deployment/deploy_gardenlet_manually.md#self-upgrades) by watching the `seedmanagement.gardener.cloud/v1alpha1.Gardenlet` (see [this](gardenlet.md#gardenlet-controller) for more details.)
>
> After a successful [`Garden` reconciliation](#main-reconciler), `gardener-operator` also updates the `.spec.deployment.helm.ociRepository.ref` to its own version in all `Gardenlet` resources labeled with `operator.gardener.cloud/auto-update-gardenlet-helm-chart-ref=true`.
> `gardenlet`s then updates themselves.
>
> ⚠️ If you prefer to manage the `Gardenlet` resources via GitOps, Flux, or similar tools, then you should better manage the `.spec.deployment.helm.ociRepository.ref` field yourself and not label the resources as mentioned above (to prevent `gardener-operator` from interfering with your desired state).
> Make sure to apply your `Gardenlet` resources (potentially containing a new version) after the `Garden` resource was successfully reconciled (i.e., after Gardener control plane was successfully rolled out, see [this](../deployment/version_skew_policy.md#supported-component-upgrade-order) for more information.)

## Webhooks

As of today, the `gardener-operator` only has one webhook handler which is now described in more detail.

### Validation

This webhook handler validates `CREATE`/`UPDATE`/`DELETE` operations on `Garden` resources.
Simple validation is performed via [standard CRD validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation).
However, more advanced validation is hard to express via these means and is performed by this webhook handler.

Furthermore, for deletion requests, it is validated that the `Garden` is annotated with a deletion confirmation annotation, namely `confirmation.gardener.cloud/deletion=true`.
Only if this annotation is present it allows the `DELETE` operation to pass.
This prevents users from accidental/undesired deletions.

Another validation is to check that there is only one `Garden` resource at a time.
It prevents creating a second `Garden` when there is already one in the system.

### Defaulting

This webhook handler mutates the `Garden` resource on `CREATE`/`UPDATE`/`DELETE` operations.
Simple defaulting is performed via [standard CRD defaulting](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#defaulting).
However, more advanced defaulting is hard to express via these means and is performed by this webhook handler.

## Using Garden Runtime Cluster As Seed Cluster

In production scenarios, you probably wouldn't use the Kubernetes cluster running `gardener-operator` and the Gardener control plane (called "runtime cluster") as seed cluster at the same time.
However, such setup is technically possible and might simplify certain situations (e.g., development, evaluation, ...).

If the runtime cluster is a seed cluster at the same time, [`gardenlet`'s `Seed` controller](./gardenlet.md#seed-controller) will not manage the components which were already deployed (and reconciled) by `gardener-operator`.
As of today, this applies to:

- `gardener-resource-manager`
- `vpa-{admission-controller,recommender,updater}`
- `etcd-druid`
- `istio` control-plane
- `nginx-ingress-controller`

Those components are so-called "seed system components".
In addition, there are a few observability components:

- `fluent-operator`
- `fluent-bit`
- `vali`
- `plutono`
- `kube-state-metrics`
- `prometheus-operator`

As all of these components are managed by `gardener-operator` in this scenario, the `gardenlet` just skips them.

> ℹ️ There is no need to configure anything - the `gardenlet` will automatically detect when its seed cluster is the garden runtime cluster at the same time.

⚠️ Note that such setup requires that you upgrade the versions of `gardener-operator` and `gardenlet` in lock-step.
Otherwise, you might experience unexpected behaviour or issues with your seed or shoot clusters.

## Credentials Rotation

The credentials rotation works in the same way as it does for `Shoot` resources, i.e. there are `gardener.cloud/operation` annotation values for starting or completing the rotation procedures.

For certificate authorities, `gardener-operator` generates one which is automatically rotated roughly each month (`ca-garden-runtime`) and several CAs which are **NOT** automatically rotated but only on demand.

**🚨 Hence, it is the responsibility of the (human) operator to regularly perform the credentials rotation.**

Please refer to [this document](../usage/shoot-operations/shoot_credentials_rotation.md#gardener-provided-credentials) for more details. As of today, `gardener-operator` only creates the following types of credentials (i.e., some sections of the document don't apply for `Garden`s and can be ignored):

- certificate authorities (and related server and client certificates)
- ETCD encryption key
- observability password for Plutono
- `ServiceAccount` token signing key
- `WorkloadIdentity` token signing key

⚠️ Rotation of static `ServiceAccount` secrets is not supported since the `kube-controller-manager` does not enable the `serviceaccount-token` controller.

When the `ServiceAccount` token signing key rotation is in `Preparing` phase, then `gardener-operator` annotates all `Seed`s with `gardener.cloud/operation=renew-garden-access-secrets`.
This causes `gardenlet` to populate new `ServiceAccount` tokens for the garden cluster to all extensions, which are now signed with the new signing key.
Read more about it [here](../extensions/garden-api-access.md#renewing-all-garden-access-secrets).

Similarly, when the CA certificate rotation is in `Preparing` phase, then `gardener-operator` annotates all `Seed`s with `gardener.cloud/operation=renew-kubeconfig`.
This causes `gardenlet` to request a new client certificate for its garden cluster kubeconfig, which is now signed with the new client CA, and which also contains the new CA bundle for the server certificate verification.
Read more about it [here](gardenlet.md#rotate-certificates-using-bootstrap-kubeconfig).

Also, when the `WorkloadIdentity` token signing key rotation is in `Preparing` phase, then `gardener-operator` annotates all `Seed`s with `gardener.cloud/operation=renew-workload-identity-tokens`.
This causes `gardenlet` to renew all workload identity tokens in the seed cluster with new tokens now signed with the new signing key.

## Migrating an Existing Gardener Landscape to `gardener-operator`

Since `gardener-operator` was only developed in 2023, six years after the Gardener project initiation, most users probably already have an existing Gardener landscape.
The most prominent installation procedure is [garden-setup](https://github.com/gardener/garden-setup), however experience shows that most community members have developed their own tooling for managing the garden cluster and the Gardener control plane components.

> Consequently, providing a general migration guide is not possible since the detailed steps vary heavily based on how the components were set up previously.
> As a result, this section can only highlight the most important caveats and things to know, while the concrete migration steps must be figured out individually based on the existing installation.
>
> Please test your migration procedure thoroughly.
Note that in some cases it can be easier to set up a fresh landscape with `gardener-operator`, restore the ETCD data, switch the DNS records, and issue new credentials for all clients.

Please make sure that you configure all your desired fields in the [`Garden` resource](#garden-resources).

### ETCD

`gardener-operator` leverages `etcd-druid` for managing the `virtual-garden-etcd-main` and `virtual-garden-etcd-events`, similar to how shoot cluster control planes are handled.
The `PersistentVolumeClaim` names differ slightly - for `virtual-garden-etcd-events` it's `virtual-garden-etcd-events-virtual-garden-etcd-events-0`, while for `virtual-garden-etcd-main` it's `main-virtual-garden-etcd-virtual-garden-etcd-main-0`.
The easiest approach for the migration is to make your existing ETCD volumes follow the same naming scheme.
Alternatively, backup your data, let `gardener-operator` take over ETCD, and then [restore](https://github.com/gardener/etcd-backup-restore/blob/master/docs/operations/manual_restoration.md) your data to the new volume.

The backup bucket must be created separately, and its name as well as the respective credentials must be provided via the `Garden` resource in `.spec.virtualCluster.etcd.main.backup`.

### `virtual-garden-kube-apiserver` Deployment

`gardener-operator` deploys a `virtual-garden-kube-apiserver` into the runtime cluster.
This `virtual-garden-kube-apiserver` spans a new cluster, called the virtual cluster.
There are a few certificates and other credentials that should not change during the migration.
You have to prepare the environment accordingly by leveraging the [secret's manager capabilities](../development/secrets_management.md#migrating-existing-secrets-to-secretsmanager).

- The existing Cluster CA `Secret` should be labeled with `secrets-manager-use-data-for-name=ca`.
- The existing Client CA `Secret` should be labeled with `secrets-manager-use-data-for-name=ca-client`.
- The existing Front Proxy CA `Secret` should be labeled with `secrets-manager-use-data-for-name=ca-front-proxy`.
- The existing Service Account Signing Key `Secret` should be labeled with `secrets-manager-use-data-for-name=service-account-key`.
- The existing ETCD Encryption Key `Secret` should be labeled with `secrets-manager-use-data-for-name=kube-apiserver-etcd-encryption-key`.

### `virtual-garden-kube-apiserver` Exposure

The `virtual-garden-kube-apiserver` is exposed via a dedicated `istio-ingressgateway` deployed to namespace `virtual-garden-istio-ingress`.
The `virtual-garden-kube-apiserver` `Service` in the `garden` namespace is only of type `ClusterIP`.
Consequently, DNS records for this API server must target the load balancer IP of the `istio-ingressgateway`.

### Virtual Garden Kubeconfig

`gardener-operator` does not generate any static token or likewise for access to the virtual cluster.
Ideally, human users access it via OIDC only.
Alternatively, you can create an auto-rotated token that you can use for automation like CI/CD pipelines:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: shoot-access-virtual-garden
  namespace: garden
  labels:
    resources.gardener.cloud/purpose: token-requestor
    resources.gardener.cloud/class: shoot
  annotations:
    serviceaccount.resources.gardener.cloud/name: virtual-garden-user
    serviceaccount.resources.gardener.cloud/namespace: kube-system
    serviceaccount.resources.gardener.cloud/token-expiration-duration: 3h
---
apiVersion: v1
kind: Secret
metadata:
  name: managedresource-virtual-garden-access
  namespace: garden
type: Opaque
stringData:
  clusterrolebinding____gardener.cloud.virtual-garden-access.yaml: |
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: gardener.cloud.sap:virtual-garden
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-admin
    subjects:
    - kind: ServiceAccount
      name: virtual-garden-user
      namespace: kube-system
---
apiVersion: resources.gardener.cloud/v1alpha1
kind: ManagedResource
metadata:
  name: virtual-garden-access
  namespace: garden
spec:
  secretRefs:
  - name: managedresource-virtual-garden-access
```

The `shoot-access-virtual-garden` `Secret` will get a `.data.token` field which can be used to authenticate against the virtual garden cluster.
See also [this document](resource-manager.md#tokenrequestor-controller) for more information about the `TokenRequestor`.

### `gardener-apiserver`

Similar to the [`virtual-garden-kube-apiserver`](#virtual-garden-kube-apiserver-deployment), the `gardener-apiserver` also uses a few certificates and other credentials that should not change during the migration.
Again, you have to prepare the environment accordingly by leveraging the [secret's manager capabilities](../development/secrets_management.md#migrating-existing-secrets-to-secretsmanager).

- The existing ETCD Encryption Key `Secret` should be labeled with `secrets-manager-use-data-for-name=gardener-apiserver-etcd-encryption-key`.

Also note that `gardener-operator` manages the `Service` and `Endpoints` resources for the `gardener-apiserver` in the virtual cluster within the `kube-system` namespace (`garden-setup` uses the `garden` namespace).

## Local Development

The easiest setup is using a local [KinD](https://kind.sigs.k8s.io/) cluster and the [Skaffold](https://skaffold.dev/) based approach to deploy and develop the `gardener-operator`.

### Setting Up the KinD Cluster (runtime cluster)

```shell
make kind-operator-up
```

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./example/gardener-local/kind/operator/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config`, since you will need to target this KinD cluster multiple times.
Alternatively, make sure to set your `KUBECONFIG` environment variable to `./example/gardener-local/kind/operator/kubeconfig` for all future steps via `export KUBECONFIG=$PWD/example/gardener-local/kind/operator/kubeconfig`.

All the following steps assume that you are using this kubeconfig.

### Setting Up Gardener Operator

```shell
make operator-up
```

This will first build the base images (which might take a bit if you do it for the first time).
Afterwards, the Gardener Operator resources will be deployed into the cluster.

### Developing Gardener Operator (Optional)

```shell
make operator-dev
```

This is similar to `make operator-up` but additionally starts a [skaffold dev loop](https://skaffold.dev/docs/workflows/dev/).
After the initial deployment, skaffold starts watching source files.
Once it has detected changes, press any key to trigger a new build and deployment of the changed components.

### Debugging Gardener Operator (Optional)

```shell
make operator-debug
```

This is similar to `make gardener-debug` but for Gardener Operator component. Please check [Debugging Gardener](../deployment/getting_started_locally.md#debugging-gardener) for details.

### Creating a `Garden`

In order to create a garden, just run:

```shell
kubectl apply -f example/operator/20-garden.yaml
```

You can wait for the `Garden` to be ready by running:

```shell
./hack/usage/wait-for.sh garden local VirtualGardenAPIServerAvailable VirtualComponentsHealthy
```

Alternatively, you can run `kubectl get garden` and wait for the `RECONCILED` status to reach `True`:

```shell
NAME    LAST OPERATION   RUNTIME   VIRTUAL   API SERVER   OBSERVABILITY   AGE
local   Processing       False     False     False        False           1s
```

(Optional): Instead of creating above `Garden` resource manually, you could execute the e2e tests by running:

```shell
make test-e2e-local-operator
```

#### Accessing the Virtual Garden Cluster

⚠️ Please note that in this setup, the virtual garden cluster is not accessible by default when you download the kubeconfig and try to communicate with it.
The reason is that your host most probably cannot resolve the DNS name of the cluster.
Hence, if you want to access the virtual garden cluster, you have to run the following command which will extend your `/etc/hosts` file with the required information to make the DNS names resolvable:

```shell
cat <<EOF | sudo tee -a /etc/hosts

# Manually created to access local Gardener virtual garden cluster.
# TODO: Remove this again when the virtual garden cluster access is no longer required.
172.18.255.3 api.virtual-garden.local.gardener.cloud
EOF
```

To access the virtual garden, you can acquire a `kubeconfig` by

```shell
kubectl -n garden get secret gardener -o jsonpath={.data.kubeconfig} | base64 -d > /tmp/virtual-garden-kubeconfig
kubectl --kubeconfig /tmp/virtual-garden-kubeconfig get namespaces
```

Note that this kubeconfig uses a token that has validity of `12h` only, hence it might expire and causing you to re-download the kubeconfig.

### Creating Seeds and Shoots

You can also create Seeds and Shoots from your local development setup.
Please see [here](../deployment/getting_started_locally.md#alternative-way-to-set-up-garden-and-seed-leveraging-gardener-operator) for details.

### Deleting the `Garden`

```shell
./hack/usage/delete garden local
```

### Tear Down the Gardener Operator Environment

```shell
make operator-down
make kind-operator-down
```
