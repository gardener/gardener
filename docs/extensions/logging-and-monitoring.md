# Logging and Monitoring for Extensions

Gardener provides an integrated logging and monitoring stack for alerting, monitoring, and troubleshooting of its managed components by operators or end users. For further information how to make use of it in these roles, refer to the corresponding guides for [exploring logs](https://github.com/gardener/logging/tree/master/docs/usage/README.md) and for [monitoring with Plutono](https://github.com/credativ/plutono).

The components that constitute the logging and monitoring stack are managed by Gardener. By default, it deploys [Prometheus](https://prometheus.io/) and [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) (managed via [`prometheus-operator`](https://github.com/prometheus-operator/prometheus-operator), and [Plutono](https://github.com/credativ/plutono) into the `garden` namespace of all seed clusters. If the logging is enabled in the `gardenlet` configuration (`logging.enabled`), it will deploy [fluent-operator](https://github.com/fluent/fluent-operator) and [Vali](https://github.com/credativ/plutono) in the `garden` namespace too.

Each shoot namespace hosts managed logging and monitoring components. As part of the shoot reconciliation flow, Gardener deploys a shoot-specific Prometheus, Plutono and, if configured, an Alertmanager into the shoot namespace, next to the other control plane components. If the logging is enabled in the `gardenlet` configuration (`logging.enabled`) and the [shoot purpose](../usage/shoot_purposes.md#behavioral-differences) is not `testing`, it deploys a shoot-specific Vali in the shoot namespace too.

The logging and monitoring stack is extensible by configuration. Gardener extensions can take advantage of that and contribute monitoring configurations encoded in `ConfigMap`s for their own, specific dashboards, alerts and other supported assets and integrate with it. As with other Gardener resources, they will be continuously reconciled. The extensions can also deploy directly fluent-operator custom resources which will be created in the seed cluster and plugged into the fluent-bit instance.

This guide is about the roles and extensibility options of the logging and monitoring stack components, and how to integrate extensions with:
- [Monitoring](#monitoring)
- [Logging](#logging)

## Monitoring

### Cache Prometheus

The central Prometheus instance in the `garden` namespace (called "cache Prometheus") fetches metrics and data from all seed cluster nodes and all seed cluster pods.
It uses the [federation](https://prometheus.io/docs/prometheus/latest/federation/) concept to allow the shoot-specific instances to scrape only the metrics for the pods of the control plane they are responsible for.
This mechanism allows to scrape the metrics for the nodes/pods once for the whole cluster, and to have them distributed afterwards.

Typically, this is not necessary, but in case an extension wants to extend the configuration for this cache Prometheus, they can create the [`prometheus-operator`'s custom resources](https://github.com/prometheus-operator/prometheus-operator?tab=readme-ov-file#customresourcedefinitions) and label them with `prometheus=cache`, for example:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    prometheus: cache
  name: cache-my-component
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

### Shoot Cluster Prometheus

The shoot-specific metrics are then made available to operators and users in the shoot Plutono, using the shoot Prometheus as data source.

Extension controllers might deploy components as part of their reconciliation next to the shoot's control plane.
Examples for this would be a cloud-controller-manager or CSI controller deployments. Extensions that want to have their managed control plane components integrated with monitoring can contribute their per-shoot configuration for scraping Prometheus metrics, Alertmanager alerts or Plutono dashboards.

#### Extensions Monitoring Integration

Before deploying the shoot-specific Prometheus instance, Gardener will read all `ConfigMap`s in the shoot namespace, which are labeled with `extensions.gardener.cloud/configuration=monitoring`.
Such `ConfigMap`s may contain four fields in their `data`:

* `scrape_config`: This field contains Prometheus scrape configuration for the component(s) and metrics that shall be scraped.
* `alerting_rules`: This field contains Alertmanager rules for alerts that shall be raised.
* `dashboard_operators`: This field contains a Plutono dashboard in JSON. Note that the former field name was kept for backwards compatibility but the dashboard is going to be shown both for Gardener operators and for shoot owners because the monitoring stack no longer distinguishes the two roles.
* `dashboard_users`: This field contains a Plutono dashboard in JSON. Note that the former field name was kept for backwards compatibility but the dashboard is going to be shown both for Gardener operators and for shoot owners because the monitoring stack no longer distinguishes the two roles.

**Example:** A `ControlPlane` controller deploying a `cloud-controller-manager` into the shoot namespace wants to integrate monitoring configuration for scraping metrics, alerting rules, dashboards, and logging configuration for exposing logs to the end users.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-controlplane-monitoring-ccm
  namespace: shoot--project--name
  labels:
    extensions.gardener.cloud/configuration: monitoring
data:
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
      - process_max_fds
      - process_open_fds

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
            description: All infrastructure specific operations cannot be completed (e.g. creating load balancers or persistent volumes).
            summary: Cloud controller manager is down.
```

## Logging

In Kubernetes clusters, container logs are non-persistent and do not survive stopped and destroyed containers. Gardener addresses this problem for the components hosted in a seed cluster by introducing its own managed logging solution. It is integrated with the Gardener monitoring stack to have all troubleshooting context in one place.

!["Cluster Logging Topology"](../usage/images/logging-architecture.png "Cluster Logging Topology")

Gardener logging consists of components in three roles - log collectors and forwarders, log persistency and exploration/consumption interfaces. All of them live in the seed clusters in multiple instances:
- Logs are persisted by Vali instances deployed as StatefulSets - one per shoot namespace, if the logging is enabled in the `gardenlet` configuration (`logging.enabled`) and the [shoot purpose](../usage/shoot_purposes.md#behavioral-differences) is not `testing`, and one in the `garden` namespace. The shoot instances store logs from the control plane components hosted there. The `garden` Vali instance is responsible for logs from the rest of the seed namespaces - `kube-system`, `garden`, `extension-*`, and others.
- Fluent-bit DaemonSets deployed by the fluent-operator on each seed node collect logs from it. A custom plugin takes care to distribute the collected log messages to the Vali instances that they are intended for. This allows to fetch the logs once for the whole cluster, and to distribute them afterwards.
- Plutono is the UI component used to explore monitoring and log data together for easier troubleshooting and in context. Plutono instances are configured to use the corresponding Vali instances, sharing the same namespace as data providers. There is one Plutono Deployment in the `garden` namespace and one Deployment per shoot namespace (exposed to the end users and to the operators).

Logs can be produced from various sources, such as containers or systemd, and in different formats. The fluent-bit design supports configurable [data pipeline](https://docs.fluentbit.io/manual/concepts/data-pipeline) to address that problem. Gardener provides such [configuration](../../pkg/component/kubeapiserver/logging.go) for logs produced by all its core managed components as `ClusterFilters` and `ClusterParsers` . Extensions can contribute their own, specific configurations as fluent-operator custom resources too. See for example the [logging configuration](https://github.com/gardener/gardener-extension-provider-aws/blob/master/charts/gardener-extension-provider-aws/templates/clusterfilters-logging.yaml) for the Gardener AWS provider extension.

### Fluent-bit Log Parsers and Filters 
To integrate with Gardener logging, extensions can and *should* specify how fluent-bit will handle the logs produced by the managed components that they contribute to Gardener. Normally, that would require to configure a *parser* for the specific logging format, if none of the available is applicable, and a *filter* defining how to apply it. For a complete reference for the configuration options, refer to fluent-bit's [documentation](https://docs.fluentbit.io/manual/).

To contribute its own configuration to the fluent-bit agents data pipelines, an extension must deploy a `fluent-operator` custom resource labeled with `fluentbit.gardener/type: seed` in the seed cluster.

> **Note:** Take care to provide the correct data pipeline elements in the corresponding fields and not to mix them.

**Example:** Logging configuration for provider-specific `cloud-controller-manager` deployed into shoot namespaces that reuses the `kube-apiserver-parser` defined in [logging.go](../../pkg/component/kubeapiserver/logging.go) to parse the component logs:

```yaml
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterFilter
metadata:
  labels:
    fluentbit.gardener/type: "seed"
  name: cloud-controller-manager-aws-cloud-controller-manager
spec:
  filters:
  - parser:
      keyName: log
      parser: kube-apiserver-parser
      reserveData: true
  match: kubernetes.*cloud-controller-manager*aws-cloud-controller-manager*
```

Further details how to define parsers and use them with examples can be found in the following [guide](../development/log_parsers.md).

### Plutono
The two types of Plutono instances found in a seed cluster are configured to expose logs of different origin in their dashboards:
- Garden Plutono dashboards expose logs from non-shoot namespaces of the seed clusters
  - [Pod Logs](../../pkg/component/plutono/dashboards/seed/pod-logs.json)
  - [Extensions](../../pkg/component/plutono/dashboards/seed/extensions-dashboard.json)
  - [Systemd Logs](../../pkg/component/plutono/dashboards/seed/systemd-logs.json)
- Shoot Plutono dashboards expose logs from the shoot cluster namespace where they belong
  - Kube Apiserver
  - Kube Controller Manager
  - Kube Scheduler
  - Cluster Autoscaler
  - VPA components
  - [Kubernetes Pods](../../pkg/component/plutono/dashboards/shoot/owners/kubernetes-pods-dashboard.json)

If the type of logs exposed in the Plutono instances needs to be changed, it is necessary to update the corresponding instance dashboard configurations.

## Tips

- Be careful to create `ClusterFilters` and `ClusterParsers` with unique names because they are not namespaced. We use `pod_name` for filters with one container and `pod_name--container_name` for pods with multiple containers.
- Be careful to match exactly the log names that you need for a particular parser in your filters configuration. The regular expression you will supply will match names in the form `kubernetes.pod_name.<metadata>.container_name`. If there are extensions with the same container and pod names, they will all match the same parser in a filter. That may be a desired effect, if they all share the same log format. But it will be a problem if they don't. To solve it, either the pod or container names must be unique, and the regular expression in the filter has to match that unique pattern. A recommended approach is to prefix containers with the extension name and tune the regular expression to match it. For example, using `myextension-container` as container name and a regular expression `kubernetes.mypod.*myextension-container` will guarantee match of the right log name. Make sure that the regular expression does not match more than you expect. For example, `kubernetes.systemd.*systemd.*` will match both `systemd-service` and `systemd-monitor-service`. You will want to be as specific as possible.
- It's a good idea to put the logging configuration into the Helm chart that also deploys the extension *controller*, while the monitoring configuration can be part of the Helm chart/deployment routine that deploys the *component* managed by the controller.

## References and Additional Resources

* [GitHub Issue Describing the Concept](https://github.com/gardener/gardener/issues/1351)
* [Exemplary Implementation (Monitoring) for the GCP Provider](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/charts/internal/seed-controlplane/charts/cloud-controller-manager/templates/configmap-observability.yaml)
* [Exemplary Implementation (ClusterFilter) for the AWS Provider](https://github.com/gardener/gardener-extension-provider-aws/blob/master/charts/gardener-extension-provider-aws/templates/clusterfilters-logging.yaml)
* [Exemplary Implementation (ClusterParser) for the Shoot DNS Service](https://github.com/gardener/gardener-extension-shoot-dns-service/blob/master/charts/gardener-extension-shoot-dns-service/templates/clusterparsers-logging.yaml)
