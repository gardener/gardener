---
title: Migrating from Vali to VictoriaLogs
gep-number: 35
creation-date: 2025-10-13
status: implementable
authors:
- "@nickytd"
- "@rrhubenov"
reviewers:
- "todo"
---

# GEP-35: Migrating from Vali to VictoriaLogs

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)

## Summary

This proposal introduces the deployment of [VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs) which will act as the replacement for [Vali](https://github.com/credativ/vali) in the Control Plane of all Shoot clusters that have the observability stack enabled, as well as in Seed clusters and Garden clusters. After the work that has been done on [GEP-34](https://github.com/gardener/gardener/pull/11861), we are in a favourable position to easily switch our storage layer for the clusters' log signals.

A new operator is introduced that will be deployed to the `garden` namespace of seed & garden clusters. This operator will manage the deployment of the new `VictoriaLogs` instances to the `garden` namespace of seed & garden clusters, as well as to the Control Plane namespace of Shoot clusters via the `VLSingle` CustomResource.

Since we'd like to make the transition from `Vali` to `VictoriaLogs` as painless as possible, this proposal also introduces multiple ways to migrate in already running landscapes without losing data that has accumulated with the existing `Vali` instances.

## Motivation

Since version v2.3.0, [Loki](https://github.com/grafana/loki) has switched from the Apache-2.0 license to a significantly more restrictive AGPLv3 licene. Since then, the Observability stack of Gardener has been using [Vali](https://github.com/credativ/vali) -- a fork of Loki 2.2.1, which is the last official version that maintains the Apache-2.0 license.

However, the fork maintains only security updates, thus leading to no new features or improvements getting integrated. As such, an upgrade is due so that we can benefit from new technologies and optimizations in the world of  log databases.

One such advancement is [OTLP](https://opentelemetry.io/docs/specs/otel/protocol/), discussed in more detail in [GEP-34](https://github.com/gardener/gardener/blob/master/docs/proposals/34-observability2.0-opentelemtry-operator-and-collectors.md).  After careful consideration, we've converged to using [VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs) as the default backend for storing logs.

### Goals

- Deploy the VictoriaOperator & VictoriaLogs statefulset in seed clusters.
- Deploy the VictoriaOperator & VictoriaLogs statefulset in the garden cluster.
- Deploy a VictoriaLogs statefulset to the Control-Plane of new & existing `Shoot` clusters.
- Give an appropriate migration strategy from Vali to VictoriaLogs for existing clusters.

### Non-Goals

- Provide tooling around the migration from `Vali` to `VictoriaLogs`

---

## Proposal

### 1. Deployment of VictoriaLogs on the Garden Cluster

This includes 2 steps:
- Deployment of a new [VictoriaOperator](https://github.com/VictoriaMetrics/operator) component during the `Garden` reconciliation flow in the `Garden` namespace.
- Deployment of a new `VictoriaLogs` statefulset during the Garden reconciliation flow but *after* the `VictoriaOperator` deployment has finished. This component will include a `VLSingle` CustomResource that the operator will reconcile in the `garden` namespace.

Both new components will strongly resemble the already existing pattern that already existing components implement (e.g. OpenTelemetry Operator, Prometheus Operator). That being an implementation of the `Deployer` interface as well as an additional step the the `Seed` reconciliation flow.

#### Access to VictoriaLogs in the `Garden` cluster

Currently, Vali access is behind a service in the `garden` namespace. This pattern will remain and log shippers will only be expected to use the `OTLP` protocol, instead of the `Loki` protocol, via HTTP when inserting logs to VictoriaLogs.

### 2. Deployment of VictoriaLogs on Seed Clusters

Analogously to the deployment in the `Garden` cluster, this includes 2 steps:
- Deployment of a new [VictoriaOperator](https://github.com/VictoriaMetrics/operator) component during the `Seed` reconciliation flow in the `Garden` namespace.
- Deployment of a new `VictoriaLogs` statefulset during the Seed reconciliation flow but *after* the `VictoriaOperator` deployment has finished. This component will include a `VLSingle` CustomResource that the operator will reconcile in the `garden` namespace.

#### Access to VictoriaLogs in the `Seed` cluster

Since the setup is the same as in the `Garden` cluster, see details in 'Access to VictoriaLogs in the `Garden` cluster'.

### 3. Deploy VictoriaLogs in Shoot Control-Planes

During the `Shoot` reconciliation flow, the new `VictoriaLogs` component will be deployed to the Control-Plane namespace of `Shoot` clusters. The operator will manage the resulting `VLSingle` CustomResource and create a `VictoriaLogs` statefulset.

Example VLSingle CR:
```
apiVersion: operator.victoriametrics.com/v1
kind: VLSingle
metadata:
  name: example
  namespace: shoot--local--local
spec:
  retentionPeriod: "12"
  storage:
    resources:
      requests:
        storage: 50Gi
  resources:
    requests:
      memory: 500Mi
      cpu: 500m
    limits:
      memory: 10Gi
      cpu: 5
```

#### Access to VictoriaLogs in the `Shoot` Control-Plane

Access to Vali is exposed via an ingress in the shoot control-plane. This will continue with VictoriaLogs as well. Log shippers will need to be configured via OTLP.

### 4. Migration from Vali to VictoriaLogs

#### Data Migration
During all the replacements of `Vali` we want to ensure that existing clusters do not get disrupted. For this reason, 3 strategies for migration have been considered:
- "One-shot" migration. Reformat all the Vali data into the VictoriaLogs format (or OTLP). Could be done via the Collector since there is no tool that exists to do this OOTB. This would include:
    Pros:
    - Migrations step is quick.
    - No friction during the migration period since there is only one source of logs after the migration.
    Cons:
    - Development of the necessary tooling to do the migration.
    - Risk of data loss if something goes wrong during the migration.
- "Rolling" migration. Get the VictoriaLogs instances up and reroute the logs to them without removing Vali. This would require both of them to be accessible for the whole rotation period of the logs and it would introduce friction for the time being since there would be 2 sources of logs.
    Pros:
    - No risk of data loss.
    - Easy to setup when using an OpenTelemetry Collector since it supports multiple backends.
    Cons:
    - Operators would need to access 2 sources of logs until the rotation period is over. Vali (Plutono) for the logs before the migration point and VictoriaLogs (VictoriaUI) for the logs after the migration point.
- "Dual" migration. Same thing as the "rolling" strategy but logs are sent to both backends simultaneously. This would remove the friction but would incur a higher storage cost due to the replication of the logs on both backends.
    Pros:
    - No risk of data loss.
    - No friction during the migration period since all logs are visible in both backends.
    Cons:
    - Higher storage cost due to the replication of the logs on both backends.

All migration strategies are viable and the choice of which one to use will depend on the specific characteristics of the landscape.
We recommend the "Rolling" migration strategy for most cases since it is the one that balances risk, cost and friction the best.

#### UI Migration
There are 2 main issues that we have with our current observability platform - Plutono:
- Plutono is a fork of Grafana and as such it lags behind the new features in Grafana. Same as Vali, it only receives security updates.
- Plutono does not support VictoriaLogs as a data source

For this reason, our UI observability platform will temporarily be replaced with the built-in web UI that VictoriaLogs ships with.
There is a plan to migrate to [Perses](https://github.com/perses/perses) in the near future, but this is out of scope of this GEP.

---

## Alternatives

### Developing our own operator for the VictoriaLogs deployments

Taking into account the work that would require maintaining such an operator and the negligible benefits that would provide, it has been decided that using the already existing community driven operator would be more beneficial in the long run.

### ClickHouse instead of VictoriaLogs

When choosing a new log database, ClickHouse was one alternative that maintains its Apache-2.0 license, is performant and widely used.
However after researching it as an option, it was found that ClickHouse:
- Is more complicated for operating
- Has an SQL-like query language for querying logs.

In contrast, VictoriaLogs:
- Is simple for deployment and operating
- Has a query language that is more similar to the LogQL that `Vali` uses making the transition easier.

As well as these 2 points, VictoriaLogs stood out with other great qualities, such as:
- Vibrant community support
- Responsiveness of the code owners
- Performance 
