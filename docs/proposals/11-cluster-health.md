# A mechanism to co-ordinate the management of Kubernetes cluster health

## Contents

- [A mechanism to co-ordinate the management of Kubernetes cluster health](#a-mechanism-to-co-ordinate-the-management-of-kubernetes-cluster-health)
   - [Contents](#contents)
   - [Motivation](#motivation)
   - [Goals](#goals)
   - [Non-goals](#non-goals)
   - [Prior Art](#prior-art)
      - [Node Problem Detector](#node-problem-detector)
      - [Cluster Registry](#cluster-registry)
   - [Proposal](#proposal)
      - [Condition Types of immediate relevance](#condition-types-of-immediate-relevance)
         - [KubeAPIServerUnreachableExternally](#kubeapiserverunreachableexternally)
            - [Possible Actions](#possible-actions)
         - [ETCDUnreachable](#etcdunreachable)
            - [Possible Actions](#possible-actions-1)
         - [KubeAPIServerUnreachableInternally](#kubeapiserverunreachableinternally)
         - [UnhealthyNodesThresholdReached](#unhealthynodesthresholdreached)
            - [Possible Actions](#possible-actions-2)
      - [Condition Types of future relevance](#condition-types-of-future-relevance)
         - [UnhealthyInternalNetwork](#unhealthyinternalnetwork)
            - [Possible Actions](#possible-actions-3)
         - [UnhealthyCloudProvider](#unhealthycloudprovider)
            - [Possible Actions](#possible-actions-4)
         - [CloudProviderQuotaExceeded](#cloudproviderquotaexceeded)
            - [Possible Actions](#possible-actions-5)
         - [ScaleThresholdReached](#scalethresholdreached)
            - [Possible Actions](#possible-actions-6)
   - [Alternative](#alternative)

## Motivation

The information about the health of the cluster is distributed, implicit and isolated among different controllers.

For example,
- Kubernetes
  - The `NodeLifecycle` [controller](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/node_lifecycle_controller.go) maintains the `zoneStates` [internally](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/node_lifecycle_controller.go#L297).
  The computation of the `zoneStates` is partially determined by configurations such as `LargeClusterSizeThreshold` and `UnhealthyZoneThreshold`.
  The action taken by the controller based on the `zoneStates` is also controlled by the configurations such as `NodeEvictionRate` and `SecondaryNodeEvictionRate`.
  All these configurations are supplied [directly](https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/nodelifecycle/config/types.go#L24:6) to the `NodeLifecycle` controller as part of the `kube-controller-manager`.
- Gardener
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

The distributed nature of such information and action is in itself not a *bad thing*.
But if the information is implicit and isolated within different controllers then there is a hard coupling between the sources of the information and the corrective actions that are taken which **is** a *bad thing*.

## Goals

- As a distributed system, the components that monitor health conditions that have cluster-wide implication should be decoupled from the components that take remedial action.
- As a distributed system, it should be possible to dynamically deploy/undeploy new components that monitor some (possibly new) health conditions that have cluster-wide implications with minimal impact.
The information to determine the such health conditions could be sourced from within the cluster, outside the cluster or some combination.
- As a distributed system, it should be possible to dynamically deploy/undeploy new components that remedy some health conditions that have cluster-wide implications with minimal impact.
- As a controller that checks for some health conditions that have cluster-wide implications, it should be possible to post the health condition status in a standard way.
- As a controller that reacts to some health conditions that have cluster-wide implications, it should be possible to watch for the relevant health condition in a standard way and take any possible remedial action.
- As an administrator, I would like to see all the health conditions that have cluster-wide implications consolidated in a standard way instead of checking the logs of many different (and changing over time) controllers.
- As an administrator, I would like to receive alerts when some health conditions that have cluster-wide implications occur and are not automatically rectified within some defined time period.
- As an administrator, I would like to be able to post certain health conditions manually for ad hoc reasons and have some remedy system components react to it just like the other automatically monitored health conditions.

## Non-goals

- It is **not** a goal to standardize the way to publish and consume any health conditions that do not have any cluster-wide implications.

## Prior Art

### Node Problem Detector

The [node-problem-detector](https://github.com/kubernetes/node-problem-detector) leverages the `status.conditions` of the `nodes` to enable different *problem daemons* to publish different `Node` health conditions.

This approach uses the `Node`'s `status.conditions` as the standard way to publish and consume different types (both [standard](https://github.com/kubernetes/node-problem-detector/blob/master/vendor/k8s.io/api/core/v1/types.go#L4157) and [custom](https://github.com/kubernetes/node-problem-detector/tree/master/config)) of health conditions.

It also separates the components that [monitor](https://github.com/kubernetes/node-problem-detector/tree/master/config) the health conditions from those that react and take the [remedial action](https://github.com/kubernetes/node-problem-detector#remedy-systems).

### Cluster Registry

The [cluster-registry](https://github.com/kubernetes/cluster-registry) is intended as a light-weight way to keep track of a list of Kubernetes clusters along with some metadata.

It defines the [`Cluster`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L30) [`CustomResourceDefinition`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) for that purpose. The resource definition includes a [`status.conditions`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L66) field which can contain an arbitrary number (and [type](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L133)) of [`ClusterCondition`s](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L147).

## Proposal

Extending the analogy of [node-problem-detector](#node-problem-detector) to the larger granularity of the Kubernetes cluster, the [`ClusterCondition`s](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L147) in the[`status`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L66) of the [`Cluster`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L30) custom resource (as defined in the [cluster-registry](#cluster-registry)) can be used as the standard way to publish and consume health conditions that have cluster-wide implication.

I.e.
- A single `Cluster` resource instance is maintained for each cluster in the shoot control-plane namespace of the seed cluster.
- Many different monitoring components (either dedicated to each shoot cluster or deployed as a common component in the seed clusters) then update the `status.conditions` on the `Cluster` resource instance(s).
- Many different remedy systems watch the `status.conditions` (either dedicated to each shoot cluster or deployed as a common component in the seed cluster) on the `Cluster` resource instance(s) for conditions of (one or more) specific condition types and take remedial action(s). 

With this approach, the monitoring components that update the health conditions can be decoupled from the remedy system components that react to such health conditions and take some remedial action.
To be clear,
- One monitoring component can be responsible for updating more than one health condition type.
- More than one remedy system component can take possible remedial action for any given health condition.

### Condition Types of immediate relevance

This is just a sample set of condition types that might be of immediate relevance. It is not an exhaustive list.
The suggested possible actions for these condition types are also just samples and not an exhaustive list.

#### KubeAPIServerUnreachableExternally

The monitoring (probing) part of the current `probe` [sub-command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/probe.go) of the [dependency-watchdog](ttps://github.com/gardener/dependency-watchdog) can be re-factored into a separate component that updates the conditions with the `KubeAPIServerUnreachableExternally` condition type.

If there is a way to detect the exact reason for this condition (for example, the loadbalancer is down), then that information could be published as a separate condition type.

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty if this condition lasts for some duration.
- The `kube-controller-manager` could be scaled down to avoid marking `nodes` and `pods` as `NotReady` unnecessarily.
This could be done by the scaler part of the current `probe` [sub-command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/probe.go) of the [dependency-watchdog](https://github.com/gardener/dependency-watchdog) but re-factored as a separate scaler component which scales some targets based on some configured conditions in the `Cluster` resource. 

#### ETCDUnreachable

The `Endpoint` monitoring part of the `root` [command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/root.go) of the [dependency-watchdog](https://github.com/gardener/dependency-watchdog) can be re-factored into a separate component that updates the conditions with the `ETCDUnreachable` condition type.

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty if this condition lasts for some duration.
- When this condition transitions from status `true` to `false` (i.e. `etcd` transitions from being unreachable to reachable), the dependent `pods` could be deleted if they are in `CrashLoopBackoff`. This is to make sure the dependent `pods` recover as fast as possible from an `etcd` down time. This could be done by the restarter part of the current `root` [command](https://github.com/gardener/dependency-watchdog/blob/master/cmd/root.go) of the [dependency-watchdog](https://github.com/gardener/dependency-watchdog) but re-factored as a separate restarter component which restarts (deletes) some targets `pods` based on some configured conditions in the `Cluster` resource but only if the `pods` are in `CrashLoopBackoff`. 

#### KubeAPIServerUnreachableInternally

The `KubeAPIServerUnreachableInternally` condition type can be implemented (both the monitoring and the remedial action parts) similar to the [`ETCDUnreachable`](#etcdunreachable) condition type.

#### UnhealthyNodesThresholdReached

A component can monitor the fraction of the `NotReady` nodes in a zone and update the conditions with the `UnhealthyNodesThresholdReached` condition type.
The list of affected zones could be maintained in the [`Message`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L168).

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty if this condition lasts for some duration.
- If necessary, `kube-controller-manager` could be scaled down as described [above](#possible-actions).
- Some resource creations (say, `pods`) could be throttled/rate-limited.
- The `machine-controller-manager` could be either scaled down as described [above](#possible-actions) or be enhanced to avoid (or ramp down) deletion of machines from the affected zones (or `machinedeployments`).
- Gardener can defer any disruptive updates until later.

### Condition Types of future relevance

This is just a sample set of condition types that might be of future relevance. It is not an exhaustive list.
The suggested possible actions for these condition types are also just samples and not an exhaustive list.

#### UnhealthyInternalNetwork

If there is a way to detect problems in the cluster network that is severe enough to affect any significant portion of the cluster then this could be encoded in a component that updates the conditions with the `UnhealthyInternalNetwork` condition type.

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty.
- Some resource creations (say, `pods`) could be throttled/rate-limited.
- Gardener can defer any disruptive updates until later.

#### UnhealthyCloudProvider

If there is information about some issues reported against the cloud provider, it could be updated in the conditions with the `UnhealthyCloudProvider` condition type.
If there is more specific information about the issue then more specific condition types can be defined.

Such conditions could be updated manually by the administrators, operators on duty based on publications of such issues by the cloud providers.
If there is a way to automatically scrape such information from well-known locations or APIs then this could even be automated.

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty.
- Controllers such as `kube-controller-manager`, `machine-controller-manager` that manage crucial resources such as `nodes` can be either scaled down as describe [above](#possible-actions) or enhanced to change their behaviour to ramp down their active interventions to the cluster.
- Some resource creations (say, `pods`) could be throttled/rate-limited.
- Gardener can defer any disruptive updates until later.

#### CloudProviderQuotaExceeded

If there is a way to detect that some cloud provider quotas are exceeded this information can be updated in the conditions with the `CloudProviderQuotaExceeded` condition type.
If more specific information is available about which particular quota is exceeded then more specific condition types can be defined.

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty.
- Some resource creations (say, `pods` or `LoadBalancer` `services`) could be throttled/rate-limited.

#### ScaleThresholdReached

If the cluster grows beyond a threshold (a threshold beyond which the cluster health may not be reliably guaranteed), this information can be updated in the conditions with `ScaleThresholdReached` condition type.
If more information is available about which aspect of the cluster cannot reliably scale anymore (e.g. the control-plane, nodes, loadbalancers etc.) then more specific condition types can be defined.

##### Possible Actions

- Alerts can be raised to the administrator of the cluster as well as the operator/support team on duty.
- Some resource creations (say, `pods` or `LoadBalancer` `services`) could be throttled/rate-limited.
- Even the request rate to the control-plane could be throttled/rate-limited in an intelligent way to so that critical control-plane and system components get the throughput they need but other non-critical components get throttled/rate-limited.
- Gardener can defer any disruptive updates until later or ramp them down.

## Alternative

Instead of re-using the [`Cluster`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L30) resource from the [cluster-registry](https://github.com/kubernetes/cluster-registry), a new [`CustomResourceDefinition`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) could be defined for this purpose.
Such a custom resource could be along pretty much the same lines as the cluster-registry but could provide an advantage of some more granular domain-specific information to be captured regarding the health conditions such as below (again, this is not an exhaustive list).

- The actual number of healthy and unhealthy nodes in case of the `UnhealthyNodesThresholdReached` condition type.
- The actual structured list of zones affected in the case of the `UnhealthyNodesThresholdReached` condition type instead of embedding it in the [`Message`](https://github.com/kubernetes/cluster-registry/blob/master/pkg/apis/clusterregistry/v1alpha1/types.go#L168).
