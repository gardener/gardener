# SecretBinding Provider Populator

This page describes the process on how to enable the SecretBinding provider populator.

## Overview

With Gardener v1.37.0 the SecretBinding resource does now contain a new optional field `.provider.type` (details about the motivation can be found in https://github.com/gardener/gardener/issues/4888). To make the process of setting the new field automated and afterwards to enforce validation on the new field in backwards compatible manner, Gardener contains several feature gates - `SecretBindingProviderPopulator`, `RequiredSecretBindingProvider`.

## Process

A Gardener landscape operator can follow the following steps:

1. Enable the `SecretBindingProviderPopulator` feature gate of Gardener Controller Manager.

   The `SecretBindingProviderPopulator` enables the SecretBinding provider populator controller in the Gardener Controller Manager. This controller is responsible to populate the `.provider.type` field of a SecretBinding based on its current usage by Shoot resources. For example if a Shoot `crazy-botany` with `.provider.type=aws` is using a SecretBinding `my-secret-binding`, then the SecretBinding provider populator controller will take care to set the `.provider.type` field of the SecretBinding to the same provider type (`aws`).
   Although that it is not recommended, the API allows Shoots from different provider types to reference the same SecretBinding (assuming that backing Secret contains data for both of the provider types). To preserve the backwards compatibility for such SecretBindings, the provider populator controller will maintain the multiple provider types in the field (it will join them with separator `,` - for example `aws,gcp`).

2. Disable the SecretBinding provider populator controller and enable `RequiredSecretBindingProvider` feature gate of Gardener API server.

   The `RequiredSecretBindingProvider` feature gate of Gardener API server enables validation that forbids creating a Shoot that has a different provider type from the referenced SecretBinding's one.
   After making sure that SecretBinding provider populator controller is enabled and it populated the `.provider.type` field of a majority of the SecretBindings on a Gardener landscape (the SecretBindings that are unused will have their provider type unset), a Gardener landscape operator has to disable the SecretBinding provider populator controller and to enable the `RequiredSecretBindingProvider` feature gate of Gardener API server. To disable the SecretBinding provider populator controller, in the ControllerManagerConfiguration set the `controller.secretBindingProviderPopulator.concurentSyncs` field to `0`.

3. Enable `ImmutableSecretBindingProvider` feature gate of Gardener API server

   The `ImmutableSecretBindingProvider` feature gate enables a validation on Gardener API server that enforces immutability on the provider type of a SecretBinding.
