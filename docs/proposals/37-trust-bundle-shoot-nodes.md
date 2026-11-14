---
title: Distribute a Custom CA Trust Bundle to Shoot Nodes
gep-number: 37
creation-date: 2026-11-04
status: implementable
authors:
- "@ccwienk"
reviewers:
- "@gardener/gardener-maintainers"
---

# GEP-37: Distribute a Custom CA Trust Bundle to Shoot Nodes

## Summary

Today, Gardener can deliver per-registry containerd CAs to shoot nodes via
`OperatingSystemConfig.spec.criConfig.containerd.registries[].hosts[].caCerts`
(see [#13751](https://github.com/gardener/gardener/issues/13751)). This covers
image pulls only. Other processes on shoot nodes — kubelet, node-local-dns, OS
daemon tools, telemetry agents, and any third-party software that honours the
underlying OS trust store — continue to trust only the CAs shipped with the
worker OS image.

This GEP proposes a mechanism that allows cluster operators to distribute a
custom CA trust bundle to the *system* trust store (`/etc/ssl/certs/`) of every
shoot node, complementing (but not changing) the existing per-registry CA
delivery.

## Motivation

Operators who run Gardener in environments that must reach services secured by
private or otherwise custom-root CAs (internal PKI, private package mirrors,
self-hosted telemetry endpoints, OIDC issuers operated by the platform) need
those CAs to be trusted on the node *before* any pod starts. Per-image trust
bakes this into the image build pipeline; per-pod trust configuration forces
every workload to recur to workarounds. A landscape-level trust bundle
installed into the node's system trust store addresses the problem at the
right layer: early, once, and for all processes on the node.

This GEP is part of a chain that makes custom CA trust bundles configurable
across a Gardener landscape:

- Gardener operator — virtual-garden control plane (separate PR, see
  [gardener/gardener#16210](https://github.com/gardener/gardener/pull/16210)):
  `Garden.spec.virtualCluster.gardener.trustBundleSecretRef`.
- gardenlet — seed-system components (separate PR):
  `Seed.spec.settings.trustBundle`.
- gardenlet + OperatingSystemConfig — shoot-node system trust store (this
  document).
- gardenlet — per-shoot `ClusterTrustBundle` (k8s &ge; 1.37) for user
  workloads (separate PR).

### Goals

- Allow operators to opt into a custom CA trust bundle being installed into
  the system trust store of every shoot node, with no changes to
  `OperatingSystemConfig` consumers (OS extensions).
- Reuse the existing node-init mechanism (bootstrap token +
  `update-ca-certificates` script installed by
  `rootcertificates.UpdateLocalCACertificatesScriptFile`) that already powers
  per-registry containerd CA delivery.
- Trigger a node roll on bundle changes, consistent with how registry CAs are
  handled today.

### Non-Goals

- Changing the behaviour of the existing per-registry containerd CAs (those
  remain orthogonal).
- Distributing the bundle to *user pods* — that is addressed separately via
  per-shoot `ClusterTrustBundle`.
- OS-specific configuration beyond what OS extensions already provide. The
  mechanism reuses the same `update-ca-certificates` integration point that
  GardenLinux-based workers already implement.
- Deciding how the operator-distribution layer (whatever writes the bundle to
  each seed) is implemented. This document only covers the flow from the seed
  into the shoot and onto the node.

## Proposal

When `Seed.spec.settings.trustBundle.secretRef` is set (see companion PR),
gardenlet renders the operating-system-config for each affected worker pool
with the following additions:

1.  **Well-known ConfigMap in the shoot** — gardenlet publishes the bundle into
    a ConfigMap named e.g. `shoot-root-ca-bundle` in the shoot cluster's
    `kube-public` namespace, with the concatenated PEM at key
    `ca-certificates.crt`. The ConfigMap is readable to the bootstrap token
    (`system:bootstrappers` group), matching the pattern of the registry CA
    delivery.
2.  **Extended node-init script** — the init script generated in
    `pkg/component/extensions/operatingsystemconfig/nodeinit/` is extended so
    that, when the bundle is configured, it:
      - downloads the ConfigMap entry using the bootstrap token,
      - writes the bundle to `/etc/ssl/certs/ca-certificates.crt`
        (alternatively: an additional file under
        `/usr/local/share/ca-certificates/` for Debian-style setups),
      - invokes the same `update-ca-certificates` helper already shipped when
        `registryCAEnabled` is set,
      - restarts `containerd` (via the existing unit-level mechanism) so the
        running daemon picks up the new trust.
3.  **Roll on change** — bundle content is hashed into the
    `OperatingSystemConfig` name / `worker.gardener.cloud/pool-version`
    annotation, so any change triggers a rolling update of the affected worker
    pool, identical to how registry CAs are handled today.

The flow is opt-in: when the field is unset, no ConfigMap is published and
the init script is unchanged.

## API Changes

No new API field is introduced by this GEP. The trigger is
`Seed.spec.settings.trustBundle.secretRef` from the companion PR. The
behaviour described here activates automatically when that field is present.

## Compatibility and Risks

- Workers with OS images that do not provide `update-ca-certificates` (or an
  equivalent mechanism) fail at node-init time with a clear error. OS
  extensions signalling support via their `OperatingSystemConfig` type
  identifier are responsible for honouring the mechanism.
- The bundle replaces the *content* of
  `/etc/ssl/certs/ca-certificates.crt`, not the entire directory. Workers
  whose OS image ships a stricter store layout are unaffected beyond the
  bundle content itself.
- The bootstrap token remains valid for the duration of the node-init phase
  only; the ConfigMap is not readable after the node has transitioned to its
  final kubeconfig.

## Alternatives Considered

1.  Extend `OperatingSystemconfig.spec.files` so gardenlet can place
    arbitrary file content on the node. This exposes a lower-level,
    harder-to-validate interface; operators would have to reconstruct the
    OS-specific update steps per worker OS.
2.  Deliver via `Secret` + per-pod volume mounts. Solves the pod side but
    not the node side; this GEP explicitly addresses the node system trust
    store.
3.  Rebuild worker OS images with the CAs baked in. Couples trust policy to
    the image build pipeline; slow to revoke and multiplies artifact count
    across landscapes.

## Open Questions

1.  Should the bundle entirely *replace* the OS's default bundle, or
    *append* to it? This proposal currently appends via
    `update-ca-certificates`'s usual semantics; a strict "fail-fast" mode
    that drops the distro bundle is out of scope but could be added as an
    additional boolean on the seed setting.
2.  Should the well-known ConfigMap live in `kube-public` or a
    gardener-owned namespace? `kube-public` matches the existing registry-CA
    pattern; a dedicated namespace is cleaner from a least-privilege
    perspective but adds a deployment step.
3.  Naming of the ConfigMap and of the well-known file
    (`ca-certificates.crt`) — alignment with the upcoming work on
    per-registry containerd CAs is desirable so tooling can discover both at
    predictable locations.

## Implementation Plan

1.  Land `Seed.spec.settings.trustBundle.secretRef` (companion PR).
2.  Extend `nodeinit.Config` and `generateInitScript` in
    `pkg/component/extensions/operatingsystemconfig/nodeinit/` with a
    `customCABundleEnabled bool` toggle and the additional download step.
3.  Extend the gardenlet seed reconciliation to publish the ConfigMap into
    each shoot whose seed has the field set, and to remove it when the field
    is cleared.
4.  Hash the bundle content into the worker pool's roll-trigger annotation.
5.  Update
    [`rootcertificates.UpdateLocalCACertificatesScriptFile`](../pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates)
    documentation and the operations guide.
