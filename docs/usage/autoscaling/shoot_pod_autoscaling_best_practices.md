# Introduction

There are two types of pod autoscaling in Kubernetes: Horizontal Pod Autoscaling (HPA) and Vertical Pod Autoscaling (VPA). HPA (implemented as part of the kube-controller-manager) scales the number of pod replicas, while VPA (implemented as independent community project) adjusts the CPU and memory requests for the pods. Both types of autoscaling aim to optimize resource usage/costs and maintain the performance and (high) availability of applications running on Kubernetes.

## [Horizontal Pod Autoscaling (HPA)](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale)

Horizontal Pod Autoscaling involves increasing or decreasing the number of pod replicas in a deployment, replica set, stateful set, or [anything really with a scale subresource that manages pods](https://github.com/kubernetes/enhancements/blob/master/keps/sig-autoscaling/853-configurable-hpa-scale-velocity/README.md#summary). HPA adjusts the number of replicas based on specified metrics, such as CPU or memory average utilization (usage divided by requests; most common) or average value (usage; less common). When the demand on your application increases, HPA automatically scales out the number of pods to meet the demand. Conversely, when the demand decreases, it scales in the number of pods to reduce resource usage.

HPA targets (mostly stateless) applications where adding more instances of the application can linearly increase the ability to handle additional load. It is very useful for applications that experience variable traffic patterns, as it allows for real-time scaling without the need for manual intervention.

> [!NOTE]
> HPA continuously monitors the metrics of the targeted pods and adjusts the number of replicas based on the observed metrics. It operates solely on the current metrics when it calculates the averages across all pods, meaning it reacts to the immediate resource usage without considering past trends or patterns. Also, all pods are treated equally based on the average metrics. This could potentially lead to situations where some pods are under high load while others are underutilized. Therefore, particular care must be applied to (fair) load-balancing (connection vs. request vs. actual resource load balancing are crucial).

### A Few Words on the [Cluster-Proportional (Horizontal) Autoscaler (CPA)](https://github.com/kubernetes-sigs/cluster-proportional-autoscaler) and the [Cluster-Proportional Vertical Autoscaler (CPVA)](https://github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler)

Besides HPA and VPA, CPA and CPVA are further options for scaling horizontally or vertically (neither is deployed by Gardener and must be deployed by the user). Unlike HPA and VPA, CPA and CPVA do not monitor the actual pod metrics, but scale solely on the number of nodes or CPU cores in the cluster. While this approach may be helpful and sufficient in a few rare cases, it is often a risky and crude scaling scheme that we do not recommend. More often than not, cluster-proportional scaling results in either under- or over-reserving your resources.

## [Vertical Pod Autoscaling (VPA)](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler#readme)

Vertical Pod Autoscaling, on the other hand, focuses on adjusting the CPU and memory resources allocated to the pods themselves. Instead of changing the number of replicas, VPA tweaks the resource requests (and limits, but only [proportionally](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#keeping-limit-proportional-to-request), if configured) for the pods in a deployment, replica set, stateful set, daemon set, or [anything really with a scale subresource that manages pods](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/faq.md#how-can-i-apply-vpa-to-my-custom-resource). This means that each pod can be given more, or fewer resources as needed.

VPA is very useful for optimizing the resource requests of pods that have dynamic resource needs over time. It does so by mutating pod requests (unfortunately, [not in-place](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates)). Therefore, in order to apply new recommendations, pods that are "out of bounds" (i.e. below a configured/computed lower or above a configured/computed upper recommendation percentile) will be evicted proactively, but also pods that are "within bounds" may be evicted after a grace period. The corresponding higher-level replication controller will then recreate a new pod that VPA will then mutate to set the currently recommended requests (and proportional limits, if configured).

> [!NOTE]
> VPA continuously monitors all targeted pods and calculates recommendations based on their usage (one recommendation for the entire target). This calculation is influenced by configurable percentiles, with a greater emphasis on recent usage data and a gradual decrease (=decay) in the relevance of older data. However, this means, that VPA doesn't take into account individual needs of single pods - eventually, all pods will receive the same recommendation, which may lead to considerable resource waste. Ideally, VPA would update pods [in-place](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/1287-in-place-update-pod-resources/README.md) depending on their individual needs, but that's (individual recommendations) not in its design, even if [in-place updates](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates) get implemented, which may be years away for VPA based on current activity on the component.

## Selecting the Appropriate Autoscaler

Before deciding on an autoscaling strategy, it's important to understand the characteristics of your application:
- **Interruptibility:** Most importantly, if the clients of your workload are too sensitive to disruptions/cannot cope well with terminating pods, then maybe neither HPA nor VPA is an option (both, HPA and VPA cause pods and connections to be terminated, though VPA even more frequently). Clients must retry on disruptions, which is a reasonable ask in a highly dynamic (and self-healing) environment such as Kubernetes, but this is often not respected (or expected) by your clients (they may not know or care you run the workload in a Kubernetes cluster and have different expectations to the stability of the workload unless you communicated those through [SLIs/SLOs/SLAs](https://cloud.google.com/blog/products/devops-sre/sre-fundamentals-slis-slas-and-slos)).
- **Statelessness:** Is your application stateless or stateful? Stateless applications are typically better candidates for HPA as they can be easily scaled out by adding more replicas without worrying about maintaining state.
- **Traffic Patterns:** Does your application experience variable traffic? If so, HPA can help manage these fluctuations by adjusting the number of replicas to handle the load.
- **Resource Usage:** Does your application's resource usage change over time? VPA can adjust the CPU and memory reservations dynamically, which is beneficial for applications with non-uniform resource requirements.
- **Scalability:** Can your application handle increased load by scaling vertically (more resources per pod) or does it require horizontal scaling (more pod instances)?

HPA is the right choice if:
- Your application is stateless and can handle increased load by adding more instances.
- You experience short-term fluctuations in traffic that require quick scaling responses.
- You want to maintain a specific performance metric, such as requests per second per pod.

VPA is the right choice if:
- Your application's resource requirements change over time, and you want to optimize resource usage without manual intervention.
- You want to avoid the complexity of managing resource requests for each pod, especially when they run code where it's impossible for you to suggest static requests.

In essence:
- For applications that can handle increased load by simply adding more replicas, HPA should be used to handle short-term fluctuations in load by scaling the number of replicas.
- For applications that require more resources per pod to handle additional work, VPA should be used to adjust the resource allocation for longer-term trends in resource usage.

Consequently, if both cases apply (VPA often applies), HPA and VPA can also be combined. However, combining both, especially on the same metrics (CPU and memory), requires understanding and care to avoid conflicts and ensure that the autoscaling actions do not interfere with and rather complement each other. For more details, see [Combining HPA and VPA](#combining-hpa-and-vpa).

# Horizontal Pod Autoscaler (HPA)

HPA operates by monitoring resource metrics for all pods in a target. It computes the desired number of replicas from the current average metrics and the desired user-defined metrics [as follows](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details):

`desiredReplicas = ceil[currentReplicas * (currentMetricValue / desiredMetricValue)]`

HPA checks the metrics at regular intervals, which can be configured by the user. [Several types of metrics](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#support-for-metrics-apis) are supported (classical resource metrics like CPU and memory, but also custom and external metrics like requests per second or queue length can be configured, if available). If a scaling event is necessary, HPA adjusts the replica count for the targeted resource.

## Defining an HPA Resource

To configure HPA, you need to create an HPA resource in your cluster. This resource specifies the target to scale, the metrics to be used for scaling decisions, and the desired thresholds. Here's an example of an HPA configuration:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: foo-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: foo-deployment
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: AverageValue
        averageValue: 2
  - type: Resource
    resource:
      name: memory
      target:
        type: AverageValue
        averageValue: 8G
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 30
      policies:
      - type: Percent
        value: 100
        periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 1800
      policies:
      - type: Pods
        value: 1
        periodSeconds: 300
```

In this example, HPA is configured to scale `foo-deployment` based on pod average CPU and memory usage. It will maintain an average CPU and memory usage (not utilization, which is usage divided by requests!) across all replicas of 2 CPUs and 8G or lower with as few replicas as possible. The number of replicas will be scaled between a minimum of 1 and a maximum of 10 based on this target.

Since a while, you can also [configure the autoscaling based on the resource usage of individual containers](https://kubernetes.io/blog/2023/05/02/hpa-container-resource-metric), not only on the resource usage of the entire pod. All you need to do is to switch the `type` from `Resource` to [`ContainerResource` and specify the container name](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#container-resource-metrics).

In the official documentation ([[1]](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale) and [[2]](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough)) you will find examples with average utilization (`averageUtilization`), not average usage (`averageValue`), but this is not particularly helpful, especially if you plan to combine HPA together with VPA on the same metrics (generally discouraged in the documentation). If you want to safely combine both on the same metrics, you should scale on average usage (`averageValue`) as shown above. For more details, see [Combining HPA and VPA](#combining-hpa-and-vpa).

Finally, the behavior section influences how fast you scale up and down. Most of the time (depends on your workload), you like to scale out faster than you scale in. In this example, the configuration will trigger a scale-out only after observing the need to scale out for 30s (`stabilizationWindowSeconds`) and will then only scale out at most 100% (`value` + `type`) of the current number of replicas every 60s (`periodSeconds`). The configuration will trigger a scale-in only after observing the need to scale in for 1800s (`stabilizationWindowSeconds`) and will then only scale in at most 1 pod (`value` + `type`) every 300s (`periodSeconds`). As you can see, scale-out happens quicker than scale-in in this example.

## HPA (actually KCM) Options

HPA is a function of the kube-controller-manager (KCM).

You can read up the [full KCM options](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager) online and set most of them conveniently in your [Gardener shoot cluster spec](https://github.com/gardener/gardener/blob/957e4c7/example/90-shoot.yaml#L221-L226):
- `downscaleStabilization` (default 5m): HPA will scale out whenever the formula (in accordance with the behavior section, if present in the HPA resource) yields a higher replica count, but it won't scale in just as eagerly. This option lets you define a trailing time window that HPA must check and only if the recommended replica count is consistently lower throughout the entire time window, HPA will scale in (in accordance with the behavior section, if present in the HPA resource). If at any point in time in that trailing time window the recommended replica count isn't lower, scale-in won't happen. This setting is just a default, if nothing is defined in the behavior section of an HPA resource. The default for the upscale stabilization is 0s and it cannot be set via a KCM option ([downscale stabilization was historically more important than upscale stabilization and when later the behavior sections were added to the HPA resources, upscale stabilization remained missing from the KCM options](https://github.com/kubernetes/enhancements/blob/master/keps/sig-autoscaling/853-configurable-hpa-scale-velocity/README.md)).
- `tolerance` (default +/-10%): HPA will not scale out or in if the desired replica count is (mathematically as a float) near the actual replica count (see [source code](https://github.com/kubernetes/kubernetes/blob/f3f5dd99ac7bdc61c61c3d587575090c3473ab5a/pkg/controller/podautoscaler/replica_calculator.go#L97-L103) for details), which is a form of hysteresis to avoid replica flapping around a threshold.

There are a few more configurable options of lesser interest:
- `syncPeriod` (default 15s): How often HPA retrieves the pods and metrics respectively how often it recomputes and sets the desired replica count.
- `cpuInitializationPeriod` (default 30s) and `initialReadinessDelay` (default 5m): Both settings only affect whether or not CPU metrics are considered for scaling decisions. They can be easily misinterpreted as the [official docs](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) are [somewhat hard to read](https://github.com/kubernetes/website/issues/12657) (see [source code](https://github.com/kubernetes/kubernetes/blob/f3f5dd99ac7bdc61c61c3d587575090c3473ab5a/pkg/controller/podautoscaler/replica_calculator.go#L399-L418) for details, which is more readable, if you ignore the comments). Normally, you have little reason to modify them, but here is what they do:
  - `cpuInitializationPeriod`: Defines a grace period after a pod starts during which HPA won't consider CPU metrics of the pod for scaling if the pod is either not ready **or** it is ready, but a given CPU metric is older than the last state transition (to ready). This is to ignore CPU metrics that predate the current readiness while still in initialization to not make scaling decisions based on potentially misleading data. If the pod is ready and a CPU metric was collected after it became ready, it is considered also within this grace period.
  - `initialReadinessDelay`: Defines another grace period after a pod starts during which HPA won't consider CPU metrics of the pod for scaling if the pod is not ready **and** it became not ready within this grace period (the docs/comments want to check whether the pod was ever ready, but the [code only checks whether the pod condition last transition time to not ready happened within that grace period](https://github.com/kubernetes/kubernetes/blob/f3f5dd99ac7bdc61c61c3d587575090c3473ab5a/pkg/controller/podautoscaler/replica_calculator.go#L411) which it could have [from being ready or simply unknown before](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions)). This is to ignore not (ever have been) ready pods while still in initialization to not make scaling decisions based on potentially misleading data. If the pod is ready, it is considered also within this grace period.

  So, regardless of the values of these settings, if a pod is reporting ready and it has a CPU metric from the time after it became ready, that pod and its metric will be considered. This holds true even if the pod becomes ready very early into its initialization. These settings cannot be used to "black-out" pods for a certain duration before being considered for scaling decisions. Instead, if it is your goal to ignore a potentially resource-intensive initialization phase that could wrongly lead to further scale-out, you would need to configure your pods to not report as ready until that resource-intensive initialization phase is over.

## Considerations When Using HPA

- **Selection of metrics:** Besides CPU and memory, HPA can also target custom or external metrics. Pick those (in addition or exclusively), if you guarantee certain SLOs in your SLAs.
- **Targeting usage or utilization:** HPA supports usage (absolute) and utilization (relative). Utilization is often preferred in simple examples, but usage is more precise and versatile.
- **Compatibility with VPA:** Care must be taken when using HPA in conjunction with VPA, as they can potentially interfere with each other's scaling decisions.

# Vertical Pod Autoscaler (VPA)

VPA operates by monitoring resource metrics for all pods in a target. It computes a resource requests recommendation from the historic and current resource metrics. VPA checks the metrics at regular intervals, which can be configured by the user. Only CPU and memory are supported. If VPA detects that a pod's resource allocation is too high or too low, it may evict pods (if within the permitted disruption budget), which will trigger the creation of a new pod by the corresponding higher-level replication controller, which will then be mutated by VPA to match resource requests recommendation. This happens in three different components that work together:
- **VPA Recommender:** The Recommender observes the historic and current resource metrics of pods and generates recommendations based on this data.
- **VPA Updater:** The Updater component checks the recommendations from the Recommender and decides whether any pod's resource requests need to be updated. If an update is needed, the Updater will evict the pod.
- **VPA Admission Controller:** When a pod is (re-)created, the Admission Controller modifies the pod's resource requests based on the recommendations from the Recommender. This ensures that the pod starts with the optimal amount of resources.

Since VPA doesn't support in-place updates, pods will be evicted. You will want to control voluntary evictions by means of [Pod Disruption Budgets (PDBs)](https://kubernetes.io/docs/tasks/run-application/configure-pdb). Please make yourself familiar with those and use them.

> [!NOTE]
> PDBs will not always work as expected and can also get in your way, e.g. if the PDB is violated or would be violated, it may possibly block evictions that would actually help your workload, e.g. to get a pod out of an `OOMKilled` `CrashLoopBackoff` (if the PDB is or would be violated, not even unhealthy pods would be evicted as they could theoretically become healthy again, which VPA doesn't know). In order to overcome this issue, it is now possible (alpha since Kubernetes `v1.26` in combination with the feature gate `PDBUnhealthyPodEvictionPolicy` on the API server, beta and enabled by default since Kubernetes `v1.27`) to configure the so-called [unhealthy pod eviction policy](https://kubernetes.io/docs/tasks/run-application/configure-pdb/#unhealthy-pod-eviction-policy). The default is still `IfHealthyBudget` as a change in default would have changed the behavior (as described above), but you can now also set `AlwaysAllow` at the PDB (`spec.unhealthyPodEvictionPolicy`). For more information, please check out [this discussion](https://github.com/kubernetes/kubernetes/issues/72320), [the PR](https://github.com/kubernetes/kubernetes/pull/105296) and [this document](https://groups.google.com/g/kubernetes-sig-apps/c/_joO4swogKY?pli=1) and balance the pros and cons for yourself. In short, the new `AlwaysAllow` option is probably the better choice in most of the cases while `IfHealthyBudget` is useful only if you have frequent temporary transitions or for special cases where you have already implemented controllers that depend on the old behavior.

## Defining a VPA Resource

To configure VPA, you need to create a VPA resource in your cluster. This resource specifies the target to scale, the metrics to be used for scaling decisions, and the policies for resource updates. Here's an example of an VPA configuration:

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: foo-vpa
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind:       Deployment
    name:       foo-deployment
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: foo-container
      controlledValues: RequestsOnly
      minAllowed:
        cpu: 50m
        memory: 200M
      maxAllowed:
        cpu: 4
        memory: 16G
```

In this example, VPA is configured to scale `foo-deployment` requests (`RequestsOnly`) from 50m cores (`minAllowed`) up to 4 cores (`maxAllowed`) and 200M memory (`minAllowed`) up to 16G memory (`maxAllowed`) automatically (`updateMode`). VPA doesn't support in-place updates, so in `updateMode` `Auto` it will evict pods under certain conditions and then mutate the requests (and possibly limits if you omit `controlledValues` or set it to `RequestsAndLimits`, which is the default) of upcoming new pods.

[Multiple update modes exist](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#quick-start). They influence eviction and mutation. The most important ones are:
- `Off`: In this mode, recommendations are computed, but never applied. This mode is useful, if you want to learn more about your workload or if you have a custom controller that depends on VPA's recommendations but shall act instead of VPA.
- `Initial`: In this mode, recommendations are computed and applied, but pods are never proactively evicted to enforce new recommendations over time. This mode is useful, if you want to control pod evictions yourself (similar to the [`StatefulSet` `updateStrategy` `OnDelete`](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#update-strategies)) or your workload is sensitive to evictions, e.g. some brownfield singleton application or a daemon set pod that is critical for the node.
- `Auto` (default): In this mode, recommendations are computed, applied, and pods are even proactively evicted to enforce new recommendations over time. This applies recommendations continuously without you having to worry too much.

As mentioned, `controlledValues` influences whether only requests or requests and limits are scaled:
- `RequestsOnly`: Updates only requests and doesn't change limits. Useful if you have defined absolute limits (unrelated to the requests).
- `RequestsAndLimits` (default): Updates requests and proportionally scales limits along with the requests. Useful if you have defined relative limits (related to the requests). In this case, the gap between requests and limits should be either zero for QoS [`Guaranteed`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed) or small for QoS [`Burstable`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#burstable) to avoid useless (way beyond the threshold of unhealthy behavior) or absurd (larger than node capacity) values.

VPA doesn't offer many more settings that can be tuned per VPA resource than you see above (different than HPA's `behavior` section). However, there is one more that isn't shown above, which allows to [scale only up or only down (`evictionRequirements[].changeRequirement`)](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#controlling-eviction-behavior-based-on-scaling-direction-and-resource), in case you need that, e.g. to provide resources when needed, but avoid disruptions otherwise.

## VPA Options

VPA is an independent community project that [consists of](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#components-of-vpa) a recommender (computing target recommendations and bounds), an updater (evicting pods that are out of recommendation bounds), and an admission controller (mutating webhook applying the target recommendation to newly created pods). As such, they have independent options.

### VPA Recommender Options

You can read up the [full VPA recommender options](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/faq.md#what-are-the-parameters-to-vpa-recommender) online and set some of them conveniently in your [Gardener shoot cluster spec](https://github.com/gardener/gardener/blob/957e4c7/example/90-shoot.yaml#L298-L307):
- `recommendationMarginFraction` (default 15%): Safety margin that will be added to the recommended requests.
- `targetCPUPercentile` (default 90%): CPU usage percentile that will be targeted with the CPU recommendation (i.e. recommendation will "fit" e.g. 90% of the observed CPU usages). This setting is relevant for balancing your requests reservations vs. your costs. If you want to reduce costs, you can reduce this value (higher risk because of potential under-reservation, but lower costs), because CPU is compressible, but then VPA may lack the necessary signals for scale-up as throttling on an otherwise fully utilized node will go unnoticed by VPA. If you want to err on the safe side, you can increase this value, but you will then target more and more a worst case scenario, quickly (maybe even exponentially) increasing the costs.
- `targetMemoryPercentile` (default 90%): Memory usage percentile that will be targeted with the memory recommendation (i.e. recommendation will "fit" e.g. 90% of the observed memory usages). This setting is relevant for balancing your requests reservations vs. your costs. If you want to reduce costs, you can reduce this value (higher risk because of potential under-reservation, but lower costs), because OOMs will trigger bump-ups, but those will disrupt the workload. If you want to err on the safe side, you can increase this value, but you will then target more and more a worst case scenario, quickly (maybe even exponentially) increasing the costs.

There are a few more configurable options of lesser interest:
- `recommenderInterval` (default 1m): How often VPA retrieves the pods and metrics respectively how often it recomputes the recommendations and bounds.

There are many more options that you can only configure if you deploy your own VPA and which we will not discuss here, but you can check them out [here](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/faq.md#what-are-the-parameters-to-vpa-recommender).

> [!NOTE]
> Due to an implementation detail (smallest bucket size), VPA cannot create recommendations below [10m cores](https://github.com/kubernetes/autoscaler/blob/1f89ff92cf87dd3700f74f9b387ae4846aa51846/vertical-pod-autoscaler/pkg/recommender/model/aggregations_config.go#L89-L99) and [10M memory](https://github.com/kubernetes/autoscaler/blob/1f89ff92cf87dd3700f74f9b387ae4846aa51846/vertical-pod-autoscaler/pkg/recommender/model/aggregations_config.go#L101-L111) even if `minAllowed` is lower.

### VPA Updater Options

You can read up the [full VPA updater options](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/faq.md#what-are-the-parameters-to-vpa-updater) online and set some of them conveniently in your [Gardener shoot cluster spec](https://github.com/gardener/gardener/blob/957e4c7/example/90-shoot.yaml#L298-L307):
- `evictAfterOOMThreshold` (default 10m): Pods where at least one container OOMs within this time period since its start will be actively evicted, which will implicitly apply the new target recommendation that will have been [bumped up after `OOMKill`](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#custom-memory-bump-up-after-oomkill). Please note, the kubelet may evict pods even before an OOM, but only if `kube-reserved` is underrun, i.e. [node-level resources are running low](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#pod-selection-for-kubelet-eviction). In these cases, eviction will happen first by pod priority and second by how much the usage overruns the requests.
- `evictionTolerance` (default 50%): Defines a threshold below which no further eligible pod will be evited anymore, i.e. limits how many eligible pods may be in eviction in parallel (but at least 1). The [threshold is computed as follows](https://github.com/kubernetes/autoscaler/blob/4d0511363eeeff657119797dd8d26e851dcc3459/vertical-pod-autoscaler/pkg/updater/eviction/pods_eviction_restriction.go#L108-L117): `running - evicted > replicas - tolerance`. Example: 10 replicas, 9 running, 8 eligible for eviction, 20% tolerance with 10 replicas which amounts to 2 pods, and no pod evicted in this round yet, then `9 - 0 > 10 - 2` is true and a pod would be evicted, but the next one would be in violation as `9 - 1 = 10 - 2` and no further pod would be evicted anymore in this round.
- `evictionRateBurst` (default 1): Defines how many eligible pods may be evicted in one go.
- `evictionRateLimit` (default disabled): Defines how many eligible pods may be evicted per second (a value of 0 or -1 disables the rate limiting).

In general, avoid modifying these eviction settings unless you have good reasons and try to rely on [Pod Disruption Budgets (PDBs)](https://kubernetes.io/docs/tasks/run-application/configure-pdb) instead. However, [PDBs are not available for daemon sets](https://github.com/kubernetes/kubernetes/issues/108124).

There are a few more configurable options of lesser interest:
- `updaterInterval` (default 1m): How often VPA evicts the pods.

There are many more options that you can only configure if you deploy your own VPA and which we will not discuss here, but you can check them out [here](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/docs/faq.md#what-are-the-parameters-to-vpa-updater).

## Considerations When Using VPA

- **Initial Resource Estimates:** VPA requires historical resource usage data to base its recommendations on. Until they kick in, your initial resource requests apply and should be sensible.
- **Pod Disruption:** When VPA adjusts the resources for a pod, it may need to "recreate" the pod, which can cause temporary disruptions. This should be taken into account.
- **Compatibility with HPA:** Care must be taken when using VPA in conjunction with HPA, as they can potentially interfere with each other's scaling decisions.

# Combining HPA and VPA

HPA and VPA serve different purposes and operate on different axes of scaling. HPA increases or decreases the number of pod replicas based on metrics like CPU or memory usage, effectively scaling the application out or in. VPA, on the other hand, adjusts the CPU and memory reservations of individual pods, scaling the application up or down.

When used together, these autoscalers can provide both horizontal and vertical scaling. However, they can also conflict with each other if used on the same metrics (e.g. both on CPU or both on memory). In particular, if VPA adjusts the requests, the utilization, i.e. the ratio between usage and requests, will approach 100% (for various reasons not exactly right, but for this consideration, close enough), which may trigger HPA to scale out, if it's configured to scale on utilization below 100% (often seen in simple examples), which will spread the load across more pods, which may trigger VPA again to adjust the requests to match the new pod usages.

This is a feedback loop and it stems from [HPA's method of calculating the desired number of replicas](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details), which is:

`desiredReplicas = ceil[currentReplicas * (currentMetricValue / desiredMetricValue)]`

If `desiredMetricValue` is utilization and VPA adjusts the requests, which changes the utilization, this may inadvertently trigger HPA and create said feedback loop. On the other hand, if `desiredMetricValue` is usage and VPA adjusts the requests now, this will have no impact on HPA anymore (HPA will always influence VPA, but we can control whether VPA influences HPA).

Therefore, to safely combine HPA and VPA, consider the following strategies:
- **Configure HPA and VPA on different metrics:** One way to avoid conflicts is to use HPA and VPA based on different metrics. For instance, you could configure HPA to scale based on requests per seconds (or another representative custom/external metric) and VPA to adjust CPU and memory requests. This way, each autoscaler operates independently based on its specific metric(s).
- **Configure HPA to scale on usage, not utilization, when used with VPA:** Another way to avoid conflicts is to use HPA not on average utilization (`averageUtilization`), but instead on average usage (`averageValue`) as replicas driver, which is an absolute metric (requests don't affect usage). This way, you can combine both autoscalers even on the same metrics.

# Pod Autoscaling and Cluster Autoscaler

Autoscaling within Kubernetes can be implemented at different levels: pod autoscaling (HPA and VPA) and cluster autoscaling (CA). While pod autoscaling adjusts the number of pod replicas or their resource reservations, cluster autoscaling focuses on the number of nodes in the cluster, so that your pods can be hosted. If your workload isn't static and especially if you make use of pod autoscaling, it only works if you have sufficient node capacity available. The most effective way to do that, without running a worst-case number of nodes, is to configure burstable worker pools in your shoot spec, i.e. define a true minimum node count and a worst-case maximum node count and leave the node autoscaling to Gardener that internally uses the Cluster Autoscaler to provision and deprovision nodes as needed.

Cluster Autoscaler automatically adjusts the number of nodes by adding or removing nodes based on the demands of the workloads and the available resources. It interacts with the cloud provider's APIs to provision or deprovision nodes as needed. Cluster Autoscaler monitors the utilization of nodes and the scheduling of pods. If it detects that pods cannot be scheduled due to a lack of resources, it will trigger the addition of new nodes to the cluster. Conversely, if nodes are underutilized for some time and their pods can be placed on other nodes, it will remove those nodes to reduce costs and improve resource efficiency.

Best Practices:
- **Resource Buffering:** Maintain a buffer of resources to accommodate temporary spikes in demand without waiting for node provisioning. This can be done by [deploying pods with low priority that can be preempted when real workloads require resources](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-configure-overprovisioning-with-cluster-autoscaler). This helps in faster pod scheduling and avoids delays in scaling out or up.
- **Pod Disruption Budgets (PDBs):** Use [PDBs](https://kubernetes.io/docs/tasks/run-application/configure-pdb) to ensure that during scale-down events, the availability of applications is maintained as the Cluster Autoscaler will not voluntarily evict a pod if a PDB would be violated.

## Interesting CA Options

CA can be configured in your [Gardener shoot cluster spec globally](https://github.com/gardener/gardener/blob/957e4c7/example/90-shoot.yaml#L281-L297) and also in parts [per worker pool](https://github.com/gardener/gardener/blob/957e4c7/example/90-shoot.yaml#L48-L53):
- Can only be configured globally:
  - `expander` (default least-waste): Defines the "expander" algorithm to use during scale-up, see [FAQ](https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#what-are-expanders).
  - `scaleDownDelayAfterAdd` (default 1h): Defines how long after scaling up a node, a node may be scaled down.
  - `scaleDownDelayAfterFailure` (default 3m): Defines how long after scaling down a node failed, scaling down will be resumed.
  - `scaleDownDelayAfterDelete` (default 0s): Defines how long after scaling down a node, another node may be scaled down.
- Can be configured globally and also overwritten individually per worker pool:
  - `scaleDownUtilizationThreshold` (default 50%): Defines the threshold below which a node becomes eligible for scaling down.
  - `scaleDownUnneededTime` (default 30m): Defines the trailing time window the node must be consistently below a certain utilization threshold before it can finally be scaled down.

There are many more options that you can only configure if you deploy your own CA and which we will not discuss here, but you can check them out [here](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca).

# Importance of Monitoring

Monitoring is a critical component of autoscaling for several reasons:
- **Performance Insights:** It provides insights into how well your autoscaling strategy is meeting the performance requirements of your applications.
- **Resource Utilization:** It helps you understand resource utilization patterns, enabling you to optimize resource allocation and reduce waste.
- **Cost Management:** It allows you to track the cost implications of scaling actions, helping you to maintain control over your cloud spending.
- **Troubleshooting:** It enables you to quickly identify and address issues with autoscaling, such as unexpected scaling behavior or resource bottlenecks.

To effectively monitor autoscaling, you should leverage the following tools and metrics:
- **[Kubernetes Metrics Server](https://sigs.k8s.io/metrics-server):** Collects resource metrics from kubelets and provides them to HPA and VPA for autoscaling decisions (automatically provided by Gardener).
- **Prometheus:** An open-source monitoring system that can collect and store custom metrics, providing a rich dataset for autoscaling decisions.
- **Grafana/Plutono:** A visualization tool that integrates with Prometheus to create dashboards for monitoring autoscaling metrics and events.
- **Cloud Provider Tools:** Most cloud providers offer native monitoring solutions that can be used to track the performance and costs associated with autoscaling.

Key metrics to monitor include:
- **CPU and Memory Utilization:** Track the resource utilization of your pods and nodes to understand how they correlate with scaling events.
- **Pod Count:** Monitor the number of pod replicas over time to see how HPA is responding to changes in load.
- **Scaling Events:** Keep an eye on scaling events triggered by HPA and VPA to ensure they align with expected behavior.
- **Application Performance Metrics:** Track application-specific metrics such as response times, error rates, and throughput.

Based on the insights gained from monitoring, you may need to adjust your autoscaling configurations:
- **Refine Thresholds:** If you notice frequent scaling actions or periods of underutilization or overutilization, adjust the thresholds used by HPA and VPA to better match the workload patterns.
- **Update Policies:** Modify VPA update policies if you observe that the current settings are causing too much or too little pod disruption.
- **Custom Metrics:** If using custom metrics, ensure they accurately reflect the load on your application and adjust them if they do not.
- **Scaling Limits:** Review and adjust the minimum and maximum scaling limits to prevent over-scaling or under-scaling based on the capacity of your cluster and the criticality of your applications.

# Quality of Service (QoS)

A few words on the [quality of service for pods](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod). Basically, there are 3 classes of QoS and they influence the eviction of pods when `kube-reserved` is underrun, i.e. [node-level resources are running low](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#pod-selection-for-kubelet-eviction):
- [`BestEffort`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#besteffort), i.e. pods where no container has CPU or memory requests or limits: **Avoid them** unless you have really good reasons. The kube-scheduler will place them just anywhere according to its policy, e.g. `balanced` or `bin-packing`, but whatever resources these pods consume, may bring other pods into trouble or even the kubelet and the container runtime itself, if it happens very suddenly.
- [`Burstable`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#burstable), i.e. pods where at least one container has CPU or memory requests and at least one has no limits or limits that don't match the requests: **Prefer them** unless you have really good reasons for the other QoS classes. Always specify proper requests or use VPA to recommend those. [This helps the kube-scheduler to make the right scheduling decisions](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#how-pods-with-resource-requests-are-scheduled). Not having limits will additionally provide upward resource flexibility, if the node is not under pressure.
- [`Guaranteed`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed), i.e. pods where all containers have CPU and memory requests and equal limits: **Avoid them** unless you really know the limits or throttling/killing is intended. While "Guaranteed" sounds like something "positive" in the English language, this class comes with the downside, that pods will be actively CPU-throttled and will actively go OOM, even if the node is not under pressure and has excess capacity left. Worse, if containers in the pod are under VPA, their CPU requests/limits will often not be scaled up as CPU throttling will go unnoticed by VPA.

# Summary

- As a rule of thumb, always set CPU and memory requests (or let VPA do that) and always avoid CPU and memory limits.
  - CPU limits aren't helpful on an under-utilized node (=may result in needless outages) and even suppress the signals for VPA to act. On a nearly or fully utilized node, CPU limits are practically irrelevant as only the requests matter, which are translated into CPU shares that provide a fair use of the CPU anyway (see [CFS](https://docs.kernel.org/scheduler/sched-design-CFS.html)).<br/>
  Therefore, if you do not know the healthy range, do not set CPU limits. If you as author of the source code know its healthy range, set them to the upper threshold of that healthy range (everything above, from your knowledge of that code, is definitely an unbound busy loop or similar, which is the main reason for CPU limits, besides batch jobs where throttling is acceptable or even desired).
  - Memory limits may be more useful, but suffer a similar, though not as negative downside. As with CPU limits, memory limits aren't helpful on an under-utilized node (=may result in needless outages), but different than CPU limits, they result in an OOM, which triggers VPA to provide more memory suddenly (modifies the currently computed recommendations by a configurable factor, defaulting to +20%, see [docs](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/README.md#custom-memory-bump-up-after-oomkill)).<br/>
  Therefore, if you do not know the healthy range, do not set memory limits. If you as author of the source code know its healthy range, set them to the upper threshold of that healthy range (everything above, from your knowledge of that code, is definitely an unbound memory leak or similar, which is the main reason for memory limits)
- Horizontal Pod Autoscaling (HPA): Use for pods that support horizontal scaling. Prefer scaling on usage, not utilization, as this is more predictable (not dependent on a second variable, namely the current requests) and conflict-free with vertical pod autoscaling (VPA).
- As a rule of thumb, set the initial replicas to the 5th percentile of the actually observed replica count in production. Since HPA reacts fast, this is not as critical, but may help reduce initial load on the control plane early after deployment. However, be cautious when you update the higher-level resource not to inadvertently reset the current HPA-controlled replica count (very easy to make mistake that can lead to catastrophic loss of pods). HPA modifies the replica count directly in the spec and you do not want to overwrite that. Even if it reacts fast, it is not instant (not via a mutating webhook as VPA operates) and the damage may already be done.
- As for minimum and maximum, let your high availability requirements determine the minimum and your theoretical maximum load determine the maximum, flanked with alerts to detect erroneous run-away out-scaling or the actual nearing of your practical maximum load, so that you can intervene.
- Vertical Pod Autoscaling (VPA): Use for containers that have a significant usage (e.g. any container above 50m CPU or 100M memory) and a significant usage spread over time (by more than 2x), i.e. ignore small (e.g. side-cars) or static (e.g. Java statically allocated heap) containers, but otherwise use it to provide the resources needed on the one hand and keep the costs in check on the other hand.
- As a rule of thumb, set the initial requests to the 5th percentile of the actually observed CPU resp. memory usage in production. Since VPA may need some time at first to respond and evict pods, this is especially critical early after deployment. The lower bound, below which pods will be immediately evicted, converges much faster than the upper bound, above which pods will be immediately evicted, but it isn't instant, e.g. after 5 minutes the lower bound is just at 60% of the computed lower bound; after 12 hours the upper bound is still at 300% of the computed upper bound (see [code](https://github.com/kubernetes/autoscaler/blob/b3a501cbe11e46bea1f8879d39c8436ef03e7139/vertical-pod-autoscaler/pkg/recommender/logic/recommender.go#L118-L143)). Unlike with HPA, you don't need to be as cautious when updating the higher-level resource in the case of VPA. As long as VPA's mutating webhook (VPA Admission Controller) is operational (which also the VPA Updater checks before evicting pods), it's generally safe to update the higher-level resource. However, if it's not up and running, any new pods that are spawned (e.g. as a consequence of a rolling update of the higher-level resource or for any other reason) will not be mutated. Instead, they will receive whatever requests are currently configured at the higher-level resource, which can lead to catastrophic resource under-reservation. Gardener deploys the VPA Admission Controller in HA - if unhealthy, it is reported under the `ControlPlaneHealthy` shoot status condition.
- As a rule of thumb, for a container under VPA always specify initial resource requests for the resources which are controlled by VPA (the `controlledResources` field; by default both cpu and memory are controlled). vpa-updater evicts immediately if a container does not specify initial resource requests for a resource controlled by VPA (see [code](https://github.com/kubernetes/autoscaler/blob/00f627fbb9f4dfba952601d1e8c4c781eb423772/vertical-pod-autoscaler/pkg/updater/priority/priority_processor.go#L79-L86)).
- As a rule of thumb, for a container under VPA do not set initial requests less than VPA's `minAllowed` or vpa-recommender's Pod minimum recommendation (`10m` and `10Mi`). In Gardener, vpa-recommender is configured to run with `--pod-recommendation-min-cpu-millicores=10` and `--pod-recommendation-min-memory-mb=10` (see [code](https://github.com/gardener/gardener/blob/b3a501cbe11e46bea1f8879d39c8436ef03e7139/pkg/component/autoscaling/vpa/recommender.go#L325-L326)). These values are synced with the smallest histogram bucket sizes - `10m` and `10M` (see [VPA Recommender Options](#vpa-recommender-options)). Note that the `--pod-recommendation-min-memory-mb` flag is in mebibytes, not megabytes (see [code](https://github.com/kubernetes/autoscaler/blob/b3a501cbe11e46bea1f8879d39c8436ef03e7139/vertical-pod-autoscaler/pkg/recommender/logic/recommender.go#L73)). If a Pod has a container under VPA with resource requests less than the VPA's `minAllowed` or `10m` and `10Mi`, the Pod will be evicted immediately. The reason is that the VPA's lower bound recommendation is set to `minAllowed` or `10m` and `10Mi`. vpa-updater evicts immediately if the container's resource requests are "out of bounds" (in this particular case - less than the lower bound).
- If you have defined absolute limits (unrelated to the requests), configure VPA to only scale the requests or else it will proportionally scale the limits as well, which can easily become useless (way beyond the threshold of unhealthy behavior) or absurd (larger than node capacity):
  ```
  spec:
    resourcePolicy:
      containerPolicies:
      - controlledValues: RequestsOnly
        ...
  ```
  If you have defined relative limits (related to the requests), the default policy to scale the limits proportionally with the requests is fine, but the gap between requests and limits must be zero for QoS [`Guaranteed`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed) and should best be small for QoS [`Burstable`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#burstable) to avoid useless or absurd limits either, e.g. prefer limits being 5 to at most 20% larger than requests as opposed to being 100% larger or more.
- As a rule of thumb, set `minAllowed` to the highest observed VPA recommendation (usually during the initialization phase or during any periodical activity) for an otherwise practically idle container, so that you avoid needless trashing (e.g. resource usage calms down over time and recommendations drop consecutively until eviction, which will then lead again to initialization or later periodical activity and higher recommendations and new evictions).<br/>⚠️ You may want to provide higher `minAllowed` values, if you observe that up-scaling takes too long for CPU or memory for a too large percentile of your workload. This will get you out of the danger zone of too few resources for too many pods at the expense of providing too many resources for a few pods. Memory may react faster than CPU, because CPU throttling is not visible and memory gets aided by OOM bump-up incidents, but still, if you observe that up-scaling takes too long, you may want to increase `minAllowed` accordingly.
- As a rule of thumb, set `maxAllowed` to your theoretical maximum load, flanked with alerts to detect erroneous run-away usage or the actual nearing of your practical maximum load, so that you can intervene. However, VPA can easily recommend requests larger than what is allocatable on a node, so you must either ensure large enough nodes (Gardener can scale up from zero, in case you like to [define a low-priority worker pool with more resources](https://gardener.cloud/docs/getting-started/features/cluster-autoscaler/#scaling-by-priority) for very large pods) and/or cap VPA's target recommendations using `maxAllowed` at the node allocatable remainder (after daemon set pods) of the largest eligible machine type (may result in under-provisioning resources for a pod). Use your monitoring and check maximum pod usage to decide about the maximum machine type.

## Recommendations in a Box

| Container | When to use | Value |
|-----------|-------------|-------|
| Requests | - **Set them (recommended)** unless:<br/>- Do not set requests for QoS [`BestEffort`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#besteffort); useful only if pod can be evicted as often as needed **and** pod can pick up where it left off without any penalty | Set `requests` to **95th percentile (w/o VPA)** of the actually observed CPU resp. memory usage in production resp. **5th percentile (w/ VPA)** (see below) |
| Limits | - **Avoid them (recommended)** unless:<br/>- Set limits for QoS [`Guaranteed`](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed); useful only if pod has strictly static resource requirements<br/>- Set CPU limits if you want to throttle CPU usage for containers that can be throttled w/o any other disadvantage than processing time (never do that when time-critical operations like leases are involved)<br/>- Set limits if you know the healthy range and want to shield against unbound busy loops, unbound memory leaks, or similar | If you really can (otherwise not), set `limits` to healthy theoretical max load |

| Scaler | When to use | Initial | Minimum | Maximum |
|--------|-------------|---------|---------|---------|
| HPA | **Use for pods that support horizontal scaling** | Set initial `replicas` to 5th percentile of the actually observed replica count in production (prefer scaling on usage, not utilization) and make sure to never overwrite it later when controlled by HPA | Set `minReplicas` to 0 (requires [feature gate](https://github.com/kubernetes/kubernetes/blob/4c7960a67c29b7954cccc6c7d77a62133af3484f/pkg/features/kube_features.go#L266-L267) and [custom/external metrics](https://github.com/kubernetes/kubernetes/pull/74526)), to 1 (regular HPA minimum), or whatever the high availability requirements of the workload demand | Set `maxReplicas` to healthy theoretical max load |
| VPA | **Use for containers that have a significant usage** (>50m/100M) **and a significant usage spread over time** (>2x) | Set initial `requests` to 5th percentile of the actually observed CPU resp. memory usage in production | Set `minAllowed` to highest observed VPA recommendation (includes start-up phase) for an otherwise practically idle container (avoids pod trashing when pod gets evicted after idling) | Set `maxAllowed` to fresh node allocatable remainder after daemonset pods (avoids pending pods when requests exceed fresh node allocatable remainder) or, if you really can (otherwise not), to healthy theoretical max load (less disruptive than limits as no throttling or OOM happens on under-utilized nodes) |
| CA | **Use for dynamic workloads**, definitely if you use HPA and/or VPA | N/A | Set `minimum` to 0 or number of nodes required right after cluster creation or wake-up | Set `maximum` to healthy theoretical max load |

> [!NOTE]
> Theoretical max load may be very difficult to ascertain, especially with modern software that consists of building blocks you do not own or know in detail. If you have comprehensive monitoring in place, you may be tempted to pick the observed maximum and add a safety margin or even factor on top (2x, 4x, or any other number), but this is not to be confused with "theoretical max load" (solely depending on the code, not observations from the outside). At any point in time, your numbers may change, e.g. because you updated a software component or your usage increased. If you decide to use numbers that are set based only on observations, make sure to flank those numbers with monitoring alerts, so that you have sufficient time to investigate, revise, and readjust if necessary.

# Conclusion

Pod autoscaling is a dynamic and complex aspect of Kubernetes, but it is also one of the most powerful tools at your disposal for maintaining efficient, reliable, and cost-effective applications. By carefully selecting the appropriate autoscaler, setting well-considered thresholds, and continuously monitoring and adjusting your strategies, you can ensure that your Kubernetes deployments are well-equipped to handle your resource demands while not over-paying for the provided resources at the same time.

As Kubernetes continues to evolve (e.g. [in-place updates](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/1287-in-place-update-pod-resources/README.md)) and as new patterns and practices emerge, the approaches to autoscaling may also change. However, the principles discussed above will remain foundational to creating scalable and resilient Kubernetes workloads. Whether you're a developer or operations engineer, a solid understanding of pod autoscaling will be instrumental in the successful deployment and management of containerized applications.