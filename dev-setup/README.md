# `dev-setup/`

This directory contains everything needed to run Gardener locally on KinD or on a remote cluster.
It replaces the former monolithic root `skaffold.yaml` and the `example/gardener-local` files with a modular, kustomize-based structure.

## Design

The local setup deploys three logical layers, each driven by a shell script and a Skaffold config:

| Layer | Script | Skaffold config | What it deploys |
|---|---|---|---|
| **Operator** | `operator.sh` | `skaffold-operator.yaml` | `gardener-operator` (Helm) + `provider-local` extension + networking extensions |
| **Garden** | `garden.sh` | — | `Garden` CR, `Extension` CRs (applied by `operator.sh` via kustomize) |
| **Seed** | `seed.sh` | `skaffold-seed.yaml` | Garden config (projects, credentials, cloud profiles) + `Gardenlet` CR |

`make gardener-up` runs all three in sequence. `make gardener-dev` does the same with Skaffold in `dev` mode (live-reload on code changes).

## Scenarios

`scenario.sh` auto-detects the cluster topology by inspecting node count, zone labels, and `providerID` scheme:

| Scenario | Nodes | Zones | Notes |
|---|---|---|---|
| `single-node` | 1 | 1 | Default `make kind-up` |
| `multi-node` | >1 | 1 | |
| `multi-zone` | >1 | 3 | |
| `remote` | any | any | Non-KinD providerID |
| `*-ipv6`, `*-dual` | — | — | Suffixed when `IPFAMILY` is set |
| `*2` | — | — | Second seed cluster (`gardener-local2` node) |

The detected scenario selects the matching Skaffold profile, which in turn selects the correct kustomize overlay.

## Kustomize structure

Each resource type (`garden/`, `gardenlet/`, `gardenconfig/`, `extensions/`) follows the same pattern:

```
<resource>/
├── base/              # Base manifests (Kustomization)
├── components/        # Reusable kustomize Components (kind: Component)
│   ├── zone0/         #   e.g., adds zone "0" via JSON6902 patch
│   ├── zone1/
│   ├── ipv6/
│   └── ...
└── overlays/          # Scenario-specific overlays (Kustomization)
    ├── single-node/   #   = base
    ├── multi-zone/    #   = base + zone1 + zone2
    ├── remote/        #   = base + remote-specific patches
    └── ...
```

**Why components?** A kustomize `Component` (`kind: Component`) is a reusable unit that can be mixed into any overlay without duplicating patches.
For example, `zone1/` adds a single zone entry to the `Garden` or `Gardenlet` spec — the `multi-zone` overlay composes `zone1` + `zone2`, while `multi-node` uses neither.

**`gardenconfig/`** is component-only (no base) because it assembles independent resources — `CloudProfile`, projects, credentials, and etcd backup config — that are toggled per scenario rather than patched onto a single base resource.

## Skaffold configs

Each `skaffold-*.yaml` contains one or more Skaffold modules (`---`-separated documents with a `metadata.name`):

| File | Modules | Build | Deploy |
|---|---|---|---|
| `skaffold-operator.yaml` | `gardener-operator`, `provider-local`, `extensions` | `ko` (Go binaries), custom (Helm charts) | Helm (operator), kubectl/kustomize (extensions) |
| `skaffold-seed.yaml` | `garden-config`, `gardenlet` | `ko`, Docker (loadbalancer, node images), custom (gardenlet chart) | kubectl/kustomize |
| `skaffold-cloud-provider-local.yaml` | `cloud-provider-local` | `ko`, Docker | Helm |
| `skaffold-gardenadm.yaml` | `gardenadm` | `ko` | kustomize |

Skaffold profiles map 1:1 to scenarios and switch the kustomize overlay path. The `operator` profiles use `activation` (auto-activated by `SCENARIO` env var); the `gardenlet` profiles are selected explicitly via `-p`.

## Other directories

| Directory | Purpose |
|---|---|
| `kind/` | KinD cluster setup: Calico CNI config (kustomize overlays for IPv4/IPv6/dual), metrics-server, node capacity patches |
| `kubeconfigs/` | Generated kubeconfig files for runtime, virtual-garden, seed, and self-hosted-shoot clusters |
| `infra/` | Docker Compose: local DNS (`bind9` for `*.local.gardener.cloud`), container registry, and registry caches |
| `gardenadm/` | Resources for `gardenadm bootstrap` scenarios (machine pods, loadbalancer services, generated manifests) |
| `remote/` | Remote cluster setup: Kyverno policies, container registry with TLS, and templated config files (`.yaml.tmpl`) |
