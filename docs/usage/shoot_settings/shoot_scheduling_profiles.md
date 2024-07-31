---
description: Introducing `balanced` and `bin-packing` scheduling profiles 
weight: 12
---

# Shoot Scheduling Profiles

This guide describes the available scheduling profiles and how they can be configured in the Shoot cluster. It also clarifies how a custom scheduling profile can be configured.

## Scheduling Profiles

The scheduling process in the [kube-scheduler](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-scheduler/) happens in a series of stages. A [scheduling profile](https://kubernetes.io/docs/reference/scheduling/config/#profiles) allows configuring the different stages of the scheduling.

As of today, Gardener supports two predefined scheduling profiles:

- `balanced` (default)

   **Overview** 

   The `balanced` profile attempts to spread Pods evenly across Nodes to obtain a more balanced resource usage. This profile provides the default kube-scheduler behavior.
   
   **How it works?**
   
   The kube-scheduler is started without any profiles. In such case, by default, one profile with the scheduler name `default-scheduler` is created. This profile includes the default plugins. If a Pod doesn't specify the `.spec.schedulerName` field, kube-apiserver sets it to `default-scheduler`. Then, the Pod gets scheduled by the `default-scheduler` accordingly.
  
- `bin-packing`

   **Overview**

   The `bin-packing` profile scores Nodes based on the allocation of resources. It prioritizes Nodes with the most allocated resources. By favoring the Nodes with the most allocation, some of the other Nodes become under-utilized over time (because new Pods keep being scheduled to the most allocated Nodes). Then, the cluster-autoscaler identifies such under-utilized Nodes and removes them from the cluster. In this way, this profile provides a greater overall resource utilization (compared to the `balanced` profile).

   > **Note:** The decision of when to remove a Node is a trade-off between optimizing for utilization or the availability of resources. Removing under-utilized Nodes improves cluster utilization, but new workloads might have to wait for resources to be provisioned again before they can run.

   **How it works?**
   
   The kube-scheduler is configured with the following bin packing profile:

   ```yaml
   apiVersion: kubescheduler.config.k8s.io/v1beta3
   kind: KubeSchedulerConfiguration
   profiles:
   - schedulerName: bin-packing-scheduler
     pluginConfig:
     - name: NodeResourcesFit
       args:
         scoringStrategy:
           type: MostAllocated
     plugins:
       score:
         disabled:
         - name: NodeResourcesBalancedAllocation
   ```

   To impose the new profile, a `MutatingWebhookConfiguration` is deployed in the Shoot cluster. The `MutatingWebhookConfiguration` intercepts `CREATE` operations for Pods and sets the `.spec.schedulerName` field to `bin-packing-scheduler`. Then, the Pod gets scheduled by the `bin-packing-scheduler` accordingly. Pods that specify a custom scheduler (i.e., having `.spec.schedulerName` different from `default-scheduler` and `bin-packing-scheduler`) are not affected.

## Configuring the Scheduling Profile

The scheduling profile can be configured via the `.spec.kubernetes.kubeScheduler.profile` field in the Shoot:

```yaml
spec:
  # ...
  kubernetes:
    kubeScheduler:
      profile: "balanced" # or "bin-packing"
```

## Custom Scheduling Profiles

The kube-scheduler's component configs allows configuring custom scheduling profiles to match the cluster needs. As of today, Gardener supports only two predefined scheduling profiles. The profile configuration in the component config is quite expressive and it is not possible to easily define profiles that would match the needs of every cluster. Because of these reasons, there are no plans to add support for new predefined scheduling profiles. If a cluster owner wants to use a custom scheduling profile, then they have to deploy (and maintain) a dedicated kube-scheduler deployment in the cluster itself.
