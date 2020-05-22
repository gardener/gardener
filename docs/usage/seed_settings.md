# Settings for `Seed`s

The `Seed` resource offers a few settings that are used to control the behaviour of certain Gardener components.
This document provides an overview over the available settings:

## Reserve Excess Capacity

If the excess capacity reservation is enabled then the Gardenlet will deploy a special `Deployment` into the `garden` namespace of the seed cluster.
This `Deployment`'s pod template has only one container, the `pause` container, which simply runs in an infinite loop.
The priority of the deployment is very low, so any other pod will preempt these `pause` pods.
This is especially useful if new shoot control planes are created in the seed.
In case the seed cluster runs at its capacity then there is no waiting time required during the scale-up.
Instead, the low-priority `pause` pods will be preempted and allow newly created shoot control plane pods to be scheduled fast.
In the meantime, the cluster-autoscaler will trigger the scale-up because the preempted `pause` pods want to run again.
However, this delay doesn't affect the important shoot control plane pods which will improve the user experience.

It can be enabled/disabled via the `.spec.settings.excessCapacityReservation.enabled` field.
It defaults to `true`.  

## Scheduling

By default, the Gardener Scheduler will consider all seed clusters when a new shoot cluster shall be created.
However, administrators/operators might want to exclude some of them from being considered by the scheduler.
Therefore, seed clusters can be marked as "invisible".
In this case, the scheduler simply ignores them as if they wouldn't exist.
Shoots can still use the invisible seed but only by explicitly specifying the name in their `.spec.seedName` field.

Seed clusters can be marked visible/invisible via the `.spec.settings.scheduling.visible` field.
It defaults to `true`.  

## Shoot DNS

Generally, the Gardenlet creates a few DNS records during the creation/reconciliation of a shoot cluster (see [here](configuration.md)).
However, some infrastructures don't need/want this behaviour.
Instead, they want to directly use the IP addresses/hostnames of load balancers.
Another use-case is a local development setup where DNS is not needed for simplicity reasons.

By setting the `.spec.settings.shootDNS.enabled` field this behavior can be controlled.

ℹ️ In previous Gardener versions (< 1.5) these settings were controlled via taint keys (`seed.gardener.cloud/{disable-capacity-reservation,disable-dns,invisible}`).
The taint keys are still supported for backwards compatibility but deprecated and will be removed in a future version.
The rationale behind it is the implementation of tolerations similar to Kubernetes tolerations.
More information about it can be found in [#2193](https://github.com/gardener/gardener/issues/2193).
