# Gardener Seed Admission Controller

The Gardener Seed admission controller is deployed by the Gardenlet as part of its seed bootstrapping phase and, consequently, running in every seed cluster.
It's main purpose is to serve webhooks (validating or mutating) in order to admit or deny certain requests to the seed's API server.

## What is it doing concretely?

### Validating Webhooks

#### Unconfirmed Deletion Prevention
As part of Gardener's [extensibility concepts](../extensions/overview.md) a lot of `CustomResourceDefinition`s are deployed to the seed clusters that serve as extension points for provider-specific controllers.
For example, the [`Infrastructure` CRD](../extensions/infrastructure.md) triggers the provider extension to prepare the IaaS infrastructure of the underlying cloud provider for a to-be-created shoot cluster.
Consequently, these extension CRDs have a lot of power and control large portions of the end-user's shoot cluster.
Accidental or undesired deletions of those resource can cause tremendous and hard-to-recover-from outages and should be prevented.

Together with the deployment of the Gardener seed admission controller a `ValidatingWebhookConfiguration` for `CustomResourceDefinitions` and most (custom) resources in the `extensions.gardener.cloud/v1alpha1` API group is registered.
It prevents `DELETE` requests for those `CustomResourceDefinitions` labeled with `gardener.cloud/deletion-protected=true`, and for all mentioned custom resources if they were not previously annotated with the `confirmation.gardener.cloud/deletion=true`.
This prevents that undesired `kubectl delete <...>` requests are accepted.

### Mutating Webhooks

The admission controller endpoint `/webhooks/default-pod-scheduler-name/gardener-kube-scheduler` mutates `pods` and adds `gardener-kube-scheduler` to `.spec.scheduleName`.

When `SeedKubeScheduler` feature gate is enabled, all control plane components are mutated. The scheduler scores `Nodes` with most resource usage higher than the rest, resulting in greater resource utilization.
