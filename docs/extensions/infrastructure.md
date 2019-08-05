# Contract: `Infrastructure` resource

Every Kubernetes cluster requires some low-level infrastructure to be setup in order to work properly.
Examples for that are networks, routing entries, security groups, IAM roles, etc.
Before introducing the `Infrastructure` extension resource Gardener was using Terraform in order to create and manage these provider-specific resources (e.g., see [here](https://github.com/gardener/gardener/tree/0.20.0/charts/seed-terraformer/charts/aws-infra)).
Now, Gardener commissions an external, provider-specific controller to take over this task.

## Which infrastructure resources are required?

Unfortunately, there is no general answer to this question as it is highly provider specific.
Consider the above mentioned resources, i.e. VPC, subnets, route tables, security groups, IAM roles, SSH key pairs.
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
Everything that it needs to know in order to do that (e.g., the network IDs, security group names, etc. (again: provider-specific)) needs to be provided as output in the `Infrastructure` resource:

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

## Non-provider specific information required for infrastructure creation

Some providers might require further information that is not provider specific but already part of the shoot resource.
One example for this is the [GCP infrastructure controller](https://github.com/gardener/gardener-extensions/tree/master/controllers/provider-gcp/pkg/controller/infrastructure) which needs the pod and the service network of the cluster in order to prepare and configure the infrastructure correctly.
As Gardener cannot know which information is required by providers it simply mirrors the `Shoot`, `Seed`, and `CloudProfile` resources into the seed.
They are part of the [`Cluster` extension resource](cluster.md) and can be used to extract information that is not part of the `Infrastructure` resource itself.

## References and additional resources

* [`Infrastructure` API (Golang specification)](../../pkg/apis/extensions/v1alpha1/types_infrastructure.go)
* [Exemplary implementation for the Azure provider](https://github.com/gardener/gardener-extensions/tree/master/controllers/provider-azure/pkg/controller/infrastructure)
