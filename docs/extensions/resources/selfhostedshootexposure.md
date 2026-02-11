# Contract: `SelfHostedShootExposure` Resource

The `SelfHostedShootExposure` resource is a concept introduced to support Self-Hosted Shoot Clusters introduced in [GEP36](https://github.com/gardener/gardener/blob/master/docs/proposals/36-self-hosted-shoot-exposure.md). 
In a self-hosted scenario, the control plane of a shoot cluster runs on dedicated nodes within the cluster itself, rather than in a separate seed cluster. 
To make the API server of such a cluster accessible from the outside (e.g., for `kubectl` access or for the Gardenlet to communicate with it), the control plane endpoints must be exposed via a stable address (e.g., a `LoadBalancer`).

The `SelfHostedShootExposure` resource abstracts the specific infrastructure or mechanism used to expose the control plane (e.g., a cloud provider `LoadBalancer`, `kube-vip`, `keepalived`, etc.) into a uniform extension API. 
This allows Gardener to be agnostic to the underlying exposure implementation.

Every Kubernetes cluster requires some low-level infrastructure to be setup in order to work properly.
Examples for that are networks, routing entries, security groups, IAM roles, etc.

## References and additional resources

* [SelfHostedShootExposure API Documentation](../../../pkg/apis/extensions/v1alpha1/types_exposure.go)
* [GEP-36: Self-Hosted Shoot Exposure](https://github.com/gardener/gardener/blob/master/docs/proposals/36-self-hosted-shoot-exposure.md)
