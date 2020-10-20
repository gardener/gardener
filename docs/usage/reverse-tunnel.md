# Reverse Tunnel Setup and Configuration 

This is a short guide describing how to enable tunneling traffic from shoot cluster to seed cluster instead of the default "seed to shoot" direction. 

## The OpenVPN Default

By default, Gardener makes use of OpenVPN to connect the shoot controlplane running on the seed cluster to the dataplane 
running on the shoot worker nodes, usually in isolated networks. This is achieved by having a sidecar to certain control plane components such as the `kube-apiserver` and `prometheus`. 

With a sidecar, all traffic directed to the cluster is intercepted by iptables rules and redirected 
to the tunnel endpoint in the shoot cluster deployed behind a cloud loadbalancer. This has the following disadvantages: 

- Every shoot would require an additional loadbalancer, this accounts for addition overhead in terms of both costs and troubleshooting efforts.
- Private access use-cases would not be possible without having a seed residing in the same private domain as a hard requirement. For example, have a look at [this issue](https://github.com/gardener/gardener-extension-provider-gcp/issues/56)
- Providing a public endpoint to access components in the shoot poses a security risk.

This is how it looks like today with the OpenVPN solution: 

`APIServer | VPN-seed ---> internet ---> LB --> VPN-Shoot (4314) --> Pods | Nodes | Services`


## Reversing the Tunnel

To address the above issues, the tunnel direction can be reverted, i.e., instead of having the client reside in the seed, 
we deploy the client in the shoot and initiate the connection from there. This way, there is no need to deploy a special purpose 
loadbalancer for the sake of addressing the dataplane, in addition to saving costs, this is considered the more secure alternative. 
For more information on how this is achieved, please have a look at the following [GEP](../proposals/11-apiserver-network-proxy.md). 

How it should look like at the end: 

`APIServer <-> Proxy-Server <--- internet <--- Proxy-Agent --> Pods | Nodes | Services`

### How to Configure

To enable the usage of the reverse tunnel feature, either the Gardenlet `KonnectivityTunnel` feature-gate must be set to `true` as shown below or the shoot must be annotated with `"alpha.featuregates.shoot.gardener.cloud/konnectivity-tunnel: true"`.

```yaml
featureGates:
  KonnectivityTunnel: true
``` 
Please refer to the examples [here](https://github.com/gardener/gardener/blob/master/example/20-componentconfig-gardenlet.yaml) for more information.

To disable the feature-gate the shoot must be annotated with `"alpha.featuregates.shoot.gardener.cloud/konnectivity-tunnel: false"`

Once the feature-gate is enabled, a `proxy-server` sidecar will be deployed alongside the `kube-apiserver` which will receive and redirect connections 
to the corresponding `proxy-agent` on the shoot cluster responsible for dataplane traffic routing. 

> Please note this feature is available ONLY for >= 1.18 kubernetes clusters. For clusters with Kubernetes version < 1.18, OpenVPN will be used by default even if the featuregate is enabled.
> Furthermore, this feature is still in Alpha, so you might see instabilities every now and then. 

