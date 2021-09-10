# Secrets and rotation

## List of secrets

**Cloudprovider Secret**

*Example*: https://github.com/gardener/gardener/blob/master/example/70-secret-provider.yaml

*Usage*: Authenticate gardener and kubernetes components for infrastructure operations

*Description*: Gardener uses the cloudprovider secret to interact with the infrastructure when setting up shoot networks or security groups via the [terraformer](https://github.com/gardener/terraformer). It is also used by the [cloud-controller-manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) of your Shoot to communicate with the infrastructure for example to create Loadbalancer services, routes or retrieve information about Node objects.
Depending on the cloudprovider the format of the secret differs. Please consult the example above and respective infrastructure extension documentation for the concrete layout.

To put it in use, a cloudprovider secret is bound to one more namespaces (and therefore projects) using a [SecretBinding](https://github.com/gardener/gardener/blob/master/example/80-secretbinding.yaml). For Shoots created in those projects the secret is synced to the shoot namespace in the seed cluster.

*Rotation*: Rotating the cloudprovider secret requires multiple steps:

1. Update the data keys in the secret.
2. :warning: Wait until all Shoots using the secret are reconciled before you disable the old secret in your infrastructure account! Otherwise the shoots will no longer function.
3. After all Shoots using the secret were reconciled you can go ahead and deactivate the old secret in your infrastructure account.
