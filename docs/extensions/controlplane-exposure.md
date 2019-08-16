# Contract: `ControlPlane` resource with purpose `exposure`

Some Kubernetes clusters require an additional deployments required by the seed cloud provider in order to work properly, e.g. AWS Load Balancer Readvertiser.
Before using ControlPlane resources with purpose `exposure` Gardener was having different Helm charts for the deployments for the various providers.
Now, Gardener commissions an external, provider-specific controller to take over this task.

## Which control plane resources are required?

As mentioned in the [controlplane](controlplane.md) document Gardener shall not deploy any other provider-specific component.
Instead, it creates a `ControlPlane` CRD with purpose `exposure` that should be picked up by provider extensions.
Its purpose is to trigger the deployment of such provider-specific components in the shoot namespace in the seed cluster that are needed to expose the kube-apiserver.
The shoot cluster's kube-apiserver are exposed via a `Service` of type `LoadBalancer`. This load balancer is part of the seed provider's infrastructure. As the seed provider might differ from the shoot provider (you may run the control plane of an Azure shoot in a GCP seed) it's the seed provider extension controller that should act on the `ControlPlane` resources with purpose `exposure`.

## What needs to be implemented to support a new infrastructure provider?

As part of the shoot flow Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: ControlPlane
metadata:
  name: control-plane-exposure
  namespace: shoot--foo--bar
spec:
  type: aws
  purpose: exposure
  region: europe-west1
  secretRef:
    name: cloudprovider
    namespace: shoot--foo--bar
```

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used for the shoot cluster.
It is most likely not needed, however, still added for some potential corner cases.
If you don't need it then just ignore it.
The `.spec.region` contains the region of the seed cluster.

In order to support a control plane provider with purpose `exposure` you need to write a controller or expand the existing [controlplane controller](controlplane.md) that watches all `ControlPlane`s with `.spec.type=<my-provider-name>` and purpose `exposure`.
You can take a look at the below referenced example implementation for the AWS provider.

## Non-provider specific information required for infrastructure creation

Most providers might require further information that is not provider specific but already part of the shoot resource.
As Gardener cannot know which information is required by providers it simply mirrors the `Shoot`, `Seed`, and `CloudProfile` resources into the seed.
They are part of the [`Cluster` extension resource](cluster.md) and can be used to extract information.

## References and additional resources

* [`ControlPlane` API (Golang specification)](../../pkg/apis/extensions/v1alpha1/types_controlplane.go)
* [Exemplary implementation for the AWS provider](https://github.com/gardener/gardener-extensions/tree/master/controllers/provider-aws/pkg/controller/controlplane)
* [AWS Load Balancer Readvertiser](https://github.com/gardener/aws-lb-readvertiser)
