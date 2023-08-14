# Secrets Management for Seed and Shoot Cluster

The gardenlet needs to create quite some amount of credentials (certificates, private keys, passwords) for seed and shoot clusters in order to ensure secure deployments.
Such credentials typically should be renewed automatically when their validity expires, rotated regularly, and they potentially need to be persisted such that they don't get lost in case of a control plane migration or a lost seed cluster.

## SecretsManager Introduction

These requirements can be covered by using the `SecretsManager` package maintained in [`pkg/utils/secrets/manager`](../../pkg/utils/secrets/manager).
It is built on top of the `ConfigInterface` and `DataInterface` interfaces part of [`pkg/utils/secrets`](../../pkg/utils/secrets) and provides the following functions:

- `Generate(context.Context, secrets.ConfigInterface, ...GenerateOption) (*corev1.Secret, error)`

  This method either retrieves the current secret for the given configuration or it (re)generates it in case the configuration changed, the signing CA changed (for certificate secrets), or when proactive rotation was triggered.
  If the configuration describes a certificate authority secret then this method automatically generates a bundle secret containing the current and potentially the old certificate.
  Available `GenerateOption`s:
  - `SignedByCA(string, ...SignedByCAOption)`: This is only valid for certificate secrets and automatically retrieves the correct certificate authority in order to sign the provided server or client certificate.
    - There are two `SignedByCAOption`s:
      - `UseCurrentCA`. This option will sign server certificates with the new/current CA in case of a CA rotation. For more information, please refer to the ["Certificate Signing"](#certificate-signing) section below.
      - `UseOldCA`. This option will sign client certificates with the old CA in case of a CA rotation. For more information, please refer to the ["Certificate Signing"](#certificate-signing) section below.
  - `Persist()`: This marks the secret such that it gets persisted in the `ShootState` resource in the garden cluster. Consequently, it should only be used for secrets related to a shoot cluster.
  - `Rotate(rotationStrategy)`: This specifies the strategy in case this secret is to be rotated or regenerated (either `InPlace` which immediately forgets about the old secret, or `KeepOld` which keeps the old secret in the system).
  - `IgnoreOldSecrets()`: This specifies that old secrets should not be considered and loaded (contrary to the default behavior). It should be used when old secrets are no longer important and can be "forgotten" (e.g. in ["phase 2" (`t2`) of the CA certificate rotation](../proposals/18-shoot-CA-rotation.md#rotation-sequence-for-cluster-and-client-ca)). Such old secrets will be deleted on `Cleanup()`.
  - `IgnoreOldSecretsAfter(time.Duration)`: This specifies that old secrets should not be considered and loaded once a given duration after rotation has passed. It can be used to clean up old secrets after automatic rotation (e.g. the Seed cluster CA is automatically rotated when its validity will soon end and the old CA will be cleaned up 24 hours after triggering the rotation).
  - `Validity(time.Duration)`: This specifies how long the secret should be valid. For certificate secret configurations, the manager will automatically deduce this information from the generated certificate.

- `Get(string, ...GetOption) (*corev1.Secret, bool)`

  This method retrieves the current secret for the given name.
  In case the secret in question is a certificate authority secret then it retrieves the bundle secret by default.
  It is important that this method only knows about secrets for which there were prior `Generate` calls.
  Available `GetOption`s:
  - `Bundle` (default): This retrieves the bundle secret.
  - `Current`: This retrieves the current secret.
  - `Old`: This retrieves the old secret.

- `Cleanup(context.Context) error`

  This method deletes secrets which are no longer required.
  No longer required secrets are those still existing in the system which weren't detected by prior `Generate` calls.
  Consequently, only call `Cleanup` after you have executed `Generate` calls for all desired secrets.

Some exemplary usages would look as follows:

```go
secret, err := k.secretsManager.Generate(
    ctx,
    &secrets.CertificateSecretConfig{
        Name:                        "my-server-secret",
        CommonName:                  "server-abc",
        DNSNames:                    []string{"first-name", "second-name"},
        CertType:                    secrets.ServerCert,
        SkipPublishingCACertificate: true,
    },
    secretsmanager.SignedByCA("my-ca"),
    secretsmanager.Persist(),
    secretsmanager.Rotate(secretsmanager.InPlace),
)
if err != nil {
    return err
}
```

As explained above, the caller does not need to care about the renewal, rotation or the persistence of this secret - all of these concerns are handled by the secrets manager.
Automatic renewal of secrets happens when their validity approaches 80% or less than `10d` are left until expiration.

In case a CA certificate is needed by some component, then it can be retrieved as follows:

```go
caSecret, found := k.secretsManager.Get("my-ca")
if !found {
    return fmt.Errorf("secret my-ca not found")
}
```

As explained above, this returns the bundle secret for the CA `my-ca` which might potentially contain both the current and the old CA (in case of rotation/regeneration).

### Certificate Signing

#### Default Behaviour

By default, client certificates are signed by the current CA while server certificate are signed by the old CA (if it exists).
This is to ensure a smooth exchange of certificate during a CA rotation (typically has two phases, ref [GEP-18](../proposals/18-shoot-CA-rotation.md#rotation-sequence-for-cluster-and-client-ca)):

- Client certificates:
  - In phase 1, clients get new certificates as soon as possible to ensure that all clients have been adapted before phase 2.
  - In phase 2, the respective server drops accepting certificates signed by the old CA.
- Server certificates:
  - In phase 1, servers still use their old/existing certificates to allow clients to update their CA bundle used for verification of the servers' certificates.
  - In phase 2, the old CA is dropped, hence servers need to get a certificate signed by the new/current CA. At this point in time, clients have already adapted their CA bundles.

#### Alternative: Sign Server Certificates with Current CA

In case you control all clients and update them at the same time as the server, it is possible to make the secrets manager generate even server certificates with the new/current CA.
This can help to prevent certificate mismatches when the CA bundle is already exchanged while the server still serves with a certificate signed by a CA no longer part of the bundle.

Let's consider the two following examples:

1. `gardenlet` deploys a webhook server (`gardener-resource-manager`) and a corresponding `MutatingWebhookConfiguration` at the same time. In this case, the server certificate should be generated with the new/current CA to avoid above mentioned certificate mismatches during a CA rotation.
2. `gardenlet` deploys a server (`etcd`) in one step, and a client (`kube-apiserver`) in a subsequent step. In this case, the default behaviour should apply (server certificate should be signed by old/existing CA).

#### Alternative: Sign Client Certificate with Old CA

In the unusual case where the client is deployed before the server, it might be useful to always use the old CA for signing the client's certificate.
This can help to prevent certificate mismatches when the client already gets a new certificate while the server still only accepts certificates signed by the old CA.

Let's consider the two following examples:

1. `gardenlet` deploys the `kube-apiserver` before the `kubelet`. However, the `kube-apiserver` has a client certificate signed by the `ca-kubelet` in order to communicate with it (e.g., when retrieving logs or forwarding ports). In this case, the client certificate should be generated with the old CA to avoid above mentioned certificate mismatches during a CA rotation.
2. `gardenlet` deploys a server (`etcd`) in one step, and a client (`kube-apiserver`) in a subsequent step. In this case, the default behaviour should apply (client certificate should be signed by new/current CA).

## Reusing the SecretsManager in Other Components

While the `SecretsManager` is primarily used by gardenlet, it can be reused by other components (e.g. extensions) as well for managing secrets that are specific to the component or extension. For example, provider extensions might use their own `SecretsManager` instance for managing the serving certificate of `cloud-controller-manager`.

External components that want to reuse the `SecretsManager` should consider the following aspects:

- On initialization of a `SecretsManager`, pass an `identity` specific to the component, controller and purpose. For example, gardenlet's shoot controller uses `gardenlet` as the `SecretsManager`'s identity, the `Worker` controller in `provider-foo` should use `provider-foo-worker`, and the `ControlPlane` controller should use `provider-foo-controlplane-exposure` for `ControlPlane` objects of purpose `exposure`.
  The given identity is added as a value for the `manager-identity` label on managed `Secret`s.
  This label is used by the `Cleanup` function to select only those `Secret`s that are actually managed by the particular `SecretManager` instance. This is done to prevent removing still needed `Secret`s that are managed by other instances.
- Generate dedicated CAs for signing certificates instead of depending on CAs managed by gardenlet.
- Names of `Secret`s managed by external `SecretsManager` instances must not conflict with `Secret` names from other instances (e.g. gardenlet).
- For CAs that should be rotated in lock-step with the Shoot CAs managed by gardenlet, components need to pass information about the last rotation initiation time and the current rotation phase to the `SecretsManager` upon initialization.
  The relevant information can be retrieved from the `Cluster` resource under `.spec.shoot.status.credentials.rotation.certificateAuthorities`.
- Independent of the specific identity, secrets marked with the `Persist` option are automatically saved in the `ShootState` resource by the gardenlet and are also restored by the gardenlet on Control Plane Migration to the new Seed.

## Migrating Existing Secrets To SecretsManager

If you already have existing secrets which were not created with `SecretsManager`, then you can (optionally) migrate them by labeling them with `secrets-manager-use-data-for-name=<config-name>`.
For example, if your `SecretsManager` generates a `CertificateConfigSecret` with name `foo` like this

```go
secret, err := k.secretsManager.Generate(
    ctx,
    &secrets.CertificateSecretConfig{
        Name:                        "foo",
        // ...
    },
)
```

and you already have an existing secret in your system whose data should be kept instead of regenerated, then labeling it with `secrets-manager-use-data-for-name=foo` will instruct `SecretsManager` accordingly.

**⚠️ Caveat: You have to make sure that the existing `data` keys match with what `SecretsManager` uses:**

| Secret Type          | Data Keys                                               |
| -------------------- |---------------------------------------------------------|
| Basic Auth           | `username`, `password`, `auth`                          |
| CA Certificate       | `ca.crt`, `ca.key`                                      |
| Non-CA Certificate   | `tls.crt`, `tls.key`                                    |
| Control Plane Secret | `ca.crt`, `username`, `password`, `token`, `kubeconfig` |
| ETCD Encryption Key  | `key`, `secret`                                         |
| Kubeconfig           | `kubeconfig`                                            |
| RSA Private Key      | `id_rsa`, `id_rsa.pub`                                  |
| Static Token         | `static_tokens.csv`                                     |
| VPN TLS Auth         | `vpn.tlsauth`                                           |

## Implementation Details

The source of truth for the secrets manager is the list of `Secret`s in the Kubernetes cluster it acts upon (typically, the seed cluster).
The persisted secrets in the `ShootState` are only used if and only if the shoot is in the `Restore` phase - in this case all secrets are just synced to the seed cluster so that they can be picked up by the secrets manager.

In order to prevent kubelets from unneeded watches (thus, causing some significant traffic against the `kube-apiserver`), the `Secret`s are marked as immutable.
Consequently, they have a unique, deterministic name which is computed as follows:

- For CA secrets, the name is just exactly the name specified in the configuration (e.g., `ca`). This is for backwards-compatibility and will be dropped in a future release once all components depending on the static name have been adapted.
- For all other secrets, the name specified in the configuration is used as prefix followed by an 8-digit hash. This hash is computed out of the checksum of the secret configuration and the checksum of the certificate of the signing CA (only for certificate configurations).

In all cases, the name of the secrets is suffixed with a 5-digit hash computed out of the time when the rotation for this secret was last started.
