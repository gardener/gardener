# Secrets Management for Seed and Shoot Cluster

🚧️ Please note that the work in the new secrets management is ongoing and hence not yet completed.
Accordingly, expect adaptations to this document and implementation details.

The gardenlet needs to create quite some amount of credentials (certificates, private keys, passwords, etc.) for seed and shoot clusters in order to ensure secure deployments.
Such credentials typically should be rotated regularly, and they potentially need to be persisted such that they don't get lost in case of a control plane migration or a lost seed cluster.

These requirements can be covered by using the `SecretsManager` package maintained in [`pkg/utils/secrets/manager`](pkg/utils/secrets/manager).
It is built on top of the `ConfigInterface` and `DataInterface` interfaces part of [`pkg/utils/secrets`](pkg/utils/secrets) and provides the following functions:

- `GetOrGenerate(context.Context, secrets.ConfigInterface, ...GetOrGenerateOption) (*corev1.Secret, error)`

  This method either retrieves the current secret for the given configuration or it (re)generates it in case the configuration changed, the signing CA changed (for certificate secrets), or when proactive rotation was triggered.
  If the configuration describes a certificate authority secret then this method automatically generates a bundle secret containing the current and potentially the old certificate.\
  Available `GetOrGenerateOption`s:
  - `SignedByCA(string)`: This is only valid for certificate secrets and automatically retrieves the correct certificate authority in order to sign the provided server or client certificate.
  - `Persist()`: This marks the secret such that it gets persisted in the `ShootState` resource in the garden cluster. Consequently, it should only be used for secrets related to a shoot cluster.
  - `Rotate(rotationStrategy)`: This specifies the strategy in case this secret is to be rotated or regenerated (either `InPlace` which immediately forgets about the old secret, or `KeepOld` which keeps the old secret in the system).
  - `IgnoreOldSecrets()`: This specifies whether old secrets should be considered and loaded (which is done by default). It should be used when old secrets are no longer important and can be "forgotten" (e.g. in ["phase 2" (`t2`) of the CA certificate rotation](../proposals/18-shoot-CA-rotation.md#rotation-sequence-for-cluster-and-client-ca)).

- `GetByName(string, ...GetByNameOption) (*corev1.Secret, error)`

  This method retrieves the current secret for the given name.
  In case the secret in question is a certificate authority secret then it retrieves the bundle secret by default.
  It is important that this method only knows about secrets for which there were prior `GetOrGenerate` calls.\
  Available `GetByNameOption`s:
  - `Bundle` (default): This retrieves the bundle secret.
  - `Current`: This retrieves the current secret.
  - `Old`: This retrieves the old secret.

- `Cleanup(context.Context) error`

  This method deletes secrets which are no longer required.
  No longer required secrets are those still existing in the system which weren't detected by prior `GetOrGenerate` calls.
  Consequently, only call `Cleanup` after you have executed `GetOrGenerate` calls for all desired secrets.

Some exemplary usages would look as follows:

```go
secret, err := k.secretsManager.GetOrGenerate(
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

As explained above, the caller does not need to care about the rotation or the persistence of this secret - all of these concerns are handled by the secrets manager.

In case a CA certificate is needed by some component then it can be retrieved as follows:

```go
caSecret, err := k.secretsManager.GetByName("my-ca")
if err != nil {
    return err
}
```

As explained above, this returns the bundle secret for the CA `my-ca` which might potentially contain both the current and the old CA (in case of rotation/regeneration).

## Implementation Details

The source of truth for the secrets manager is the list of `Secret`s in the Kubernetes cluster it acts upon (typically, the seed cluster).
The persisted secrets in the `ShootState` are only used if and only if the shoot is in the `Restore` phase - in this case all secrets are just synced to the seed cluster so that they can be picked up by the secrets manager.

In order to prevent kubelets from unneeded watches (thus, causing some significant traffic against the `kube-apiserver`), the `Secret`s are marked as immutable.
Consequently, they have a unique, deterministic name which is computed as follows:

- For CA secrets, the name is just exactly the name specified in the configuration (e.g., `ca`). This is for backwards-compatibility and will be dropped in a future release once all components depending on the static name have been adapted.
- For all other secrets, the name specified in the configuration is used as prefix followed by an 8-digit hash. This hash is computed out of the checksum of the secret configuration and the checksum of the certificate of the signing CA (only for certificate configurations).

In all cases, the name of the secrets is suffixed with a 5-digit hash computed out of the time when the rotation for this secret was last started.
