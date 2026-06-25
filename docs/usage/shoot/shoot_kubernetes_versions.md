---
title: Shoot cluster supported Kubernetes versions and specifics
description: Defining the differences and requirements for upgrading to a supported Kubernetes version
---

# Shoot Kubernetes Minor Version Upgrades

Breaking changes may be introduced with new Kubernetes versions.
This documentation describes the Gardener specific differences and requirements for upgrading to a supported Kubernetes version.
For Kubernetes specific upgrade notes the upstream Kubernetes release notes, [changelogs](https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG) and release blogs should be considered before upgrade.

## Upgrading to Kubernetes `v1.36`

- **`StrictIPCIDRValidation` is now enabled by default.** The kube-apiserver rejects IPs and CIDRs with leading zeros (e.g. `010.0.0.1` or `10.0.0.0/024`). `Shoot` owners must ensure all network CIDRs in their `Shoot` spec do not contain leading zeros before upgrading.
- **`MaxUnavailableStatefulSet` is disabled by default again.** This feature gate was unintentionally enabled by default in Kubernetes v1.35.0–v1.35.3, causing a StatefulSet rolling-update deadlock ([kubernetes#137409](https://github.com/kubernetes/kubernetes/issues/137409)). It is disabled by default in v1.36. `Shoot` owners who wish to continue using this behavior must explicitly enable the feature gate via `.spec.kubernetes.kubeControllerManager.featureGates`.
- The alpha API groups `autoscaling/v2beta1` and `autoscaling/v2beta2` have been removed. Any resources still using these groups must be migrated to `autoscaling/v2` before upgrading.
- The alpha API group `scheduling.k8s.io/v1alpha1` has been removed and replaced by `scheduling.k8s.io/v1alpha2`.

## Upgrading to Kubernetes `v1.35`

- The `Shoot`'s `.spec.kubernetes.kubeAPIServer.enableAnonymousAuthentication` field is forbidden. Gardener continues to disable anonymous authentication by default. If you need to configure anonymous authentication, use [Structured Authentication Configuration](shoot_access.md#configuring-anonymous-authentication) with the [anonymous authenticator](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration) instead.
- The `Shoot`'s `.spec.addons` field is forbidden. The retirement of the previously contained components [Kubernetes Dashboard](https://github.com/kubernetes-retired/dashboard) and [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx), requires owners to remove any existing addon configurations from the `Shoot`.
- The `Shoot`'s `.spec.kubernetes.kubeAPIServer.watchCacheSizes.default` field is forbidden. Watch cache sizes are automatically sized by Kubernetes.
- The `Shoot`'s `.spec.kubernetes.kubeScheduler.kubeMaxPDVols` field is forbidden. The maximum number of attachable volumes is maintained by the respective CSI plugin.
- The `Shoot`'s `.spec.dns.providers[].secretName` field is forbidden, use `.spec.dns.providers[].credentialsRef` instead.

## Upgrading to Kubernetes `v1.34`

- The `Shoot`'s `.spec.cloudProfileName` field is forbidden. `Shoot` owners must migrate their `CloudProfile` reference to the new `spec.cloudProfile.name` field.
- The `Shoot`'s `.spec.secretBindingName` field is forbidden. `Shoot` owners must migrate their `SecretBinding` references to `CredentialsBinding` and use the new `.spec.credentialsBindingName` field. For more information, see the [SecretBinding to CredentialsBinding migration guide](../shoot-operations/secretbinding-to-credentialsbinding-migration.md).
- The `Shoot`'s operation annotations `rotate-etcd-encryption-key-(start|complete)` are forbidden. `Shoot` owners must use the `rotate-etcd-encryption-key` operation annotation instead, which performs a complete etcd encryption key rotation. `Shoot` clusters with an ongoing etcd encryption key rotation that is currently in the `Prepared` phase will move forward to the `Completing` phase.

## Upgrading to Kubernetes `v1.33`

- A new `deny-all` `NetworkPolicy` is deployed into the `kube-system` namespace of the `Shoot` cluster. `Shoot` owners that run workloads in the `kube-system` namespace are required to explicitly allow their expected `Ingress` and `Egress` traffic in `kube-system` via `NetworkPolicies`.
- The `Shoot`'s `.spec.kubernetes.kubeControllerManager.podEvictionTimeout` field is forbidden. `Shoot` owners should use the `.spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds` and `.spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds` fields.
- The `Shoot`'s `.spec.kubernetes.clusterAutoscaler.maxEmptyBulkDelete` field is forbidden. `Shoot` owners should use the `.spec.kubernetes.clusterAutoscaler.maxScaleDownParallelism` field.
- The `Shoot`'s `.spec.cloudProfileName` field is deprecated. `Shoot` owners should migrate their `CloudProfile` reference to the new `.spec.cloudProfile.name` field.
