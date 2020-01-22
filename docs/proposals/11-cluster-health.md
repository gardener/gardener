# A mechanism to co-ordinate the management of Kubernetes cluster health

## Contents

* [A mechanism to co-ordinate the management of Kubernetes cluster health](#a-mechanism-to-co-ordinate-the-management-of-kubernetes-cluster-health)
   * [Contents](#contents)
   * [Terminology](#terminology)
      * [Cluster Health Condition](#cluster-health-condition)
      * [Actionable Cluster Health Condition](#actionable-cluster-health-condition)
   * [Motivation](#motivation)
   * [Goals](#goals)
   * [Non-goals](#non-goals)
   * [Prior Art](#prior-art)
      * [Node Problem Detector](#node-problem-detector)
      * [Cluster Registry](#cluster-registry)
      * [Gardener Shoot Resource](#gardener-shoot-resource)
      * [Gardener Extensions Cluster Resource](#gardener-extensions-cluster-resource)
   * [Proposal](#proposal)
      * [Condition Types of immediate relevance](#condition-types-of-immediate-relevance)
         * [KubeAPIServerReachableExternally](#kubeapiserverreachableexternally)
            * [Possible Actions](#possible-actions)
         * [ETCDReachable](#etcdreachable)
            * [Possible Actions](#possible-actions-1)
         * [KubeAPIServerReachableInternally](#kubeapiserverreachableinternally)
         * [UnhealthyNodesThresholdReached](#unhealthynodesthresholdreached)
            * [Possible Actions](#possible-actions-2)
      * [Condition Types of future relevance](#condition-types-of-future-relevance)
         * [InternalNetworkHealthy](#internalnetworkhealthy)
            * [Possible Actions](#possible-actions-3)
         * [CloudProviderHealthy](#cloudproviderhealthy)
            * [Possible Actions](#possible-actions-4)
         * [CloudProviderQuotaExceeded](#cloudproviderquotaexceeded)
            * [Possible Actions](#possible-actions-5)
         * [ScaleThresholdReached](#scalethresholdreached)
            * [Possible Actions](#possible-actions-6)
   * [Alternatives](#alternatives)
      * [Gardener Shoot resource](#gardener-shoot-resource-1)
      * [Gardener Extensions Cluster Resource](#gardener-extensions-cluster-resource-1)
      * [Custom Cluster Resource](#custom-cluster-resource)
## Terminology

### Cluster Health Condition

Any health condition in a Kubernetes cluster that is significant in the scope of the cluster.

For example, the cluster `kube-apiserver` being not reachable is a cluster health condition.

As a counter-example, a single `Node` being `NotReady` may not be a cluster health condition. However, a significant percentage (say, 20%) of the total `nodes` in the cluster being `NotReady` might be cluster health condition indicative of some cluster-wide (or infrastructure) issue.

### Actionable Cluster Health Condition

Any cluster health condition on which some actions can be taken. Such actions could be automated or manual.
Also, different purposes could be motivating such actions.
Some such motivations could be as mentioned below.
- Corrective - remedy the condition and restore the cluster back to health.
- Alerting - notify or alert the relevant target audience about the cluster health condition.
- Mitigating - mitigate further (potentially cascading) damage to the cluster while the cluster health condition lasts.

## Motivation

While some of the information about the [cluster health conditions](#cluster-health-condition) for a shoot cluster is published in a [standard way](#gardener-shoot-resource) (in the `Shoot` resource), [actionable cluster health conditions](#actionable-cluster-health-condition) (say some threshold percentage of `nodes` are `NotReady` or that the cluster apiserver is not reachable externally but reachable internally from the seed cluster) are more distributed, implicit and isolated among different extensions and controllers.

Where such information is standardized, it is not uniformly convenient for all potential consuming extensions and controllers (both internal and external to Gardener).

For example,
- Gardener

  - The [shoot care controlelr](https://github.com/gardener/gardener/blob/master/pkg/gardenlet/controller/shoot/shoot_care_control.go#L160) maintains some important the shoot cluster health conditions in the Gardener `Shoot` [resource](#gardener-shoot-resource).
  This is a good standard place to publish cluster health conditions for Gardener end-user consumption.
  However, it may not be a good place to consume actionable cluster health conditions for extensions and controllers (especially, in the seed cluster) lest the access to the garden cluster proliferate.

  - The `probe` [sub-command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/probe.go) of the `dependency-watchdog` probes the external and internal endpoints of `kube-apiserver` of all the shoots (hosted on the seed cluster).
  If it finds that any of the external endpoints are not healthy while the internal endpoints are, then it [scales down](https://github.com/gardener/gardener/blob/master/charts/seed-bootstrap/charts/dependency-watchdog/templates/probe-configmap.yaml#L22) the corresponding shoot's `kube-controller-manager` to `0`.
  This is to prevent the `NodeLifecycle` controller of the `kube-controller-manager` from [marking](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/node_lifecycle_controller.go#L860) all the `nodes` and their `pods` in the cluster as `NotReady` because the cause of the `nodes` being not ready has probably nothing to do with the `nodes` themselves.
  It is probably because the `kube-apiserver` endpoint is not reachable externally due to loadbalancer or some other network issue.

  - The `root` [command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/root.go) of the `dependency-watchdog` watches the Kubernetes `endpoints` of `etcd` and `kube-apiserver` of all the shoots (hosted in the seed cluster).
  If any of the endpoints transition from unhealthy to healthy it restarts (deletes) the corresponding [downstream](https://github.com/gardener/gardener/blob/master/charts/seed-bootstrap/charts/dependency-watchdog/templates/endpoint-configmap.yaml#L41) [dependent](https://github.com/gardener/gardener/blob/master/charts/seed-bootstrap/charts/dependency-watchdog/templates/endpoint-configmap.yaml#L16) `pods` but only if they are in `CrashLoopBackoff`.
  This is to speed up the cluster recovery from `etcd` or `kube-apiserver` unavailability.

  - The `etcd` `readinessProbe` [points](https://github.com/gardener/gardener/blob/master/charts/seed-controlplane/charts/etcd/templates/etcd-statefulset.yaml#L43) to the health of the [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) [sidecar](https://github.com/gardener/gardener-extensions/blob/master/pkg/webhook/controlplane/etcd.go#L44).
  This makes the `etcd` of the control-plane of the shoot clusters unavailable if the backup infrastructure is unavailable for any reason (even if the `etcd` container itself is healthy).

  - The `machine-controller-manager` can in principle (it currently doesn't) notice issues in the cloud-provider that other Kubernetes and or Gardener controllers do not notice such as machines not joining in the given timeout period, cloud-provider API requests failing etc.

- Kubernetes

  - The `NodeLifecycle` [controller](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/node_lifecycle_controller.go) maintains the `zoneStates` [internally](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/node_lifecycle_controller.go#L297).
  The computation of the `zoneStates` is partially determined by configurations such as `LargeClusterSizeThreshold` and `UnhealthyZoneThreshold`.
  The action taken by the controller based on the `zoneStates` is also controlled by the configurations such as `NodeEvictionRate` and `SecondaryNodeEvictionRate`.
  All these configurations are supplied [directly](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/config/types.go#L24:6) to the `NodeLifecycle` controller as part of the `kube-controller-manager`.

The distributed nature of such information and action is in itself not a *bad thing*.
But if the information is implicit and isolated within different controllers then there is a hard coupling between the sources of the information and the corrective actions that are taken which **is** a *bad thing*.

## Goals

- As a distributed system, the components that monitor cluster health conditions should be decoupled from the components that take actions when such conditions occur.
This is in the interest of enabling more than one component to consume a particular actionable cluster health conditions as well as to enable a component to consume more than one particular actionable cluster health condition.
The same applies to the components that monitor and maintain such cluster health conditions.
- As a distributed system, it should be possible to dynamically deploy/undeploy (with minimal impact) new components that monitor some (possibly new) cluster health conditions.
This does not mean that each such components _must_ be deployed separately.
It would be perfectly valid to club together multiple such controllers in a controller manager.
The goal is to make it *possible* to deploy/undeploy such components sepaerately, *if required*.
The information to determine such cluster health conditions could be sourced from within the cluster, outside the cluster or some combination.
An example of how a decoupled design can enable such a separation of concerns is the way the [problem daemons](https://github.com/kubernetes/node-problem-detector#problem-daemon) of the [node-problem-detector](#node-problem-detector) can be deployed either as an aggregation of [plugins](https://github.com/kubernetes/node-problem-detector/blob/master/config/custom-plugin-monitor.json) or as separate daemons.
- As a distributed system, it should be possible to dynamically deploy/undeploy (with minimal impact) new components that take some actions when such cluster health conditions occur.
This is the same principle as above applied to the consumers of such actionable cluster health conditions.
- As a controller that monitors/checks for some cluster health conditions, it should be possible to maintain the health condition status in a standard way.
Such as standard way must be conducive for other components to consume such information to take actions.
- As a controller that reacts to some cluster health conditions, it should be possible to watch for the relevant health condition in a standard way and take any possible action.
- As an administrator, I would like to see all the cluster health conditions consolidated in a standard way instead of checking the logs of many different (and changing over time) controllers.
- As an administrator, I would like to be able to post certain cluster health conditions manually for ad hoc reasons and have some remedy system components react to it just like the other automatically monitored cluster health conditions.
- As a Kubernetes-based component, it is desirable to use the Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) for this purpose.
- As an administrator, if determining some cluster health condtiions require some domain knowledge that is difficult to encode in the existing monitoring and alerting mechanism, it would be desirable to be able to receive alerts triggered based on some of the cluster health conditions that is minitored by a component that encodes such domain knowledge.

## Non-goals

- It is **not** a goal to standardize the way to publish and consume any health conditions that do not come under the category of [cluster health condition](#cluster-health-condition).
- It is **not** a goal to replace [existing](https://github.com/gardener/gardener/tree/master/charts/seed-monitoring/charts) monitoring and alerting mechanism to trigger alerts based on cluster health conditions.
However, this proposal might be useful to implement alerts in the cases where some domain knowledge (perhaps even specific to the cloud provider) is required to determine some health condition which might be difficult to encode in the existing monitoring and alerting mechanism.
- It is **not** a goal to define a new external-facing API for the end-users of Gardener to publich and consume cluster health conditions.
The `Shoot` [resource](#gardener-shoot-resource) is perfectly suited for that.
The goal of this proposal is to define what might be crudely called an *internal-facing* API between the components (used by Gardener) to monitor cluster health conditions and the components (used by Gardener) to take any action based on such conditions.
A side effect could be that such an internal-facing API could be replicated to the the external-facing API for consistency (say, either by the same component posting the conditions to two different resources or by some central component doing replication).
- It is **not** a goal to propose mehanisms/approaches to monitory and take action on any generic Kubernetes cluster.
This proposal is focused only on such a(n) mechanism/approach for Gardener shoot clusters.

## Prior Art

### Node Problem Detector

The [node-problem-detector](https://github.com/kubernetes/node-problem-detector) leverages the `status.conditions` of the `nodes` to enable different *problem daemons* to publish different `Node` health conditions.

This approach uses the `Node`'s `status.conditions` as the standard way to publish and consume different types (both [standard](https://github.com/kubernetes/node-problem-detector/blob/master/vendor/k8s.io/api/core/v1/types.go#L4157) and [custom](https://github.com/kubernetes/node-problem-detector/tree/master/config)) of health conditions.

It also separates the components that [monitor](https://github.com/kubernetes/node-problem-detector/tree/master/config) the health conditions from those that react and take the [remedial action](https://github.com/kubernetes/node-problem-detector#remedy-systems).

### Cluster Registry

The [cluster-registry](https://github.com/kubernetes/cluster-registry) is intended as a light-weight way to keep track of a list of Kubernetes clusters along with some metadata.

It defines the [`Cluster`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L30) [`CustomResourceDefinition`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) for that purpose. The resource definition includes a [`status.conditions`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L66) field which can contain an arbitrary number (and [type](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L133)) of [`ClusterCondition`s](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L147).

### Gardener Shoot Resource

The `Shoot` [resource](https://github.com/gardener/gardener/blob/master/pkg/apis/garden/types.go#L776) represents the desired and actual state of a shoot cluster.
It has the `status.conditions` field which is updated by many components including the [shoot care controller](https://github.com/gardener/gardener/blob/master/pkg/gardenlet/controller/shoot/shoot_care_control.go) to [maintain](https://github.com/gardener/gardener/blob/master/pkg/gardenlet/controller/shoot/shoot_care_control.go#L160) shoot cluster health conditions.

This is the standard API to publish such cluster health conditions to the end-users (administrators) for manual or automated consumption.

### Gardener Extensions Cluster Resource

The `Cluster` [resource](https://github.com/gardener/gardener/blob/master/pkg/apis/extensions/v1alpha1/types_cluster.go#L30) is part of the Gardener [extensions API](https://github.com/gardener/gardener/blob/master/pkg/apis/extensions/v1alpha1/) and is used as a point of co-ordination for the different extensions and controllers in the seed cluster to maintain spec, state and the co-ordinate action for each shoot cluster whose contol-plane is hosted in that seed cluster.

This resource is [non-namespaced](https://github.com/gardener/gardener/blob/master/pkg/apis/extensions/v1alpha1/types_cluster.go#L26) and the part to maintain the shoot status conditions in it is [unstructured](https://github.com/gardener/gardener/blob/master/pkg/apis/extensions/v1alpha1/types_cluster.go#L57).

## Proposal

Extending the analogy of [node-problem-detector](#node-problem-detector) to the larger granularity of the Kubernetes cluster, the [`ClusterCondition`s](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L147) in the[`status`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L66) of the [`Cluster`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L30) custom resource (as defined in the [cluster-registry](#cluster-registry)) can be used as the standard way to publish and consume cluster health conditions.

I.e.
- A single `Cluster` resource instance is maintained for each cluster in the shoot control-plane namespace of the seed cluster.
- Many different monitoring components (either dedicated to each shoot cluster or deployed as a common component in the seed clusters) then update the `status.conditions` on the `Cluster` resource instance(s).
- Many different remedy systems watch the `status.conditions` (either dedicated to each shoot cluster or deployed as a common component in the seed cluster) on the `Cluster` resource instance(s) for conditions of (one or more) specific condition types and take remedial action(s). 

With this approach, the monitoring components that update the health conditions can be decoupled from the remedy system components that react to such health conditions and take some remedial action.

However, to be clear,
- One monitoring component can be responsible for updating more than one health condition type.
- More than one remedy system component can take possible remedial action for any given health condition.

Also, with this approach, it would be possible to write both cluster health condition monitoring and consuming components that encode some domain knowledge (both in monitoring and taking action).
Such domain knowledge can include cloud provider specific knowledge.
In such cases, the correspoding components would need to align with [Gardener Extensibility](https://github.com/gardener/gardener/blob/master/docs/extensions/overview.md).

### Condition Types of immediate relevance

This is just a sample set of condition types that might be of immediate relevance. It is not an exhaustive list.
The suggested possible actions for these condition types are also just samples and not an exhaustive list.
This is list contains only the conditions and actions that are already implemented in Gardener in some way.

#### KubeAPIServerReachableExternally

The monitoring (probing) part of the current `probe` [sub-command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/probe.go) of the [dependency-watchdog](ttps://github.com/gardener/dependency-watchdog) can be re-factored into a separate component that updates the conditions with the `KubeAPIServerReachableExternally` condition type.

If there is a way to detect the exact reason for this condition to be unhealthy (for example, the loadbalancer is down), then that information could be published as a separate as a condition [`Reason`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L164).

##### Possible Actions

- As a mitigating action, the `kube-controller-manager` could be scaled down to avoid marking `nodes` and `pods` as `NotReady` unnecessarily.
This could be done by the scaler part of the current `probe` [sub-command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/probe.go) of the [dependency-watchdog](https://github.com/gardener/dependency-watchdog) but re-factored as a separate scaler component which scales some targets based on some configured conditions in the `Cluster` resource. 
- Alternatively, patch the `kube-controller-manager` `Deployment` to disable the `nodelifecycle` controller.

#### ETCDReachable

The `Endpoint` monitoring part of the `root` [command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/root.go) of the [dependency-watchdog](https://github.com/gardener/dependency-watchdog) can be re-factored into a separate component that updates the conditions with the `ETCDReachable` condition type.

##### Possible Actions

- When this condition transitions from status `false` to `true` (i.e. `etcd` transitions from being unreachable to reachable), the dependent `pods` could be deleted but only if they are in `CrashLoopBackoff`. This is to make sure the dependent `pods` recover as fast as possible from an `etcd` down time. This could be done by the restarter part of the current `root` [command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/root.go) of the [dependency-watchdog](https://github.com/gardener/dependency-watchdog) but re-factored as a separate restarter component which restarts (deletes) some targets `pods` based on some configured (via command-line argument, environment variables, config files etc.) conditions in the `Cluster` resource but only if the `pods` are in `CrashLoopBackoff`. 

#### KubeAPIServerReachableInternally

The `KubeAPIServerReachableInternally` condition type can be implemented (both the monitoring and the remedial action parts) similar to the [`ETCDReachable`](#etcdreachable) condition type.

This condition may not be directly usable to take action.
However, it might be useful in avoiding taking some pointless (and potentially harmful) actions.
For example, if `KubeAPIServerReachableInternally` is `false` then we can avoid [scaling down the `kube-controller-manager`](#possible-actions) even if `KubeAPIServerReachableExternally` is `false`.

#### UnhealthyNodesThresholdReached

A component can monitor the fraction of the `NotReady` nodes in a zone and update the conditions with the `UnhealthyNodesThresholdReached` condition type.
The list of affected zones could be maintained in the [`Message`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L168).

##### Possible Actions

- If necessary, `kube-controller-manager` could be scaled down as described [above](#possible-actions).
- Some resource creations (say, `pods`) could be throttled/rate-limited.
- The `machine-controller-manager` could be either scaled down as described [above](#possible-actions) or be enhanced to avoid (or ramp down) deletion of machines from the affected zones (or `machinedeployments`).
- Gardener can defer any disruptive updates until later.

### Condition Types of future relevance

This is just a sample set of condition types that might be of future relevance. It is not an exhaustive list.
The suggested possible actions for these condition types are also just samples and not an exhaustive list.

#### InternalNetworkHealthy

If there is a way to detect problems in the cluster network that is severe enough to affect any significant portion of the cluster then this could be encoded in a component that updates the conditions with the `InternalNetworkHealthy` condition type.

##### Possible Actions

- Some resource creations (say, `pods`) could be throttled/rate-limited.
- Gardener can defer any disruptive updates until later.

#### CloudProviderHealthy

If there is information about some issues reported against the cloud provider, it could be updated in the conditions with the `CloudProviderHealthy` condition type.
If there is more specific information about the issue then more specific condition types can be defined.

Such conditions could be updated manually by the administrators, operators on duty based on publications of such issues by the cloud providers.
If there is a way to automatically scrape such information from well-known locations or APIs then this could even be automated.
Such conditions need not be manually maintained in every single `Cluster` resource instance for each shoot cluster. We can leverage the advantages of operator design-pattern and build on this basic functionality by introducing higher order resources (such as to capture the cloud provider health conditions) and propagate the such conditions to all the relevant `Cluster` resources automatically.

##### Possible Actions

- Controllers such as `kube-controller-manager`, `machine-controller-manager` that manage crucial resources such as `nodes` can be either scaled down as describe [above](#possible-actions) or enhanced to change their behaviour to ramp down their active interventions to the cluster.
- Some resource creations (say, `pods`) could be throttled/rate-limited.
- Gardener can defer any disruptive updates until later.

#### CloudProviderQuotaExceeded

If there is a way to detect that some cloud provider quotas are exceeded this information can be updated in the conditions with the `CloudProviderQuotaExceeded` condition type.
If more specific information is available about which particular quota is exceeded then more specific condition types can be defined.

##### Possible Actions

- Some resource creations (say, `pods` or `LoadBalancer` `services`) could be throttled/rate-limited.

#### ScaleThresholdReached

If the cluster grows beyond a threshold (a threshold beyond which the cluster health may not be reliably guaranteed), this information can be updated in the conditions with `ScaleThresholdReached` condition type.
If more information is available about which aspect of the cluster cannot reliably scale anymore (e.g. the control-plane, nodes, loadbalancers etc.) then more specific condition types can be defined.

##### Possible Actions

- Some resource creations (say, `pods` or `LoadBalancer` `services`) could be throttled/rate-limited.
- Even the request rate to the control-plane could be throttled/rate-limited in an intelligent way to so that critical control-plane and system components get the throughput they need but other non-critical components get throttled/rate-limited.
- Gardener can defer any disruptive updates until later or ramp them down.

## Alternatives

### Gardener Shoot resource

The `status.conditions` part of the Gardener `Shoot` [resource](#gardener-shoot-resource) could be used to publish and take action on the cluster health conditions.

One possible disadvantage to this approach is that the the `Shoot` resource is hosted in the garden cluster, so any extension or controller that wants to take corrective or mitigating action based the cluster health conditions will need access to the garden cluster. Only, the `gardenlet` needs access to the garden cluster and it might be an anti-pattern to proliferate such access to other controllers in the seed cluster.

A second disadvantage is that the `Shoot` resource is specific to the Gardener and some generic extensions or controllers might prefer to avoid adding a direct dependency to the `Gardener` project.

### Gardener Extensions Cluster Resource

One instance of the Gardener extensions `Cluster` [resource](#gardener-extensions-cluster-resource) is maintained per shoot cluster in the seed cluster whose control-plane is hosted in that seed cluster. So, in principle, this can be used for the purpose of publishing and consuming cluster health conditions.

One of the disadvantages of this approach is that `Cluster` resource is [non-namespaced](https://github.com/gardener/gardener/blob/master/pkg/apis/extensions/v1alpha1/types_cluster.go#L26) which might be suitable for centrally deployed extensions and controllers deployed in the `garden` namespaces but may not be suitable for the shoot-specific controllers deployed per shoot control-plane (deployed in the shoot namespace in the seed cluster).

A second disadvantage is that the part where the shoot status conditions can be maintained is [unstructured](https://github.com/gardener/gardener/blob/master/pkg/apis/extensions/v1alpha1/types_cluster.go#L57) making it unsuitable as an API for publishing and consuming actionable cluster health conditions.

A third disadvantage is the same as the `Shoot` [resource](#gardener-shoot-resouce-1) approach, that this would require the extensions and controllers add a direct dependency to the Gardener project which may not always be desirable.

### Custom Cluster Resource

Instead of re-using [any](#proposal) [of](#gardener-shoot-resource) [the](#gardener-extensions-cluster-resource) available resources mentioned above, a new [`CustomResourceDefinition`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) could be defined for this purpose.
Such a custom resource could be along pretty much the same lines as the cluster-registry but could provide an advantage of some more granular domain-specific information to be captured regarding the health conditions such as below (again, this is not an exhaustive list).

- The actual number of healthy and unhealthy nodes in case of the `UnhealthyNodesThresholdReached` condition type.
- The actual structured list of zones affected in the case of the `UnhealthyNodesThresholdReached` condition type instead of embedding it in the [`Message`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L168).
