# Monitoring

## Roles of the different Prometheus instances

![monitoring](./images/monitoring.png)

### Prometheus

Deployed in the `garden` namespace. Important scrape targets:

- cadvisor
- node-exporter
- kube-state-metrics

**Purpose**: Acts as a cache for other Prometheus instances. The metrics are kept for a short amount of time (~2 hours) due to the high cardinality. For example if another Prometheus needs access to cadvisor metrics it will query this Prometheus instead of the cadvisor. This also reduces load on the kubelets and API Server.

Some of the high cardinality metrics are aggregated with recording rules. These _pre-aggregated_ metrics are scraped by the [Aggregate Prometheus](#aggregate-prometheus).

This Prometheus is not used for alerting.

### Aggregate Prometheus

Deployed in the `garden` namespace. Important scrape targets:

- other prometheus instances
- logging components

**Purpose**: Store pre-aggregated data from [prometheus](#prometheus) and [shoot prometheus](#shoot-prometheus). An ingress exposes this Prometheus allowing it to be scraped from another cluster.

### Seed Prometheus

Deployed in the `garden` namespace. Important scrape targets:

- pods in extension namespaces annotated with:
```
prometheus.io/scrape=true
prometheus.io/port=<port>
```
- cadvisor metrics from pods in the garden and extension namespaces

**Purpose**: Entrypoint for operators when debugging issues with extensions or other garden components.

### Shoot Prometheus

Deployed in the shoot control plane namespace. Important scrape targets:

- control plane components
- shoot nodes (node-exporter)
- blackbox-exporter used to measure [connectivity](connectivity.md)

**Purpose**: Monitor all relevant components belonging to a shoot cluster managed by Gardener. Shoot owners can view the metrics in Grafana dashboards and receive [alerts](user_alerts.md) based on these metrics. Gardener operators will receive a different set of [alerts](operator_alerts.md). For alerting internals refer to [this](alerting.md) document.
