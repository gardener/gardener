---
title: ControlPlane
---

# Contract: `ControlPlane` Resource

Most Kubernetes clusters require a `cloud-controller-manager` or CSI drivers in order to work properly.
Before introducing the `ControlPlane` extension resource Gardener was having several different Helm charts for the `cloud-controller-manager` deployments for the various providers.
Now, Gardener commissions an external, provider-specific controller to take over this task.

## Which control plane resources are required?

As mentioned in the [controlplane customization webhooks](../controlplane-webhooks.md) document, Gardener shall not deploy any `cloud-controller-manager` or any other provider-specific component.
Instead, it creates a `ControlPlane` CRD that should be picked up by provider extensions.
Its purpose is to trigger the deployment of such provider-specific components in the shoot namespace in the seed cluster.

## What needs to be implemented to support a new infrastructure provider?

As part of the shoot flow Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: ControlPlane
metadata:
  name: control-plane
  namespace: shoot--foo--bar
spec:
  type: openstack
  region: europe-west1
  secretRef:
    name: cloudprovider
    namespace: shoot--foo--bar
  providerConfig:
    apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
    kind: ControlPlaneConfig
    loadBalancerProvider: provider
    zone: eu-1a
    cloudControllerManager:
      featureGates:
        CustomResourceValidation: true
  infrastructureProviderStatus:
    apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
    kind: InfrastructureStatus
    networks:
      floatingPool:
        id: vpc-1234
      subnets:
      - purpose: nodes
        id: subnetid
```

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used for the shoot cluster.
However, the most important section is the `.spec.providerConfig` and the `.spec.infrastructureProviderStatus`.
The first one contains an embedded declaration of the provider specific configuration for the control plane (that cannot be known by Gardener itself).
You are responsible for designing how this configuration looks like.
Gardener does not evaluate it but just copies this part from what has been provided by the end-user in the `Shoot` resource.
The second one contains the output of the [`Infrastructure` resource](./infrastructure.md) (that might be relevant for the CCM config).

In order to support a new control plane provider, you need to write a controller that watches all `ControlPlane`s with `.spec.type=<my-provider-name>`.
You can take a look at the below referenced example implementation for the Alicloud provider.

The control plane controller as part of the `ControlPlane` reconciliation often deploys resources (e.g. pods/deployments) into the Shoot namespace in the `Seed` as part of its `ControlPlane` reconciliation loop.
Because the namespace contains [network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) that per default [deny all ingress and egress traffic](https://kubernetes.io/docs/concepts/services-networking/network-policies/#default-deny-all-ingress-and-all-egress-traffic),
the pods may need to have proper labels matching to the selectors of the network policies in order to allow the required network traffic.
Otherwise, they won't be allowed to talk to certain other components (e.g., the kube-apiserver of the shoot).
For more information, see [`NetworkPolicy`s In Garden, Seed, Shoot Clusters](../../operations/network_policies.md).

## Non-Provider Specific Information Required for Infrastructure Creation

Most providers might require further information that is not provider specific but already part of the shoot resource.
One example for this is the [GCP control plane controller](https://github.com/gardener/gardener-extension-provider-gcp/tree/master/pkg/controller/controlplane), which needs the Kubernetes version of the shoot cluster (because it already uses the in-tree Kubernetes cloud-controller-manager).
As Gardener cannot know which information is required by providers, it simply mirrors the `Shoot`, `Seed`, and `CloudProfile` resources into the seed.
They are part of the [`Cluster` extension resource](../cluster.md) and can be used to extract information that is not part of the `Infrastructure` resource itself.

## References and Additional Resources

* [`ControlPlane` API (Golang Specification)](../../../pkg/apis/extensions/v1alpha1/types_controlplane.go)
* [Exemplary Implementation for the Alicloud Provider](https://github.com/gardener/gardener-extension-provider-alicloud/tree/master/pkg/controller/controlplane)
