# `NetworkPolicy`s In Garden, Seed, Shoot Clusters

This document describes which [Kubernetes `NetworkPolicy`s](https://kubernetes.io/docs/concepts/services-networking/network-policies/) deployed by Gardener into the various clusters.

## Garden Cluster

*(via `gardener-operator` and `gardener-resource-manager`)*

N/A (in development)

## Seed Cluster

*(via `gardenlet` and `gardener-resource-manager`)*

The `gardenlet` runs a [`NetworkPolicy` controller](../concepts/gardenlet.md#networkpolicy-controllerpkggardenletcontrollernetworkpolicy) which is responsible for the following namespaces:

- `garden`
- `istio-system`
- `istio-ingress-*`
- `shoot-*`
- `extension-*` (only when the [`FullNetworkPoliciesInRuntimeCluster` feature gate](../deployment/feature_gates.md) is enabled)

It deploys the following so-called "general `NetworkPolicy`s":

| Name                         | Purpose                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `deny-all`                   | [Denies all ingress and egress traffic](https://kubernetes.io/docs/concepts/services-networking/network-policies/#default-deny-all-ingress-and-all-egress-traffic) for all pods in this namespace. Hence, all traffic must be explicitly allowed.                                                                                                                                                                                                                                         |
| `allow-to-dns`               | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-dns=allowed` to DNS pods running in the `kube-sytem` namespace. In practice, most of the pods performing network egress traffic need this label.                                                                                                                                                                                                                                                               |
| `allow-to-runtime-apiserver` | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-runtime-apiserver=allowed` to the API server of the runtime cluster.                                                                                                                                                                                                                                                                                                                                           |
| `allow-to-blocked-cidrs`     | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-blocked-cidrs=allowed` to explicitly blocked addresses configured by human operators (configured via `.spec.networking.blockedCIDRs` in the `Seed`). For instance, this can be used to block the cloud provider's metadata service.                                                                                                                                                                            |
| `allow-to-public-networks`   | Allows egress traffic from pods labeled with `networking.gardener.cloud/allow-to-public-networks=allowed` to all public network IPs, except for private networks (RFC1918), carrier-grade NAT (RFC6598), and explicitly blocked addresses configured by human operators for all pods labeled with `networking.gardener.cloud/to-public-networks=allowed`. In practice, this blocks egress traffic to all networks in the cluster and only allows egress traffic to public IPv4 addresses. |
| `allow-to-private-networks`  | Allows egress traffic from pods labeled with `networking.gardener.cloud/allow-to-private-networks=allowed` to the private networks (RFC1918) and carrier-grade NAT (RFC6598) except for cluster-specific networks (configured via `.spec.networks` in the `Seed`).                                                                                                                                                                                                                        | 
| `allow-to-shoot-networks`    | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-shoot-networks=allowed` to IPv4 blocks belonging to the shoot networks (configured via `.spec.networking` in the `Shoot`). In practice, this should be used by components which use VPN tunnel to communicate to pods in the shoot cluster. Note that this policy only exists in `shoot-*` namespaces.                                                                                                         |

Apart from those, the `gardenlet` also enables the [`NetworkPolicy` controller of `gardener-resource-manager`](../concepts/resource-manager.md#networkpolicy-controllerpkgresourcemanagercontrollernetworkpolicy).
Please find more information in the linked document.
In summary, most of the pods that initiate connections with other pods will have labels with `networking.resources.gardener.cloud/` prefixes.
This way, they leverage the automatically created `NetworkPolicy`s by the controller.
As a result, in most cases no special/custom-crafted `NetworkPolicy`s must be created anymore.

### Logging & Monitoring

#### Seed System Namespaces

As part of the seed reconciliation flow, the `gardenlet` deploys various Prometheus instances into the `garden` namespace.
See also [this document](../development/monitoring-stack.md) for more information.
Each pod that should be scraped for metrics be these instances must have a `Service` which is annotated with

```yaml
annotations:
  networking.resources.gardener.cloud/from-policy-pod-label-selector: all-seed-scrape-targets
  networking.resources.gardener.cloud/from-policy-allowed-ports: '[{"port":<metrics-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
```

If the respective pod is not running in the `garden` namespace, the `Service` needs these annotations in addition:

```yaml
annotations:
  networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]'
```

If the respective pod is running in an `extension-*` namespace, the `Service` needs this annotation in addition:

```yaml
annotations:
  networking.resources.gardener.cloud/pod-label-selector-namespace-alias: extensions
```

This automatically allows the needed network traffic from the respective Prometheus pods.

#### Shoot Namespaces

As part of the shoot reconciliation flow, the `gardenlet` deploys a shoot-specific Prometheus into the shoot namespace. 
Each pod that should be scraped for metrics must have a `Service` which is annotated with

```yaml
annotations:
  networking.resources.gardener.cloud/from-policy-pod-label-selector: all-scrape-targets
  networking.resources.gardener.cloud/from-policy-allowed-ports: '[{"port":<metrics-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
```

This automatically allows the needed network traffic from the Prometheus pod.

## Shoot Cluster

*(via `gardenlet`)*

For shoot clusters, the concepts mentioned above don't apply and are not enabled.
Instead, `gardenlet` only deploys a few "custom" `NetworkPolicy`s for the shoot system components running in the `kube-system` namespace.
All other namespaces in the shoot cluster do not contain network policies deployed by ``gardenlet``.

As a best practice, every pod deployed into the `kube-system` namespace should use appropriate `NetworkPolicy` in order to only allow **required** network traffic.
Therefore, pods should have labels matching to the selectors of the available network policies.

`gardenlet` deploys the following `NetworkPolicy`s:

```text
NAME                                       POD-SELECTOR
gardener.cloud--allow-dns                  k8s-app in (kube-dns)
gardener.cloud--allow-from-seed            networking.gardener.cloud/from-seed=allowed
gardener.cloud--allow-to-dns               networking.gardener.cloud/to-dns=allowed
gardener.cloud--allow-to-apiserver         networking.gardener.cloud/to-apiserver=allowed
gardener.cloud--allow-to-from-nginx        app=nginx-ingress
gardener.cloud--allow-to-kubelet           networking.gardener.cloud/to-kubelet=allowed
gardener.cloud--allow-to-public-networks   networking.gardener.cloud/to-public-networks=allowed
gardener.cloud--allow-vpn                  app=vpn-shoot
```

Note that a `deny-all` policy will not be created by `gardenlet`.
Shoot owners can create it manually if needed/desired.
Above listed `NetworkPolicy`s ensure that the traffic for the shoot system components is allowed in case such `deny-all` policies is created.

## Implications for Gardener Extensions

Gardener extensions sometimes need to deploy additional components into the shoot namespace in the seed cluster hosting the control plane.
For example, the [`gardener-extension-provider-aws`](https://github.com/gardener/gardener-extension-provider-aws) deploys the `cloud-controller-manager` into the shoot namespace.
In most cases, such pods require network policy labels to allow the traffic they are initiating.

For components deployed in the `kube-system` namespace of the shoots (e.g., CNI plugins or CSI drivers, etc.), custom `NetworkPolicy`s might be required to ensure the respective components can still communicate in case the user creates a `deny-all` policy.
