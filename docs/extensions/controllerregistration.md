# Register extension controllers

Extensions are registered in the garden cluster via [`ControllerRegistration`](../../example/25-controllerregistration.yaml) resources.
Gardener is evaluating the registrations and creates [`ControllerInstallation`](../../example/25-controllerinstallation.yaml) resources which describe the request "please install this controller `X` to this seed `Y`".

Similar to how `CloudProfile` or `Seed` resources get into the system, the Gardener administrator must deploy the `ControllerRegistration` resources (this does not happen automatically in any way - the administrator decides which extensions shall be enabled).

The specification mainly describes which of Gardener's extension CRDs are managed, for example:

```yaml
apiVersion: core.gardener.cloud/v1alpha1
kind: ControllerRegistration
metadata:
  name: os-coreos
spec:
  resources:
  - kind: OperatingSystemConfig
    type: coreos
```

This information tells Gardener that there is an extension controller that can handle `OperatingSystemConfig` resources of type `coreos`.
It will now create `ControllerInstallation` resources:

```yaml
apiVersion: core.gardener.cloud/v1alpha1
kind: ControllerInstallation
metadata:
  name: os-coreos
spec:
  registrationRef:
    apiVersion: core.gardener.cloud/v1alpha1
    kind: ControllerRegistration
    name: os-coreos
  seedRef:
    apiVersion: core.gardener.cloud/v1alpha1
    kind: Seed
    name: aws-eu1
```

This resource expresses that Gardener requires the `os-coreos` extension controller to run on the `aws-eu1` seed cluster.

PS: Currently, for the sake of implementation simplicity, Gardener demands every extension controller for every seed cluster (although, an AWS controller might not make much sense to run on a GCP seed cluster). We plan to change this in the future, i.e., to make Gardener more intelligent so that it can automatically determine which extension is required on which seed cluster.

## How do extension controllers get deployed to seeds?

After Gardener has written the `ControllerInstallation` resource some component must satisfy this request and start deploying the extension controller to the seed.
Depending on the complexity of the controllers lifecycle management, configuration, etc. there are two possible scenarios:

### Scenario 1: Deployed by Gardener

In many cases the extension controllers are easy to deploy and configure.
It is sufficient to simply create a Helm chart (standardized way of packaging software in the Kubernetes context) and deploy it together with some static configuration values.
Gardener supports this scenario and allows to provide arbitrary deployment information in the `ControllerRegistration` resource's `.spec` section:

```yaml
...
spec:
  ...
  deployment:
    type: helm
    providerConfig:
      chart: H4sIFAAAAAAA/yk...
      values:
        foo: bar
```

If `.spec.deployment.type=helm` then Gardener itself will take over the responsibility the deployment.
It base64-decodes the provided Helm chart (`.spec.deployment.providerConfig.chart`) and deploys it with the provided static configuration (`.spec.deployment.providerConfig.values`).
The chart and the values can be updated at any time - Gardener will recognize and re-trigger the deployment process.

In order to allow extensions to get information about the garden and the seed cluster Gardener does mix-in certain properties into the values (root level) of every deployed Helm chart:

```yaml
gardener:
  garden:
    identifier: <uuid-of-gardener-installation>
  seed:
    identifier: <seed-name>
    region: europe
```

Extensions can use this information in their Helm chart in case they require knowledge about the garden and the seed environment.
The list might be extended in the future.

:information_source: Gardener uses the UUID of the `garden` `Namespace` object in the `.gardener.garden.identifier` property.

### Scenario 2: Deployed by a (non-human) Kubernetes operator

Some extension controllers might be more complex and require additional domain-specific knowledge wrt. lifecycle or configuration.
In this case, we encourage to follow the Kubernetes operator pattern and deploy a dedicated operator for this extension into the garden cluster.
The `ControllerResource`'s `.spec.deployment.type` field would then not be `helm`, and no Helm chart or values need to be provided there.
Instead, the operator itself knows how to deploy the extension into the seed.
It must watch `ControllerInstallation` resources and act one those referencing a `ControllerRegistration` the operator is responsible for.

In order to let Gardener know that the extension controller is ready and running in the seed the `ControllerInstallation`'s `.status` field supports two conditions: `RegistrationValid` and `InstallationSuccessful` - both must be provided by the responsible operator:

```yaml
...
status:
  conditions:
  - lastTransitionTime: "2019-01-22T11:51:11Z"
    lastUpdateTime: "2019-01-22T11:51:11Z"
    message: Chart could be rendered successfully.
    reason: RegistrationValid
    status: "True"
    type: Valid
  - lastTransitionTime: "2019-01-22T11:51:12Z"
    lastUpdateTime: "2019-01-22T11:51:12Z"
    message: Installation of new resources succeeded.
    reason: InstallationSuccessful
    status: "True"
    type: Installed
```

Additionally, the `.status` field has a `providerStatus` section into which the operator can (optionally) put any arbitrary data associated with this installation.

## Extensions in the garden cluster itself

The `Shoot` resource itself will contain some provider-specific data blobs.
As a result, some extensions might also want to run in the garden cluster, e.g., to provide `ValidatingWebhookConfiguration`s for validating the correctness of their provider-specific blobs:

```yaml
apiVersion: gardener.cloud/v1alpha1
kind: Shoot
metadata:
  name: johndoe-aws
  namespace: garden-dev
spec:
  ...
  cloud:
    type: aws
    region: eu-west-1
    providerConfig:
      apiVersion: aws.cloud.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        vpc: # specify either 'id' or 'cidr'
        # id: vpc-123456
          cidr: 10.250.0.0/16
        internal:
        - 10.250.112.0/22
        public:
        - 10.250.96.0/22
        workers:
        - 10.250.0.0/19
      zones:
      - eu-west-1a
...
```

In the above example, Gardener itself does not understand the AWS-specific provider configuration for the infrastructure.
However, if this part of the `Shoot` resource should be validated then you should run an AWS-specific component in the garden cluster that registers a webhook. You can do it similarly if you want to default some fields of a resource (by using a `MutatingWebhookConfiguration`).

Again, similar to how Gardener is deployed to the garden cluster, these components must be deployed and managed by the Gardener administrator.
