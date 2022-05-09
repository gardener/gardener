# Credentials Rotation For Shoot Clusters

There are a lot of different credentials for `Shoot`s to make sure that the various components can communicate with each other, and to make sure it is usable and operable.

This page explains how the varieties of credentials can be rotated so that the cluster can be considered secure.

## Cloud Provider Keys

End-users must provide credentials such that Gardener and Kubernetes controllers can communicate with the respective cloud provider APIs in order to perform infrastructure operations.
For example, Gardener uses them to setup and maintain the networks, security groups, subnets, etc., while the [cloud-controller-manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) uses them to reconcile load balancers and routes, and the [CSI controller](https://kubernetes-csi.github.io/docs/) uses them to reconcile volumes and disks.

Depending on the cloud provider, the required [data keys of the `Secret` differ](https://github.com/gardener/gardener/blob/master/example/70-secret-provider.yaml).
Please consult the documentation of the respective provider extension documentation to get to know the concrete data keys (e.g., [this document for AWS](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage-as-end-user.md#provider-secret-data)).

**It is the responsibility of the end-user to regularly rotate those credentials.**
The following steps are required to perform the rotation:

1. Update the data in the `Secret` with new credentials.
2. ⚠️ Wait until all `Shoot`s using the `Secret` are reconciled before you disable the old credentials in your cloud provider account! Otherwise, the `Shoot`s will no longer work as expected. Check out [this document](shoot_operations.md#immediate-reconciliation) to learn how to trigger a reconciliation of your `Shoot`s.
3. After all `Shoot`s using the `Secret` were reconciled, you can go ahead and deactivate the old credentials in your provider account account.
