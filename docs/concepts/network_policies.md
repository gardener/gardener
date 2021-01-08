# Network Policies in Gardener

As `Seed` clusters can host the [Kubernetes control planes](https://kubernetes.io/docs/concepts/#kubernetes-control-plane) of many `Shoot` clusters, it is necessary to isolate the control planes from each other for security reasons.
Besides deploying each control plane in its own namespace, Gardener creates [network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) to also isolate the networks. 
Essentially, network policies make sure that pods can only talk to other pods over the network they are supposed to.
As such, network policies are an important part of Gardener's tenant isolation.

Gardener deploys network policies into
 - each namespace hosting the Kubernetes control plane of the Shoot cluster.
 - the namespace dedicated to Gardener seed-wide global controllers. This namespace is often called `garden` and contains e.g. the [Gardenlet](https://github.com/gardener/gardener/blob/15cae57db802cbe460ff4cb3f80c26b2fc15e26f/docs/concepts/gardenlet.md).
 - the `kube-system` namespace in the Shoot.
 
The aforementioned namespaces in the Seed contain a `deny-all` network policy that [denies all ingress and egress traffic](https://kubernetes.io/docs/concepts/services-networking/network-policies/#default-deny-all-ingress-and-all-egress-traffic).
This [secure by default](https://en.wikipedia.org/wiki/Secure_by_default) setting requires pods to allow network traffic.
This is done by pods having [labels matching to the selectors of the network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/#networkpolicy-resource) deployed by Gardener.

More details on the deployed network policies can be found in the [development](https://github.com/gardener/gardener/tree/master/docs/development/seed_network_policies.md) and [usage](https://github.com/gardener/gardener/tree/master/docs/usage/shoot_network_policies.md) sections.
 