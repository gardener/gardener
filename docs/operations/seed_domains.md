---
title: Domains of a Seed
---

# Domains of a Seed

A `Seed` configures two kinds of domains in `.spec.dns`:

- *Default domains* (`.spec.dns.defaults`) give an external domain to `Shoot`s that do not specify their own domain.
- *Internal domains* (`.spec.dns.internalDomains`) give an internal domain to every `Shoot`.

See [DNSRecord](../extensions/resources/dnsrecord.md#what-does-gardener-create-dns-records-for) for the purpose of the domains.

## Change the Configuration Through the `gardenlet`

The `gardenlet` overwrites the `Seed` spec with `.seedConfig.spec` of its configuration at startup.
Thus, change the domains in the `gardenlet` configuration, i.e., in the `Gardenlet` resource (`.spec.config.seedConfig.spec.dns`), in the `ManagedSeed`, or in the Helm values of the `gardenlet`, but not in the `Seed` object.

For a `ManagedSeed`, the `gardener-apiserver` checks the `Seed` spec when the `ManagedSeed` is updated, and refuses the removal of a domain that is still in use.
For the `Gardenlet` resource and the Helm values, the `gardener-apiserver` checks the `Seed` spec only when the `gardenlet` registers its `Seed` at startup.

> [!IMPORTANT]
> If the `gardener-apiserver` refuses the `Seed` spec from the configuration (e.g., because a domain that is still in use was removed), the new `gardenlet` cannot register its `Seed` and crash-loops.
> Check the logs of the `gardenlet` and add the removed domain to the configuration again.

## Default Domains

A new `Shoot` without its own domain gets `<shoot-name>.<project-name>.<default-domain>` as external domain, with the first entry of `.spec.dns.defaults` as `<default-domain>`.
For the DNS provider, zone, and credentials of the domain of a `Shoot`, Gardener uses the first entry of `.spec.dns.defaults` whose domain is a suffix of the domain of the `Shoot`.

For example, a `Seed` has this configuration:

```yaml
spec:
  dns:
    defaults:
    - type: aws-route53
      domain: dev.example.com
      credentialsRef: {...}
    - type: aws-route53
      domain: example.com
      credentialsRef: {...}
```

- A new `Shoot` `my-shoot` in the project `team-a` gets the domain `my-shoot.team-a.dev.example.com`.
- This domain has the suffixes `dev.example.com` and `example.com`. Gardener uses the first entry, `dev.example.com`.
- An existing `Shoot` with the domain `my-shoot.team-a.example.com` has only the suffix `example.com`. Gardener uses the second entry.

With the `MutableShootDomains` feature gate, a `Shoot` can move to another default domain of its `Seed` with a CA rotation, see [Changing the External Domain](../usage/shoot-operations/shoot_credentials_rotation.md#changing-the-external-domain).
To offer a new default domain, add it to `.spec.dns.defaults`, and keep the old default domain in the list.
Make sure that a `DNSRecord` extension for the type of the new domain is registered, and that its credentials exist.

A default domain cannot be removed while a `Shoot` scheduled onto the `Seed` uses it in `.spec.dns.domain`.
This check does not consider the old domain of a running migration, which is only contained in the `prior-external` entry of `.status.advertisedAddresses`.
Thus, remove an old default domain only after the CA rotations of all `Shoot`s that moved away from it are completed.

## Internal Domains

> [!NOTE]
> Changing, adding, or disabling internal domains of a `Seed` with `Shoot`s scheduled onto it is an alpha feature.
> It requires the `MutableShootDomains` feature gate in `gardener-apiserver` and `gardenlet`, see [GEP-0066](https://github.com/gardener/enhancements/tree/main/geps/0066-make-shoot-domains-mutable).

Every `Shoot` gets an *internal domain* `<shoot-name>.<project-name>.<seed-internal-domain>` (see [DNSRecord](../extensions/resources/dnsrecord.md#internal-domain-name)).
It is used by the control plane components, the nodes (`kubelet`, `gardener-node-agent`), the system components in the `Shoot`, and as default issuer of `ServiceAccount` tokens (`https://api.<internal-domain>`).

The internal domains are configured like this:

```yaml
spec:
  dns:
    internalDomains:
    - type: aws-route53
      domain: internal-new.dev.example.com
      credentialsRef:
        apiVersion: v1
        kind: Secret
        name: internal-domain-new
        namespace: garden
    - type: aws-route53
      domain: internal.dev.example.com
      credentialsRef:
        apiVersion: v1
        kind: Secret
        name: internal-domain-secret
        namespace: garden
  # internalDomainEnabled: false
```

- The first entry of `internalDomains` is the internal domain for new `Shoot`s.
- All other entries are former internal domains which may still be used by `Shoot`s. The `gardenlet` reads the DNS provider, zone, and credentials of the domain a `Shoot` uses from this list.
- `internalDomainEnabled: false` disables the internal domain for new `Shoot`s and for migrated `Shoot`s. Defaults to `true`.
- The deprecated field `internal` is defaulted to the first entry of `internalDomains` and vice versa. If both are set, `internal` must be equal to the first entry of `internalDomains`.

### Migrate the `Shoot`s to a New Internal Domain

1. Add the new internal domain as **first** entry of `internalDomains`, and keep the old domain in the list.
   Make sure that a `DNSRecord` extension for the type of the new domain is registered, and that its credentials exist.
   New `Shoot`s get the new domain immediately. Existing `Shoot`s keep their internal domain and get the constraint `InternalDomainUpToDate` with status `False` (see [Shoot Status](../usage/shoot/shoot_status.md#constraints)).
2. The `Shoot` owners (or the operator) migrate each `Shoot` with a CA rotation, see [Migrating the Internal Domain](../usage/shoot-operations/shoot_credentials_rotation.md#migrating-the-internal-domain).
   The old and the new internal domain are served until the rotation is completed.
3. When no `Shoot` uses the old domain any more, remove it from `internalDomains`.
   A `Shoot` uses a domain if its `.status.advertisedAddresses` contain the domain in the `internal` or `prior-internal` entry.
   Delete the credentials of the old domain only after you removed the domain from `internalDomains`.
   The `gardenlet` reads the credentials of all internal domains of the `Seed`, and a missing credential blocks the reconciliation of all `Shoot`s of the `Seed`.

The `gardener-apiserver` protects the internal domains of a `Seed` with `Shoot`s scheduled onto it:

- An internal domain that is used by a `Shoot` cannot be removed.
- The first internal domain cannot be removed in the same update that replaces it. Make another domain the first one before (step 1).
- No internal domain can be removed before the `gardenlet` has observed the current generation of the `Seed`. This does not apply to the `gardenlet` of the `Seed`, which registers its `Seed` at startup.
- A `Shoot` cannot be moved to another `Seed` (control plane migration) if the target `Seed` does not list the internal domains used by the `Shoot`.

> [!NOTE]
> The domain migration of self-hosted `Shoot`s is not supported yet.

### Disable the Internal Domain

Without an internal domain, the components use the external domain of the `Shoot`, and the default `ServiceAccount` issuer is `https://api.<external-domain>`.

1. Set `internalDomainEnabled: false`, and keep the internal domains in the list.
   This is refused while a `Shoot` without a managed external domain (`.spec.dns.domain` with a DNS provider that is not `unmanaged`) is scheduled onto the `Seed`.
   New `Shoot`s without such a domain are not scheduled onto the `Seed` any more.
2. Existing `Shoot`s get the constraint `InternalDomainUpToDate` with status `False` and are migrated with a CA rotation, the same as above.
3. Remove the internal domains that are no longer used, and delete their credentials afterwards.

To enable the internal domain again, set `internalDomainEnabled: true` (or remove the field) and migrate the `Shoot`s the same way.

### Impact on the `ServiceAccount` Issuer

If a `Shoot` uses the default `ServiceAccount` issuer, the migration of its internal domain changes the issuer, see [Migrating the Internal Domain](../usage/shoot-operations/shoot_credentials_rotation.md#migrating-the-internal-domain).
If a `Shoot` uses a custom or managed issuer, the issuer stays, but tokens of the old default issuer are no longer accepted after the CA rotation is completed.

> [!WARNING]
> Parties outside of the `Shoot` that trust the old issuer (e.g., an identity federation that validates the `iss` claim of `ServiceAccount` tokens) stop accepting new tokens of the `Shoot` as soon as the migration starts.
> Update them to the new issuer before the CA rotation is started.
