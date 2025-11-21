---
title: Migrating from Vali to VictoriaLogs
gep-number: 35
creation-date: 2025-10-13
status: implementable
authors:
- "@nickytd"
- "@rrhubenov"
reviewers:
- "@plkokanov"
- "@ScheererJ"
- "@vpnachev"
---

# GEP-35: Migrating from Vali to VictoriaLogs

## Table of Contents

- [Terminology](#terminology)
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)

## Terminology

Since this document relies on work that has already been done in [GEP-34](./34-observability2.0-opentelemtry-operator-and-collectors.md), the same terminology applies here as well:
- [OTLP](https://opentelemetry.io/docs/specs/otlp/): OpenTelemetry Protocol
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/): A component that can receive, process, and export telemetry data (logs, metrics, traces).
- [Vali](https://github.com/credativ/vali): A fork of Loki 2.2.1 that is used as the log database in the current Gardener observability stack.
- [Loki](https://github.com/grafana/loki): A log database that is part of the Grafana ecosystem.

## Summary

This proposal introduces the deployment of [VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs) which will act as the replacement for [Vali](https://github.com/credativ/vali) in the control plane of all `Shoot` clusters that have the logging stack enabled, as well as in `Seed` clusters and `Garden` clusters.
After the work that has been done on [GEP-34](https://github.com/gardener/gardener/pull/11861), we are in a favourable position to easily switch our storage layer for the clusters' log signals.

This document fully relies on [GEP-34](./34-observability2.0-opentelemtry-operator-and-collectors.md) having already been implemented. 
For this reason, it's expected that the reader is familiar with the concepts and terminology introduced in that GEP.

A new operator is introduced that will be deployed to the `garden` namespace of `Seed` and `Garden` clusters.
This operator will manage the k8s deployment of the new `VictoriaLogs` instances to the `garden` namespace of `Seed` and `Garden` clusters, as well as to the control plane namespace of `Shoot` clusters via the [`VLSingle` CustomResource](https://github.com/VictoriaMetrics/operator/blob/471ecf6e0ad8839e2e88a6668d962e52e8fd677a/docs/resources/vlsingle.md).

Since we'd like to make the transition from `Vali` to `VictoriaLogs` as painless as possible, this proposal also discusses migration in already running landscapes without losing data that has accumulated with the existing `Vali` instances and gives an implementation plan for that.

## Motivation

Since version v2.3.0, [Loki](https://github.com/grafana/loki) has switched from the Apache-2.0 license to a significantly more restrictive AGPLv3 license.
Since then, the Observability stack of Gardener has been using [Vali](https://github.com/credativ/vali) - a fork of Loki 2.2.1, which is the last official version that maintains the Apache-2.0 license.

However, the fork maintains only security updates, thus leading to no new features or improvements getting integrated.
As such, an upgrade is due so that we can benefit from new technologies and optimizations in the world of log databases.

One such advancement is [OTLP](https://opentelemetry.io/docs/specs/otel/protocol/), discussed in more detail in [GEP-34](https://github.com/gardener/gardener/blob/master/docs/proposals/34-observability2.0-opentelemtry-operator-and-collectors.md).
After careful consideration, we've converged to using [VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs) as the backend for storing logs. See the [Alternatives](#alternatives) section for more details on why this choice was made.

### Goals

- Deploy a `VictoriaOperator` instance in the `garden` namespace of the `Garden` cluster.
- Deploy a `VictoriaLogs` instance in the `garden` namespace of the `Garden` cluster via the `VLSingle` CustomResource.
- Deploy a `VictoriaOperator` instance to the `garden` namespace of the `Seed` clusters.
- Deploy a `VictoriaLogs` instance in the control plane namespaces of new and existing `Shoot` clusters via the `VLSingle` CustomResource.
- Give an appropriate migration strategy from `Vali` to `VictoriaLogs` for existing clusters.

### Non-Goals

- Provide general-purpose tools for migration from `Vali` to `VictoriaLogs`

## Proposal

### 1. Deployment of `VictoriaLogs` on the `Garden` Cluster

This includes 2 steps:
- Deployment of a new [VictoriaOperator](https://github.com/VictoriaMetrics/operator) component during the `Garden` reconciliation flow in the `garden` namespace.
- Deployment of a `VLSingle` CustomResource during the Garden reconciliation flow *after* the `VictoriaOperator` k8s deployment has finished.

The `VLSingle` CustomResource will be managed by the operator and will create a `VictoriaLogs` k8s deployment in the `garden` namespace.

Both new components will implement the already existing pattern that components implement (e.g. OpenTelemetry Operator, Prometheus Operator).
That being an implementation of the `Deployer` interface as well as an additional step the the `Garden` reconciliation flow.

#### Sending Logs to `VictoriaLogs` in the `Garden` Cluster

Currently, `Vali` access is behind a service in the `garden` namespace.
This pattern will remain and log shippers will only be expected to use the `OTLP` protocol, instead of the `Loki` protocol, via HTTP when inserting logs to VictoriaLogs.
In the `Garden` cluster, the only log shippers are the `FluentBit` instances. They will be configured to send logs via `OTLP`.

In other words, the `VictoriaLogs` instance will be exposed via a service in the `garden` namespace.

### 2. Deployment of `VictoriaLogs` on `Seed` Clusters

Analogously to the deployment in the `Garden` cluster, this includes 2 steps:
- Deployment of a new [VictoriaOperator](https://github.com/VictoriaMetrics/operator) component during the `Seed` reconciliation flow in the `garden` namespace.
- Creation of a `VLSingle` CustomResource in the `garden` namespace during the `Seed` reconciliation flow *after* the `VictoriaOperator` k8s deployment has finished.

The `VLSingle` CustomResource will be managed by the operator and will create a `VictoriaLogs` k8s deployment in the `garden` namespace.

#### Sending Logs to `VictoriaLogs` in the `Seed` Cluster

Since the setup is the same as in the `Garden` cluster, see details in [Sending Logs to VictoriaLogs in the `Garden` cluster](#sending-logs-to-victorialogs-in-the-garden-cluster).

### 3. Deployment of `VictoriaLogs` in the `Shoot` Control Plane

During the `Shoot` reconciliation flow, a `VLSingle` CustomResource will be created in the control plane namespace of the `Shoot` cluster.
The `VictoriaOperator` operator deployed in the `Seed` cluster will manage the `VLSingle` CustomResource and create a `VictoriaLogs` k8s deployment.

Example `VLSingle` CR:
```
apiVersion: operator.victoriametrics.com/v1
kind: VLSingle
metadata:
  name: example
  namespace: shoot--local--local
spec:
  storage:
    resources:
      requests:
        storage: 30Gi
  resources:
    requests:
      memory: 100Mi
      cpu: 10m
    limits:
      memory: 300Mi
      cpu: 50m
```

#### Sending Logs to VictoriaLogs in the `Shoot` Control Plane

Currently, `Vali` access is behind a service in the namespace of the `Shoot` control plane.
For external access, there exists an ingress in front of the `OpenTelemetry Collector` in the control plane of `Shoot` clusters that is used to post logs to `Vali`.
See [GEP-34](./34-observability2.0-opentelemtry-operator-and-collectors.md) for more details.

The same setup will continue with `VictoriaLogs`, with the only difference being that the `OpenTelemetry Collector` will be configured to ship logs to `VictoriaLogs` via `OTLP` instead of to `Vali` via the `Loki` protocol.

### 4. Migration from Vali to VictoriaLogs

#### Data Migration
During all the replacements of `Vali` we want to ensure that existing clusters do not get disrupted.
For this reason, 2 strategies for migration have been considered:
- "One-shot" migration.

Reformat all the Vali data into the VictoriaLogs format (or OTLP).
There is no tool that exists to do this out of the box.

  | Pros | Cons |
  |-----|----|
  |Migration step is quick.|  Development of the necessary tooling to do the migration.|
  |No friction during the migration period since there is only one source of logs after the migration.| Risk of data loss if something goes wrong during the migration.|

- "Dual" migration.

`VictoriaLogs` get deployed alongside `Vali` and *both* backends receive logs.

  | Pros | Cons |
  |-----|----|
  |No risk of data loss.| Higher storage cost due to the replication of the logs on both backends.|
  |No friction during the migration period since all logs are visible in both backends. |

Due to the "dual" migration not requiring the development of new tooling and having no risk of data loss, it has been chosen as the migration strategy.
Both `Garden` clusters and `Seed` clusters will rely on this strategy, as well as new and existing `Shoot` clusters.
The deployment alongside `Vali` will be controlled using two feature gates.

##### Dual Migration Implementation Details

To facilitate the "dual" migration, a feature gate `DeployVictoriaLogs` will be introduced for the `gardenlet` and the `gardener-operator` components.
When this feature gate is enabled:
- New and existing clusters will have `VictoriaLogs` deployed *alongside* `Vali`.
- The OpenTelemetry Collector will be reconfigured to ship logs to *both* `Vali` and `VictoriaLogs`.
- `FluentBit` instances will be reconfigured to ship logs to *both* `Vali` and `VictoriaLogs`.

##### Removing Vali after the migration period

Removal of Vali will further rely on another feature gate called `RemoveVali` that will rely on the `VictoriaLogs` already being enabled for the specific component.
This feature gate will be introduced for the `gardenlet` and the `gardener-operator` components.

When this feature gate is enabled, based on whether the retention period has passed since `VictoriaLogs` has been deployed, `Vali` will be removed from the cluster. 
Making the migration into 2 feature gates allows us more safety if any issue occurs, by allowing us to rollback only one of the feature gates.
First the `DeployVictoriaLogs` feature gate can mature, then the `RemoveVali` feature gate, at which point both can simultaneously be removed.

##### Resource utilization during the migration period

During the migration period, `Vali` and `VictoriaLogs` will be running alongside each other.
This introduces more resource usage on clusters in the form of:
- CPU Utilization
- Memory Utilization
- Storage Utilization (in the form of k8s PersistentVolumes and PersistentVolumeClaims)

However, since CPU Utilization and Memory Utilization have been seen to be negligant, the main concern is the Storage Utilization.
More precisely, `Seed` and `Garden` clusters will see an increase in volume usage by exactly the number of already existing `Vali` volumes.
This concern is primarily for `Seed` clusters that are hosting a large number of `Shoot` clusters.

Thus, it is advised that the `DeployVictoriaLogs` feature gate is enabled intelligently - starting with less critical clusters, batching the number of `Seed` clusters that have the feature gate enabled, distributing the feature gate equally among regions so that no single region gets overloaded, etc.

#### UI Migration

There are 2 main issues that we have with our current observability platform - Plutono:
- Plutono is a fork of Grafana and as such it lags behind the new features in Grafana.
  Same as Vali, it only receives security updates.
- Plutono does not support VictoriaLogs as a data source

For this reason, our UI for logs will temporarily be replaced with the built-in web UI that VictoriaLogs ships with.
Plutono will remain as the dashboard for visualising metrics and dashboards.
[There is a plan](https://github.com/gardener/monitoring/issues/52) to migrate to [Perses](https://github.com/perses/perses) in the near future, which will unify the interfaces again, but this is out of scope of this GEP.

---

## Alternatives

### Developing our own operator for the VictoriaLogs deployments

Taking into account the work that would require maintaining such an operator and the negligible benefits that would provide, it has been decided that using the already existing community driven operator would be more beneficial in the long run.

### ClickHouse instead of VictoriaLogs

When choosing a new log database, [ClickHouse](https://github.com/ClickHouse/ClickHouse) was the only mainstream alternative that maintains its Apache-2.0 license, is performant and widely used.
However after researching it as an option, it was found that ClickHouse:
- Is more complicated for operating
- Has an SQL-like query language for querying logs.

In contrast, VictoriaLogs:
- Is simple for deployment and operating
- Has a query language that is more similar to the LogQL that `Vali` uses making the transition easier.

Furthermore, VictoriaLogs stood out with other great qualities, such as:
- Vibrant community support
- Responsiveness of the code owners
- Performance 
