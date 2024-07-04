---
title: Audit a Kubernetes Cluster
description: How to define a Custom Audit Policy through a `ConfigMap` and reference it in the shoot spec
---

# Audit a Kubernetes Cluster

The shoot cluster is a Kubernetes cluster and its `kube-apiserver` handles the audit events. In order to define which audit events must be logged, a proper audit policy file must be passed to the Kubernetes API server. You could find more information about auditing a kubernetes cluster in the [Auditing](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/) topic.

## Default Audit Policy

By default, the Gardener will deploy the shoot cluster with audit policy defined in the [kube-apiserver package](../../pkg/component/kubernetes/apiserver/secrets.go).

## Custom Audit Policy

If you need specific audit policy for your shoot cluster, then you could deploy the required audit policy in the garden cluster as `ConfigMap` resource and set up your shoot to refer this `ConfigMap`. Note that the policy must be stored under the key `policy` in the data section of the `ConfigMap`.

For example, deploy the auditpolicy `ConfigMap` in the same namespace as your `Shoot` resource:

```bash
kubectl apply -f example/95-configmap-custom-audit-policy.yaml
```

then set your shoot to refer that `ConfigMap` (only related fields are shown):

```yaml
spec:
  kubernetes:
    kubeAPIServer:
      auditConfig:
        auditPolicy:
          configMapRef:
            name: auditpolicy
```

Gardener validate the `Shoot` resource to refer only existing `ConfigMap` containing valid audit policy, and rejects the `Shoot` on failure.
If you want to switch back to the default audit policy, you have to remove the section

```yaml
auditPolicy:
  configMapRef:
    name: <configmap-name>
```

from the shoot spec.

## Rolling Out Changes to the Audit Policy

Gardener is not automatically rolling out changes to the Audit Policy to minimize the amount of Shoot reconciliations in order to prevent cloud provider rate limits, etc.
Gardener will pick up the changes on the next reconciliation of Shoots referencing the Audit Policy ConfigMap.
If users want to immediately rollout Audit Policy changes, they can manually trigger a Shoot reconciliation as described in [triggering an immediate reconciliation](shoot_operations.md#immediate-reconciliation).
This is similar to changes to the cloud provider secret referenced by Shoots.
