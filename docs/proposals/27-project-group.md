---
title: Project Groups
gep-number: 27
creation-date: 2024-02-25
status: implementable
authors:
- "@tobschli"
- "@oliver-goetz"
reviewers:
- "@rfranzke"
- "@ScheererJ"
---

# GEP-27: Project Groups to enable Private Seeds

## Table of Contents

- [GEP-27: Project Groups to enable Private Seeds](#gep-27-project-groups-to-enable-private-seeds)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
      - [ProjectGroup](#projectgroup)
      - [SeedBinding](#seedbinding)
  - [Alternatives](#alternatives)
    - [Implementation without ProjectGroup](#implementation-without-projectgroup)
    - [Resource sharing via hierarchical namespaces](#resource-sharing-via-hierarchical-namespaces)
    - [Exclusively leveraging existing Taints and Tolerations](#exclusively-leveraging-existing-taints-and-tolerations)

## Summary

## Motivation

Currently, all users of Gardener share all available seeds which each other. 
As there is an interest to conveniently have the ability to schedule shoots on seeds with the ensurement that no other user groups will have shoot control planes on this seed, a new system or functionality needs to be introduced to systematically enable such a scenario.

### Goals

The overarching goal of this GEP is to lay some foundational work to enable homogeneous sovereign cloud scenarios.

- Enable projects to use some seeds exclusively by adapting the shoot scheduling ([`SeedBinding`s](#seedbinding))
- Share those, so called, seed bindings to different Projects that logically belong together in a concept called [`ProjectGroup`s](#projectgroup)
- Introduce new role for project group administrators

### Non-Goals

- Let users exclusively bind seeds on their own
## Proposal

#### ProjectGroup

For that, a ProjectGroup API will be introduced that abstracts certain information over one or more projects. With this, it will become possible to homogeneously share resources between projects.

Such a ProjectGroup manifest looks like this:
```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ProjectGroup
metadata:
  name: group1
spec:
  namespace: group1-projectgroup
  projects:
    - Project1
    - Project2
```

The main idea behind `ProjectGroup`s is that they, like projects, have an own namespace.
Every resource that is shareable (at first only `SeedBinding`s), which is in this namespace, will be shared to all `Project`s in this ProjectGroup.

This sharing mechanism will be achieved by copying the objects from the project group namespace to the namespaces of the projects that belong to the project group.

In the future, more objects can be copied like this or in a similar way with e.g. filtering to allow the sharing of secrets within a project group.

It will not be possible to add any arbitrary project to a project group. Only certain people should be allowed to add projects to a project group.

For this, a new role will be introduced in project groups, that will allow them to add projects, in which they are admin in, to the project group.

Furthermore will it not be possible to add projects to a project group which do not exist.

The `.spec.namespace` field will behave equally as the project `.spec.namespace` field.

#### SeedBinding

To enable the functionality of having exclusive or "private" seeds, a mechanism will be introduced that makes it possible to schedule shoots to specific seeds exclusively to one or more `Project`s.

Seed bindings will be considered during the scheduling of a shoot, in a way that if a shoot belongs to a project with a seed binding, a seed that is targeted by the seed binding will be selected for scheduling.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: SeedBinding
metadata:
  name: project1-privateseed-aws-1
  namespace: project1-projectgroup
spec:
  taintSeed: false
  seedRef: project1-aws-ha1
  seedSelector:
    region: eu-west1

```

If `.spec.taintSeed` is set to `true`, the specified seed(s) will be tainted, so that no other projects outside the project group can schedule shoots on them.

Such tainting bindings will only be allowed to be created by Gardener operators, so that not any arbitrary user can exclusively bind seeds.

Seed bindings will only work in either project or project group namespaces.

As mentioned in [ProjectGroup](#projectgroup), when a seed binding is present in a project group namespace, it will be copied to the respective project namespace.

It will also be possible to allow seed bindings to be in a project namespace, in which it becomes available for a singular project.

The `.spec.seedRef` field specifies which Seed is bound to the project group / project.

Alternatively to such a specific seed, it will be possible to select one or more seeds by a label selector defined in `.spec.seedSelector`.

Either `.spec.seedRef` and `.spec.seedSelector` will only be usable exclusively.

## Alternatives

### Implementation without ProjectGroup

Alternatively to introducing a new concept that spans above the hierarchy of projects, a mechanism could be introduced that lies on the level of projects itself, like sharing resources from one project to another.

A downside of this could be that in actual use, a singular project could be used as a kind of "template" from which related projects get the relevant resources or information to schedule shoots on a private seed. As a result, this template project would have the same function as a project group, but from an architectual point of view, it would not be so clear that it is just that.

Therefore this concept is not really feasible.

### Resource sharing via hierarchical namespaces

The [Hierarchical Namespace Controller](https://github.com/kubernetes-sigs/hierarchical-namespaces?tab=readme-ov-file) would allow to share the proposed `SeedBinding`s natively. Even though the repository still received commits fairly recently, there is no general availability yet and the latest [release]((https://github.com/kubernetes-sigs/hierarchical-namespaces/releases/tag/v1.1.0)) is quite old.

In addition to that, this new dependency would require reasonable additional effort to implement into the Gardener project, so intuitively it does not seem reasonable to use this controller in this context.

### Exclusively leveraging existing Taints and Tolerations

It could be possible to completely rely on the existing tainting functionality of seeds and tolerations of shoots, but a problem arises, when two projects bind the same seed. This could be the case, when a user group wants to ensure e.g. that their control plane lands on a seed in a specific country. This would mean a taint for this project would be set. As all taints need to be tolerated by the shoot, a scenario like this would not be possible.