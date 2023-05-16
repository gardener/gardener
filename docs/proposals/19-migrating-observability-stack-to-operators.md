---
title: 19 Observability Stack - Migrating to the prometheus-operator and fluent-bit-operator
gep-number: 0019
creation-date: 2022-06-21
status: implementable
authors:
- "@wyb1"
- "@istvanballok"
- "@nickytd"
- "@Kristian-ZH"
reviewers:
- "@rfranzke"
- "@ialidzhikov"
- "@istvanballok"
- "@timebertt"
---

# GEP-19: Observability Stack - Migrating to the prometheus-operator and fluent-bit-operator

## Table of Contents

- [GEP-19: Observability Stack - Migrating to the prometheus-operator and fluent-bit-operator](#gep-19-observability-stack---migrating-to-the-prometheus-operator-and-fluent-bit-operator)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [API](#api)
    - [Prometheus Operator CRDs](#prometheus-operator-crds)
    - [Shoot Monitoring](#shoot-monitoring)
    - [Seed Monitoring](#seed-monitoring)
    - [BYOMC (Bring your own monitoring configuration)](#byomc-bring-your-own-monitoring-configuration)
    - [Grafana Sidecar](#grafana-sidecar)
    - [Fluent-bit Operator CRDs](#fluent-bit-operator-crds)
    - [Fluent-bit Filters and Parsers](#fluent-bit-filters-and-parsers)
    - [BYOLC (Bring Your Own Logging Configuration)](#byolc-bring-your-own-logging-configuration)
    - [Migration](#migration)
      - [Prometheus Operator](#prometheus-operator)
      - [Fluent-bit Operator](#fluent-bit-operator)
  - [Alternatives](#alternatives)

> **Note:** FluentBit Operator is now known as Fluent Operator. The proposal has been left unchanged.

## Summary

As Gardener has grown, the observability configurations have also evolved with it.
Many components must be monitored and the configuration for these components
must also be managed. This poses a challenge because the configuration is
distributed across the Gardener project among different folders and even
different repositories (for example, extensions). While it is not possible to
centralize the configuration, it is possible to improve the developer experience
and improve the general stability of the monitoring and logging. This can be done by
introducing Kubernetes operators such as the [prometheus-operator] for monitoring stack
and [fluent-bit-operator] for the logging stack.
These operators will make it easier for monitoring and logging configurations to be discovered and picked up with the use of the respective custom resources:

1. [Prometheus Custom Resources][prom-crds] provided by the prometheus-operator.
1. [Fluent-bit Custom Resources][fluent-bit-crds] provided by the fluent-bit-operator.

These resources can also be directly referenced in Go and be deployed by their
respective components, instead of creating `.yaml` files in Go, or templating
charts. With the addition of the operators it should is easier to provide
flexible deployment layouts of the components or even introduce new features,
such as Thanos in the case of prometheus-operator

## Motivation

Simplify monitoring and logging updates and extensions with the use of the
[prometheus-operator] and [fluent-bit-operator]. The current extension contract is described
[here][extension-contract]. This document aims to define a new contract.

Make it easier to add new monitoring and logging features and make new changes. For example,
when using the operators components can bring their own monitoring
configuration and specify exactly how they should be monitored without needing
to add this configuration directly into Prometheus. Or components can defined how their logs
shall be parsed or enhanced before being indexed.

Both operators handles validation of the respective configurations. It will
be more difficult to give Prometheus or Fluent-Bit apps an invalid config.

### Goals

- Migrate from the current monitoring stack to the prometheus-operator.

- Improve the monitoring extensibility and improve developer experience when
  adding or editing configuration. This includes the monitoring extensions in
  addition to core Gardener components.

- Provide a clear direction on how monitoring resources should be managed.
  Currently, there are many ways that monitoring configuration is being deployed
  and this should be unified.

- Improve how dashboards are discovered and provisioned for Grafana. Currently,
  all dashboards are appended into a single configmap. This can be an issue if
  the maximum configmap size of 1MiB is ever exceeded.

- Migrate the life-cycle of log shippers to an operator.

- Introduce a stable, declarative API for defining filters and parsers for gardener
  core components and extensions.

- Full match of the current log shippers configurations in gardener

### Non-Goals

- Feature parity between the current solution and the one proposed in this GEP.
  The new stack should provide similar monitoring coverage, but it will be very
  difficult to evaluate if there is feature parity. Perhaps some features will
  be missing, but others may be added.

## Proposal

Today, Gardener provides monitoring and logging for shoot clusters (i.e. system components
and the control plane) and for the seed cluster. The proposal is to migrate
these stacks to use the [prometheus-operator] and [fluent-bit-operator].
The proposal is lined out below:

### API

Use the [API] provided by the [prometheus-operator] and create Go structs.

### Prometheus Operator CRDs

Deploy the [prometheus-operator] and its CRDs. These components can be deployed
via `ManagedResources`. The operator itself and some other components outlined
in the GEP will be deployed in a new namespace called `monitoring`. The CRDs for
the [prometheus-operator] and the operator itself can be found
[here][prom-crd-bundle]. This step is a prerequisite for all other steps.

### Shoot Monitoring

Gardener will create a monitoring stack similar to the current one with the
[prometheus-operator] custom resources.

1. Most of the shoot monitoring is deployed via this
    [chart][shoot-monitoring]. The goal is to create a similar stack, but not
    necessarily with feature parity, using the [prometheus-operator].

    - An example Prometheus object that would be deployed in a shoot's
      control plane.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: Prometheus
    metadata:
      labels:
        app: prometheus
      name: prometheus
      namespace: shoot--project--name
    spec:
      enableAdminAPI: false
      logFormat: logfmt
      logLevel: info
      image: image:tag
      paused: false
      portName: web
      replicas: 1
      retention: 30d
      routePrefix: /
      serviceAccountName: prometheus
      serviceMonitorNamespaceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
          - shoot--project--name
      podMonitorNamespaceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
          - shoot--project--name
      ruleNamespaceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
          - shoot--project--name
      serviceMonitorSelector:
        matchLabels:
          monitoring.gardener.cloud/monitoring-target: shoot-control-plane
      podMonitorSelector:
        matchLabels:
          monitoring.gardener.cloud/monitoring-target: shoot-control-plane
      storage:
        volumeClaimTemplate:
          spec:
            accessModes:
            - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
      version: v2.35.0
    ```

1. Contract between the shoot `Prometheus` and its configuration.

    - `Prometheus` can discover `*Monitors` in different namespaces and also
      by using labels.

    - In some cases, specific configuration is required (e.g. specific
      configuration due to K8s versions). In this case, the configuration will
      also be deployed in the shoot's namespace and Prometheus will also be able
      to discover this configuration.

    - Prometheus must also distinguish between `*Monitors` relevant for shoot
      control plane and shoot targets. This can be done with a
      `serviceMonitorSelector` and `podMonitorSelector` where
      `monitoring.gardener.cloud/monitoring-target=shoot-control-plane`. For a
      `ServiceMonitor` it would look like this:

        ```yaml
        serviceMonitorSelector:
          matchLabels:
            monitoring.gardener.cloud/monitoring-target: shoot-control-plane
        ```

    - In addition to a Prometheus, the configuration must also be created. To
      do this, each `job` in the Prometheus configuration will need to be
      replaced with either a `ServiceMonitor`, `PodMonitor`, or `Probe`. This
      `ServiceMonitor` will be picked up by the Prometheus defined in the
      previous step. This `ServiceMonitor` will scrape any
      service that has the label `app=prometheus` on the port called `metrics`.

        ```yaml
        apiVersion: monitoring.coreos.com/v1
        kind: ServiceMonitor
        metadata:
          labels:
            monitoring.gardener.cloud/monitoring-target: shoot-control-plane
          name: prometheus-job
          namespace: shoot--project--name
        spec:
          endpoints:
          - port: metrics
          selector:
            matchLabels:
              app: prometheus
        ```

1. Prometheus needs to discover targets running in the shoot cluster.
    Normally, this is done by changing the `api_server` field in the config
    ([example][apiserver-example]). This is currently not possible with the
    prometheus-operator, but there is an open [issue][prom-op-issue].

    - Preferred approach: A second Prometheus can be created that is running
      in [agent mode]. This Prometheus can also be deployed/managed by the
      [prometheus-operator]. The agent Prometheus can be configured to use
      the API Server for the shoot cluster and use service discovery in the
      shoot. The metrics can then be written via remote write to the
      "normal" Prometheus or federated. This Prometheus will also discover
      configuration in the same way as the other Prometheus with 1
      difference. Instead of discovering configuration with the label
      `monitoring.gardener.cloud/monitoring-target=shoot-control-plane` it will find configuration
      with the label `monitoring.gardener.cloud/monitoring-target=shoot`.

    - Alternative: Use [additional scrape config]. In this case, the
      Prometheus config snippet is put into a secret and the
      [prometheus-operator] will append it to the config. The downside here is
      that it is only possible to have 1 `additional-scrape-config` per
      Prometheus. This could be an issue if multiple components will need to
      use this.

1. Deploy an optional [alertmanager][shoot-alertmanager] that is deployed
    whenever the end-user [specifies][alerting-doc] alerting.

    - Create an `Alertmanager` resource

    - Create the `AlertmanagerConfig`

1. Health checks - The gardenlet periodically checks the status of the
    Prometheus `StatefulSet` among other components in the shoot care
    controller. The gardenlet knows which component to check based on labels.
    Since the gardenlet is no longer deploying the `StatefulSet` directly and
    rather a `Prometheus` resource, it does not know which labels are
    attached to the Prometheus `StatefulSet`. However, the
    [prometheus-operator] will create `StatefulSets` with the same labelset
    that the `Prometheus` resource has. The gardenlet will be able to
    discover the `StatefulSet` because it knows the labelset of the
    `Prometheus` resource.

### Seed Monitoring

There is a monitoring stack deployed for each seed cluster. A similar setup must
also be provided using the [prometheus-operator]. The steps for this are very
similar to the shoot monitoring.

- Replace the Prometheis and their configuration.

- Replace the [alertmanager][seed-alertmanager] and its configuration.

### BYOMC (Bring your own monitoring configuration)

- In general, components should bring their own monitoring configuration.
  Gardener currently does this for some components such as the
  [gardener-resource-manager]. This configuration is then appended to the
  existing Prometheus configuration. The goal is to replace the inline
  `yaml` with `PodMonitors` and/or `ServiceMonitors` instead.

- If alerting rules or recording rules need to be created for a component,
  it can bring its own `PrometheusRules`.

- The same thing can potentially be done for components such as
  kube-state-metrics which are still currently deployed via the
  [seed-bootstrap].

- If an extension requires monitoring it must bring its own configuration in
  the form of `PodMonitors`, `ServiceMonitors` or `PrometheusRules`.

  - Adding monitoring config to the control plane: In some scenarios
    extensions will add components to the controlplane that need to be
    monitored. An example of this is the provider-aws extension that
    deploys a `cloud-controller-manager`. In the current setup, if an
    extension needs something to be monitored in the control plane, it
    brings its own configmap with Prometheus config. The configmap has the
    label `extensions.gardener.cloud/configuration=monitoring` to specify
    that the config should be appended to the current Prometheus config.
    Below is an example of what this looks like for the cloud controller
    manager.

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      labels:
        extensions.gardener.cloud/configuration: monitoring
      name: cloud-controller-manager-observability-config
      namespace: shoot--project--name
    data:
      alerting_rules: |
        cloud-controller-manager.rules.yaml: |
        groups:
        - name: cloud-controller-manager.rules
          rules:
          - alert: CloudControllerManagerDown
          expr: absent(up{job="cloud-controller-manager"} == 1)
          for: 15m
          labels:
            service: cloud-controller-manager
            severity: critical
            type: seed
            visibility: all
          annotations:
            description: All infrastruture specific operations cannot be completed (e.g. creating loadbalancers or persistent volumes).
            summary: Cloud controller manager is down.
      observedComponents: |
        observedPods:
        - podPrefix: cloud-controller-manager
        isExposedToUser: true
      scrape_config: |
        - job_name: cloud-controller-manager
          scheme: https
          tls_config:
            insecure_skip_verify: true
          authorization:
            type: Bearer
            credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
          honor_labels: false
          kubernetes_sd_configs:
          - role: endpoints
            namespaces:
              names: [shoot--project--name]
          relabel_configs:
          - source_labels:
            - __meta_kubernetes_service_name
            - __meta_kubernetes_endpoint_port_name
            action: keep
            regex: cloud-controller-manager;metrics
          # common metrics
          - action: labelmap
              regex: __meta_kubernetes_service_label_(.+)
          - source_labels: [ __meta_kubernetes_pod_name ]
              target_label: pod
          metric_relabel_configs:
          - source_labels: [ __name__ ]
            regex: ^(rest_client_requests_total|process_max_fds|process_open_fds)$
            action: keep
    ```

- This configmap will be split up into 2 separate resources. One for the
  `alerting_rules` and another for the `scrape_config`. The `alerting_rules`
  should be converted into a `PrometheusRules` object. Since the
  `scrape_config` only has one `job_name` we will only need one
  `ServiceMonitor` or `PodMonitor` for this. The following is an example of
  how this could be done. There are multiple ways to get the same results
  and this is just one example.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: ServiceMonitor
    metadata:
      labels:
        cluster: shoot--project--name
      name: cloud-controller-manager
      namespace: shoot--project--name
    spec:
      endpoints:
      - port: metrics # scrape the service port with name `metrics`
        bearerTokenFile: /var/run/secrets/gardener.cloud/shoot/token/token # could also be replaced with a secret
        metricRelabelings:
        - sourceLabels: [ __name__ ]
          regex: ^(rest_client_requests_total|process_max_fds|process_open_fds)$
          action: keep
      namespaceSelector:
        matchNames:
        - shoot--project--name
      selector:
        matchLabels:
          role: cloud-controller-manager # discover any service with this label
    ```

  ```yaml
  apiVersion: monitoring.coreos.com/v1
  kind: PrometheusRule
  metadata:
    labels:
      cluster: shoot--project--name
    name: cloud-controller-manager-rules
    namespace: shoot--project--name
  spec:
    groups:
    - name: cloud-controller-manager.rules
      rules:
      - alert: CloudControllerManagerDown
        expr: absent(up{job="cloud-controller-manager"} == 1)
        for: 15m
        labels:
          service: cloud-controller-manager
          severity: critical
          type: seed
          visibility: all
        annotations:
          description: All infrastruture specific operations cannot be completed (e.g. creating loadbalancers or persistent volumes).
          summary: Cloud controller manager is down.
  ```

- Adding meta monitoring for the extensions: If the extensions need to be
  scraped for monitoring, the extensions must bring their own [Custom
  Resources][prom-crds].
  - Currently the contract between extensions and gardener is that
    anything that needs to be scraped must have the labels:
    `prometheus.io/scrape=true` and `prometheus.io/port=<port>`. This is
    defined [here][prom-config]. This is something that we can still
    support with a `PodMonitor` that will scrape any pod in a specified
    namespace with these labels.

### Grafana Sidecar

Add a [sidecar][grafana-sidecar] to Grafana that will pickup dashboards and provision them. Each dashboard gets its own configmap.

- Grafana in the control plane

  - Most dashboards provisioned by Grafana are the same for each shoot
    cluster. To avoid unnecessary duplication of configmaps, the dashboards
    could be added once in a single namespace. These "common" dashboards can
    then be discovered by each Grafana and provisioned.

  - In some cases, dashboards are more "specific" because they are related to
    a certain Kubernetes version.

  - Contract between dashboards in configmaps and the Grafana sidecar.

    - Label schema: `monitoring.gardener.cloud/dashboard-{seed,shoot,shoot-user}=true`

    - Each common dashboard will be deployed in the `monitoring` namespace
      as a configmap. If the dashboard should be provisioned by the user
      Grafana in a shoot cluster it should have the label
      `monitoring.gardener.cloud/dashboard-shoot-user=true`. For dashboards
      that should be provisioned by the operator Grafana the label
      `monitoring.gardener.cloud/dashboard-shoot=true` is required.

    - Each specific dashboard will be deployed in the shoot namespace. The
      configmap will use the same label scheme.

    - The Grafana [sidecar][grafana-sidecar] must be [configured][sidecar-configuration] with:

    ```yaml
      env:
      - name: METHOD
        value: WATCH
      - name: LABEL
        value: monitoring.gardener.cloud/dashboard-shoot # monitoring.gardener.cloud/dashboard-shoot-user for user Grafana
      - name: FOLDER
        value: /tmp/dashboards
      - name: NAMESPACE
        value: monitoring,<shoot namespace>
    ```

- Grafana in the seed

  - There is also a Grafana deployed in the seed. This Grafana will be
    configured in a very similar way, except it will discover dashboards
    with a different label.

  - The seed Grafana can discover configmaps labeled with
    `monitoring.gardener.cloud/dashboard-seed`.

  - The sidecar will be configured in a similar way:

  ```yaml
    env:
    - name: METHOD
      value: WATCH
    - name: LABEL
      value: monitoring.gardener.cloud/dashboard-seed
    - name: FOLDER
      value: /tmp/dashboards
    - name: NAMESPACE
      value: monitoring,garden
  ```

- Dashboards can have multiple labels and be provisioned in a seed and/or shoot Grafana.

### Fluent-bit Operator CRDs

The fluent-bit operator oversees two types for resources:

1. `FluentBit` resource defining the properties of the fluent-bit deamonset.
1. `ClusterInputs`, `ClusterOutputs`, `ClusterFilters` and `ClusterParsers` defining the fluent-but app configuration.

1. `FluentBit` custom resource

```yaml
apiVersion: fluentbit.fluent.io/v1alpha2
kind: FluentBit
metadata:
  name: fluent-bit
  namespace: garden
  labels:
    app.kubernetes.io/name: fluent-bit
spec:
  image: kubesphere/fluent-bit:v1.9.9
  fluentBitConfigName: fluent-bit-config
  # workload properties
  annotations: {}
  resources: {}
  nodeSelector: {}
  tolerations: {}
  priorityClassName: ""
  ...
  # fluent-bit configurations
  # container runtime output path
  containerLogRealPath: ""
  # Recommended in case of input tail plugin
  # holds persisted events in fluent-bit supporting re-emitting
  positionDB: {}
```

[fluent-bit-resource] carries the usual Kubernetes workload properties,
such as resource definitions, node selectors, pod/node affinity, and so on. The sole purpose
is to construct the [fluent-bit-daemonset] spec managing the fluent-bit application instances
on the cluster nodes.

The upstream project has been enhanced by Gardener and now it supports adding fluent-bit custom plugins.
The latter is required by Gardener to maintain the custom `ordering` plugin.
In its current state, the fluent-bit operator can be used by Gardener "as is", without the need of forking it.

### Fluent-bit Filters and Parsers

The second major function of the fluent-bit operator is to compile a valid application configuration aggregating declarations supplied by fluent-bit custom resources such as ClusterFitlers, ClusterParsers, and so on, as shown by the example.

1. Fluent-Bit Inputs, Outputs, Filters and Parsers

```yaml
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterFluentBitConfig
metadata:
  name: fluent-bit-config
  labels:
    app.kubernetes.io/name: fluent-bit
spec:
  service:
    parsersFile: parsers.conf
  filterSelector:
    matchLabels:
      fluentbit.fluent.io/enabled: "true"
  parserSelector:
    matchLabels:
      fluentbit.fluent.io/enabled: "true"

---
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterFilter
metadata:
  name: parser
  labels:
    fluentbit.fluent.io/enabled: "true"
spec:
  match: "*"
  filters:
  - parser:
      keyName: log
      parser: my-regex

---
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterParser
metadata:
  name: my-regex
  labels:
    fluentbit.fluent.io/enabled: "true"
spec:
  regex:
    timeKey: time
    timeFormat: "%d/%b/%Y:%H:%M:%S %z"
    types: "code:integer size:integer"
    regex: '^(?<host>[^ ]*) [^ ]* (?<user>[^ ]*) \[(?<time>[^\]]*)\] "(?<method>\S+)(?: +(?<path>[^\"]*?)(?: +\S*)?)?" (?<code>[^ ]*) (?<size>[^ ]*)(?: "(?<referer>[^\"]*)" "(?<agent>[^\"]*)")?$'

```

In this example, we have a fluent bit [filter-plugin] of type parser and the corresponding [regex-parser]. In the Gardener context, these configuration resources are supplied by the core components and extensions.

### BYOLC (Bring Your Own Logging Configuration)

Since fluent-bit uses [input-tail] plugin and reads any container output under `/var/log/pods`, it processes logs outputs of all workloads. In this case, any workload may bring its own set of filters and parsers if needed, using the declarative APIs supported by the operator.

### Migration

#### Prometheus Operator

1. Deploy the [prometheus-operator] and its custom resources.
1. Delete the old monitoring-stack.
1. Configure `Prometheus` to "reuse" the `pv` from the old Prometheus's `pvc`. An init container will be temporarily needed for this migration. This ensures that no data is lost and provides a clean migration.
1. Any extension or monitoring configuration that is not migrated to the [prometheus-operator] right away will be collected and added to an `additionalScrapeConfig`. Once all extensions and components have migrated, this can be dropped.

#### Fluent-bit Operator

1. Add fluent-bit operator CRDs in Gardener.
1. Add `ClusterFilters` and `ClusterParsers` resources in all extensions which are deploying ConfigMap with label: `extensions.gardener.cloud/configuration: logging`
1. Add the Fluent operator in Gardener in place of fluent-bit.

## Alternatives

1. Continue using the current setup.

[additional scrape config]: https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/additional-scrape-config.md
[agent mode]: https://prometheus.io/blog/2021/11/16/agent/
[alerting-doc]: https://github.com/gardener/gardener/blob/master/docs/monitoring/alerting.md#alerting-for-users
[API]: https://github.com/prometheus-operator/prometheus-operator/tree/main/pkg/apis/monitoring
[apiserver-example]: https://github.com/gardener/gardener/blob/0f4d22270927e2aee8b821f858fb76162ccd8a86/charts/seed-monitoring/charts/core/charts/prometheus/templates/config.yaml#L311
[extension-contract]: https://github.com/gardener/gardener/blob/master/docs/extensions/logging-and-monitoring.md
[gardener-resource-manager]: https://github.com/gardener/gardener/blob/eec37223cb90475ec3e023136a7d5ba28ad48f0d/pkg/operation/botanist/component/resourcemanager/monitoring.go
[grafana-sidecar]: https://github.com/kiwigrid/k8s-sidecar
[prom-config]: https://github.com/gardener/gardener/blob/201673c1f8a356a63b21505ca9c7f6efe725bd48/charts/seed-bootstrap/charts/monitoring/templates/config.yaml#L14-L36
[prom-crd-bundle]: https://github.com/prometheus-operator/prometheus-operator/blob/main/bundle.yaml
[prom-crds]: https://github.com/prometheus-operator/prometheus-operator#customresourcedefinitions
[prom-op-issue]: https://github.com/prometheus-operator/prometheus-operator/issues/4828
[prometheus-operator]: https://github.com/prometheus-operator/prometheus-operator
[fluent-bit-operator]: https://github.com/fluent/fluent-operator
[fluent-bit-crds]: https://github.com/fluent/fluent-operator/tree/master/apis/fluentbit/v1alpha2
[fluent-bit-resource]: https://github.com/fluent/fluent-operator/blob/4b984f7c9440119ebacc2c31b8ec376bfd0d63c4/apis/fluentbit/v1alpha2/fluentbit_types.go#L31
[fluent-bit-daemonset]: https://github.com/fluent/fluent-operator/blob/4b984f7c9440119ebacc2c31b8ec376bfd0d63c4/pkg/operator/daemonset.go#L13
[filter-plugin]: https://docs.fluentbit.io/manual/pipeline/filters/parser
[regex-parser]: https://docs.fluentbit.io/manual/pipeline/parsers/regular-expression
[input-tail]: https://docs.fluentbit.io/manual/pipeline/inputs/tail
[seed-alertmanager]: https://github.com/gardener/gardener/blob/0f4d22270927e2aee8b821f858fb76162ccd8a86/charts/seed-bootstrap/templates/alertmanager/alertmanager.yaml
[shoot-alertmanager]: https://github.com/gardener/gardener/tree/master/charts/seed-monitoring/charts/alertmanager
[shoot-monitoring]: https://github.com/gardener/gardener/tree/master/charts/seed-monitoring/charts
[sidecar-configuration]: https://github.com/kiwigrid/k8s-sidecar#configuration-environment-variables
[vpa]: https://github.com/gardener/gardener/tree/master/pkg/operation/botanist/component/vpa
