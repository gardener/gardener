# Audit a Kubernetes Cluster

The shoot cluster is a kubernetes cluster and its `kube-apiserver` handles the audit events. In order to define which audit events must be logged, a proper audit policy file must be passed to the kubernetes API server. You could find more information about auditing a kubernetes cluster [here](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/).

## Default Audit Policy

By default, the Gardener will deploy the shoot cluster with  audit policy defined in the [kube-apiserver chart](https://github.com/gardener/gardener/blob/master/charts/seed-controlplane/charts/kube-apiserver/templates/audit-policy.yaml).

## Custom Audit Policy

If you need specific audit policy for your shoot cluster, then you could deploy the required audit policy in the garden cluster as `ConfigMap` resource and set up your shoot to refer this `ConfigMap`. Note, the policy must be stored under the key `policy` in the data section of the `ConfigMap`.

For example, deploy the auditpolicy `ConfigMap` in the same namespace as your `Shoot` resource:

```
kubectl apply -f $GOPATH/src/github.com/gardener/gardener/example/95-configmap-custom-audit-policy.yaml
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

The Gardener validate the `Shoot` resource to refer only existing `ConfigMap` containing valid audit policy, and rejects the `Shoot` on failure.
If you want to switch back to the default audit policy, you have to remove the section

```yaml
auditPolicy:
  configMapRef:
    name: <configmap-name>
```

from the shoot spec.

## Change Audit Policy on the Fly

The Gardener is watching for changes in the referred `ConfigMap` containing the audit policy. Hence, once the audit policy is modified, the Gardener will schedule for reconciliation the affected Shoots and will apply the new audit policy.