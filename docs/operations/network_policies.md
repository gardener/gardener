# `NetworkPolicy`s In Garden, Seed, Shoot Clusters

This document describes which [Kubernetes `NetworkPolicy`s](https://kubernetes.io/docs/concepts/services-networking/network-policies/) deployed by Gardener into the various clusters.

## Garden Cluster

*(via `gardener-operator` and `gardener-resource-manager`)*

The `gardener-operator` runs a [`NetworkPolicy` controller](../concepts/operator.md#networkpolicy-controller-registrar) which is responsible for the following namespaces:

- `garden`
- `istio-system`
- `*istio-ingress-*`
- `shoot-*`
- `extension-*` (in case the garden cluster is a seed cluster at the same time)

It deploys the following so-called "general `NetworkPolicy`s":

| Name                         | Purpose                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `deny-all`                   | [Denies all ingress and egress traffic](https://kubernetes.io/docs/concepts/services-networking/network-policies/#default-deny-all-ingress-and-all-egress-traffic) for all pods in this namespace. Hence, all traffic must be explicitly allowed.                                                                                                                                                                                                                                         |
| `allow-to-dns`               | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-dns=allowed` to DNS pods running in the `kube-system` namespace. In practice, most of the pods performing network egress traffic need this label.                                                                                                                                                                                                                                                               |
| `allow-to-runtime-apiserver` | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-runtime-apiserver=allowed` to the API server of the runtime cluster.                                                                                                                                                                                                                                                                                                                                           |
| `allow-to-blocked-cidrs`     | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-blocked-cidrs=allowed` to explicitly blocked addresses configured by human operators (configured via `.spec.networking.blockedCIDRs` in the `Seed`). For instance, this can be used to block the cloud provider's metadata service.                                                                                                                                                                            |
| `allow-to-public-networks`   | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-public-networks=allowed` to all public network IPs, except for private networks (RFC1918), carrier-grade NAT (RFC6598), and explicitly blocked addresses configured by human operators for all pods labeled with `networking.gardener.cloud/to-public-networks=allowed`. In practice, this blocks egress traffic to all networks in the cluster and only allows egress traffic to public IPv4 addresses.       |
| `allow-to-private-networks`  | Allows egress traffic from pods labeled with `networking.gardener.cloud/to-private-networks=allowed` to the private networks (RFC1918) and carrier-grade NAT (RFC6598) except for cluster-specific networks (configured via `.spec.networks` in the `Seed`).                                                                                                                                                                                                                              |

Apart from those, the `gardener-operator` also enables the [`NetworkPolicy` controller of `gardener-resource-manager`](../concepts/resource-manager.md#networkpolicy-controller).
Please find more information in the linked document.
In summary, most of the pods that initiate connections with other pods will have labels with `networking.resources.gardener.cloud/` prefixes.
This way, they leverage the automatically created `NetworkPolicy`s by the controller.
As a result, in most cases no special/custom-crafted `NetworkPolicy`s must be created anymore.

### Logging & Monitoring

As part of the garden reconciliation flow, the `gardener-operator` deploys various Prometheus instances into the `garden` namespace.
Each pod that should be scraped for metrics by these instances must have a `Service` which is annotated with

```yaml
annotations:
  networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports: '[{"port":<metrics-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
```

If the respective pod is not running in the `garden` namespace, the `Service` needs these annotations in addition:

```yaml
annotations:
  networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]'
  networking.resources.gardener.cloud/pod-label-selector-namespace-alias: extensions
```

This automatically allows the needed network traffic from the respective Prometheus pods.

## Seed Cluster

*(via `gardenlet` and `gardener-resource-manager`)*

In seed clusters it works the same way as in the garden cluster managed by `gardener-operator`.
When a seed cluster is the garden cluster at the same time, `gardenlet` does not enable the `NetworkPolicy` controller (since `gardener-operator` already runs it).
Otherwise, it uses the exact same controller and code like `gardener-operator`, resulting in the same behaviour in both garden and seed clusters.

### Logging & Monitoring

#### Seed System Namespaces

As part of the seed reconciliation flow, the `gardenlet` deploys various Prometheus instances into the `garden` namespace.
See also [this document](../development/monitoring-stack.md) for more information.
Each pod that should be scraped for metrics by these instances must have a `Service` which is annotated with

```yaml
annotations:
  networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports: '[{"port":<metrics-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
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
  networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports: '[{"port":<metrics-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
```

This automatically allows the network traffic from the Prometheus pod.

### Webhook Servers

Components serving webhook handlers that must be reached by `kube-apiserver`s of the virtual garden cluster or shoot clusters just need to annotate their `Service` as follows:

```yaml
annotations:
  networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports: '[{"port":<server-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
```

This automatically allows the network traffic from the API server pods.

In case the servers run in a different namespace than the `kube-apiserver`s, the following annotations are needed:

```yaml
annotations:
  networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports: '[{"port":<server-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
  networking.resources.gardener.cloud/pod-label-selector-namespace-alias: extensions
  # for the virtual garden cluster:
  networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]'
  # for shoot clusters:
  networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"gardener.cloud/role":"shoot"}}]'
```

## Additional Namespace Coverage in Garden/Seed Cluster

In some cases, garden or seed clusters might run components in dedicated namespaces which are not covered by the controller by default (see list above).
Still, it might(/should) be desired to also include such "custom namespaces" into the control of the `NetworkPolicy` controllers.

In order to do so, human operators can adapt the component configs of `gardener-operator` or `gardenlet` by providing label selectors for additional namespaces:

```yaml
controllers:
  networkPolicy:
    additionalNamespaceSelectors:
    - matchLabels:
        foo: bar
```

### Communication With `kube-apiserver` For Components In Custom Namespaces

### Egress Traffic

Component running in such custom namespaces might need to initiate the communication with the `kube-apiserver`s of the virtual garden cluster or a shoot cluster.
In order to achieve this, their custom namespace must be labeled with `networking.gardener.cloud/access-target-apiserver=allowed`.
This will make the `NetworkPolicy` controllers automatically provisioning the required policies into their namespace.

As a result, the respective component pods just need to be labeled with

- `networking.resources.gardener.cloud/to-garden-virtual-garden-kube-apiserver-tcp-443=allowed` (virtual garden cluster)
- `networking.resources.gardener.cloud/to-all-shoots-kube-apiserver-tcp-443=allowed` (shoot clusters)

### Ingress Traffic

Components running in such custom namespaces might serve webhook handlers that must be reached by the `kube-apiserver`s of the virtual garden cluster or a shoot cluster.
In order to achieve this, their `Service` must be annotated.
Please refer to [this section](#webhook-servers) for more information.

## Shoot Cluster

*(via `gardenlet`)*

For shoot clusters, the concepts mentioned above don't apply and are not enabled.
Instead, `gardenlet` only deploys a few "custom" `NetworkPolicy`s for the shoot system components running in the `kube-system` namespace.
All other namespaces in the shoot cluster do not contain network policies deployed by `gardenlet`.

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

### Webhook Servers in Shoot Clusters

Shoot components serving webhook handlers must be reached by `kube-apiserver`s of the shoot cluster.
However, the control plane components, e.g. `kube-apiserver`, run on the seed cluster decoupled by a [VPN connection](../proposals/14-reversed-cluster-vpn.md).
Therefore, shoot components serving webhook handlers need to allow the VPN endpoints in the shoot cluster as clients to allow `kube-apiserver`s to call them.

For the `kube-system` namespace, the network policy `gardener.cloud--allow-from-seed` fulfils the purpose to allow pods to mark themselves as targets for such calls, allowing corresponding traffic to pass through.

For custom namespaces, operators can use the network policy `gardener.cloud--allow-from-seed` as a template.
Please note that the label selector may change over time, i.e. with Gardener version updates.
This is why a simpler variant with a reduced label selector like the example below is recommended:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-from-seed
  namespace: custom-namespace
spec:
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          gardener.cloud/purpose: kube-system
      podSelector:
        matchLabels:
          app: vpn-shoot
```

## Implications for Gardener Extensions

Gardener extensions sometimes need to deploy additional components into the shoot namespace in the seed cluster hosting the control plane.
For example, the [`gardener-extension-provider-aws`](https://github.com/gardener/gardener-extension-provider-aws) deploys the `cloud-controller-manager` into the shoot namespace.
In most cases, such pods require network policy labels to allow the traffic they are initiating.

For components deployed in the `kube-system` namespace of the shoots (e.g., CNI plugins or CSI drivers, etc.), custom `NetworkPolicy`s might be required to ensure the respective components can still communicate in case the user creates a `deny-all` policy.
