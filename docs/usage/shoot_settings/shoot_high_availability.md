---
weight: 7
description: Node and Zone Failure Tolerance and possible mitigations for Zone Outages
---
# Highly Available Shoot Control Plane

Shoot resource offers a way to request for a highly available control plane.

## Failure Tolerance Types

A highly available shoot control plane can be setup with either a failure tolerance of `zone` or `node`.

### `Node` Failure Tolerance

The failure tolerance of a `node` will have the following characteristics:

* Control plane components will be spread across different nodes within a single availability zone. There will not be
  more than one replica per node for each control plane component which has more than one replica.
* `Worker pool` should have a minimum of 3 nodes.
* A multi-node etcd (quorum size of 3) will be provisioned, offering zero-downtime capabilities with each member in a
  different node within a single availability zone.

### `Zone` Failure Tolerance

The failure tolerance of a `zone` will have the following characteristics:

* Control plane components will be spread across different availability zones. There will be at least
  one replica per zone for each control plane component which has more than one replica.
* Gardener scheduler will automatically select a `seed` which has a minimum of 3 zones to host the shoot control plane.
* A multi-node etcd (quorum size of 3) will be provisioned, offering zero-downtime capabilities with each member in a
  different zone.

## Shoot Spec

To request for a highly available shoot control plane Gardener provides the following configuration in the shoot spec:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  controlPlane:
    highAvailability:
      failureTolerance:
        type: <node | zone>
```

**Allowed Transitions**

If you already have a shoot cluster with non-HA control plane, then the following upgrades are possible:
* Upgrade of non-HA shoot control plane to HA shoot control plane with `node` failure tolerance.
* Upgrade of non-HA shoot control plane to HA shoot control plane with `zone` failure tolerance. However, it is essential that the `seed` which is currently hosting the shoot control plane should be `multi-zonal`. If it is not, then the request to upgrade will be rejected.

> **Note:** There will be a small downtime during the upgrade, especially for etcd, which will transition from a single node etcd cluster to a multi-node etcd cluster.

**Disallowed Transitions**

If you already have a shoot cluster with HA control plane, then the following transitions are not possible:
* Upgrade of HA shoot control plane from `node` failure tolerance to `zone` failure tolerance is currently not supported, mainly because already existing volumes are bound to the zone they were created in originally.
* Downgrade of HA shoot control plane with `zone` failure tolerance to `node` failure tolerance is currently not supported, mainly because of the same reason as above, that already existing volumes are bound to the respective zones they were created in originally.
* Downgrade of HA shoot control plane with either `node` or `zone` failure tolerance, to a non-HA shoot control plane is currently not supported, mainly because [etcd-druid](https://github.com/gardener/etcd-druid) does not currently support scaling down of a multi-node etcd cluster to a single-node etcd cluster.

## Zone Outage Situation

Implementing highly available software that can tolerate even a zone outage unscathed is no trivial task. You may find our [HA Best Practices](../shoot_high_availability_best_practices.md) helpful to get closer to that goal. In this document, we collected many options and settings for you that also Gardener internally uses to provide a highly available service.

During a zone outage, you may be forced to change your cluster setup on short notice in order to compensate for failures and shortages resulting from the outage.
For instance, if the shoot cluster has worker nodes across three zones where one zone goes down, the computing power from these nodes is also gone during that time.
Changing the worker pool (`shoot.spec.provider.workers[]`) and infrastructure (`shoot.spec.provider.infrastructureConfig`) configuration can eliminate this disbalance, having enough machines in healthy availability zones that can cope with the requests of your applications.

Gardener relies on a sophisticated reconciliation flow with several dependencies for which various flow steps wait for the _readiness_ of prior ones.
During a zone outage, this can block the entire flow, e.g., because all three `etcd` replicas can never be ready when a zone is down, and required changes mentioned above can never be accomplished.
For this, a special one-off annotation `shoot.gardener.cloud/skip-readiness` helps to skip any readiness checks in the flow.

> The `shoot.gardener.cloud/skip-readiness` annotation serves as a last resort if reconciliation is stuck because of important changes during an AZ outage. Use it with caution, only in exceptional cases and after a case-by-case evaluation with your Gardener landscape administrator. If used together with other operations like Kubernetes version upgrades or credential rotation, the annotation may lead to a severe outage of your shoot control plane.
