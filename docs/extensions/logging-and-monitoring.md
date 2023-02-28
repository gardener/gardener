# Logging and Monitoring for Extensions

Gardener provides an integrated logging and monitoring stack for alerting, monitoring, and troubleshooting of its managed components by operators or end users. For further information how to make use of it in these roles, refer to the corresponding guides for [exploring logs](https://github.com/gardener/logging/tree/master/docs/usage/README.md) and for [monitoring with Grafana](https://grafana.com/docs/grafana/latest/getting-started/getting-started/#all-users).

The components that constitute the logging and monitoring stack are managed by Gardener. By default, it deploys [Prometheus](https://prometheus.io/), [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/), and [Grafana](https://grafana.com/) into the `garden` namespace of all seed clusters. If the logging is enabled in the `gardenlet` configuration (`logging.enabled`), it will deploy [fluent-bit](https://fluentbit.io/) and [Loki](https://grafana.com/oss/loki/) in the `garden` namespace too.

Each shoot namespace hosts managed logging and monitoring components. As part of the shoot reconciliation flow, Gardener deploys a shoot-specific Prometheus, Grafana and, if configured, an Alertmanager into the shoot namespace, next to the other control plane components. If the logging is enabled in the `gardenlet` configuration (`logging.enabled`) and the [shoot purpose](../usage/shoot_purposes.md#behavioral-differences) is not `testing`, it deploys a shoot-specific Loki in the shoot namespace too.

The logging and monitoring stack is extensible by configuration. Gardener extensions can take advantage of that and contribute configurations encoded in `ConfigMap`s for their own, specific dashboards, alerts, log parsers and other supported assets and integrate with it. As with other Gardener resources, they will be continuously reconciled.

This guide is about the roles and extensibility options of the logging and monitoring stack components, and how to integrate extensions with:
- [Monitoring](#monitoring)
- [Logging](#logging)

## Monitoring

The central Prometheus instance in the `garden` namespace fetches metrics and data from all seed cluster nodes and all seed cluster pods.
It uses the [federation](https://prometheus.io/docs/prometheus/latest/federation/) concept to allow the shoot-specific instances to scrape only the metrics for the pods of the control plane they are responsible for.
This mechanism allows to scrape the metrics for the nodes/pods once for the whole cluster, and to have them distributed afterwards.

The shoot-specific metrics are then made available to operators and users in the shoot Grafana, using the shoot Prometheus as data source.

Extension controllers might deploy components as part of their reconciliation next to the shoot's control plane.
Examples for this would be a cloud-controller-manager or CSI controller deployments. Extensions that want to have their managed control plane components integrated with monitoring can contribute their per-shoot configuration for scraping Prometheus metrics, Alertmanager alerts or Grafana dashboards.

### Extensions Monitoring Integration

Before deploying the shoot-specific Prometheus instance, Gardener will read all `ConfigMap`s in the shoot namespace, which are labeled with `extensions.gardener.cloud/configuration=monitoring`.
Such `ConfigMap`s may contain four fields in their `data`:

* `scrape_config`: This field contains Prometheus scrape configuration for the component(s) and metrics that shall be scraped.
* `alerting_rules`: This field contains Alertmanager rules for alerts that shall be raised.
* `dashboard_operators`: This field contains a Grafana dashboard in JSON. Note that the former field name was kept for backwards compatibility but the dashboard is going to be shown both for Gardener operators and for shoot owners because the monitoring stack no longer distinguishes the two roles.
* `dashboard_users`: This field contains a Grafana dashboard in JSON. Note that the former field name was kept for backwards compatibility but the dashboard is going to be shown both for Gardener operators and for shoot owners because the monitoring stack no longer distinguishes the two roles.

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
- Logs are persisted by Loki instances deployed as StatefulSets - one per shoot namespace, if the logging is enabled in the `gardenlet` configuration (`logging.enabled`) and the [shoot purpose](../usage/shoot_purposes.md#behavioral-differences) is not `testing`, and one in the `garden` namespace. The shoot instances store logs from the control plane components hosted there. The `garden` Loki instance is responsible for logs from the rest of the seed namespaces - `kube-system`, `garden`, `extension-*`, and others.
- Fluent-bit DaemonSets deployed on each seed node collect logs from it. A custom plugin takes care to distribute the collected log messages to the Loki instances that they are intended for. This allows to fetch the logs once for the whole cluster, and to distribute them afterwards.
- Grafana is the UI component used to explore monitoring and log data together for easier troubleshooting and in context. Grafana instances are configured to use the corresponding Loki instances, sharing the same namespace as data providers. There is one Grafana Deployment in the `garden` namespace and one Deployment per shoot namespace (exposed to the end users and to the operators).

Logs can be produced from various sources, such as containers or systemd, and in different formats. The fluent-bit design supports configurable [data pipeline](https://docs.fluentbit.io/manual/concepts/data-pipeline) to address that problem. Gardener provides such [configuration](../../charts/seed-bootstrap/charts/fluent-bit/templates/fluent-bit-configmap.yaml) for logs produced by all its core managed components as a `ConfigMap`. Extensions can contribute their own, specific configurations as `ConfigMap`s too. See for example the [logging configuration](https://github.com/gardener/gardener-extension-provider-aws/blob/master/charts/gardener-extension-provider-aws/templates/configmap-logging.yaml) for the Gardener AWS provider extension. The Gardener reconciliation loop watches such resources and updates the fluent-bit agents dynamically.

### Fluent-bit Log Parsers and Filters
To integrate with Gardener logging, extensions can and *should* specify how fluent-bit will handle the logs produced by the managed components that they contribute to Gardener. Normally, that would require to configure a *parser* for the specific logging format, if none of the available is applicable, and a *filter* defining how to apply it. For a complete reference for the configuration options, refer to fluent-bit's [documentation](https://docs.fluentbit.io/manual/).

> **Note:** At the moment, only *parser* and *filter* configurations are supported.

To contribute its own configuration to the fluent-bit agents data pipelines, an extension must provide it as a `ConfigMap` labeled `extensions.gardener.cloud/configuration=logging` and deployed in the seed's `garden` namespace. Unlike the monitoring stack, where configurations are deployed per shoot, here a *single* configuration `ConfigMap` is sufficient and it applies to all fluent-bit agents in the seed. Its `data` field can have the following properties:
- `filter-kubernetes.conf` - configuration for data pipeline [filters](https://docs.fluentbit.io/manual/concepts/data-pipeline/filter)
- `parser.conf` - configuration for data pipeline [parsers](https://docs.fluentbit.io/manual/concepts/data-pipeline/parser)

> **Note:** Take care to provide the correct data pipeline elements in the corresponding data field and not to mix them.

**Example:** Logging configuration for provider-specific (OpenStack) worker controller deploying a `machine-controller-manager` component into a shoot namespace that reuses the `kubeapiserverParser` defined in [fluent-bit-configmap.yaml](../../charts/seed-bootstrap/charts/fluent-bit/templates/fluent-bit-configmap.yaml#L304-L309) to parse the component logs:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gardener-extension-provider-openstack-logging-config
  namespace: garden
  labels:
    extensions.gardener.cloud/configuration: logging
data:
  filter-kubernetes.conf: |
    [FILTER]
        Name                parser
        Match               kubernetes.machine-controller-manager*openstack-machine-controller-manager*
        Key_Name            log
        Parser              kubeapiserverParser
        Reserve_Data        True
```

Further details how to define parsers and use them with examples can be found in the following [guide](../development/log_parsers.md).

### Grafana
The two types of Grafana instances found in a seed cluster are configured to expose logs of different origin in their dashboards:
- Garden Grafana dashboards expose logs from non-shoot namespaces of the seed clusters
  - [Pod Logs](../../charts/seed-bootstrap/dashboards/pod-logs.json)
  - [Extensions](../../charts/seed-bootstrap/dashboards/extensions-dashboard.json)
  - [Systemd Logs](../../charts/seed-bootstrap/dashboards/systemd-logs.json)
- Shoot Grafana dashboards expose logs from the shoot cluster namespace where they belong
  - Kube Apiserver
  - Kube Controller Manager
  - Kube Scheduler
  - Cluster Autoscaler
  - VPA components
  - [Kubernetes Pods](../../charts/seed-monitoring/charts/grafana/dashboards/owners/kubernetes-pods-dashboard.json)

If the type of logs exposed in the Grafana instances needs to be changed, it is necessary to update the corresponding instance dashboard configurations.

## Tips

- Be careful to match exactly the log names that you need for a particular parser in your filters configuration. The regular expression you will supply will match names in the form `kubernetes.pod_name.<metadata>.container_name`. If there are extensions with the same container and pod names, they will all match the same parser in a filter. That may be a desired effect, if they all share the same log format. But it will be a problem if they don't. To solve it, either the pod or container names must be unique, and the regular expression in the filter has to match that unique pattern. A recommended approach is to prefix containers with the extension name and tune the regular expression to match it. For example, using `myextension-container` as container name and a regular expression `kubernetes.mypod.*myextension-container` will guarantee match of the right log name. Make sure that the regular expression does not match more than you expect. For example, `kubernetes.systemd.*systemd.*` will match both `systemd-service` and `systemd-monitor-service`. You will want to be as specific as possible.
- It's a good idea to put the logging configuration into the Helm chart that also deploys the extension *controller*, while the monitoring configuration can be part of the Helm chart/deployment routine that deploys the *component* managed by the controller.

## References and Additional Resources

* [GitHub Issue Describing the Concept](https://github.com/gardener/gardener/issues/1351)
* [Exemplary Implementation (Monitoring) for the GCP Provider](https://github.com/gardener/gardener-extension-provider-gcp/blob/master/charts/internal/seed-controlplane/charts/cloud-controller-manager/templates/configmap-observability.yaml)
* [Exemplary Implementation (Logging) for the OpenStack Provider](https://github.com/gardener/gardener-extension-provider-openstack/blob/master/charts/gardener-extension-provider-openstack/templates/configmap-logging.yaml)
