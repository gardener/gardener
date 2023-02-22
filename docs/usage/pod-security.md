# Migrating from `PodSecurityPolicy`s to PodSecurity Admission Controller

Kubernetes has deprecated the `PodSecurityPolicy` API in `v1.21` and it will be removed in `v1.25`. With `v1.23`, a new feature called [`PodSecurity`](https://kubernetes.io/docs/concepts/security/pod-security-admission/) was promoted to beta. From `v1.25` onwards, there will be no API serving `PodSecurityPolicy`s, so you have to cleanup all the existing PSPs before upgrading your cluster. Detailed migration steps are described in [Migrate from PodSecurityPolicy to the Built-In PodSecurity Admission Controller](https://kubernetes.io/docs/tasks/configure-pod-container/migrate-from-psp/).

After migration, you should disable the `PodSecurityPolicy` admission plugin. To do so, you have to add: 
```yaml
admissionPlugins:
- name: PodSecurityPolicy
  disabled: true
```
in `spec.kubernetes.kubeAPIServer.admissionPlugins` field in the `Shoot` resource. Please refer the example `Shoot` manifest in [90-shoot.yaml](../../example/90-shoot.yaml).

Only if the `PodSecurityPolicy` admission plugin is disabled the cluster can be upgraded to `v1.25`.

> :warning: You should disable the admission plugin and wait until Gardener finishes at least one `Shoot` reconciliation before upgrading to `v1.25`. This is to make sure all the `PodSecurityPolicy` related resources deployed by Gardener are cleaned up.

## Admission Configuration for the `PodSecurity` Admission Plugin

If you wish to add your custom configuration for the `PodSecurity` plugin and your cluster version is `v1.23+`, you can do so in the Shoot spec under `.spec.kubernetes.kubeAPIServer.admissionPlugins` by adding:

```yaml
admissionPlugins:
- name: PodSecurity
  config:
    apiVersion: pod-security.admission.config.k8s.io/v1
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
    # - specific version like "v1.25"
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

⚠️ Note that the `pod-security.admission.config.k8s.io/v1` configuration requires `v1.25`+. For `v1.23` and `v1.24`, use `pod-security.admission.config.k8s.io/v1beta1`. For `v1.22`, use `pod-security.admission.config.k8s.io/v1alpha1`.

Also note that in `v1.22`, the feature gate `PodSecurity` is not enabled by default. You have to add:

```yaml
featureGates:
  PodSecurity: true
```

under `.spec.kubernetes.kubeAPIServer`.

For proper functioning of Gardener, `kube-system` namespace will also be automatically added to the `exemptions.namespaces` list.

## `.spec.kubernetes.allowPrivilegedContainers` in the Shoot Spec

If this field is set to `true` then all authenticated users can use the "gardener.privileged" `PodSecurityPolicy`, allowing full unrestricted access to Pod features. However, the `PodSecurityPolicy` admission plugin is removed in Kubernetes `v1.25` and `PodSecurity` has taken its place as its successor. Therefore, this field doesn't have any relevance in versions `>= v1.25` anymore. If you need to set a default pod admission level for your cluster, follow [this documentation](#admission-configuration-for-the-podsecurity-admission-plugin).

> **Note:** You should remove this field from the `Shoot` spec for `v1.24` clusters after migrating to the new `PodSecurity` admission controller, before upgrading your cluster to `v1.25`.
