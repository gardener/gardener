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
        - [Permissions](#permissions)
      - [SeedBinding](#seedbinding)
        - [Permissions](#permissions-1)
  - [Alternatives](#alternatives)
    - [Implementation without ProjectGroup](#implementation-without-projectgroup)
    - [Resource sharing via hierarchical namespaces](#resource-sharing-via-hierarchical-namespaces)
    - [Exclusively leveraging existing Taints and Tolerations](#exclusively-leveraging-existing-taints-and-tolerations)

## Summary

## Motivation

Currently, all users of Gardener share all available seeds which each other.

There are users who are bound to legal obligations in regards to geographic location of control planes.

Sometimes it is also required that control planes of users are not allowed to share a seed with other third parties. A seed with those requirements will be called a "private seed".

At the moment, [such a scenario is enabled](https://gardener.cloud/docs/guides/security-and-compliance/regional-restrictions/) by Gardener operators manually tainting seeds and allowing projects of said users to set tolerations for their shoots.

A problem arises when such users want not only one, but multiple projects to be restricted to the same private seed(s). This would result in additional work for the Gardener operators to repeatedly set the same settings to the different projects of the user.

The work of Gardener operators should be decreased by this enhancement. There is still some support required by them, because it should not be allowed for any arbitrary user to "reserve" a seed by themselves. It shall still be the decision of Gardener operators to decide if a user is allowed to have such a private seed.

To summarize, there are two scenarios which need to be systematically enabled:

1. Enable the assured scheduling of shoot control planes to specific geographical regions
2. Let users have the opportunity to have a "private seed" (enabled by Gardener operators)

Both scenarios are currently already technically possible, but require strong involvement of Gardener operators.

Also users can easily ignore those restrictions, by simply setting the `.spec.seedSelector` of a shoot to match a seed without the taint / outside the required geographical location.

In order to enforce something like this, a new resource called [`SeedBinding`s](#seedbinding) will be introduced.

Furthermore, [`ProjectGroup`s](#projectgroup) should decrease the work of Gardener operators by letting users group different projects into a single project group, where seed bindings will automatically be shared among them.

### Goals

The overarching goal of this GEP is to lay the foundational work to enable homogeneous sovereign cloud scenarios.

- Enable projects to use some seeds exclusively by using the Taints and Tolerations feature ([`SeedBinding`s](#seedbinding))
- Enhance projects so that shoot control planes can be restricted to certain geographical regions ([`SeedBinding`s](#seedbinding))
- Share those, so called, seed bindings to different projects that logically belong together in a concept called [`ProjectGroup`s](#projectgroup)
- Introduce new role for project group administrators, that are allowed to add projects to a project group

### Non-Goals

- Let users exclusively bind seeds on their own

## Proposal

#### ProjectGroup

A ProjectGroup API will be introduced that abstracts certain information over one or more projects. With this, it will become possible to homogeneously share resources between projects.

Such a ProjectGroup manifest looks like this:
```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ProjectGroup
metadata:
  name: group1
spec:
  namespace: gardenprojectgroup-group1
  owner:
    apiGroup: rbac.authorization.k8s.io
    kind: User
    name: john.doe@example.com
  members:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: alice.doe@example.com
    role: admin
  projects:
    - Project1
    - Project2
```

The main idea behind `ProjectGroup`s is that they, like projects, have their own namespace.
Every resource that is shareable (at first only `SeedBinding`s) in this namespace, will be shared to all projects in this project group.

This sharing mechanism will be achieved by the `ProjectGroup` controller.
It will copy the objects from the project group namespace to the namespaces of the projects that belong to the project group.

When the object in the project group namespace is changed, the controller will immediately update the objects in the namespaces of the projects.

To let a user in a project know that such an object is being kept in sync with an object in a project group, a `projectgroup.gardener.cloud/copied-from: <projectgroup-name>` label will be added to it.

This copying feature can become useful for future use cases, for example for sharing infrastructure credentials across different projects, by copying `SecretBinding`s.

It will not be possible to add non-existent project to a project group. This also means that if a project is deleted that is part of a project group, the controller should remove it.

The `.spec.namespace` field in `ProjectGroup` will behave equally as the `Project` `.spec.namespace` field.

##### Permissions

Anybody should be able to create a project group.

But adding a project to a project group is a sensitive operation.

If anyone could add any project to their project group, a bad actor could add a project of a third party to their project group and restrict them from using any seed except the ones bound in the bad actors project group.

To circumvent a scenario like this, a new role, called `project-group-assigner` will be introduced that allows people with this role to add the project to a project group, in which they have the same role.

#### SeedBinding

To enable the general functionality of the two scenarios described in the [Motivation](#motivation) in a singular concept, `SeedBinding`s shall be introduced.

Seed bindings will be considered during the scheduling of a shoot, in a way that if a shoot belongs to a project with a seed binding, only seeds that are targeted by the seed binding will be selected for scheduling.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: SeedBinding
metadata:
  name: privateseed-provider-1
  namespace: garden-project1
taintSeed: false
seedSelector:
  matchLabels:
    foo: bar

```

If `.taintSeed` is set to `true`, the specified seed(s) will be tainted, so that no other projects outside the project group can schedule shoots on them.

Projects that have such a tainting binding will receive the `.spec.tolerations.[]whitelist` and `.spec.tolerations.[]defaults` counterpart toleration by the seed binding controller.

The name of the taint will be derived from its `.metadata.name`. With that, there will be no conflict when the seed binding gets shared to other namespaces.

What this also means is that the names of seed bindings need to be globally unique.

If a tainting binding is created for a seed that already has a taint, an error will be returned.

Seed bindings will only work in project namespaces.

The field `.seedSelector` determines which seeds shall be schedulable. A selecting mechanism, like the one in `Shoot.spec.seedSelector` will be used.

##### Permissions

Tainting bindings (seed bindings with `.taintSeed: true`) will only be allowed to be created by Gardener operators, so that not any arbitrary user can exclusively bind seeds.

## Alternatives

### Implementation without ProjectGroup

Alternatively to introducing a new concept that spans above the hierarchy of projects, a mechanism could be introduced that lies on the level of projects itself, like sharing resources from one project to another.

A downside of this could be that in actual use, a singular project could be used as a kind of "template" from which related projects get the relevant resources to schedule shoots on a private seed. As a result, this template project would have the same function as a project group, but from an architectual point of view, it would not be so clear that it is just that.

Therefore this concept is not really convenient.

To decrease this inconvenience, [`ProjectGroup`s](#projectgroup) will be proposed.

### Resource sharing via hierarchical namespaces

The [Hierarchical Namespace Controller](https://github.com/kubernetes-sigs/hierarchical-namespaces?tab=readme-ov-file) would allow to share the proposed `SeedBinding`s natively. Even though the repository still received commits fairly recently, there is no general availability yet and the latest [release]((https://github.com/kubernetes-sigs/hierarchical-namespaces/releases/tag/v1.1.0)) is quite old.

In addition to that, this new dependency would require reasonable additional effort to implement into the Gardener project, so intuitively it does not seem reasonable to use this controller in this context.

### Exclusively leveraging existing Taints and Tolerations

It could be imaginable to completely rely on the existing tainting functionality of seeds and tolerations of shoots, but a problem arises, when two projects want to use the same seed with seed bindings.

This could be the case, when a user has the first use case, described in [Motivation](#motivation).

When only relying on the tainting mechanism, this would mean a taint for this seed would be set.

When another user has the same use case with the same seed, another toleration will be set.

Now, none of the users can schedule shoots on this seed, as neither tolerate both taints.

Some special logic would need to be implemented in order to solve this problem. This would break the terminology, as it would not conform with the existing taint / toleration feature of Nodes / Pods.