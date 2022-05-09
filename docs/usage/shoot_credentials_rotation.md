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

## Kubeconfig

If the `.spec.kubernetes.enableStaticTokenKubeconfig` field is set to `true` (default) then Gardener generates a `kubeconfig` with `cluster-admin` privileges for the `Shoot`s containing credentials for communication with the `kube-apiserver` (see [this document](shoot_access.md#static-token-kubeconfig) for more information).

This `Secret` is stored with name `<shoot-name>.kubeconfig` in the project namespace in the garden cluster and has multiple data keys:

- `kubeconfig`: the completed kubeconfig
- `token`: token for `system:cluster-admin` user
- `username`/`password`: basic auth credentials (if enabled via `Shoot.spec.kubernetes.kubeAPIServer.enableBasicAuthentication`)
- `ca.crt`: the CA bundle for establishing trust to the API server (same as in the [Cluster CA bundle secret](#cluster-certificate-authority-bundle))

> `Shoots` created with Gardener <= 0.28 used to have a `kubeconfig` based on a client certificate instead of a static token. With the first kubeconfig rotation, such clusters will get a static token as well.
>
> ⚠️ This does not invalidate the old client certificate. In order to do this, you should perform a rotation of the CAs (see section below).

**It is the responsibility of the end-user to regularly rotate those credentials (or disable this `kubeconfig` entirely).**
In order to rotate the `token` in this `kubeconfig`, annotate the `Shoot` with `gardener.cloud/operation=rotate-kubeconfig-credentials`.
This operation is not allowed for `Shoot`s that are already marked for deletion.
Please note that only the token (and basic auth password, if enabled) are exchanged.
The CA certificate remains the same (see section below for information about the rotation).

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=rotate-kubeconfig-credentials
```

> You can check the `.status.credentials.rotation.kubeconfig` field in the `Shoot` to see when the rotation was last initiated and last completed.

## Observability Password(s) For Grafana

For `Shoot`s with `.spec.purpose!=testing`, Gardener deploys an observability stack with Prometheus for monitoring, Alertmanager for alerting (optional), Loki for logging, and Grafana for visualization.
The Grafana instance is exposed via `Ingress` and accessible for end-users via basic authentication credentials generated and managed by Gardener.

Those credentials are stored in a `Secret` with name `<shoot-name>.monitoring` in the project namespace in the garden cluster and has multiple data keys:

- `username`: the user name
- `password`: the password
- `basic_auth.csv`: the user name and password in CSV format
- `auth`: the user name with SHA-1 representation of the password

**It is the responsibility of the end-user to regularly rotate those credentials.**
In order to rotate the `password`, annotate the `Shoot` with `gardener.cloud/operation=rotate-observability-credentials`.
This operation is not allowed for `Shoot`s that are already marked for deletion.

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=rotate-observability-credentials
```

> You can check the `.status.credentials.rotation.observability` field in the `Shoot` to see when the rotation was last initiated and last completed.

### Operators

Gardener operators have separate credentials to access their own Grafana instance or Prometheus, Alertmanager, Loki directly.
These credentials are only stored in the shoot namespace in the seed cluster and can be retrieved as follows:

```bash
kubectl -n shoot--<project>--<name> get secret -l name=observability-ingress,managed-by=secrets-manager,manager-identity=gardenlet
```

These credentials are only valid for `30d` and get automatically rotated with the next `Shoot` reconciliation when 80% of the validity approaches or when there are less than `10d` until expiration.
There is no way to trigger the rotation manually.

## SSH Key Pair For Worker Nodes

Gardener generates an SSH key pair whose public key is propagated to all worker nodes of the `Shoot`.
The private key can be used to establish an SSH connection to the workers for troubleshooting purposes.
It is recommended to use [`gardenctl-v2`](https://github.com/gardener/gardenctl-v2/) and its `gardenctl ssh` command since it is required to first open up the security groups and create a bastion VM (no direct SSH access to the worker nodes is possible).

The private key is stored in a `Secret` with name `<shoot-name>.ssh-keypair` in the project namespace in the garden cluster and has multiple data keys:

- `id_rsa`: the private key
- `id_rsa.pub`: the public key for SSH

In order to rotate the keys, annotate the `Shoot` with `gardener.cloud/operation=rotate-ssh-keypair`.
This will propagate a new key to all worker nodes while keeping the old key active and valid as well (it will only be invalidated/removed with the next rotation).

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=rotate-ssh-keypair
```

> You can check the `.status.credentials.rotation.sshKeypair` field in the `Shoot` to see when the rotation was last initiated or last completed.

The old key is stored in a `Secret` with name `<shoot-name>.ssh-keypair.old` in the project namespace in the garden cluster and has the same data keys as the regular `Secret`.

> Note that the SSH keypairs for shoot clusters are rotated automatically during maintenance time window when the `RotateSSHKeypairOnMaintenance` feature gate is enabled.

## ETCD Encryption Key

This key is used to encrypt the data of `Secret` resources inside etcd.
It is currently **not** rotated automatically and there is no way to trigger it manually.

## `ServiceAccount` Token Signing Key

This key is used to sign the tokens for `ServiceAccount`s.
It is currently **not** rotated automatically and there is no way to trigger it manually.
Note that there is the option to pass such key to Gardener, please see [this document](shoot_serviceaccounts.md#signing-key-secret) for more details.

## OpenVPN TLS Auth Keys

This key is used to ensure encrypted communication for the VPN connection between the control plane in the seed cluster and the shoot cluster.
It is currently **not** rotated automatically and there is no way to trigger it manually.
