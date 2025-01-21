# Contract: `Infrastructure` Resource

Every Kubernetes cluster requires some low-level infrastructure to be setup in order to work properly.
Examples for that are networks, routing entries, security groups, IAM roles, etc.
Before introducing the `Infrastructure` extension resource Gardener was using Terraform in order to create and manage these provider-specific resources (e.g., see [here](https://github.com/gardener/gardener/tree/0.20.0/charts/seed-terraformer/charts/aws-infra)).
Now, Gardener commissions an external, provider-specific controller to take over this task.

## Which infrastructure resources are required?

Unfortunately, there is no general answer to this question as it is highly provider specific.
Consider the above mentioned resources, i.e., VPC, subnets, route tables, security groups, IAM roles, SSH key pairs.
Most of the resources are required in order to create VMs (the shoot cluster worker nodes), load balancers, and volumes.

## What needs to be implemented to support a new infrastructure provider?

As part of the shoot flow Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Infrastructure
metadata:
  name: infrastructure
  namespace: shoot--foo--bar
spec:
  type: azure
  region: eu-west-1
  secretRef:
    name: cloudprovider
    namespace: shoot--foo--bar
  providerConfig:
    apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
    kind: InfrastructureConfig
    resourceGroup:
      name: mygroup
    networks:
      vnet: # specify either 'name' or 'cidr'
      # name: my-vnet
        cidr: 10.250.0.0/16
      workers: 10.250.0.0/19
```

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used to create the needed resources.
However, the most important section is the `.spec.providerConfig`.
It contains an embedded declaration of the provider specific configuration for the infrastructure (that cannot be known by Gardener itself).
You are responsible for designing how this configuration looks like.
Gardener does not evaluate it but just copies this part from what has been provided by the end-user in the `Shoot` resource.

After your controller has created the required resources in your provider's infrastructure it needs to generate an output that can be used by other controllers in subsequent steps.
An example for that is the `Worker` extension resource controller.
It is responsible for creating virtual machines (shoot worker nodes) in this prepared infrastructure.
Everything that it needs to know in order to do that (e.g. the network IDs, security group names, etc. (again: provider-specific)) needs to be provided as output in the `Infrastructure` resource:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Infrastructure
metadata:
  name: infrastructure
  namespace: shoot--foo--bar
spec:
  ...
status:
  lastOperation: ...
  providerStatus:
    apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
    kind: InfrastructureStatus
    resourceGroup:
      name: mygroup
    networks:
      vnet:
        name: my-vnet
      subnets:
      - purpose: nodes
        name: my-subnet
    availabilitySets:
    - purpose: nodes
      id: av-set-id
      name: av-set-name
    routeTables:
    - purpose: nodes
      name: route-table-name
    securityGroups:
    - purpose: nodes
      name: sec-group-name
```

In order to support a new infrastructure provider you need to write a controller that watches all `Infrastructure`s with `.spec.type=<my-provider-name>`.
You can take a look at the below referenced example implementation for the Azure provider.

## Dynamic nodes network for shoot clusters

Some environments do not allow end-users to statically define a CIDR for the network that shall be used for the shoot worker nodes.
In these cases it is possible for the extension controllers to dynamically provision a network for the nodes (as part of their reconciliation loops), and to provide the CIDR in the `status` of the `Infrastructure` resource:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Infrastructure
metadata:
  name: infrastructure
  namespace: shoot--foo--bar
spec:
  ...
status:
  lastOperation: ...
  providerStatus: ...
  nodesCIDR: 10.250.0.0/16
```

Gardener will pick this `nodesCIDR` and use it to configure the VPN components to establish network connectivity between the control plane and the worker nodes.
If the `Shoot` resource already specifies a nodes CIDR in `.spec.networking.nodes` and the extension controller provides also a value in `.status.nodesCIDR` in the `Infrastructure` resource then the latter one will always be considered with higher priority by Gardener.

## Non-provider specific information required for infrastructure creation

Some providers might require further information that is not provider specific but already part of the shoot resource.
One example for this is the [GCP infrastructure controller](https://github.com/gardener/gardener-extension-provider-gcp/tree/master/pkg/controller/infrastructure) which needs the pod and the service network of the cluster in order to prepare and configure the infrastructure correctly.
As Gardener cannot know which information is required by providers it simply mirrors the `Shoot`, `Seed`, and `CloudProfile` resources into the seed.
They are part of the [`Cluster` extension resource](../cluster.md) and can be used to extract information that is not part of the `Infrastructure` resource itself.

## Implementation details

### `Actuator` interface

Most existing infrastructure controller implementations follow a common pattern where a generic `Reconciler` delegates to [an `Actuator` interface](../../../extensions/pkg/controller/infrastructure/actuator.go) that contains the methods `Reconcile`, `Delete`, `Migrate`, and `Restore`. These methods are called by the generic `Reconciler` for the respective operations, and should be implemented by the extension according to the contract described here and the [migration guidelines](../migration.md).

### `ConfigValidator` interface

For infrastructure controllers, the generic `Reconciler` also delegates to [a `ConfigValidator` interface](../../../extensions/pkg/controller/infrastructure/configvalidator.go) that contains a single `Validate` method. This method is called by the generic `Reconciler` at the beginning of every reconciliation, and can be implemented by the extension to validate the `.spec.providerConfig` part of the `Infrastructure` resource with the respective cloud provider, typically the existence and validity of cloud provider resources such as AWS VPCs or GCP Cloud NAT IPs.

The `Validate` method returns a list of errors. If this list is non-empty, the generic `Reconciler` will fail with an error. This error will have the error code `ERR_CONFIGURATION_PROBLEM`, unless there is at least one error in the list that has its `ErrorType` field set to `field.ErrorTypeInternal`.

## References and additional resources

* [`Infrastructure` API (Golang specification)](../../../pkg/apis/extensions/v1alpha1/types_infrastructure.go)
* [Sample implementation for the Azure provider](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/infrastructure)
* [Sample ConfigValidator implementation](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller/infrastructure/configvalidator.go)
