---
title: Autoscaling Shoot `kube-apiserver` via Independently Driven HPA and VPA
gep-number: 0023
creation-date: 2023-01-31
status: implementable
authors:
- "@andrerun"
- "@voelzmo"
reviewers:
- "@oliver-goetz"
- "@timebertt"
---

# GEP-23: Autoscaling Shoot kube-apiserver via Independently Driven HPA and VPA

## Table of Contents
- [GEP-23: Autoscaling Shoot kube-apiserver via Independently Driven HPA and VPA](#gep-23-autoscaling-shoot-kube-apiserver-via-independently-driven-hpa-and-vpa)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Existing Solution](#existing-solution)
    - [Deficiencies of the Existing Solution](#deficiencies-of-the-existing-solution)
      - [Algorithmic limitation: fusing the behavior of two blackbox systems](#algorithmic-limitation-fusing-the-behavior-of-two-blackbox-systems)
      - [HVPA stuck in unintended stable equilibrium due to feedback of non-deterministic "replica count" signal](#hvpa-stuck-in-unintended-stable-equilibrium-due-to-feedback-of-non-deterministic-replica-count-signal)
      - [Maintaining inactive code](#maintaining-inactive-code)
      - [Inefficiency induced by initial horizontal-only scaling stage](#inefficiency-induced-by-initial-horizontal-only-scaling-stage)
      - ['MinChange' stabilisation prevents response to node CPU exhaustion](#minchange-stabilisation-prevents-response-to-node-cpu-exhaustion)
      - [HPA-VPA policy fusion model does not account for multiplicative interaction](#hpa-vpa-policy-fusion-model-does-not-account-for-multiplicative-interaction)
    - [Other Benefits to Replacing the Existing Solution](#other-benefits-to-replacing-the-existing-solution)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [Rationale](#rationale)
    - [Design Outline](#design-outline)
    - [Element: Gardener Custom Metrics Provider Component](#element-gardener-custom-metrics-provider-component)
    - [Element: New Custom Pod Metric for ShootKapis](#element-new-custom-pod-metric-for-shootkapis)
    - [Element: HPA](#element-hpa)
    - [Element: VPA](#element-vpa)
    - [Transition Strategy](#transition-strategy)
  - [Discussion and Limitations](#discussion-and-limitations)
      - [Gardener-custom metrics precludes the use of other sources of custom metrics](#gardener-custom-metrics-precludes-the-use-of-other-sources-of-custom-metrics)
      - [Observed fluctuations in actual compute efficiency affecting ShootKapi workloads](#observed-fluctuations-in-actual-compute-efficiency-affecting-shootkapi-workloads)
  - [Alternatives](#alternatives)
  - [References](#references)

## Summary
When it comes to autoscaling shoot control plane `kube-apiserver` instances (ShootKapi hereafter), Gardener needs
both stability and efficiency (accurate scaling). The existing approach of fusing HPA and VPA into the 2-dimensional
autoscaler which is HVPA, poses severe algorithmic limitations which manifest as stability and efficiency issues in the
field. 

This document outlines the main deficiencies in the existing solution and proposes a simple design which overcomes them,
reduces maintenance effort by reducing Gardener's custom code base, and enables subsequent reduction in ShootKapi
compute cost.

## Motivation
Gardener uses a relatively complex approach for scaling shoot control plane `kube-apiserver` instances (ShootKapi).
This need is a consequence of the following concerns:
1. Gardener supplies K8s clusters of highly diverse compute scale and utilization - ranging from a couple of small
   practically idle nodes, to hundreds of heavily utilized giants. Furthermore, the ratio of 'cluster compute capacity'
   to 'ShootKapi load' substantially varies between different applications utilizing Gardener shoots. As a result,
   compute resources required by ShootKapis of different shoots vary by two orders of magnitude.
2. The compute cost of ShootKapi workloads constitutes a major part of Gardener's overall compute cost. Small relative
   waste in sizing ShootKapis results in considerable absolute cost.
3. Kube-apiserver does not scale well horizontally, in a sense that extra replicas come with substantial compute overhead.   

In short, ShootKapi needs to be scaled:
- Within a broad range
- With reasonable accuracy, relative to actual resource consumption

The implication is that ShootKapi requires both horizontal and vertical scaling. The existing implementation utilized 
to that end is [HVPA].

### Existing Solution
[HVPA] strives to achieve a fusion of HPA and VPA recommendations:
`HvpaRecommendedCapacity = k * HpaRecommendedCapacity + (1 - k) * VpaRecommendedCapacity`
The underlying goal is to allow HPA and VPA to be driven by the same CPU and memory usage metrics, and overcome the
HPA-VPA incompatibility known to exist in that mode.

### Deficiencies of the Existing Solution
HVPA strives to achieve a linear combination of HPA's and VPA's scaling policies:<br/>
`HvpaRecommendedCapacity = k * HpaRecommendedCapacity + (1 - k) * VpaRecommendedCapacity`<br/>
However, due to technical factors discussed below, it is not possible to fulfil this goal. At different
points in time, HVPA ends up mixing horizontal and vertical scaling in proportion which arbitrarily varies in the
entire range between horizontal-only scaling and vertical-only scaling.

#### Algorithmic limitation: fusing the behavior of two blackbox systems 
There is a nonobvious obstacle before the intent to fuse HPA and VPA behavior in a pre-selected proportion:
Both HPA and VPA's policies are practically unavailable -
HVPA does not know how HPA/VPA would scale the workload, as function of circumstances. The only information available
is "how would HPA/VPA change the existing workload right now". So, in place of the original intent, HVPA resorts to:
`dHvpaRecommendedCapacity = k * dHpaRecommendedCapacity + (1 - k) * dVpaRecommendedCapacity` (the 'd' symbols stand for
"rate of change").

Had HPA and VPA recommendations been always deterministically available, up to date, and with absolute accuracy,
that would result in the originally intended behavior. Simply put, in a 3-D space where one coordinate is the real-world
utilization data, and the other two are the horizontal and vertical scale suggested by the recommender, this is a line -
a line which is a weighted average of the two lines which are individual HPA and VPA recommendations (Fig. 1).

![HVPA scaling space](./assets/gep23-hvpa-scaling-space.png "HVPA scaling space")

_Fig. 1: HVPA policy as a weighted average of HPA and VPA policies. HPA and VPA policies in blue. HVPA intended policy
in green lies in the plane defined by HPA and VPA. In orange, a vertical-only and
horizontal-only scaling events, leading to deviation from the intended policy._

In practice though, while the HPA and VPA
abstract algorithms are deterministic, **when** they will be applied is not. At any point of the original "line", there
is a race between HPA and VPA, who will act first. If either HPA or VPA manages to act first, the line gets pulled only
in the direction of the horizontal or vertical scale axis. Not only is this new point not on the intended line, but this
new state feeds back into HPA and VPA's decisions, further influencing them. It is noteworthy that there is no force
acting toward convergence to the original line, because once a single-axis scaling event occurs, it alleviates the
pressure to scale on the other axis.

To illustrate the above, if VPA is slow, it is possible for the horizontal scale to reach its allowed maximum, with
vertical scaling never occurring.

In result, instead of a line in said space, the behavior of HVPA corresponds to an entire 2-D surface. For any value of
the actual workload resource consumption, there are multiple combinations of horizontal vs. vertical scale, which HVPA
would recommend, and the choice among these is accidental. Furthermore,
HPA and VPA do not act in accord, and when driven by the same input metrics, as is the case with
HVPA, they have different scaling trigger threshold levels. The mutual relation between these levels is entirely
accidental. Similarly, the resulting autoscaler lacks joint hysteresis behavior, and instead applies hysteresis
separately on the horizontal and vertical scaling axis, on arbitrarily aligned levels. 

The cumulative result is that HVPA's recommendations have intrinsic uncertainty between the choice of horizontal vs.
vertical scaling, and a unique risk of oscillating on closed contours on the aforementioned 2-D surface, circumventing
hysteresis thresholds, and arbitrarily trading horizontal for vertical scale and vice versa.

<details>
  <summary>Example: extreme single axis scaling (click to expand)</summary>

    As example of extreme single axis scaling, consider the following. A workload with constant memory consumption is
    being scaled by a HVPA configured at 50% HPA + 50% VPA, starting at 1 replica with 1 core. The expectation is that
    HVPA will maintain the number of replicas approximately equal to the number of cores per replica. Consider the
    following sequence of events, as CPU demand increases:
    1. CPU usage starts at 0.7 cores. HVPA is stable at 1 replica, 1 core
    2. CPU usage increases to 1.5 cores. HPA acts first, recommending 2 replicas, a change of +1. 50% of +1 is +0.5 replicas,
       rounded up to +1. HVPA applies +1 replica. The workload is now at Consumption=1.5,Replicas=2,CoresPerReplica=1
    3. VPA acts, but does not recommend upscaling because CPU is currently over-provisioned. It does not recommend
       downscaling because MinAllowed is 1 core.
    4. CPU usage increases to 2.1 cores. HPA acts first, recommending 3 replicas, a change of +1. HVPA applies
       2 + ceil(50% * +1) = 3 replicas. The workload is now at Consumption=2.1,Replicas=3,CoresPerReplica=1
    ...and so on. HVPA acts as 100% HPA + 0% VPA, instead of the configured 50%/50%.
</details>

#### HVPA stuck in unintended stable equilibrium due to feedback of non-deterministic "replica count" signal
HVPA is organized around a concept of multiple scaling stages, where each stage has a different quotient for mixing
HPA and VPA recommendations, and the choice of stage is determined by the number of replicas. The intent is to achieve
a "horizontal first" or "vertical first" type of behavior. The proper functioning of this behavior relies on HVPA
decremental scaling returning on approximately the same trajectory, which is followed during incremental scaling.
However, due to the intrinsic uncertainty between the choice of horizontal vs. vertical scaling, this is not the case.
For example, workloads with "horizontal first" HVPA, which applies only HPA recommendations at low replica counts,
have been occasionally observed to grow to a large number of large replicas, then to reduce the number of replicas
before they have been downscaled vertically, and to end up in the low replica count, "horizontal first" stage, where
vertical scaling is not applied, with large replica(s), preserving large idle pods indefinitely.

#### Maintaining inactive code
As a mitigation of the horizontal vs. vertical scaling uncertainty, it is currently prescribed that HVPA only be used
with weight coefficient (`k` in above formulae) of `100%` or `0%`. As far as HVPA core logic for fusing HPA and VPA is
concerned, that renders HVPA functionally equivalent to a much simpler implementation, reducing said core logic to
nothing more than unnecessary maintenance burden.

#### Inefficiency induced by initial horizontal-only scaling stage 
Kube-apiserver is currently scaled with HVPA strategy where the early stage is HPA-only and that stage results in an
excessive number of small replicas. VPA is unused until the number of replicas reaches the specified maximum,
and replicas retain their initial, minimal size. Kube-apiserver has low compute efficiency in that mode. As a mitigation,
a `ScalingClass` heuristic estimate is applied, to predict the overall kube-apiserver load for the cluster and set a
better, but still fixed, initial replica size. However, while this substantially improves efficiency, it is still less
efficient than using VPA, as it is impossible for a single, fixed value to match the computing resource needs of a given
cluster at all times, let alone of all clusters in the scaling class, at all times.

#### 'MinChange' stabilisation prevents response to node CPU exhaustion 
There is a known caveat in the current HVPA design, independent of what was discussed above, which has infrequent but
severe impact on system stability:
The vertical scaling stabilization algorithm used by HVPA revolves around a 'MinChange' concept, essentially suppressing
vertical scaling until the recommendation differs from the current allocation (request) by said amount.
Alone, this concept is functionally incorrect when applied to scale-up: scaling is suppressed in a state where VPA is
indicating that the workload is starving for resources. To mitigate this problem, the existing design relies on
resource headroom existing on the node, which can temporarily accommodate the starving workload's needs. In the field,
this approach has been repeatedly observed to cause problems for etcd, for which a particularly high value of MinChange
is used, as frequent evictions are highly undesirable. As an illustration, take a pod with MinChange for CPU set to
1000 millicores, current CPU request at X millicores, actual pod need at X+2000, and available node headroom at 500.
What happens is that actual pod usage extends as far as headroom allows, reaching X+500 and is starving for extra 1500
millicores. Based on pod usage at X+500, VPA recommends e.g. X+900. This recommendation is below the MinChange threshold
of X+1000. HVPA does not act, and the pod is permanently stuck starving for 1500 millicores of CPU.

#### HPA-VPA policy fusion model does not account for multiplicative interaction 
Lastly, HVPA applies an additive model to the interaction between HPA and VPA, with respect to provided computing power, but
that interaction is actually multiplicative. As an extreme example, take a workload running at one replica with one core.
Then consumption changes to 5 cores, leading HPA to recommend 5 single-core replicas - a change of +4 replicas. VPA
suggests adding 4 cores to the existing single replica. A HVPA configured at 50% HPA + 50% VPA, would apply half of
each recommendation, +2 cores and +2 replicas, resulting in (1 + 2) replicas * (1 + 2) cores. Failing to account for
the multiplicative interaction between changes, HVPA provides a total of 9 cores to a workload which only needs 5. 

### Other Benefits to Replacing the Existing Solution
HVPA applies recommendations via direct edits to the controlled object (e.g. `Deployment`). This approach requires
nonobvious special handling by Gardener code outside HVPA, which is error-prone.

### Goals
- Improve ShootKapi autoscaling stability by eliminating the stability issues existing in the current HVPA-based solution.
- Improve ShootKapi operational efficiency at low application load levels, by avoiding excessive replicas driven by
  HVPA's policy for early horizontal scale-out.
- Allow usage of VPA for right-sizing the resource requests during the entire time and not only under certain conditions.
- Enable future migration away from HVPA by removing all dependency on it for scaling ShootKapi.
- Reduce Gardener custom code base and ongoing maintenance effort.

### Non-Goals
There are other components currently scaled via HVPA. Changing the autoscaling mode of these, and of `kube-apiserver`s
other than the one in the shoot control plane, is outside the scope of this GEP. These workloads are subject to a
different mode of autoscaling (e.g. vertical-only, where HVPA is only used to stabilize VPA behavior), and it is
advantageous from project execution perspective to address them as a separate concern.

The scaling approach hereby proposed may be suitable for scaling the `virtual-garden-kube-apiserver` and
`gardener-apiserver` components, which are part of the virtual garden cluster control plane. Scaling those components
is outside the scope of this GEP.

## Proposal
ShootKapi HVPA is replaced by individual HPA and VPA. Undesirable interference between HPA and VPA is avoided through
the use of sufficiently independent driving signals. HPA is driven by the rate of HTTP requests to kube-apiserver. VPA is
driven by CPU and memory usage metrics.

### Rationale
A rough, heuristic horizontal sizing sets the stage for optimal VPA operation (horizontal size is within stable and
reasonably efficient range). Vertical scaling fine-tunes requests, ensuring high efficiency (requests not too high)
and stability (requests not too low).

### Design Outline
One HPA and one VPA resources are deployed alongside each ShootKapi deployment object. The VPA controls CPU and memory
requests but not limits. The HPA is driven by the average rate of API requests per pod of the target kube-apiserver
deployment.

A new component named gardener-custom-metrics is deployed on each seed. It directly scrapes metrics data from all
ShootKapi pods on the seed and derives custom metrics based on it. The component is registered as an extension API
service to the seed kube-apiserver. It occupies the custom metrics API extension point and is responsible for providing
all custom metrics for the seed kube-apiserver, including the one driving the ShootKapi HPA.

![Design outline](./assets/gep23-design-outline.png "Design outline")

_Fig 2: Proposed design_

### Element: Gardener Custom Metrics Provider Component 
A new component, named gardener-custom-metrics is added to seed clusters. It periodically scrapes the metrics endpoints
of all ShootKapi pods on the seed. It implements the K8s custom metrics API and provides K8s metrics specific to
Gardener, based on custom calculations. The proposed design can naturally be extended to multiple sources of
input data and a non-volatile cache for acquired data. However, in the scope of this proposal, the only data source
is the aforementioned metrics scrape, and the calculated values only need to be briefly cached in memory.

A kube-apiserver's metrics response measures megabytes in size. To reduce the amount of network traffic, this proposal
utilizes compressed HTTP responses when scraping metrics. Outside the scope of this proposal, a future enhancement is
possible, in which a metrics filter component is added as a sidecar to each ShootKapi pod. Each such sidecar would
mirror the metrics endpoint of its respective ShootKapi, with the only addition of support for filtering based on an
HTTP request parameter (e.g. `GET /metrics?name=apiserver_request_total` would forward a `GET /metrics` request to the
ShootKapi, and then would reduce the response to only `apiserver_request_total` counters, before passing it on to the
caller). 

#### High availability operation
A single gardener-custom-metrics replica does not satisfy Gardener's high availability (HA) requirements. Autoscaling
operations can tolerate brief (1-2 minutes) periods of metrics signal outage without disruption to autoscaling service
availability. However, node provisioning delays easily exceed that tolerable amount, and do not allow timely fail over
via creation of a new replica after the existing one fails.

For a seed in high availability mode, gardener-custom-metrics is deployed in a multi-replica active/passive arrangement,
based on the well established controller leader election mechanism used by other Gardener components.
Passive replicas do not respond to metrics requests. Readiness probes report "ready" status on the active replica,
and "not ready" on passive ones. All metrics requests are routed to the one active replica, by means of a K8s service
acting on the readiness probes.

Since gardener-custom-metrics replicas do not interfere with each other, an active/passive arrangement is not
necessitated by functional requirements. A simpler active/active is technically possible, but pragmatically undesirable
due to the substantial amount of cross availability zone network traffic each active replica generates.

### Element: New Custom Pod Metric for ShootKapis
A new `shoot:apiserver_request_total:rate` pod custom metric is made available for each ShootKapi pod on the
seed. It is provided by the gardener-custom-metrics component. It is the rate of API requests per second, broken down
by ShootKapi pod.

### Element: HPA
ShootKapi is scaled horizontally via HPA operating on the aforementioned average kube-apiserver request rate custom
metric. Scale-in is delayed by a stabilization window. This avoids unnecessary flapping, the effects of which would
be further amplified by VPA-driven evictions, and serves as a general precaution against HPA-VPA resonance,
by ensuring that the two autoscalers operate with different frequency.

The scaling threshold value for HPA is initially set to a conservative (lower-biased) estimate of the requests/replica
ratio observed with the existing solution. It will be then gradually fine-tuned to the maximum value which does not
impact quality of service.

This proposal is the first stage of an incremental, two stage approach. It aims primarily at resolving the existing
stability issues inherent to HVPA and enabling (but not executing) scaling efficiency optimizations. The second stage
will be focused solely on further fine-tuning scaling efficiency to optimize ShootKapi compute cost over HVPA.
Details follow in the [Discussion and Limitations](#discussion-and-limitations) section.

### Element: VPA
A standard VPA operates on ShootKapi. Its primary purpose is to improve resource efficiency over what's possible via
HPA's coarse, replica-granularity scaling, by shrinking replicas down to match actual utilization.

This improves over the existing solution by having vertical scaling active the entire time and not only during certain
periods.

### Transition Strategy
The following steps will be executed over time, and provide a non-disruptive transition from the existing implementation
to the one hereby proposed.

1. Proposed scaling approach will be deployed behind a feature gate and disabled by default. Existing HVPA feature flag
   will be preserved at this point, as its use is not limited to kube-apiserver scaling.
2. New feature gate gradually enabled until all ShootKapi workloads scaled by new approach (Note: this step applies to
   individual garden/seed instances, and not to the code base as a whole).
3. At this point, the preexisting HVPA feature flag has no effects on ShootKapi scaling (Note: this step applies to
   individual garden/seed instances, and not to the code base as a whole).
4. New feature flag promoted and eventually removed.
5. (Outside the scope of this proposal) Use of HVPA removed for other workloads and the preexisting HVPA flag removed.

## Discussion and Limitations
Overall, multidimensional autoscaling is a job which cannot be accomplished efficiently with the one-dimensional
autoscalers currently available in the K8s ecosystem. The proposed approach is a compromise which relies on estimating
actual compute resource demand based on an application metric (API request rate). Such estimate is inherently inaccurate,
creating a need for a safety margin (excess replicas), and a risk of resource exhaustion under highly unusual application
loads.

#### Gardener-custom metrics precludes the use of other sources of custom metrics
With current K8s design, only one API extension can serve custom metrics. Per this GEP, the custom metrics extension
point is occupied by gardener-custom-metrics. If necessary in the future, this limitation can be mitigated via
gardener-custom-metrics aggregating other sources of custom metrics, and selectively forwarding queries to them. 

#### Observed fluctuations in actual compute efficiency affecting ShootKapi workloads
There is a known source of horizontal scaling inefficiency, which presents an opportunity for substantial further
improvement. It is not addressed by this proposal and instead deferred to a separate implementation stage, as it
requires a strictly incremental effort on top of this GEP.

An approximate 50% reduction (factor 0.5) of the effective compute power per unit of CPU was occasionally observed on
some productive ShootKapi instances. The change occurs with a quick cutoff transition, and has been observed for periods
or up to 2 hours. This variance currently forces substantial safety margin upon HPA's target metric value, effectively
necessitating more replicas to be deployed than required during usual operation.

The source is believed to be competition for compute resources with sibling workloads, either on the K8s, or
infrastructure level, but the circumstances have not been conductive to investigation after the fact.
A subsequent stage, outside the scope of this GEP is planned to focus on understanding the root cause, and further
optimizing HPA's target metric value accordingly.

Once this biggest source of inefficiency has been resolved, yet another round of scaling performance fine-tuning
is planned, where instead of overall request rate, HPA will be driven by a weighted average, reflecting the different
compute cost of different request categories (e.g. a cluster-scoped LIST is more cpu intensive than a resource-scoped PUT).

## Alternatives
Below is a list of the most promising alternatives to the proposed approach. 
1. **Using VPA, plus a simple custom horizontal autoscaler which acts when VPA recommendation goes below or above a
   predetermined acceptable range:**
   Initial research indicates to be a promising solution. The solution described in this GEP was ultimately preferred
   because it does not require building custom components.   
2. **Adding the missing stabilization features to VPA via a custom recommender which is a minimal fork of the default 
   VPA recommender:**
   Not a standalone solution, but a potential part of any solution which leverages VPA but requires less disruptive
   scaling behavior. Research indicates to be a pragmatically viable solution, incurring minimal ongoing maintenance
   cost.

Continued use of (an improved) HVPA does not look promising, due to the inherent algorithmic issues described above. 

## References
- [HVPA]

[HVPA]: https://github.com/gardener/hvpa-controller
