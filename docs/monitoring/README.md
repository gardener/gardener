# Monitoring

## Roles of the different Prometheus instances

![monitoring](./images/monitoring.png)

### Cache Prometheus

Deployed in the `garden` namespace. Important scrape targets:

- cadvisor
- node-exporter
- kube-state-metrics

**Purpose**: Act as a reverse proxy that supports server-side filtering, which is not supported by Prometheus exporters but by federation. Metrics in this Prometheus are kept for a short amount of time (~1 day) since other Prometheus instances are expected to federate from it and move metrics over. For example, the [shoot Prometheus](#shoot-prometheus) queries this Prometheus to retrieve metrics corresponding to the shoot's control plane. This way, we achieve isolation so that shoot owners are only able to query metrics for their shoots. Please note Prometheus does not support isolation features. Another example is if another Prometheus needs access to cadvisor metrics, which does not support server-side filtering, so it will query this Prometheus instead of the cadvisor. This strategy also reduces load on the kubelets and API Server.

Note some of these Prometheus' metrics have high cardinality (e.g., metrics related to all shoots managed by the seed). Some of these are aggregated with recording rules. These _pre-aggregated_ metrics are scraped by the [aggregate Prometheus](#aggregate-prometheus).

This Prometheus is not used for alerting.

### Aggregate Prometheus

Deployed in the `garden` namespace. Important scrape targets:

- other Prometheus instances
- logging components

**Purpose**: Store pre-aggregated data from the [cache Prometheus](#cache-prometheus) and [shoot Prometheus](#shoot-prometheus). An ingress exposes this Prometheus allowing it to be scraped from another cluster. Such pre-aggregated data is also used for alerting.

### Seed Prometheus

Deployed in the `garden` namespace. Important scrape targets:

- pods in extension namespaces annotated with:
```
prometheus.io/scrape=true
prometheus.io/port=<port>
prometheus.io/name=<name>
```
- cadvisor metrics from pods in the garden and extension namespaces

The job name label will be applied to all metrics from that service.

**Purpose**: Entrypoint for operators when debugging issues with extensions or other garden components.

This Prometheus is not used for alerting.

### Shoot Prometheus

Deployed in the shoot control plane namespace. Important scrape targets:

- control plane components
- shoot nodes (node-exporter)
- blackbox-exporter used to measure [connectivity](connectivity.md)

**Purpose**: Monitor all relevant components belonging to a shoot cluster managed by Gardener. Shoot owners can view the metrics in Plutono dashboards and receive alerts based on these metrics. For alerting internals refer to [this](alerting.md) document.

#### Federate from the shoot Prometheus

Shoot owners that are interested in collecting metrics for their shoot's kube API servers can deploy their own Prometheus and federate metrics from the shoot Prometheus. Scraping the shoot's kube API server directly from within the shoot, while technically possible, will only result in meaningless metrics because the shoot's API server pods are behind a Load Balancer, and it is impossible to control which API server pod is targeted.

The following snippet is a configuration example to federate shoot's kube API server metrics from the shoot Prometheus. The federated metrics will have a `pod` label to distinguish between the different API server pods. The credentials and endpoint for the shoot Prometheus are exposed via the dashboard, or programmatically in the shoot's project namespace in the garden virtual cluster as a secret:<br/>`<shoot-name>.monitoring`.

```yaml
scrape_configs:
- job_name: "prometheus"
  scheme: https
  basic_auth:
    username: admin
    password: <password>
  metrics_path: /federate
  params:
    match[]:
    - '{job="kube-apiserver"}'
  static_configs:
  - targets:
    - p-<project-name>--<shoot-name>.ingress.<domain>
```

## Collect all shoot Prometheus with remote write

An optional collection of all shoot Prometheus metrics to a central Prometheus (or cortex) instance is possible with the `monitoring.shoot` setting in `GardenletConfiguration`:
```
monitoring:
  shoot:
    remoteWrite:
      url: https://remoteWriteUrl # remote write URL
      keep:# metrics that should be forwarded to the external write endpoint. If empty all metrics get forwarded
      - kube_pod_container_info
    externalLabels: # add additional labels to metrics to identify it on the central instance
      additional: label
```

If basic auth is needed it can be set via secret in garden namespace (Gardener API Server). [Example secret](../../example/10-secret-remote-write.yaml)

## Disable Gardener Monitoring

If you wish to disable metric collection for every shoot and roll your own then you can simply set.
```
monitoring:
  shoot:
    enabled: false
```
