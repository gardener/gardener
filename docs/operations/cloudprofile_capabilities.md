# CloudProfile Capabilities

This document describes the capability mechanism in `CloudProfile.spec.machineCapabilities`. Capabilities let CloudProfile operators express the compatibility matrix between **machine types** and **machine image versions** so that Gardener can:

- reject incompatible worker pool combinations at admission time,
- pick a compatible image when the user does not specify one,
- restrict automated maintenance upgrades to compatible images, and
- give the Gardener Dashboard the data it needs to filter incompatible images out of selection lists.

This is the operator-facing description of [GEP-0033](https://github.com/gardener/enhancements/tree/main/geps/0033-machine-image-capabilities).

## Table of Contents

- [Why capabilities](#why-capabilities)
- [Activation](#activation)
- [Anatomy of a capability](#anatomy-of-a-capability)
- [Where capabilities appear in a CloudProfile](#where-capabilities-appear-in-a-cloudprofile)
- [Defaulting: omission means "all registered values"](#defaulting-omission-means-all-registered-values)
- [Order matters](#order-matters)
- [The `architecture` capability](#the-architecture-capability)
- [Provider extension contract](#provider-extension-contract)
- [Matching and selection algorithm](#matching-and-selection-algorithm)
- [NamespacedCloudProfile rules](#namespacedcloudprofile-rules)
- [Designing a good capability](#designing-a-good-capability)
- [Migration from legacy fields](#migration-from-legacy-fields)
- [End-to-end example](#end-to-end-example)
- [Troubleshooting](#troubleshooting)

## Why capabilities

On every cloud provider, not every machine type is compatible with every machine image. The exact axes of incompatibility differ between providers, but the shape of the problem is universal:

- Machine types fall into hardware classes — for example **virtualised** versus **bare-metal** instances — and an image is built for one class or the other. A virtualised image will not boot on a bare-metal machine, and vice versa.
- Boot firmware modes, storage controllers (SCSI vs. NVMe), accelerated networking, and similar low-level traits all create the same kind of compatibility split.

 Without capabilities, the mismatch is only caught when the Kubernetes nodes are created, after the shoot has already been admitted.

Before capabilities, operators worked around this by smuggling the information into pre-release version tags (e.g. an image version `1.0.0-metal` to mark a bare-metal artifact) or by leaving image versions unclassified. Both workarounds break automatic maintenance and confuse end-users.

Capabilities replace those workarounds with a structured compatibility contract that lives in the CloudProfile and is understood by Gardener core, the Gardener Dashboard, and provider extensions.

## Activation

Capabilities are gated by the `CloudProfileCapabilities` feature gate on `gardener-apiserver`:

| State | Versions |
|---|---|
| Alpha (off by default) | `1.117` – `1.145` |
| Beta (enabled by default) | `1.146` and later |

In Gardener `1.146` and later, the feature is enabled by default. If disabled, `spec.machineCapabilities` is rejected on CloudProfile admission.

**Per-CloudProfile opt-in:** The mechanism does not require migrating all CloudProfiles at once. A CloudProfile without `spec.machineCapabilities` continues using the legacy format (dedicated `architecture` fields) and is validated against legacy rules. There is no automatic conversion.

## Anatomy of a capability

A capability is a pair of `(name, values)`. It is registered once in `spec.machineCapabilities` and may then be referenced by every machine type and machine image flavor in that CloudProfile.

```yaml
spec:
  machineCapabilities:
  - name: architecture
    values: [amd64, arm64]
  - name: machineHostType
    values: [virtual, metal]
  - name: storageAccess
    values: [NVMe, SCSI]
```

Rules enforced by CloudProfile admission:

- Names must be unique within `spec.machineCapabilities`.
- Each capability must declare at least one value, and values must be unique within that capability.
- Only capability names registered in `spec.machineCapabilities` may be used on `machineTypes[*].capabilities` or `machineImages[*].versions[*].capabilityFlavors[*]`. 
- Using an unregistered name/value is rejected with `Unsupported value`.

The capability vocabulary in `spec.machineCapabilities` is the **single source of truth** for that CloudProfile.

## Where capabilities appear in a CloudProfile

```yaml
spec:
  # 1. Vocabulary: defines all allowed capability names and values.
  machineCapabilities: [...]

  machineTypes:
  - name: general-medium
    capabilities:                 # 2. Per machine type: what this hardware supports.
      architecture: [amd64]
      machineHostType: [virtual, metal]

  machineImages:
  - name: local
    versions:
    - version: 1.0.0
      capabilityFlavors:          # 3. Per image version: one entry per provider image reference.
      - architecture: [amd64]
        machineHostType: [virtual]
      - architecture: [amd64]
        machineHostType: [metal]
      - architecture: [arm64]
        machineHostType: [virtual]
```

Each entry in `capabilityFlavors` represents **one image artifact** in the provider's catalog (one image ID, one container image, one cloud-specific image reference, etc.). A single image *version* can therefore carry multiple flavors when the OS ships separate artifacts for different architectures, hardware classes, or storage controllers.

## Defaulting: omission means "all registered values"

This is the most important rule to internalise. When a machine type or an image flavor **does not declare** a capability that is registered in `spec.machineCapabilities`, the omitted capability is treated as supporting **every registered value**, not "no values".

```yaml
spec:
  machineCapabilities:
  - name: architecture
    values: [amd64]
  - name: machineHostType
    values: [virtual, metal]

  machineTypes:
  - name: general-medium
    capabilities:
      architecture: [amd64]
      # machineHostType omitted → treated as [virtual, metal]
```

The omission is convenient: a machine type that genuinely supports both `virtual` and `metal` does not have to repeat the registered values. **⚠️ But the inverse is a trap**: omitting a capability does not mean "this hardware is opaque" — it means "this hardware supports everything the CloudProfile defines". If you want to express that a machine type is `metal`-only, you must list it explicitly:

```yaml
machineTypes:
- name: bare-metal-medium
  capabilities:
    architecture: [amd64]
    machineHostType: [metal]      # explicit narrowing
```

The same rule applies to image flavors. An image flavor that omits a registered capability is treated as supporting all registered values for that capability.

## Order matters

The order of entries in `spec.machineCapabilities` and the order of values within each entry are both significant. They encode **preference**, and Gardener uses them as the deterministic tie-breaker when multiple image flavors are compatible with a given machine type and worker pool:

1. **Capability order** (most to least significant): The first capability in `spec.machineCapabilities` is the primary tie-breaker. For example, if `architecture` is listed first, Gardener prioritizes candidates on `architecture` before considering any other capability.
2. **Value order** (most to least preferred): Within each capability, the first value is the most-preferred. For example, `machineHostType: [virtual, metal]` means "prefer virtual; use metal only if necessary."

Practical guidance:

- **List values from best to legacy.** Put modern/performant options first. Example: `machineHostType: [virtual, metal]` steers maintenance upgrades toward `virtual` whenever both are compatible.
- **Reordering is a behavioral change.** It does not affect validation, but it changes which image the maintenance controller picks when multiple are valid. Test carefully.

## The `architecture` capability

When `spec.machineCapabilities` is non-empty, registering `architecture` is **mandatory**. Allowed values are constrained to the set Gardener supports (currently `amd64`, `arm64`).

The legacy `MachineType.architecture` field and the legacy `MachineImageVersion.architectures` list are soft-deprecated in favour of the `architecture` capability, but during the transition both must remain consistent:

- If both the legacy field and the capability are set, their values **must agree**. Mismatches are rejected at admission.
- Operators do not have to maintain the legacy fields by hand. When only the capability is set, Gardener populates the legacy field from the capability on every CloudProfile update, so consumers that still read the legacy field keep working.

**Why architecture is mandatory:** Architecture is mandatory because every image artifact has exactly one architecture. If the capability were optional, the defaulting rule ("omission means all values") would falsely claim that any image supports all architectures (amd64, arm64, etc.), which is physically impossible.

Once `architecture` is registered with more than one value:

- Every image version **must** define `capabilityFlavors`, each with exactly one architecture.
- Every machine type **must** declare the `architecture` capability with exactly one architecture.

## Provider extension contract

Capability declaration is split across two locations in a CloudProfile:

- **Gardener core** (`spec.machineImages.versions.capabilityFlavors`): Describes the *compatibility groups*. Each entry lists a set of capability values that represent one category of image artifact (e.g., "amd64 + virtual + NVMe").
- **Provider extension** (`spec.providerConfig.machineImages.versions.capabilityFlavors`): Lists the actual provider-specific image references (cloud image IDs, URIs, container digests, etc.) that correspond to each compatibility group.

Each provider extension is responsible for validating that its half is consistent with Gardener core — one entry in the provider section per entry in the core section. In practice, most provider extensions populate the core section from their own data, so the contract is typically maintained by construction.

## Matching and selection algorithm

For each capability registered in `spec.machineCapabilities`, Gardener takes the value sets from the candidate image flavor and the machine type, applies the defaulting rule above to fill in omitted capabilities, and intersects them.

A combination is **compatible** if and only if every capability has a non-empty intersection. Otherwise the combination is rejected.

<details>
<summary>When <strong>more than one image flavor is compatible</strong>, the selection is deterministic.</summary>

1. Compare the candidates capability by capability, in the order of `spec.machineCapabilities`.
2. For each capability, compare the most-preferred supported value of each candidate (using the value order in `spec.machineCapabilities`).
3. The first capability where the candidates differ decides the winner.

</details>

This algorithm is invoked at four call sites:

- **Shoot validator admission** — rejects user-selected machine type / image combinations that are incompatible.
- **Shoot mutator admission** — selects a default image when the user does not specify one.
- **Maintenance controller** — picks the upgrade target during automatic image maintenance.
- **Provider extension worker controller** — selects the actual provider image reference for a given flavor.

## NamespacedCloudProfile rules

`NamespacedCloudProfile` resources participate in the capability mechanism with three constraints:

- **Never declare `spec.machineCapabilities`.** The vocabulary is inherited from the parent `CloudProfile`.
- **For versions that override a parent version:** Capabilities are inherited; do not re-declare them. Just update the version number, classification, or providerConfig as needed.
- **For custom versions (unique to the namespaced profile):** Declare capabilities as you would in a `CloudProfile`, including corresponding `providerConfig` entries.

### Example

If the parent `CloudProfile` has image version `1.26.0` with certain capabilities, and the namespaced profile adds version `1.26.1` (new) and overrides version `1.26.0` (update):

```yaml
# Parent CloudProfile
machineImages:
- name: ubuntu
  versions:
  - version: 1.26.0
    capabilityFlavors: [...]  # capabilities defined here

# NamespacedCloudProfile
machineImages:
- name: ubuntu
  versions:
  - version: 1.26.0
    capabilityFlavors: []     # ❌ WRONG: re-declaring inherited capabilities
  - version: 1.26.0           # ✅ CORRECT: inherit from parent, just update patches
    expirationDate: "2027-12-31T00:00:00Z"
  - version: 1.26.1
    capabilityFlavors: [...]  # ✅ CORRECT: new version, declare capabilities
```

If a parent `CloudProfile` later adds a version that the namespaced profile already defines, the namespaced profile's definition wins in the rendered status.

## Designing a good capability

The capability mechanism is intentionally small but rigid. A handful of design rules will keep CloudProfiles maintainable:

1. **Frame values positively.** A capability's values must enumerate everything supported, so that intersection of value sets is meaningful. `storageAccess: [NVMe, SCSI]` is correct; `supportsNVMe: [true, false]` does not compose with intersection and breaks future extension.
2. **Order from best to legacy.** This ensures that maintenance auto-upgrades steer fleets toward the best possible image when multiple flavors are valid. Example: `machineHostType: [virtual, metal]` prefers virtual.
3. **Design for stability within the CloudProfile.** A capability should capture a meaningful axis of compatibility that won't change frequently. It's fine to add new values to an existing capability or rename it within a CloudProfile — just update all declarations consistently. The risk is Shoots or NamespacedCloudProfiles that depend on the capability; removing or drastically redefining capabilities they reference requires careful coordination.
4. **Watch the size budget.** A `CloudProfile` is an etcd object with the standard 1.5 MiB limit. Capability flavors multiply per image version; a profile with many images and many flavors can grow quickly.

## Migration from legacy fields

The capability mechanism is purely additive. A staged migration looks like this:

1. **Gardener core** is upgraded to a version with the `CloudProfileCapabilities` feature gate Beta or GA. Nothing changes for existing CloudProfiles.
2. **Provider extensions** are upgraded to a version that understands `capabilityFlavors` in their `providerConfig` section. CloudProfiles still work without capabilities.
3. **Per CloudProfile**, the operator opts in by adding `spec.machineCapabilities` and migrating each machine type and each image version. The legacy `architecture` fields do not need to be removed: Gardener keeps them in sync with the capability values automatically (see [The `architecture` capability](#the-architecture-capability)).

There is no requirement to migrate all CloudProfiles at the same time. Profiles that have not opted in continue to be validated against the legacy rules.

## End-to-end example

The following profile demonstrates a multi-architecture, multi-host-class setup. The example uses the `local` provider with provider-specific image references in `providerConfig`.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: example
spec:
  type: local
  machineCapabilities:
  - name: architecture                    # mandatory; values constrained to amd64/arm64
    values: [amd64, arm64]
  - name: machineHostType                 # most-preferred first
    values: [virtual, metal]
  - name: storageAccess
    values: [NVMe, SCSI]

  machineTypes:
  - name: general-medium                  # virtualised, supports both storage controllers
    cpu: "4"
    gpu: "0"
    memory: 16Gi
    capabilities:
      architecture: [amd64]
      machineHostType: [virtual]
      # storageAccess omitted → defaults to [NVMe, SCSI]
  - name: metal-medium                    # bare-metal, legacy storage only
    cpu: "4"
    gpu: "0"
    memory: 16Gi
    capabilities:
      architecture: [amd64]
      machineHostType: [metal]            # explicit narrowing
      storageAccess: [SCSI]
  - name: arm-medium                      # arm64, virtualised
    cpu: "4"
    gpu: "0"
    memory: 16Gi
    capabilities:
      architecture: [arm64]
      machineHostType: [virtual]

  machineImages:
  - name: local
    updateStrategy: minor
    versions:
    - version: 1.0.0
      classification: supported
      capabilityFlavors:
      - architecture: [amd64]
        machineHostType: [virtual]
        storageAccess: [NVMe, SCSI]
      - architecture: [amd64]
        machineHostType: [metal]
        storageAccess: [SCSI]
      - architecture: [arm64]
        machineHostType: [virtual]
        storageAccess: [NVMe]

  providerConfig:
    apiVersion: local.provider.extensions.gardener.cloud/v1alpha1
    kind: CloudProfileConfig
    machineImages:
    - name: local
      versions:
      - version: 1.0.0
        capabilityFlavors:                 # one entry per concrete image artifact
        - image: registry.example/node-amd64-virtual:v1.0.0
          capabilities:
            architecture: [amd64]
            machineHostType: [virtual]
        - image: registry.example/node-amd64-metal:v1.0.0
          capabilities:
            architecture: [amd64]
            machineHostType: [metal]
            storageAccess: [SCSI]
        - image: registry.example/node-arm64-virtual:v1.0.0
          capabilities:
            architecture: [arm64]
            machineHostType: [virtual]
            storageAccess: [NVMe]

  kubernetes:
    versions:
    - version: 1.36.0
  regions:
  - name: local
```

How this is interpreted:

- A worker pool using `general-medium` is admitted against the first flavor only (`virtual`/`amd64`). The maintenance controller will pick its `NVMe` provider image because `NVMe` is the first-listed (most-preferred) value of `storageAccess` and the flavor offers both.
- A worker pool using `metal-medium` only matches the second flavor (`metal`/`amd64`/`SCSI`). The other two flavors are rejected because their `machineHostType` intersection with the machine type is empty.
- A worker pool using `arm-medium` only matches the third flavor (`arm64`).

## Troubleshooting

Common admission errors and what they mean.

| Error | Cause |
|---|---|
| `machineCapabilities are not allowed with disabled CloudProfileCapabilities feature gate` | The feature gate is off but the CloudProfile registers `machineCapabilities`. Enable the gate or remove the field. |
| `architecture capability is required` | `spec.machineCapabilities` is non-empty but `architecture` is missing. It is mandatory. |
| `must provide at least one image flavor when multiple architectures are defined in spec.machineCapabilities` | An image version has no `capabilityFlavors` while the global vocabulary declares more than one architecture. Add an explicit flavor per architecture. |
| `must specify one architecture explicitly as multiple architectures are defined in spec.machineCapabilities` | A `capabilityFlavor` omits `architecture` while the global vocabulary has multiple architectures. Pick one. |
| `must not define more than one architecture within an image flavor` | A flavor lists more than one architecture value. Split it into separate flavors. |
| `architecture field values set (...) conflict with the capability architectures (...)` | The deprecated `architectures` list and the architectures expressed via `capabilityFlavors` disagree. Make them consistent or drop the legacy field. |
| `machine type architecture (...) conflicts with the capability architecture (...)` | The deprecated `MachineType.architecture` field and `MachineType.capabilities.architecture` disagree. Make them consistent. |
| `must not provide capabilities without global definition` | A machine type or image version uses `capabilities` / `capabilityFlavors` while `spec.machineCapabilities` is empty. Either register the global vocabulary, or remove the per-resource fields. |
| `Unsupported value: <name>` on a capability key | A machine type or flavor references a capability name not registered in `spec.machineCapabilities`. |
| `Unsupported value: <value>` on a capability value | A machine type or flavor uses a value not registered for that capability. |
