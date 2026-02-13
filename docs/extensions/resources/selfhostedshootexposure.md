# Contract: `SelfHostedShootExposure` Resource

The `SelfHostedShootExposure` resource is a concept introduced to support Self-Hosted Shoot Clusters introduced in [GEP36](https://github.com/gardener/gardener/blob/master/docs/proposals/36-self-hosted-shoot-exposure.md). 
In a self-hosted scenario, the control plane of a shoot cluster runs on dedicated nodes within the cluster itself, rather than in a separate seed cluster. 
To make the API server of such a cluster accessible from the outside (e.g., for `kubectl` access), the control plane endpoints must be exposed via a stable address (e.g., a `LoadBalancer`).

The `SelfHostedShootExposure` resource abstracts the specific infrastructure or mechanism used to expose the control plane (e.g., a cloud provider `LoadBalancer`, `kube-vip`, `keepalived`, etc.) into a uniform extension API. 
This allows Gardener to be agnostic to the underlying exposure implementation.

## Resource Details

The `SelfHostedShootExposure` resource is reconciled by an extension controller. The controller is responsible for:

1.  Reading the endpoints listed in `.spec.endpoints`. These endpoints represent the nodes where the shoot control plane components (specifically the API server) are running.
2.  Provisioning a load balancer (or similar mechanism) that routes traffic to these endpoints on the specified port.
3.  Updating the `.status.ingress` field with the public address (IP or hostname) of the provisioned load balancer.

### Example

Below is an example of a `SelfHostedShootExposure` resource:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: SelfHostedShootExposure
metadata:
  name: self-hosted-exposure
  namespace: shoot--my-project--my-shoot
spec:
  type: provider-stackit
  endpoints:
  - nodeName: node-1
    addresses:
    - type: InternalIP
      address: 10.0.1.10
    port: 6443
  - nodeName: node-2
    addresses:
    - type: InternalIP
      address: 10.0.1.11
    port: 6443
status:
  ingress:
  - ip: 203.0.113.10
    hostname: load-balancer-hostname.example.com
```


## References and additional resources

* [SelfHostedShootExposure API Documentation](../../../pkg/apis/extensions/v1alpha1/types_selfhostedshootexposure.go)
* [GEP-36: Self-Hosted Shoot Exposure](https://github.com/gardener/enhancements/tree/main/geps/0036-self-hosted-shoot-exposure)
