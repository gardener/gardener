## Network Policies in the Shoot Cluster

In addition to deploying network policies [into the Seed](../development/seed_network_policies.md),
Gardener deploys network policies into the `kube-system` namespace of the Shoot.
These network policies are used by Shoot system components (that are not part of the control plane).
Other namespaces in the Shoot do not contain network policies deployed by Gardener.

As a best practice, every pod deployed into the `kube-system` namespace should use appropriate network policies in order to only allow **required** network traffic.
Therefore, pods should have labels matching to the selectors of the available network policies.

Gardener deploys the following network policies:
```
NAME                                       POD-SELECTOR
gardener.cloud--allow-dns                  k8s-app in (kube-dns)
gardener.cloud--allow-from-seed            networking.gardener.cloud/from-seed=allowed
gardener.cloud--allow-to-apiserver         networking.gardener.cloud/to-apiserver=allowed
gardener.cloud--allow-to-dns               networking.gardener.cloud/to-dns=allowed
gardener.cloud--allow-to-from-nginx        app=nginx-ingress
gardener.cloud--allow-to-kubelet           networking.gardener.cloud/to-kubelet=allowed
gardener.cloud--allow-to-public-networks   networking.gardener.cloud/to-public-networks=allowed
gardener.cloud--allow-vpn                  app=vpn-shoot
```

Additionally, there can be network policies deployed by Gardener extensions such as [extension-calico](https://github.com/gardener/gardener-extension-networking-calico).
```
NAME                                       POD-SELECTOR
gardener.cloud--allow-from-calico-node     k8s-app=calico-typha
```
