# High Availability Of Deployed Components

`gardenlet` and extension controllers are deploying components via `Deployment`s, `StatefulSet`s, etc. as part of the shoot control plane, or the seed or shoot system components.

Before we commence, all components should be generally equipped with `PodDisruptionBudget`s with `.spec.maxUnavailable=1`:

```yaml
spec:
  maxUnavailable: 1
  selector:
    matchLabels: ...
```

Some of the above component deployments must be further tuned to improve fault tolerance / resilience of the service. This document outlines what needs to be done to achieve this goal.

## Seed Clusters

The worker nodes of seed clusters can be deployed to one or multiple availability zones.
The `Seed` specification allows to provide the information which zones are available:

```yaml
spec:
  provider:
    region: europe-1
    zones:
    - europe-1a
    - europe-1b
    - europe-1c
```

Independent of the number of zones, seed system components like `gardenlet` or extension controllers themselves, or others like `etcd-druid`, `dependency-watchdog`, etc. should always be running with multiple replicas.

Concretely, all seed system components should respect the following conventions:

- **Replica Counts**

  | Component Type                      | ` < 3` Zones | `>= 3` Zones | Comment                                |
  | ----------------------------------- | ------------ | ------------ | -------------------------------------- |
  | Observability (Monitoring, Logging) | 1            | 1            | Downtimes accepted due to cost reasons |
  | Controllers                         | 2            | 2            | /                                      |
  | (Webhook) Servers                   | 2            | 2            | /                                      |

  Apart from the above, there might be special cases where these rules do not apply, for example:

  - `istio-ingressgateway` is scaled horizontally, hence the above numbers are the minimum values.
  - `nginx-ingress-controller` in the seed cluster is only used to advertise observability endpoints, hence it also only runs with `1` replica at all times. In the future, this component might disappear in favor of the `istio-ingressgateway` anyways.

- **Topology Spread Constraints**

  When the component has `>= 2` replicas ...

  - ... then it should also have a `topologySpreadConstraint` ensuring the replicas are spread over the nodes:

    ```yaml
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: kubernetes.io/hostname
        whenUnsatisfiable: ScheduleAnyway
        matchLabels: ...
    ```

    Hence, the node spread is done on best-effort basis only.

  - ... and the seed cluster has `>= 2` zones, then the component should also have a second `topologySpreadConstraint` ensuring the replicas are spread over the zones:

    ```yaml
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule
        matchLabels: ...
    ```

> According to these conventions, even seed clusters with only one availability zone try to be highly available "as good as possible" by spreading the replicas across multiple nodes.
> Hence, while such seed clusters obviously cannot handle zone outages, they can at least handle node failures.

## Shoot Clusters

The `Shoot` specification allows configuring "high availability" as well as the failure tolerance type for the control plane components, see [this document](../usage/shoot_high_availability.md) for details.

Regarding the seed cluster selection, the only constraint is that shoot clusters with failure tolerance type `zone` are only allowed to run on seed clusters with at least three zones.
All other shoot clusters (non-HA or those with failure tolerance type `node`) can run on seed clusters with any number of zones.

### Control Plane Components

All control plane components should respect the following conventions:

- **Replica Counts**

  | Component Type                      | w/o HA | w/ HA (`node`) | w/ HA (`zone`) | Comment                                |
  | ----------------------------------- | ------ | -------------- | -------------- | -------------------------------------- |
  | Observability (Monitoring, Logging) | 1      | 1              | 1              | Downtimes accepted due to cost reasons |
  | Controllers                         | 1      | 2              | 2              | /                                      |
  | (Webhook) Servers                   | 2      | 2              | 2              | /                                      |

  Apart from the above, there might be special cases where these rules do not apply, for example:

  - `etcd` is a server, though the most critical component of a cluster requiring a quorum to survive failures. Hence, it should have `3` replicas even when the failure tolerance is `node` only.
  - `kube-apiserver` is scaled horizontally, hence the above numbers are the minimum values (even when the shoot cluster is not HA there might be multiple replicas).

- **Topology Spread Constraints**

  When the component has `>= 2` replicas ...

  - ... then it should also have a `topologySpreadConstraint` ensuring the replicas are spread over the nodes:

    ```yaml
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: kubernetes.io/hostname
        whenUnsatisfiable: ScheduleAnyway
        matchLabels: ...
    ```

    Hence, the node spread is done on best-effort basis only.
    
    However, if the shoot cluster has defined a failure tolerance type, the `whenUnsafisfiable` field should be set to `DoNotSchedule`.

  - ... and the failure tolerance type of the shoot cluster is `zone`, then the component should also have a second `topologySpreadConstraint` ensuring the replicas are spread over the zones:

    ```yaml
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule
        matchLabels: ...
    ```

- **Node Affinity**

  The `gardenlet` annotates the shoot namespace in the seed cluster with the `high-availability-config.resources.gardener.cloud/zones` annotation.

  - If the shoot cluster is non-HA or has failure tolerance type `node` then the value will be always exactly one zone (e.g., `high-availability-config.resources.gardener.cloud/zones=europe-1b`).
  - If the shoot cluster has failure tolerance type `zone` then the value will always contain exactly three zones (e.g.,  `high-availability-config.resources.gardener.cloud/zones=europe-1a,europe-1b,europe-1c`).
  
  For backwards-compatibility, this annotation might contain multiple zones for shoot clusters created before `gardener/gardener@v1.60` and not having failure tolerance type `zone`.
  This is because their volumes might already exist in multiple zones, hence pinning them to only one zone would not work.

  Hence, in case this annotation is present, the components should have the following node affinity:

  ```yaml
  spec:
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: topology.kubernetes.io/zone
              operator: In
              values:
              - europe-1a
            # - ...
  ```

  This is to ensure all pods are running in the same (set of) availability zone(s) such that cross-zone network traffic is avoided as much as possible (such traffic is typically charged by the underlying infrastructure provider).

### System Components

The availability of system components is independent of the control plane since they run on the shoot worker nodes while the control plane components run on the seed worker nodes ([ref](../concepts/architecture.md)).
Hence, it only depends on the number of availability zones configured in the shoot worker pools via `.spec.provider.workers[].zones`.
Concretely, the highest number of zones of a worker pool with `systemComponents.allow=true` is considered.

All system components should respect the following conventions:

- **Replica Counts**

  | Component Type                      | `1` or `2` Zones | `>= 3` Zones |
  | ----------------------------------- | ---------------- | ------------ |
  | Controllers                         | 2                | 2            |
  | (Webhook) Servers                   | 2                | 2            |

  Apart from the above, there might be special cases where these rules do not apply, for example:

  - `coredns` is scaled horizontally (today), hence the above numbers are the minimum values (possibly, scaling these components vertically may be more appropriate, but that's unrelated to the HA subject matter).
  - Optional addons like `nginx-ingress` or `kubernetes-dashboard` are only provided on best-effort basis for evaluation purposes, hence they run with `1` replica at all times.

- **Topology Spread Constraints**

  When the component has `>= 2` replicas ...

  - ... then it should also have a `topologySpreadConstraint` ensuring the replicas are spread over the nodes:

    ```yaml
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: kubernetes.io/hostname
        whenUnsatisfiable: ScheduleAnyway
        matchLabels: ...
    ```

    Hence, the node spread is done on best-effort basis only.

  - ... and the cluster has `>= 2` zones, then the component should also have a second `topologySpreadConstraint` ensuring the replicas are spread over the zones:

    ```yaml
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule
        matchLabels: ...
    ```

## Convenient Application Of These Rules

According to above scenarios and conventions, the `replicas`, `topologySpreadConstraints` or `affinity` settings of the deployed components might need to be adapted.

In order to apply those conveniently and easily for developers, Gardener installs a mutating webhook into both seed and shoot clusters which reacts on `Deployment`s and `StatefulSet`s deployed to namespaces with the `high-availability-config.resources.gardener.cloud/consider=true` label set.

The only action developers have to do is labeling their `components` with `high-availability-config.resources.gardener.cloud/type` where the following two values are possible:

- `controller`
- `server`

You can read more about the webhook's internals in [this document](../concepts/resource-manager.md#high-availability-config).

## `gardenlet` Internals

Make sure you have read above document about the webhook internals before continuing reading this section.

### `Seed` Controller

The `gardenlet` performs the following changes on all namespaces running seed system components:

- add label `high-availability-config.resources.gardener.cloud/consider=true`.
- add annotation `high-availability-config.resources.gardener.cloud/zones=<zones>` where `<zones>` is the list provided in `.spec.provider.zones[]` in the `Seed` specification.

Note that the `high-availability-config.resources.gardener.cloud/failure-tolerance-type` annotation is not set, hence the node affinity would never be touched by the webhook.

### `Shoot` Controller

#### Control Plane

The `gardenlet` performs the following changes on the namespace running the shoot control plane components:

- add label `high-availability-config.resources.gardener.cloud/consider=true`. This makes the webhook mutate the replica count and the topology spread constraints.
- add annotation `high-availability-config.resources.gardener.cloud/failure-tolerance-type` with value equal to `.spec.controlPlane.highAvailability.failureTolerance.type` (or `""`, if `.spec.controlPlane.highAvailability=nil`). This makes the webhook mutate the node affinity according to the specified zone(s).
- add annotation `high-availability-config.resources.gardener.cloud/zones=<zones>` where `<zones>` is a ...
  - ... random zone chosen from the `.spec.provider.zones[]` list in the `Seed` specification (always only one zone (even if there are multiple available in the seed cluster)) in case the `Shoot` has no HA setting (i.e., `spec.controlPlane.highAvailability=nil`) or when the `Shoot` has HA setting with failure tolerance type `node`.
  - ... list of three randomly chosen zones from the `.spec.provider.zones[]` list in the `Seed` specification in case the `Shoot` has HA setting with failure tolerance type `zone`.

#### System Components

The `gardenlet` performs the following changes on all namespaces running shoot system components:

- add label `high-availability-config.resources.gardener.cloud/consider=true`. This makes the webhook mutate the replica count and the topology spread constraints.
- add annotation `high-availability-config.resources.gardener.cloud/zones=<zones>` where `<zones>` is the merged list of zones provided in `.zones[]` with `systemComponents.allow=true` for all worker pools in `.spec.provider.workers[]` in the `Shoot` specification.

Note that the `high-availability-config.resources.gardener.cloud/failure-tolerance-type` annotation is not set, hence the node affinity would never be touched by the webhook.
