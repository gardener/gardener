---
title: ETCD Encryption Config
description: Specifying resource types for encryption with `spec.kubernetes.kubeAPIServer.encryptionConfig`
---

# ETCD Encryption Config

The `spec.kubernetes.kubeAPIServer.encryptionConfig` field in the Shoot API allows users to customize encryption configurations for the API server. It provides options to specify additional resources for encryption beyond secrets.

## Usage Guidelines

- The `resources` field can be used to specify resources that should be encrypted in addition to secrets. Secrets are always encrypted.
  - Each item is a Kubernetes resource name in plural (resource or resource.group). Wild cards are not supported.
  - Adding an item to this list will cause patch requests for all the resources of that kind to encrypt them in the etcd. See [Encrypting Confidential Data at Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data) for more details.
  - Removing an item from this list will cause patch requests for all the resources of that type to decrypt and rewrite the resource as plain text. See [Decrypt Confidential Data that is Already Encrypted at Rest](https://kubernetes.io/docs/tasks/administer-cluster/decrypt-data/) for more details.
- The `provider` field specifies which provider type is used for encryption.
  - Supported provider types:
    - `aescbc`
    - `aesgcm`
    - `secretbox`
  - The default encryption provider is `aesgcm`. Shoot clusters that were created before Gardener `v1.137` have defaulted to the `aescbc` provider.
  - This field is immutable.
  - **Important for `aesgcm`**: The `aesgcm` provider uses 96-bit IVs (nonces), and per [NIST SP 800-38D](https://csrc.nist.gov/pubs/sp/800/38/d/final), the total number of encryption invocations with a given key should not exceed 2³². To mitigate the risk of nonce collisions, Gardener defaults the ETCD encryption key auto-rotation period to **28 days** for newly created Shoots using `aesgcm`. The maximum allowed rotation period for `aesgcm` is **90 days**.

## Example Usage in a `Shoot`

```yaml
spec:
  kubernetes:
    kubeAPIServer:
      encryptionConfig:
        resources:
          - configmaps
          - statefulsets.apps
          - customresource.fancyoperator.io
        provider:
          type: "secretbox"
```
