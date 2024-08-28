---
title: CA Rotation
---

# CA Rotation in Extensions

[GEP-18](../proposals/18-shoot-CA-rotation.md) proposes adding support for automated rotation of Shoot cluster certificate authorities (CAs).
This document outlines all the requirements that Gardener extensions need to fulfill in order to support the CA rotation feature.

## Requirements for Shoot Cluster CA Rotation

- Extensions must not rely on static CA `Secret` names managed by the gardenlet, because their names are changing during CA rotation.
- Extensions cannot issue or use client certificates for authenticating against shoot API servers. Instead, they should use short-lived auto-rotated `ServiceAccount` tokens via gardener-resource-manager's `TokenRequestor`. Also see [Conventions](./conventions.md) and [`TokenRequestor`](../concepts/resource-manager.md#tokenrequestor) documents.
- Extensions need to generate dedicated CAs for signing server certificates (e.g. `cloud-controller-manager`). There should be one CA per controller and purpose in order to bind the lifecycle to the reconciliation cycle of the respective object for which it is created.
- CAs managed by extensions should be rotated in lock-step with the shoot cluster CA.
  When the user triggers a rotation, the gardenlet writes phase and initiation time to `Shoot.status.credentials.rotation.certificateAuthorities.{phase,lastInitiationTime}`. See [GEP-18](../proposals/18-shoot-CA-rotation.md#rotation-sequence-for-cluster-and-client-ca) for a detailed description on what needs to happen in each phase.
  Extensions can retrieve this information from [`Cluster.shoot.status`](./cluster.md).

## Utilities for Secrets Management

In order to fulfill the requirements listed above, extension controllers can reuse the [`SecretsManager`](../development/secrets_management.md) that the gardenlet uses to manage all shoot cluster CAs, certificates, and other secrets as well.
It implements the core logic for managing secrets that need to be rotated, auto-renewed, etc.

Additionally, there are utilities for reusing `SecretsManager` in extension controllers.
They already implement the above requirements based on the `Cluster` resource and allow focusing on the extension controllers' business logic.

For example, a simple `SecretsManager` usage in an extension controller could look like this:

```go
const (
  // identity for SecretsManager instance in ControlPlane controller
  identity = "provider-foo-controlplane"
  // secret config name of the dedicated CA
  caControlPlaneName = "ca-provider-foo-controlplane"
)

func Reconcile() {
  var (
    cluster *extensionscontroller.Cluster
    client  client.Client

    // define wanted secrets with options
    secretConfigs = []extensionssecretsmanager.SecretConfigWithOptions{
      {
        // dedicated CA for ControlPlane controller
        Config: &secretutils.CertificateSecretConfig{
          Name:       caControlPlaneName,
          CommonName: "ca-provider-foo-controlplane",
          CertType:   secretutils.CACert,
        },
        // persist CA so that it gets restored on control plane migration
        Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
      },
      {
        // server cert for control plane component
        Config: &secretutils.CertificateSecretConfig{
          Name:       "cloud-controller-manager",
          CommonName: "cloud-controller-manager",
          DNSNames:   kutil.DNSNamesForService("cloud-controller-manager", namespace),
          CertType:   secretutils.ServerCert,
        },
        // sign with our dedicated CA
        Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caControlPlaneName)},
      },
    }
  )

  // initialize SecretsManager based on Cluster object
  sm, err := extensionssecretsmanager.SecretsManagerForCluster(ctx, logger.WithName("secretsmanager"), clock.RealClock{}, client, cluster, identity, secretConfigs)

  // generate all wanted secrets (first CAs, then the rest)
  secrets, err := extensionssecretsmanager.GenerateAllSecrets(ctx, sm, secretConfigs)

  // cleanup any secrets that are not needed any more (e.g. after rotation)
  err = sm.Cleanup(ctx)
}
```

Please pay attention to the following points:
- There should be one `SecretsManager` identity per controller (and purpose if applicable) in order to prevent conflicts between different instances.
  E.g., there should be different identities for `Infrastructrue`, `Worker` controller, etc., and the `ControlPlane` controller should use dedicated `SecretsManager` identities per purpose (e.g. `provider-foo-controlplane` and `provider-foo-controlplane-exposure`).
- All other points in [Reusing the SecretsManager in Other Components](../development/secrets_management.md#reusing-the-secretsmanager-in-other-components).
