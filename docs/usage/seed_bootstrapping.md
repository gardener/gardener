# Seed Bootstrapping

Whenever the gardenlet is responsible for a new `Seed` resource its "seed controller" is being activated.
One part of this controller's reconciliation logic is deploying certain components into the `garden` namespace of the seed cluster itself.
These components are required to spawn and manage control planes for shoot clusters later on.
This document is providing an overview which actions are performed during this bootstrapping phase, and it explains the rationale behind them.

## Dependency Watchdog

The dependency watchdog (abbreviation: DWD) is a component developed separately in the [gardener/dependency-watchdog](https://github.com/gardener/dependency-watchdog) GitHub repository.
Gardener is using it for two purposes:

1. Prevention of melt-down situations when the load balancer used to expose the kube-apiserver of shoot clusters goes down while the kube-apiserver itself is still up and running.
1. Fast recovery times for crash-looping pods when depending pods are again available.

For the sake of separating these concerns, two instances of the DWD are deployed by the seed controller.

### Probe

The `dependency-watchdog-probe` deployment is responsible for above mentioned first point.

The `kube-apiserver` of shoot clusters is exposed via a load balancer, usually with an attached public IP, which serves as the main entry point when it comes to interaction with the shoot cluster (e.g., via `kubectl`).
While end-users are talking to their clusters via this load balancer, other control plane components like the `kube-controller-manager` or `kube-scheduler` run in the same namespace/same cluster, so they can communicate via the in-cluster `Service` directly instead of using the detour with the load balancer.
However, the worker nodes of shoot clusters run in isolated, distinct networks.
This means that the `kubelet`s and `kube-proxy`s also have to talk to the control plane via the load balancer.

The `kube-controller-manager` has a special control loop called [`nodelifecycle`](https://github.com/kubernetes/kubernetes/tree/master/pkg/controller/nodelifecycle) which will set the status of `Node`s to `NotReady` in case the kubelet stops to regularly renew its lease/to send its heartbeat.
This will trigger other self-healing capabilities of Kubernetes, for example, the eviction of pods from such "unready" nodes to healthy nodes.
Similarly, the `cloud-controller-manager` has a control loop that will disconnect load balancers from "unready" nodes, i.e., such workload would no longer be accessible until moved to a healthy node.

While these are awesome Kubernetes features on their own, they have a dangerous drawback when applied in the context of Gardener's architecture:
When the `kube-apiserver` load balancer fails for whatever reason, then the `kubelet`s can't talk to the `kube-apiserver` to renew their lease anymore.
After a minute or so the `kube-controller-manager` will get the impression that all nodes have died and will mark them as `NotReady`.
This will trigger above mentioned eviction as well as detachment of load balancers.
As a result, the customer's workload will go down and become unreachable.

This is exactly the situation that the DWD prevents:
It regularly tries to talk to the `kube-apiserver`s of the shoot clusters, once by using their load balancer, and once by talking via the in-cluster `Service`.
If it detects that the `kube-apiserver` is reachable internally but not externally, it scales down the `kube-controller-manager` to `0`.
This will prevent it from marking the shoot worker nodes as "unready".
As soon as the `kube-apiserver` is reachable externally again, the `kube-controller-manager` will be scaled up to `1` again.

### Endpoint

The `dependency-watchdog-endpoint` deployment is responsible for the above mentioned second point.

Kubernetes is restarting failing pods with an exponentially increasing backoff time.
While this is a great strategy to prevent system overloads, it has the disadvantage that the delay between restarts is increasing up to multiple minutes very fast.

In the Gardener context, we are deploying many components that are depending on other components.
For example, the `kube-apiserver` is depending on a running `etcd`, or the `kube-controller-manager` and `kube-scheduler` are depending on a running `kube-apiserver`.
In case such a "higher-level" component fails for whatever reason, the dependent pods will fail and end-up in crash-loops.
As Kubernetes does not know anything about these hierarchies, it won't recognize that such pods can be restarted faster as soon as their dependents are up and running again.

This is exactly the situation in which the DWD will become active:
If it detects that a certain `Service` is available again (e.g., after the `etcd` was temporarily down while being moved to another seed node), then DWD will restart all crash-looping dependant pods.
These dependant pods are detected via a pre-configured label selector.

As of today, the DWD is configured to restart a crash-looping `kube-apiserver` after `etcd` became available again, or any pod depending on the `kube-apiserver` that has a `gardener.cloud/role=controlplane` label (e.g., `kube-controller-manager`, `kube-scheduler`).
