---
title: Resource Admission in the Garden Cluster
---

# Validating and Mutating Resources in the Garden Cluster

The `Shoot` resource itself can contain some extension-specific data blobs (see `providerConfig`):

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: johndoe-aws
  namespace: garden-dev
spec:
  ...
  region: eu-west-1
  provider:
    type: aws
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

In the above example, Gardener itself does not understand the AWS-specific provider configuration for the infrastructure. However, if this part of the `Shoot` resource should be validated, then you should run an AWS-specific component in the garden cluster that registers a webhook. You can do it similarly if you want to default some fields of a resource (by using a `MutatingWebhookConfiguration`). Similarly to how Gardener is deployed to the garden cluster, these components must be deployed and managed by the Gardener administrator.

Examples of extensions performing validation:
- [provider extensions](../../extensions/README.md#infrastructure-provider) would validate `spec.provider.infrastructureConfig` and `spec.provider.controlPlaneConfig` in the `Shoot` resource and `spec.providerConfig` in the `CloudProfile` resource.
- [networking extensions](../../extensions/README.md#network-plugin) would validate `spec.networking.providerConfig` in the `Shoot` resource.

As a best practice, the validation should be performed only if there is a change in the `spec` of the resource. Please find an exemplary implementation in the [gardener/gardener-extension-provider-aws](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/admission/validator) repository.

## `extensions.gardener.cloud` Labeling

When an admission relevant resource (e.g., `BackupEntry`s, `BackupBucket`s, `CloudProfile`s, `Seed`s, `SecretBinding`s, and `Shoot`s) is newly created or updated in the garden cluster, Gardener adds an extension label to it. This label is of the form `<extension-type>.extensions.gardener.cloud/<extension-name> : "true"`. For example, an extension label for a provider extension type `aws` looks like `provider.extensions.gardener.cloud/aws : "true"`. The extensions should add object selectors in their admission webhooks for these labels to filter out the objects they are responsible for. Please see the [types_constants.go](../../pkg/apis/core/v1beta1/constants/types_constants.go) file for the full list of extension labels.
