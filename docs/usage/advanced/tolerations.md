---
title: Taints and Tolerations for Seeds and Shoots
---

# Taints and Tolerations for `Seed`s and `Shoot`s

Similar to [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) for `Node`s and `Pod`s in Kubernetes, the `Seed` resource supports specifying taints (`.spec.taints`, see [this example](../../../example/50-seed.yaml#L48-L55)) while the `Shoot` resource supports specifying tolerations (`.spec.tolerations`, see [this example](../../../example/90-shoot.yaml#L268-L269)).
The feature is used to control scheduling to seeds as well as decisions whether a shoot can use a certain seed.

Compared to Kubernetes, Gardener's taints and tolerations are very much down-stripped right now and have some behavioral differences.
Please read the following explanations carefully if you plan to use them.

## Scheduling

When scheduling a new shoot, the gardener-scheduler will filter all seed candidates whose taints are not tolerated by the shoot.
As Gardener's taints/tolerations don't support `effect`s yet, you can compare this behaviour with using a `NoSchedule` effect taint in Kubernetes.
   
Be reminded that taints/tolerations are no means to define any affinity or selection for seeds - please use `.spec.seedSelector` in the `Shoot` to state such desires.

⚠️ Please note that - unlike how it's implemented in Kubernetes - a certain seed cluster **may** only be used when the shoot tolerates **all** the seed's taints.
This means that specifying `.spec.seedName` for a seed whose taints are not tolerated will make the gardener-apiserver reject the request.

Consequently, the taints/tolerations feature can be used as means to restrict usage of certain seeds.

## Toleration Defaults and Whitelist

The `Project` resource features a `.spec.tolerations` object that may carry `defaults` and a `whitelist` (see [this example](../../../example/05-project-dev.yaml#L33-L37)).
The corresponding `ShootTolerationRestriction` admission plugin (cf. Kubernetes' `PodTolerationRestriction` admission plugin) is responsible for evaluating these settings during creation/update of `Shoot`s.

### Whitelist

If a shoot gets created or updated with tolerations, then it is validated that only those tolerations may be used that were added to either a) the `Project`'s `.spec.tolerations.whitelist`, or b) to the global whitelist in the `ShootTolerationRestriction`'s admission config (see [this example](../../../example/20-admissionconfig.yaml#L7-L14)).

⚠️ Please note that the tolerations whitelist of `Project`s can only be changed if the user trying to change it is bound to the `modify-spec-tolerations-whitelist` custom RBAC role, e.g., via the following `ClusterRole`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: full-project-modification-access
rules:
- apiGroups:
  - core.gardener.cloud
  resources:
  - projects
  verbs:
  - create
  - patch
  - update
  - modify-spec-tolerations-whitelist
  - delete
```  

### Defaults

If a shoot gets created, then the default tolerations specified in both the `Project`'s `.spec.tolerations.defaults` and the global default list in the `ShootTolerationRestriction` admission plugin's configuration will be added to the `.spec.tolerations` of the `Shoot` (unless it already specifies a certain key).
