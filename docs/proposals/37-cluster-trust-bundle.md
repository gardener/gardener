# GEP-37: Per-Shoot `ClusterTrustBundle` for Custom CA Trust Anchors

> Note to reviewers: new GEPs live in [gardener/enhancements](https://github.com/gardener/enhancements/tree/main/geps).
> This document is submitted as a draft here on purpose — once its shape is agreed, it will be
> moved over as `geps/0037-cluster-trust-bundle`. Discussion on substance is welcome right here.

## Summary

When a landscape operator configures a custom CA trust bundle on a `Seed`
(via `.spec.settings.trustBundle.secretRef` — see the related PR chain), gardenlet creates
one `ClusterTrustBundle` object (`certificates.k8s.io/v1`, GA since Kubernetes v1.37) in every
shoot cluster running k8s ≥ 1.37, holding that bundle. Workloads can then mount the bundle via
`projected.volumes[].sources[].clusterTrustBundle` — the Kubernetes-native vehicle for shipping
cluster trust anchors to pods.

## Motivation

Operators that must trust additional certificate authorities (internal PKI endpoints, corporate
proxies, private registries) currently have poor options for user workloads on shoot clusters:

- mounting a shared `ConfigMap` per namespace — must be replicated into every namespace,
  rotation is ad-hoc, and every author must know to do it;
- baking CAs into application images — couples trust policy to the image build pipeline and
  makes short-term revocation effectively impossible;
- mutating admission webhooks that inject the bundle into every pod — operationally fragile,
  easy to bypass, surprising for users.

`ClusterTrustBundle` exists precisely for this use case: a cluster-scoped, declaratively
consumable set of trust anchors, selectable by name or by `signerName`, mounted as an ordinary
projected volume source by the kubelet.

### Goals

- for shoots running k8s ≥ 1.37 on a seed with `.spec.settings.trustBundle.secretRef` set,
  gardenlet reconciles a `ClusterTrustBundle` object containing the bundle;
- removing the bundle from the seed (or the shoot being deleted) removes the object;
- changes to the referenced Secret propagate on the next seed/shoot reconcile;
- workload consumption is strictly opt-in via the pod spec.

### Non-Goals

- forcing the bundle on all pods of a shoot (e.g. by admission webhook) — opt-in by design;
- supporting shoots on k8s < 1.37 (the API is alpha→GA across 1.27–1.37; landscapes on older
  versions keep using whatever per-namespace mechanisms they have today);
- installing the bundle into the nodes' *system* trust store — that is covered by a sibling
  proposal extending gardenlet's node-init flow;
- managing signer-linked bundles (`.spec.signerName`) — the bundle is an additional trust
  anchor set, not tied to any particular signer.

## Proposal

gardenlet monitors the `Seed`'s `.spec.settings.trustBundle.secretRef`. For every shoot

- scheduled onto that seed, and
- running Kubernetes v1.37 or newer,

it reconciles a `ClusterTrustBundle`:

```yaml
apiVersion: certificates.k8s.io/v1
kind: ClusterTrustBundle
metadata:
  name: gardener.cloud/trust-bundle
spec:
  trustBundle: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

- `metadata.name` is a reserved, gardener-owned name: `gardener.cloud/trust-bundle`
  (name selection via the `gardener.cloud/` domain prefix).
- `spec.signerName` is empty.
- `spec.trustBundle` mirrors the PEM data of the referenced Secret.

When the reference is removed, or the shoot migrates off the seed, the object is deleted
(garbage-collected via the shoot reconcile loop, not via owner references, since the
`ClusterTrustBundle` is cluster-scoped and its "owner" logically lives in the garden cluster).

Shoots on k8s < 1.37 are skipped; no API object is created and no error is raised — the
feature simply becomes available upon upgrading the shoot.

### Workload Consumption

Workload authors opt in with:

```yaml
spec:
  containers:
  - name: my-app
    volumeMounts:
    - name: trust-bundle
      mountPath: /etc/ssl/certs
      readOnly: true
  volumes:
  - name: trust-bundle
    projected:
      sources:
      - clusterTrustBundle:
          name: gardener.cloud/trust-bundle
          path: ca-certificates.crt
```

The kubelet resolves the bundle at pod admission time and writes the PEM at
`path` inside the projected volume; updates to the object propagate like ConfigMap updates.

### Risks and Mitigations

- *Version skew between garden cluster and shoot*: the garden cluster must understand
  `ClusterTrustBundle` to let gardenlet manage it; in turn, shoots are gated on ≥ 1.37.
  Mitigation: gardenlet checks the shoot's k8s version before creating/updating the object and
  no-ops below 1.37.
- *Object hijacking pre-GA*: a user could pre-create the same name on a shoot. Mitigation:
  gardenlet uses server-side apply/field ownership and reconciles drift, same as other
  gardener-managed shoot resources.
- *Accidental trust of a hostile CA*: configuring `trustBundle.secretRef` on a `Seed` is a
  privileged operation (write access to `Seed` resources); the risk profile is the same as for
  any seed-level configuration.

## Design Details

- Reconcile entry point: gardenlet's shoot reconcile flow, after the shoot's system components
  are deployed (the object is independent of the shoot's control plane).
- Version detection: `shoot.Spec.Kubernetes.Version` of the (effective) shoot spec.
- Data flow: `Seed.spec.settings.trustBundle.secretRef` → Secret in the seed's `garden`
  namespace (key `ca-certificates.crt`) → shoot `ClusterTrustBundle.spec.trustBundle`.
- Naming: a constant well-known name is preferred over per-shoot names so that workload
  manifests can be written portably across shoots. `gardener.cloud/trust-bundle` follows the
  name-domain pattern used by other gardener-managed resources.

## Drawbacks

- Adds one more object gardeners reconcile per shoot (trivial cost).
- Users on k8s < 1.37 do not benefit; a landscape wide end-to-end story still requires
  upgrading shoots.
- A well-known name must be agreed on and documented; later renames are breaking changes.

## Alternatives

- *Mutating webhook injecting the bundle into every pod*: automatic but fragile and
  non-transparent; wide blast radius if the webhook fails closed.
- *Per-namespace ConfigMap replication*: requires a replication controller and still leaves
  every workload author to wire the volume manually — strictly worse than a projected
  `clusterTrustBundle` source.
- *Container runtime / OS-level injection on nodes*: covered by a sibling GEP (node system
  trust store); orthogonal — workloads that do not run as root inside their container may not
  share the node's store semantics, and container images ship their own bundles.

## Open Questions

- Final object name (`gardener.cloud/trust-bundle` vs. other reserved names).
- Whether a future extension should also support signer-linked bundles
  (`spec.signerName: gardener.cloud/trust-bundle…` or similar).
- Interaction with the node-level (system trust store) distribution — naming should make it
  obvious that both flows consume the same `Seed` configuration.
