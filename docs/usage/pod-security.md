# Migrating to PodSecurity

Kubernetes has deprecated `PodSecurityPolicy`s in v1.21 and it will be removed in v1.25. With v1.23, a new feature called [`PodSecurity`](https://kubernetes.io/docs/concepts/security/pod-security-admission/) was promoted to beta. From `v1.25` onwards, there will be no API serving `PodSecurityPolicy`, so you have to cleanup all the existing PSPs before upgrading their cluster. Detailed migration steps are described [here](https://kubernetes.io/docs/tasks/configure-pod-container/migrate-from-psp/).

After migration, you should disable the `PodSecurityPolicy` admission plugin. For this you have to add: 
```
admissionPlugins:
- name: PodSecurityPolicy
  disabled: true
```
in `kubernetes.kubeAPIServer.AdmissionPlugins` field in the shoot spec. Please refer the example shoot manifest [here](../../example/90-shoot.yaml).

Only if this field is set, the cluster can be upgraded to kubernetes `v1.25`.

<strong>Note:</strong> You should disable the admission plugin and wait till Gardener finish at least one shoot reconciliation before upgrading to `v1.25`. This is to make sure all the `PodSecurityPolicy` related resources deployed by Gardener are cleaned up.
