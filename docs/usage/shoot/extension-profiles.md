---
title: ExtensionProfiles
---

# `ExtensionProfile`s

> [!NOTE]
> This feature is introduced by [GEP-78](https://github.com/gardener/enhancements/blob/master/geps/0078-extension-profile/README.md) and is guarded by the `ExtensionComponentVersions` feature gate (alpha, disabled by default). When the gate is disabled, extensions behave exactly as before and the fields described here are inert.

`ExtensionProfile`s give extension-managed components (for example Traefik, Envoy Gateway, Falco) the same version-lifecycle experience that `CloudProfile` already provides for Kubernetes and machine-image versions. A landscape operator lists the offered component versions with a `preview → supported → deprecated → expired` lifecycle, and a cluster owner pins a version on the `Shoot` and optionally opts in to auto-upgrade.

The unit of versioning is the **component** the extension installs, not the extension controller binary (which remains an operator concern).

## The `ExtensionProfile` resource (operator)

An `ExtensionProfile` is a cluster-scoped `core.gardener.cloud` resource in the garden cluster. The operator maintains **one profile per participating extension type**. The binding to the extension type is the object's name: `metadata.name` equals the extension `type` (the value used in `Shoot.spec.extensions[].type`). There is no separate `spec.type` field. Because the name is the extension type, it must be a valid extension-type string (a DNS1123 label); `gardener-apiserver` rejects any other name.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ExtensionProfile
metadata:
  name: shoot-traefik           # equals the extension type
spec:
  # Default auto-update policy for this extension type. `default` applies to shoots that do not set an
  # explicit autoUpdate; `supported` is the allow-list of strategies the extension implements. A strategy
  # not in `supported` (including the resolved default) is rejected.
  updateStrategy:
    default: patch               # patch | minor | major
    supported: [patch, minor, major]
  components:
  - name: traefik
    # Optional per-component override of the profile-level updateStrategy.
    updateStrategy:
      default: patch
      supported: [patch, minor]
    versions:
    - version: "3.1.4"
      # Optional compatibility envelope: CEL rules (ANDed) evaluated against the shoot on admission.
      # The CEL environment binds the variable `shoot` to the Shoot being admitted.
      compatibility:
        validations:
        - message: "Requires Kubernetes 1.32 or newer"
          rule: "shoot.spec.kubernetes.version.matches('^1\\.(3[2-9]|[4-9][0-9])')"
      # Same lifecycle shape as CloudProfile versions (GEP-32).
      lifecycle:
      - classification: preview
      - classification: supported
        startTime: "2026-07-15T00:00:00Z"
      - classification: deprecated
        startTime: "2026-11-01T00:00:00Z"
      - classification: expired
        startTime: "2026-12-15T00:00:00Z"
    - version: "3.2.0"
      # Optional, opaque, extension-owned per-version configuration. It is not written to the Shoot spec;
      # gardenlet carries it into the seed Extension resource when this version is resolved.
      providerConfig:
        apiVersion: traefik.extensions.gardener.cloud/v1alpha1
        kind: TraefikComponentConfig
      lifecycle:
      - classification: preview
        startTime: "2026-08-01T00:00:00Z"
```

A profile offers one or more named `components`, each with an independent list of `versions`. Each version carries its own `lifecycle` and optional `compatibility` and `providerConfig`, and is promoted/deprecated/expired as a unit.

### Version-string format (semver facade)

Component versions **must be semantic versions**. The lifecycle classifier and the auto-upgrade logic both require a total order over the offered versions, and nothing works without one. A component that publishes non-semver versions must be wrapped in a semver-compatible facade in the profile, exactly as GardenLinux already does for OS versions. For example, a calendar tag like `2026.03` is listed as `2026.3.0`.

## Pinning a version (cluster owner)

Cluster owners select component versions on `Shoot.spec.extensions[]`, the same entry used today, extended with an extension-level `autoUpdate` default and a `components` list:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
spec:
  extensions:
  - type: shoot-traefik
    # Extension-level default auto-update policy for every component that does not set its own autoUpdate.
    autoUpdate:
      enabled: true
      updateStrategy: patch       # patch | minor | major
    components:
    - name: traefik
      version: "3.1.4"            # per-component pin (optional; a partial version like "3.1" is resolved)
      # Optional per-component override of the extension-level autoUpdate.
      autoUpdate:
        enabled: true
        updateStrategy: patch
```

- **Pinning is optional.** If a shoot lists no `components[].version`, the extension behaves exactly as before and the profile is not consulted.
- **A pin requires a matching profile.** `gardener-apiserver` rejects pinning a version for an extension type that has no `ExtensionProfile`.
- **Only selectable classifications are accepted.** When the pin changes, the selected version must be classified `preview`, `supported`, or `deprecated`; `unavailable` and `expired` are rejected. A `preview` version may be explicitly pinned but is never an auto-upgrade or defaulting target.
- **Partial versions are resolved.** A partial pin (e.g. `3.1`) is resolved to the highest matching supported version at admission and persisted.
- **Compatibility is checked old-vs-new.** The version's `compatibility.validations[]` CEL rules are evaluated against the shoot; admission rejects only a violation the update introduces.
- **In-use versions are protected.** The operator cannot remove a version from a profile (or delete the profile) while a shoot still pins that version.

### Auto-update strategies

| Strategy | Behaviour while `autoUpdate.enabled: true` |
| --- | --- |
| `patch` | Upgrade to newer supported patch releases within the pinned minor (`3.1.4 → 3.1.7`). |
| `minor` | Upgrade to newer supported minor releases within the pinned major (`3.1.4 → 3.2.0`). |
| `major` | Always track the newest supported version (crosses major boundaries). |

Setting `autoUpdate.enabled: false` freezes the component on its pinned version until that version expires, at which point a force-upgrade fires regardless (see GEP-5 semantics). The effective policy for a component is resolved in order: its own `autoUpdate`, then the extension-level `autoUpdate`, then the component's `updateStrategy.default`, then the profile's `updateStrategy.default`, falling back to `patch`.

> [!NOTE]
> Classification, auto-upgrade, force-upgrade, and the resolution of the pinned version into the seed `Extension` resource are implemented by `gardener-controller-manager` and `gardenlet` (later work packages of GEP-78). This document describes the API surface and admission behaviour served by `gardener-apiserver`.
