# Shoot resource customization webhooks

Gardener deploys several components/resources into the shoot cluster.
Some of these resources are essential (like the `kube-proxy`), others are optional addons (like the `kubernetes-dashboard` or the `nginx-ingress-controller`).
In either case, some provider extensions might need to mutate these resources and inject provider-specific bits into it.

## What's the approach to implement such mutations?

Similar to how [control plane components in the seed](controlplane-webhooks.md) are modified we are using `MutatingWebhookConfiguration`s to achieve the same for resources in the shoot.
Both, the provider extension and the kube-apiserver of the shoot cluster are running in the same seed.
Consequently, the kube-apiserver can talk cluster-internally to the provider extension webhook which makes such operations even faster.

## How is the `MutatingWebhookConfiguration` object created in the shoot?

The preferred approach is to use a `ManagedResource` (see also [this document](managedresources.md)) in the seed cluster.
This way the `gardener-resource-manager` ensures that end-users cannot delete/modify the webhook configuration.
The provider extension doesn't need to care about the same.

## What else is needed?

The shoot's kube-apiserver must be allowed to talk to the provider extension.
To achieve this you need to create a `NetworkPolicy` in the shoot namespace.
Our [extension controller library](https://github.com/gardener/gardener-extensions) provides easy-to-use utilities and hooks to implement such a webhook.
You may want to consider to take a look at [this example implementation](https://github.com/gardener/gardener-extensions/commit/b8986482878573d35831f86cdd7eb41160e647ad).
