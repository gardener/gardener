# Migrating From `PodSecurityPolicy`s To PodSecurity Admission Controller

Kubernetes has deprecated the `PodSecurityPolicy` API in v1.21 and it will be removed in v1.25. With v1.23, a new feature called [`PodSecurity`](https://kubernetes.io/docs/concepts/security/pod-security-admission/) was promoted to beta. From `v1.25` onwards, there will be no API serving `PodSecurityPolicy`s, so you have to cleanup all the existing PSPs before upgrading your cluster. Detailed migration steps are described [here](https://kubernetes.io/docs/tasks/configure-pod-container/migrate-from-psp/).

After migration, you should disable the `PodSecurityPolicy` admission plugin. To do so, you have to add: 
```yaml
admissionPlugins:
- name: PodSecurityPolicy
  disabled: true
```
in `spec.kubernetes.kubeAPIServer.admissionPlugins` field in the `Shoot` resource. Please refer the example `Shoot` manifest [here](../../example/90-shoot.yaml).

Only if the `PodSecurityPolicy` admission plugin is disabled the cluster can be upgraded to `v1.25`.

> :warning: You should disable the admission plugin and wait until Gardener finish at least one `Shoot` reconciliation before upgrading to `v1.25`. This is to make sure all the `PodSecurityPolicy` related resources deployed by Gardener are cleaned up.

## Admission configuration for the `PodSecurity` admission plugin

If you wish to add your custom configuration for the `PodSecurity` plugin, you can do so in the Shoot spec under `.spec.kubernetes.kubeAPIServer.admissionPlugins` by adding:
```yaml
admissionPlugins:
- name: PodSecurity
  config:
    apiVersion: pod-security.admission.config.k8s.io/v1beta1
    kind: PodSecurityConfiguration
    # Defaults applied when a mode label is not set.
    #
    # Level label values must be one of:
    # - "privileged" (default)
    # - "baseline"
    # - "restricted"
    #
    # Version label values must be one of:
    # - "latest" (default) 
    # - specific version like "v1.24"
    defaults:
      enforce: "privileged"
      enforce-version: "latest"
      audit: "privileged"
      audit-version: "latest"
      warn: "privileged"
      warn-version: "latest"
    exemptions:
      # Array of authenticated usernames to exempt.
      usernames: []
      # Array of runtime class names to exempt.
      runtimeClasses: []
      # Array of namespaces to exempt.
      namespaces: []
```
For proper functioning of Gardener, if not present, `kube-system` namespace will be added exempted to the exempted namespaces by Gardener.