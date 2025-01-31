# Checklist For Adding New Components

Adding new components that run in the garden, seed, or shoot cluster is theoretically quite simple - we just need a `Deployment` (or other similar workload resource), the respective container image, and maybe a bit of configuration.
In practice, however, there are a couple of things to keep in mind in order to make the deployment production-ready.
This document provides a checklist for them that you can walk through.

## General

1. **Avoid usage of Helm charts** ([example](https://github.com/gardener/gardener/tree/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver))

   Nowadays, we use [Golang components](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/interfaces.go) instead of Helm charts for deploying components to a cluster.
   Please find a typical structure of such components in the provided [metrics_server.go](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L80-L97) file (configuration values are typically managed in a `Values` structure).
   There are a few exceptions (e.g., [Istio](https://github.com/gardener/gardener/tree/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/istio)) still using charts, however the default should be using a Golang-based implementation.
   For the exceptional cases, use Golang's [embed](https://pkg.go.dev/embed) package to embed the Helm chart directory ([example 1](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/istio/istiod.go#L59-L60), [example 2](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/istio/istiod.go#L297-L313)).

2. **Choose the proper deployment way** ([example 1 (direct application w/ client)](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L212-L232), [example 2 (using `ManagedResource`)](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L447-L488), [example 3 (mixed scenario)](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubestatemetrics/kube_state_metrics.go#L120))

   For historic reasons, resources related to shoot control plane components are applied directly with the client.
   All other resources (seed or shoot system components) are deployed via `gardener-resource-manager`'s [Resource controller](../concepts/resource-manager.md#managedresource-controller) (`ManagedResource`s) since it performs health checks out-of-the-box and has a lot of other features (see its documentation for more information).
   Components that can run as both seed system component or shoot control plane component (e.g., VPA or `kube-state-metrics`) can make use of [these utility functions](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/resourceconfig.go).

3. **Use unique `ConfigMap`s/`Secret`s** ([example 1](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L183-L190), [example 2](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L353))

   [Unique `ConfigMap`s/`Secret`s](https://kubernetes.io/docs/concepts/configuration/configmap/#configmap-immutable) are immutable for modification and have a unique name.
   This has a couple of benefits, e.g. the `kubelet` doesn't watch these resources, and it is always clear which resource contains which data since it cannot be changed.
   As a consequence, unique/immutable `ConfigMap`s/`Secret` are superior to checksum annotations on the pod templates.
   Stale/unused `ConfigMap`s/`Secret`s are garbage-collected by `gardener-resource-manager`'s [GarbageCollector](../concepts/resource-manager.md#garbage-collector-for-immutable-configmapssecrets).
   There are utility functions (see examples above) for using unique `ConfigMap`s/`Secret`s in Golang components.
   It is essential to inject the annotations into the workload resource to make the garbage-collection work.\
   Note that some `ConfigMap`s/`Secret`s should not be unique (e.g., those containing monitoring or logging configuration).
   The reason is that the old revision stays in the cluster even if unused until the garbage-collector acts.
   During this time, they would be wrongly aggregated to the full configuration.

4. **Manage certificates/secrets via [secrets manager](https://github.com/gardener/gardener/tree/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/utils/secrets/manager)** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L100-L109))

   You should use the [secrets manager](secrets_management.md) for the management of any kind of credentials.
   This makes sure that credentials rotation works out-of-the-box without you requiring to think about it.
   Generally, do not use client certificates (see the [Security section](#security)).

5. **Consider hibernation when calculating replica count** ([example](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/kubescheduler.go#L36))

   Shoot clusters can be [hibernated](../usage/shoot/shoot_hibernate.md) meaning that all control plane components in the shoot namespace in the seed cluster are scaled down to zero and all worker nodes are terminated.
   If your component runs in the seed cluster then you have to consider this case and provide the proper replica count.
   There is a utility function available (see example).

6. **Ensure task dependencies are as precise as possible in shoot flows** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/shoot/shoot/reconciler_reconcile.go#L508-L512), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/shoot/shoot/reconciler_delete.go#L368-L372))

   Only define the minimum of needed dependency tasks in the [shoot reconciliation/deletion flows](https://github.com/gardener/gardener/tree/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/shoot/shoot).

7. **Handle shoot system components**

   Shoot system components deployed by `gardener-resource-manager` are labelled with `resource.gardener.cloud/managed-by: gardener`. This makes Gardener adding required label selectors and tolerations so that non-`DaemonSet` managed `Pod`s will exclusively run on selected nodes (for more information, see [System Components Webhook](../concepts/resource-manager.md#system-components-webhook)).
   `DaemonSet`s on the other hand, should generally tolerate any `NoSchedule` or `NoExecute` taints so that they can run on any `Node`, regardless of user added taints.

## Images

1. **Do not hard-code container image references** ([example 1](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/charts/images.yaml#L130-L133), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/metricsserver.go#L28-L31), [example 3](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L82-L83))

   We define all image references centrally in the [`imagevector/containers.yaml`](../../imagevector/containers.yaml) file.
   Hence, the image references must not be hard-coded in the pod template spec but read from this so-called [image vector](../deployment/image_vector.md) instead.

2. **Do not use container images from registries that don't support IPv6** (example: [image vector](https://github.com/gardener/gardener/blob/6f4e64fe9494cafb5c5da9a2c0a491a5690161b6/charts/images.yaml#L619-L622), [prow configuration](https://github.com/gardener/ci-infra/blob/92782bedd92815639abf4dc14b2c484f77c6e57d/config/images/images.yaml#L37-L40))

   Registries such as ECR, GHCR (`ghcr.io`), MCR (`mcr.microsoft.com`) don't support pulling images over IPv6.

   Check if the upstream image is being also maintained in a registry that support IPv6 natively such as Artifact Registry, Quay (`quay.io`). If there is such image, use the image from registry with IPv6 support.

   If the image is not available in a registry with IPv6 then copy the image to the gardener GCR. There is a [prow job](https://github.com/gardener/ci-infra/blob/92782bedd92815639abf4dc14b2c484f77c6e57d/config/jobs/ci-infra/copy-images.yaml) copying images that are needed in gardener components from a source registry to the gardener GCR under the prefix `europe-docker.pkg.dev/gardener-project/releases/3rd/` (see the [documentation](https://github.com/gardener/ci-infra/tree/master/config/images) or [gardener/ci-infra#619](https://github.com/gardener/ci-infra/issues/619)).

   If you want to use a new image from a registry without IPv6 support or upgrade an already used image to a newer tag, please open a PR to the ci-infra repository that modifies the job's list of images to copy: [`images.yaml`](https://github.com/gardener/ci-infra/blob/master/config/images/images.yaml).

3. **Do not use container images from Docker Hub** (example: [image vector](https://github.com/gardener/gardener/blob/6f4e64fe9494cafb5c5da9a2c0a491a5690161b6/charts/images.yaml#L619-L622), [prow configuration](https://github.com/gardener/ci-infra/blob/92782bedd92815639abf4dc14b2c484f77c6e57d/config/images/images.yaml#L37-L40))

   There is a strict [rate-limit](https://docs.docker.com/docker-hub/download-rate-limit/) that applies to the Docker Hub registry. As described in 2., use another registry (if possible) or copy the image to the gardener GCR.

4. **Do not use Shoot container images that are not multi-arch**

   Gardener supports Shoot clusters with both `amd64` and `arm64` based worker Nodes. `amd64` container images cannot run on `arm64` worker Nodes and vice-versa.

## Security

1. **Use a [dedicated `ServiceAccount`](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) and disable auto-mount** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L145-L151))

   Components that need to talk to the API server of their runtime cluster must always use a dedicated `ServiceAccount` (do not use `default`), with `automountServiceAccountToken` set to `false`.
   This makes `gardener-resource-manager`'s [TokenInvalidator](../concepts/resource-manager.md#tokeninvalidator) invalidate the static token secret and its [`ProjectedTokenMount` webhook](../concepts/resource-manager.md#auto-mounting-projected-serviceaccount-tokens) inject a projected token automatically.

2. **Use shoot access tokens instead of a client certificates** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L234-L236))

   For components that need to talk to a target cluster different from their runtime cluster (e.g., running in seed cluster but talking to shoot) the `gardener-resource-manager`'s [TokenRequestor](../concepts/resource-manager.md#tokenrequestor) should be used to manage a so-called "shoot access token".

3. **Define RBAC roles with minimal privileges** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L153-L223))

   The component's `ServiceAccount` (if it exists) should have as little privileges as possible.
   Consequently, please define proper [RBAC roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) for it.
   This might include a combination of `ClusterRole`s and `Role`s.
   Please do not provide elevated privileges due to laziness (e.g., because there is already a `ClusterRole` that can be extended vs. creating a `Role` only when access to a single namespace is needed).

4. **Use [`NetworkPolicy`s](https://kubernetes.io/docs/concepts/services-networking/network-policies/) to restrict network traffic**

   You should restrict both ingress and egress traffic to/from your component as much as possible to ensure that it only gets access to/from other components if really needed.
   Gardener provides a few default policies for typical usage scenarios. For more information, see [`NetworkPolicy`s In Garden, Seed, Shoot Clusters](../operations/network_policies.md).

5. **Do not run containers in privileged mode** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/nodelocaldns/nodelocaldns.go#L324-L328), [example 2](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/nodelocaldns/nodelocaldns.go#L501))

   Avoid running containers with `privileged=true`. Instead, define the needed [Linux capabilities](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container).

6. **Do not allow privilege escalation for containers** ([example](https://github.com/gardener/gardener/blob/84e7b436cc5d58efdefd768b8556abec0e3083b6/pkg/component/networking/coredns/coredns.go#L658))

   Explicitly set `securityContext.allowPrivilegeEscalation=false`, in cases when possible. 

7. **Do not run containers as root** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/Dockerfile#L12))

   Avoid running containers as root. Usually, components such as Kubernetes controllers and admission webhook servers don't need root user capabilities to do their jobs.

   The problem with running as root, starts with how the container is first built. Unless a non-privileged user is configured in the `Dockerfile`, container build systems by default set up the container with the root user. Add a non-privileged user to your `Dockerfile` or use a base image with a non-root user (for example the `nonroot` images from [distroless](https://github.com/GoogleContainerTools/distroless) such as `gcr.io/distroless/static-debian12:nonroot`).

   If the image is an upstream one, then consider configuring a securityContext for the container/Pod with a non-privileged user. For more information, see [Configure a Security Context for a Pod or Container](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).

8. **Choose the proper Seccomp profile** ([example 1](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/nodelocaldns/nodelocaldns.go#L283-L287), [example 2](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/nginxingress/nginxingress.go#L447))

   For components deployed in the Seed cluster, the [Seccomp profile](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-seccomp-profile-for-a-container) will be defaulted to `RuntimeDefault` by `gardener-resource-manager`'s SeccompProfile webhook which works well for the majority of components. However, in some special cases you might need to overwrite it.

   The `gardener-resource-manager`'s SeccompProfile webhook is not enabled for a Shoot cluster. For components deployed in the Shoot cluster, it is required [*] to explicitly specify the Seccomp profile.

   <sub>[*] It is required because if a component deployed in the Shoot cluster does not specify a Seccomp profile and cannot run with the `RuntimeDefault` Seccomp profile, then enabling the `.spec.kubernetes.kubelet.seccompDefault` field in the Shoot spec would break the corresponding component.</sub>

## High Availability / Stability

1. **Specify the component type label for high availability** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L241))

   To support high-availability deployments, `gardener-resource-manager`s [HighAvailabilityConfig](../concepts/resource-manager.md#high-availability-config) webhook injects the proper specification like replica or topology spread constraints.
   You only need to specify the type label. For more information, see [High Availability Of Deployed Components](high-availability-of-components.md).

2. **Define a `PodDisruptionBudget`** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L384-L408))

   Closely related to high availability but also to stability in general: The definition of a [`PodDisruptionBudget`](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) with `maxUnavailable=1` should be provided by default.

3. **Choose the right `PriorityClass`** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L307))

   Each cluster runs many components with different priorities.
   Gardener provides a set of default [`PriorityClass`es](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass). For more information, see [Priority Classes](priority-classes.md).

4. **Consider defining liveness and readiness probes** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L321-L344))

   To ensure smooth rolling update behaviour, consider the definition of [liveness and/or readiness probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/).

5. **Mark node-critical components** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubeproxy/resources.go#L328))

   To ensure user workload pods are only scheduled to `Nodes` where all node-critical components are ready, these components need to tolerate the `node.gardener.cloud/critical-components-not-ready` taint (`NoSchedule` effect).
   Also, such `DaemonSets` and the included `PodTemplates` need to be labelled with `node.gardener.cloud/critical-component=true`.
   For more information, see [Readiness of Shoot Worker Nodes](../usage/advanced/node-readiness.md).

6. **Consider making a `Service` topology-aware** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/vpa/admissioncontroller.go#L154))

   To reduce costs and to improve the network traffic latency in multi-zone Seed clusters, consider making a `Service` topology-aware, if applicable. In short, when a `Service` is topology-aware, Kubernetes routes network traffic to the `Endpoint`s (`Pod`s) which are located in the same zone where the traffic originated from. In this way, the cross availability zone traffic is avoided. See [Topology-Aware Traffic Routing](../operations/topology_aware_routing.md).

7. **Enable leader election unconditionally for controllers** ([example 1](https://github.com/gardener/gardener/blob/bd0a90854f0a8751d361e6f1cedd97ce5a746e3c/pkg/component/kubernetes/controllermanager/controllermanager.go#L677), [example 2](https://github.com/gardener/gardener/blob/bd0a90854f0a8751d361e6f1cedd97ce5a746e3c/pkg/component/kubernetes/scheduler/scheduler.go#L75-L76), [example 3](https://github.com/gardener/gardener/blob/bd0a90854f0a8751d361e6f1cedd97ce5a746e3c/pkg/component/gardener/resourcemanager/resource_manager.go#L519-L523))

   Enable leader election unconditionally for controllers independently from the number of replicas or from the high availability configurations. Having leader election enabled even for a single replica Deployment prevents having two Pods active at the same time. Otherwise, there are some corner cases that can result in two active Pods - Deployment rolling update or kubelet stops running on a Node and is not able to terminate the old replica while kube-controller-manager creates a new replica to match the Deployment's desired replicas count.

## Scalability

1. **Provide resource requirements** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L345-L353))

   All components should define reasonable (initial) [CPU and memory `requests`](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container) and avoid limits (especially CPU limits) unless you know the healthy range for your component (almost impossible with most components today), but no more than the node allocatable remainder (after daemonset pods) of the largest eligible machine type. [Scheduling only takes `requests` into account](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#how-pods-with-resource-requests-are-scheduled)!

2. **Define a `VerticalPodAutoscaler`** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L416-L444))

   We typically (need to) perform vertical auto-scaling for containers that have a significant usage (>50m/100M) and a significant usage spread over time (>2x) by defining a [`VerticalPodAutoscaler`](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#intro) with `updatePolicy.updateMode` [`Auto`](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#quick-start), `containerPolicies[].controlledValues` [`RequestsOnly`](https://github.com/kubernetes/autoscaler/blob/6da986f4ccefd2c2632e184f22cce30390dfb7d6/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1/types.go#L244-L245), reasonable `minAllowed` configuration and no `maxAllowed` configuration (will be taken care of in Gardener environments for you/capped at the largest eligible machine type).

3. **Define a `HorizontalPodAutoscaler` if needed** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/coredns/coredns.go#L671-L726))

   If your component is capable of scaling horizontally, you should consider defining a [`HorizontalPodAutoscaler`](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale).

> [!NOTE]
> For more information and concrete configuration hints, please see our [best practices guide for pod auto scaling](../usage/autoscaling/shoot_pod_autoscaling_best_practices.md) and especially the [summary](../usage/autoscaling/shoot_pod_autoscaling_best_practices.md#summary) and [recommendations](../usage/autoscaling/shoot_pod_autoscaling_best_practices.md#recommendations-in-a-box) sections.

## Observability / Operations Productivity

1. **Provide monitoring scrape config and alerting rules** ([example 1](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/coredns/monitoring.go), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/monitoring.go#L97))

   Components should provide scrape configuration and alerting rules for Prometheus/Alertmanager if appropriate.
   This should be done inside a dedicated `monitoring.go` file.
   Extensions should follow the guidelines described in [Extensions Monitoring Integration](../extensions/logging-and-monitoring.md#extensions-monitoring-integration).

2. **Provide logging parsers and filters** ([example 1](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/coredns/logging.go), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/gardenlet/controller/seed/seed/reconciler_reconcile.go#L563))

   Components should provide parsers and filters for fluent-bit, if appropriate.
   This should be done inside a dedicated `logging.go` file.
   Extensions should follow the guidelines described in [Fluent-bit log parsers and filters](../extensions/logging-and-monitoring.md#fluent-bit-log-parsers-and-filters).

3. **Set the `revisionHistoryLimit` to `2` for `Deployment`s** ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/metricsserver/metrics_server.go#L272))

   In order to allow easy inspection of two `ReplicaSet`s to quickly find the changes that lead to a rolling update, the [revision history limit](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit) should be set to `2`.

4. **Define health checks** ([example 1](https://github.com/gardener/gardener/blob/180951eac9b8183175d4dcadc305c7722ce8122d/pkg/gardenlet/controller/shoot/care/health.go#L763-L795))

   [`gardener-operators`'s](../concepts/operator.md#controllers) and [`gardenlet`'s](../concepts/gardenlet.md#controllers) care controllers regularly check the health status of components relevant to the respective cluster (garden/seed/shoot).
   For shoot control plane components, you need to enhance the lists of components to make sure your component is checked, see example above.
   For components deployed via `ManagedResource`, please consult the respective care controller documentation for more information ([garden](../concepts/operator.md#care-reconciler), [seed](../concepts/gardenlet.md#-care--reconciler-1), [shoot](../concepts/gardenlet.md#-care--reconciler-2)).

5. **Configure automatic restarts in shoot maintenance time window** ([example 1](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler.go#L250), [example 2](https://github.com/gardener/gardener/blob/6a0fea86850ffec8937d1956bdf1a8ca6d074f3b/pkg/operation/botanist/coredns.go#L90-L107))

   Gardener offers to restart components during the maintenance time window. For more information, see [Restart Control Plane Controllers](../usage/shoot/shoot_maintenance.md#restart-control-plane-controllers) and [Restart Some Core Addons](../usage/shoot/shoot_maintenance.md#restart-some-core-addons).
   You can consider adding the needed label to your control plane component to get this automatic restart (probably not needed for most components).
