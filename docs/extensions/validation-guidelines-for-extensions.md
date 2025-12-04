# Validation Guidelines For Extensions

This document provides general developer guidelines on validation practices and conventions used in the Gardener extensions codebase.

## Validation of Provider Configuration

With the implementation of [GEP-01](../proposals/01-extensibility.md) the cloud provider, operating system and networking specific knowledge is extracted to external extension controllers.
Gardener itself is provider-agnostic.
The API resources contain provider configurations.
The Gardener API Server does not understand these provider configurations and cannot validate them.
They should be validated by extension admission components.

Provider configuration fields are of type `*runtime.RawExtension`.
Extension admission components should decode the provider configuration and perform validation of it.

### `ConfigValidator` Interfaces

Validation code that performs requests to an external system (e.g. the cloud provider API) should be written in `ConfigValidator` implementations.
Performing requests to an external system in the extension admission component (or in a webhook, in general) is considered a bad practice.
A downtime of the external system then results in rejection of requests by the webhook.
For this purpose, the extension library leverages the `ConfigValidator` interfaces (e.g. see the [`ConfigValidator` interface for Infrastructure](./resources/infrastructure.md#configvalidator-interface)).
They validate the provider configuration against the cloud provider API - e.g. validate the provided AWS VPC ID exists and conforms to the provider extension requirements.
The `ConfigValidator` implementations are invoked by the corresponding extension controller before the main reconciliation logic of the extension resource.
A validation failure in the provider configuration results in a reconciliation error.
Usually, an appropriate non-retryable error code is returned and later on propagated up to the Shoot status.
A failure to perform request to the cloud provider API results in a reconciliation error.
In this way, a failure to perform the validation due to unavailability of a cloud provider API results in a reconciliation error instead of a rejected CREATE/UPDATE request for a Shoot by an extension admission component.

## Validation of Referenced Resources

The provider configuration may contain references to another resources.
Currently, only `Secret`s and `ConfigMap`s are allowed to be referenced.
For more details, see [Referenced Resources](../extensions/referenced-resources.md).

The referenced resources should be validated by the extension admission components.
Usually, the validation consists of checking if the required data keys are present and if the values are in the expected format.
However, it is challenging to validate `UPDATE` operations on referenced resources.
Extension admission components have the following implementation options to cover validation for `UPDATE`:
- Check if a `Secret` or `ConfigMap` is in-use as a referenced resource by a Shoot from the corresponding extension type. This usually increases the memory usage of the component due to client caches for `Secret`s/`ConfigMap`s.
- Enforce the referenced resource to be immutable. In this way, the referenced resource cannot be updated. Hence, there is no need to validate update. However, this approach leads to poor end-user experience. Every time end-users have to update the referenced resource, they have to create a new one and update the reference in the corresponding Shoots.

With https://github.com/gardener/gardener/issues/12582, we want to adapt Gardener to maintain extension-specific label on referenced resources. With this, extension admission components will be able to define an `objectSelector` and filter only the `Secret`s/`ConfigMap`s which are in-use by Shoots using the corresponding extension type.

## General Guidelines

- **Extension should leverage admission component to validate provider configuration and referenced resources**.
- Extension admission and controller components should **decode the provider configurations to the internal type** of their APIs.
  - This way, the extensions always work, independent of what external version the end-users have provided in the provider configuration.
  - See [example](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/provider-local/admission/validator/namespacedcloudprofile.go#L50-L55).
- Extension admission components should decode the provider configurations using **strict mode**.
  - Strict mode forces [additional verifications](https://github.com/kubernetes-sigs/json/blob/cfa47c3a1cc8ff0eff148aa9ec5b0226d0909e87/json.go#L89-L91) on the decoded data such as:
    - ensures no duplicate fields
    - ensures no unknown fields when decoding into typed structs
  - See [example](https://github.com/gardener/gardener/blob/f6fb7e2ca019fdd2a09c0a5da6475bf5d6bd2430/pkg/provider-local/controller/worker/actuator.go#L53).
- Webhooks of extension admission components should use `objectSelector` to filter only requests for resources that use the extension.
  - The extension admission components should use the extension-specific labels maintained on the API resources by the [`ExtensionLabels` admission plugin](../concepts/apiserver-admission-plugins.md#extensionlabels).
- See the [General Guidelines for Gardener core components](../development/validation-guidelines.md#general-guidelines).
