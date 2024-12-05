---
title: Cloud Profile Version Classification Lifecycles
gep-number: 32
creation-date: 2024-12-03
status: implementable
authors:
- "@crigertg"
- "@Gerrit91"
- "@LucaBernstein"
- "@vknabel"
reviewers:
- "@maboehm"
- "@rfranzke"
- "@timebertt"
---

# GEP-32: Cloud Profile Version Classification Lifecycles

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
    - [Backwards Compatibility](#backwards-compatibility)
- [Considered Alternatives](#considered-alternatives)
    - [Consequent Continuation of Current Approach](#consequent-continuation-of-current-approach)
    - [Introducing Lifecycle Map](#introducing-lifecycle-map)
    - [Classification Field Patching](#classification-field-patching)
    - [Implementation Without the Status Field](#implementation-without-the-status-field)

## Motivation

At the current stage of implementation, Gardener administrators may classify Kubernetes versions and machine image versions using the `CloudProfile` spec.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  kubernetes:
    versions:
      - version: 1.26.0
        classification: supported
        expirationDate: "2024-06-01T00:00:00Z"
      - version: 1.27.0
        classification: deprecated
      - version: 1.28.0
        classification: supported
```

Typically, administrators move a version through the classification stages manually over time. However, there is a dedicated field called `expirationDate`, that allows setting a deadline after which a version is interpreted as expired without manual intervention.

However, manually moving versions through the stages is cumbersome, so there should be a way for adminstrators to define an entire version lifecycle.

While using the expiration date is certainly convenient for administrators, it is confusing that the classification pretends to be for example `supported` or `deprecated` while the expiration date marks it as `expired` at a certain point in time.

In addition to that, there is no way to schedule an introduction of a new version at a specific point in the future.

### Goals

 - Allow administrators to define a classification lifecycle for expirable versions in the `CloudProfile`.
 - Directly reflect the actual state of a classification, which is not the case with the fields `classification` and `expirationDate`.
 - Maintain basic backwards compatibility.
 - Keep the possibility to only specify the `version` field without any classification lifecycle.
 - Do not break deployments of the `CloudProfile` through CD pipelines by accidental field overwrites.

### Non-Goals

 - Allowing third-parties to introduce own classification stages.
 - Let users arbitrarily move versions through stages like going from `expired` to `supported`.

## Proposal

The idea is to deprecate both existing fields `classification` and `expirationDate` in the `CloudProfile` and replace them with a more powerful field called `lifecycle`. This field contains a slice of classification stages that start at a given date.

With this change we also introduce a resource status for the `CloudProfile` to improve the issue that the actual classification stage is not immediately obvious. The status makes it more readable for API consumers and they do not need to calculate the actual classification stage on their own.

```yaml
# assume that the current date is 2024-12-03
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  kubernetes:
    versions:
      - version: 1.30.6
        lifecycle:
          - classification: preview # starts in preview because no start time is defined
          - classification: supported
            startTime: "2024-12-01T00:00:00Z"
          - classification: deprecated
            startTime: "2025-03-01T00:00:00Z"
          - classification: expired
            startTime: "2025-04-01T00:00:00Z"
status:
  kubernetes:
    versions:
      - version: 1.30.6
        classificationState: supported
```

In addition to the existing classification stages, we add one more stage with the name `unavailable` to the API. An `unavailable` version is planned to become available in the future. It is not possible to reference this version in this stage and can be used by administrators to schedule a new version release.

The `expired` classification, which existed only implicitly in the API, now becomes a dedicated value.

So, the resulting list of classification stages will be:

- `unavailable`
- `preview`
- `supported`
- `deprecated`
- `expired`

There are rules for the new lifecycle, some of them need to be ensured through validations:

- Classification stages in a lifecycle must only appear ordered from `unavailable` to `expired` as described in the list above.
- It is not required that every classification stage is present in the lifecycle.
- Start times are always monotonically increasing.
- The first start date is optional and interpreted as zero time, meaning it has already started.
- If no lifecycle is given, it defaults to a lifecycle definition with one `supported` stage.
- If all start times are in the future, the resulting classification is `unavailable`.

There is already a controller in place for reconciling the `CloudProfile` (by now it's primarily handling finalizers only), which is going to be extended by reconciling the version classification statuses. If there are remaining stages inside `lifecycle` the next reconcile needs to be scheduled at its `startTime`.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  kubernetes:
    versions:
      # if an administrator deploys just the version without any lifecycle,
      # the reconciler will evaluate the classification status to supported
      - version: 1.27.0

      # when introducing a new version it may not contain any deprecation or expiration date yet
      - version: 1.28.0
        lifecycle:
          - classification: preview
          - classification: supported
            startTime: "2024-12-01T00:00:00Z"

      # it is not strictly required that every lifecycle stage must occur,
      # they can also be dropped as long as their general order is maintained
      - version: 1.18.0
        lifecycle:
          - classification: supported
          - classification: expired
            startTime: "2022-06-01T00:00:00Z"

      # to schedule a new version release, the administrator can define the start times
      # of all lifecycle events in the future, such that the classification status will
      # be evaluated to unavailable
      - version: 2.0.0
        lifecycle:
          - classification: preview
            startTime: "2036-02-07T06:28:16Z"
status:
  kubernetes:
    versions:
      - version: 1.27.0
        classificationState: supported
      - version: 1.28.0
        classificationState: supported
      - version: 1.18.0
        classificationState: expired
      - version: 2.0.0
        classificationState: unavailable
```

### Backwards Compatibility

The existing fields continue to function as before but are deprecated. `lifecycle` cannot be combined with the usage of the existing `classification` and `expirationDate` fields though.

The `status` always reflects the current state of a classification no matter if the new or deprecated API is used. Specifically when using the old API this means that if the `expirationDate` has passed, the resulting status is evaluated as `expired`, overwriting the actual `classification` value.

## Considered Alternatives

In addition to the proposed approach, we considered several alternatives or variations of approaches. The main candidates are described below.

### Consequent Continuation of Current Approach

The first idea was to just extend the current API by adding further fields for the classification stages:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  kubernetes:
    versions:
      - version: 1.30.6
        classification: unavailable
        previewDate: "2025-01-01T00:00:00Z"
        supportedDate: "2025-01-14T00:00:00Z"
        deprecationDate: "2025-03-01T00:00:00Z"
        expirationDate: "2025-06-01T00:00:00Z"
```

While this approach has the advantage that it just integrates with the current implementation (existing behavior is maintained), it was rejected because:

- Classification stages are defined as keys in the API, which feels wrong because these are enums and when adding a new stage the API definition is required to change. So this is considered an anti-pattern.
- All consumers of the `CloudProfile` are required to calculate the effective classification state that depends on time.
- The `*Date` suffix for the new fields still imply that a date would be sufficient without a time, which is not the case.

### Introduction of a Lifecycle Map

The next approach keeps the `classification` field itself, but moves the date fields into a new object to not pollute the `ExpirableVersion` struct.
This also offers the oppurtunity to better express the fact that date times are required to schedule the lifecycle of a version classification instead of just plain dates.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  kubernetes:
    versions:
      - version: 1.30.6
        classification: supported
        lifecycle:
          preview:
            startTime: "2025-01-01T00:00:00Z"
          supported:
            startTime: "2025-01-14T00:00:00Z"
          deprecation:
            startTime: "2025-03-01T00:00:00Z"
          expiration:
            startTime: "2025-06-01T00:00:00Z"
```

In this case only the `expirationDate` needs to be deprecated. We discarded this approach mostly for the same reasons as the previous one:

- Classification stages are defined as keys in the API, which feels wrong because these are enums and when adding a new stage the API definition is required to change. So this is considered an anti-pattern.
- All consumers of the `CloudProfile` are required to calculate the effective classification state that depends on time.

### Status vs. Classification Field Patching

This consideration tries to avoid the introduction of a `status` field in the `CloudProfile` and instead updates the `spec` itself.

Here the `CloudProfile` reconciler patches the currently computed classification stage of a version back into `classification` or an eventually newly introduced sibling field like `currentClassification`.

```yaml
# assume that the current date is 2024-12-03
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: local
spec:
  kubernetes:
    versions:
      - version: 1.30.6
        classification: supported # the classification is patched by the reconciler and not set by the administrator
        lifecycle:
          - classification: preview
          - classification: supported
            startTime: "2024-12-01T00:00:00Z"
          - classification: deprecated
            startTime: "2025-03-01T00:00:00Z"
          - classification: expired
            startTime: "2025-04-01T00:00:00Z"
```

While this variant offers a user to directly see the computed classification stage in a field of the specification, we opted against it due to the following reasons:

- As it patches the spec, the administrator can no longer be seen as the sole owner of this resource. This breaks the goal to stay compatible with typical deployment strategies (deployment and reconciler may toggle the field value consistently).
- The gardener-apiserver validation needs to prevent setting the `classification` to a value that contradicts the stages inside `lifecycle`. When the gardener-controller-manager patches the field, potential time drifts of servers must be considered for the implementation, which is complex.

### Implementation Without the Status Field

This variant is more or less a placeholder for dropping the goal of reflecting the currently computed classification stage. Clients that consume the classification stage need to compute the current classification stage on their own.

We do not want to give up on this goal for the following reasons:

- If there is still a `classification` field, this is confusing for the human reader because four additional date time fields need to be considered.
- Every consumer of the `CloudProfile` needs to duplicate the computation of the actual classification stage. With one additional field this was fine enough, but with a complex lifecycle it certainly isn't.
