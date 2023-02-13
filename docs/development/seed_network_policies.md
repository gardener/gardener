# Network Policies in the Seed Cluster

This document describes the [Kubernetes network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) deployed by Gardener into the Seed cluster.
For network policies deployed into the Shoot `kube-system` namespace, please see the [usage section](../usage/shoot_network_policies.md).

Network policies deployed by Gardener have names and annotations describing their purpose, so this document only highlights a subset of the policies in detail.

## Network Policies in the Shoot Namespace in the Seed

The network policies in the Shoot namespace in the Seed can roughly be grouped into policies required for the control plane components and policies required for logging & monitoring.

The network policy `deny-all` plays a special role. This policy [denies all ingress and egress traffic](https://kubernetes.io/docs/concepts/services-networking/network-policies/#default-deny-all-ingress-and-all-egress-traffic) from each pod in the Shoot namespace.
So per default, a pod running in the control plane cannot talk to any other pod in the whole Seed cluster.
This means the pod needs to have labels matching to appropriate network policies allowing it to talk to exactly the components required to execute its desired functionality.
[This has also implications for Gardener extensions](#implications-for-gardener-extensions) that need to deploy additional components into the `Shoot's` control plane.

### Network Policies for Control Plane Components

This section highlights a selection of network policies that exist in the Shoot namespace in the Seed cluster.
In general, the control plane components serve different purposes and thus need access to different pods and network ranges.

In contrast to other network policies, the policy `allow-to-shoot-networks` is tailored to the individual Shoot cluster, 
because it is based on the network configuration in the Shoot manifest.
It allows pods with the label `networking.gardener.cloud/to-shoot-networks=allowed` to access pods in the Shoot pod, 
service and node CIDR range. This is used by the Shoot API Server and the Prometheus pods to communicate over VPN/proxy with pods in the Shoot cluster.
This network policy is only useful if reversed vpn is disabled, as otherwise the vpn-seed-server pod in the control plane is the only pod with layer 3 routing to the shoot network.

The policy `allow-to-blocked-cidrs` allows pods with the label `networking.gardener.cloud/to-blocked-cidrs=allowed` to access IPs that are explicitly blocked for all control planes in a Seed cluster (configurable via `spec.networks.blockCIDRS`). 
This is used for instance to block the cloud provider's metadata service.

Another network policy to be highlighted is `allow-to-runtime-apiserver`.
Some components need access to the Seed API Server. This can be allowed by labeling the pod with `networking.gardener.cloud/to-runtime-apiserver=allowed`.
This policy allows exactly the IPs of the `kube-apiserver` of the Seed.
While all other policies have a static set of permissions (do not change during the lifecycle of the Shoot), the policy `allow-to-runtime-apiserver` is reconciled to reflect the endpoints in the `default` namespace.
This is required because endpoint IPs are not necessarily stable (think of scaling the Seed API Server pods or hibernating the Seed cluster (acting as a managed seed) in a local development environment).

Furthermore, the following network policies exist in the Shoot namespace.
These policies are the same for every Shoot control plane.

```
NAME                              POD-SELECTOR      
# Pods that need to access the Shoot API server. Used by all Kubernetes control plane components.
allow-to-shoot-apiserver          networking.gardener.cloud/to-shoot-apiserver=allowed

# allows access to kube-dns/core-dns pods for DNS queries                       
allow-to-dns                      networking.gardener.cloud/to-dns=allowed

# allows access to private IP address ranges 
allow-to-private-networks         networking.gardener.cloud/to-private-networks=allowed

# allows access to all but private IP address ranges 
allow-to-public-networks          networking.gardener.cloud/to-public-networks=allowed

# allows Ingress to etcd pods from the Shoot's Kubernetes API Server
allow-etcd                        app=etcd-statefulset,gardener.cloud/role=controlplane

# used by the Shoot API server to allows ingress from pods labeled
# with'networking.gardener.cloud/to-shoot-apiserver=allowed', from Prometheus, and allows Egress to etcd pods
allow-kube-apiserver              app=kubernetes,gardener.cloud/role=controlplane,role=apiserver
```


### Network Policies for Logging & Monitoring

Gardener currently introduces a logging stack based on [Loki](https://github.com/grafana/loki). So this section is subject to change. 
For more information, see the [Loki Gardener Community Meeting](https://www.youtube.com/watch?v=345b8xCcB-U&t=1166s).

These are the logging and monitoring related network policies:
```
NAME                                POD-SELECTOR                                                             
allow-from-prometheus (deprecated!) networking.gardener.cloud/from-prometheus=allowed
allow-grafana                       component=grafana,gardener.cloud/role=monitoring
allow-prometheus                    app=prometheus,gardener.cloud/role=monitoring,role=monitoring
allow-to-aggregate-prometheus       networking.gardener.cloud/to-aggregate-prometheus=allowed
allow-to-loki                       networking.gardener.cloud/to-loki=allowed
```

As part of the shoot reconciliation flow, Gardener deploys a shoot-specific Prometheus into the shoot namespace. 
Each pod that should be scraped for metrics must have a `Service` which is annotated with

```yaml
annotations:
  networking.resources.gardener.cloud/from-policy-pod-label-selector: all-scrape-targets
  networking.resources.gardener.cloud/from-policy-allowed-ports: '[{"port":<metrics-port-on-pod>,"protocol":"<protocol, typically TCP>"}]'
```

This automatically allows the needed network traffic from the Prometheus pod.
For more information, see take a look at [this document](../concepts/resource-manager.md#networkpolicy-controllerpkgresourcemanagercontrollernetworkpolicy).

### Implications for Gardener Extensions

Gardener extensions sometimes need to deploy additional components into the Shoot namespace in the Seed hosting the control plane. 
For example, the Gardener extension [provider-aws](https://github.com/gardener/gardener-extension-provider-aws) deploys the `MachineControllerManager` into the Shoot namespace, that is ultimately responsible to create the VMs with the cloud provider AWS.

Every Shoot namespace in the Seed contains the network policy `deny-all`.
This requires a pod deployed by a Gardener extension to have labels from network policies that exist in the Shoot namespace, that allow the required network ranges. 

Additionally, extensions could also deploy their own network policies. This is used e.g by the Gardener extension [provider-aws](https://github.com/gardener/gardener-extension-provider-aws) 
to serve [Admission Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for the Shoot API server that need to be reachable from within the Shoot namespace.

The pod can use an arbitrary combination of network policies.

## Network Policies in the `garden` Namespace

The network policies in the `garden` namespace are, with a few exceptions (e.g Kubernetes control plane specific policies), the same as in the Shoot namespaces.
For your reference, these are all the deployed network policies.
```
NAME                              POD-SELECTOR  
allow-fluentbit                   app=fluent-bit,gardener.cloud/role=logging,role=logging              
allow-from-aggregate-prometheus   networking.gardener.cloud/from-aggregate-prometheus=allowed              
allow-to-aggregate-prometheus     networking.gardener.cloud/to-aggregate-prometheus=allowed                
allow-to-all-shoot-apiservers     networking.gardener.cloud/to-all-shoot-apiservers=allowed                
allow-to-blocked-cidrs            networking.gardener.cloud/to-blocked-cidrs=allowed                       
allow-to-dns                      networking.gardener.cloud/to-dns=allowed                                 
allow-to-loki                     networking.gardener.cloud/to-loki=allowed                       
allow-to-private-networks         networking.gardener.cloud/to-private-networks=allowed                    
allow-to-public-networks          networking.gardener.cloud/to-public-networks=allowed                     
allow-to-runtime-apiserver        networking.gardener.cloud/to-runtime-apiserver=allowed                                                    
```

This section describes the network policies that are unique to the `garden` namespace.

The network policy `allow-to-all-shoot-apiservers` allows pods to access every `Shoot` API server in the `Seed`.
This is, for instance, used by the [dependency watchdog](https://github.com/gardener/dependency-watchdog) to regularly check 
the health of all the Shoot API servers.

[Gardener deploys a central Prometheus instance](https://github.com/gardener/gardener/blob/master/docs/extensions/logging-and-monitoring.md#monitoring) in the `garden` namespace that fetches metrics and data from all seed cluster nodes and all seed cluster pods.
The network policies `allow-to-aggregate-prometheus` and `allow-from-aggregate-prometheus` allow traffic from and to this Prometheus instance.

Worth mentioning is, that the network policy `allow-to-shoot-networks` does not exist in the `garden` namespace. This is to forbid Gardener system components to talk to workload deployed in the Shoot VPC.
