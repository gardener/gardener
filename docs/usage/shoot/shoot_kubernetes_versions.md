---
title: Shoot cluster supported Kubernetes versions and specifics
description: Defining the differences and requirements for updating to a supported Kubernetes version
---

# Shoot Kubernetes version update

Breaking changes may be introduced with new Kubernetes versions. This documentation describes the differences and requirements for updating to a supported Kubernetes version.

## Updating to Kubernetes `v1.33`

When updating to Kubernetes version `v1.33` a new `deny-all` `NetworkPolicy` is deployed in the `kube-system` namespace of the `Shoot` cluster. For `Shoot` owners that have workloads in the `kube-system` namespace it would be needed from them to specify the expected `Ingress` and `Egress` traffic in `kube-system` `NetworkPolicies`.
