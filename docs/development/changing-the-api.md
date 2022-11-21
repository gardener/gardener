---
title: Changing the APIs
---

# Extending the API

This document describes the steps that need to be performed when changing the API.
It provides guidance for API changes to both (Gardener system in general or component configurations).

Generally, as Gardener is a Kubernetes-native extension, it follows the same API conventions and guidelines like Kubernetes itself.
[This document](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md) as well as [this document](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md) already provide a good overview and general explanation of the basic concepts behind it.
We are following the same approaches.

## Gardener API

The Gardener API is defined in `pkg/apis/{core,extensions,settings}` directories and is the main point of interaction with the system.
It must be ensured that the API is always backwards-compatible.

#### Changing the API

**Checklist** when changing the API:

1. Modify the field(s) in the respective Golang files of all external and the internal version.
    1. Make sure new fields are being added as "optional" fields, i.e., they are of pointer types, they have the `// +optional` comment, and they have the `omitempty` JSON tag.
    1. Make sure that the existing field numbers in the protobuf tags are not changed.
1. If necessary then implement/adapt the conversion logic defined in the versioned APIs (e.g., `pkg/apis/core/v1beta1/conversions*.go`).
1. If necessary then implement/adapt defaulting logic defined in the versioned APIs (e.g., `pkg/apis/core/v1beta1/defaults*.go`).
1. Run the code generation: `make generate`
1. If necessary then implement/adapt validation logic defined in the internal API (e.g., `pkg/apis/core/validation/validation*.go`).
1. If necessary then adapt the exemplary YAML manifests of the Gardener resources defined in `example/*.yaml`.
1. In most cases it makes sense to add/adapt the documentation for administrators/operators and/or end-users in the `docs` folder to provide information on purpose and usage of the added/changed fields.
1. When opening the pull request then always add a release note so that end-users are becoming aware of the changes.

#### Removing a field

If fields shall be removed permanently from the API then a proper deprecation period must be adhered to so that end-users have enough time adapt their clients. The followed process for removing a field is:
1. The field in the external version(s) has to be commented out with appropriate doc string that the protobuf number of the corresponding field is reserved. Example:

   ```diff
   -	SeedTemplate *gardencorev1beta1.SeedTemplate `json:"seedTemplate,omitempty" protobuf:"bytes,2,opt,name=seedTemplate"`

   +	// SeedTemplate is tombstoned to show why 2 is reserved protobuf tag.
   +	// SeedTemplate *gardencorev1beta1.SeedTemplate `json:"seedTemplate,omitempty" protobuf:"bytes,2,opt,name=seedTemplate"`
   ```

   The reasoning behind this is to prevent the same protobuf number to be used by a new field. Introducing a new field with the same protobuf number would be a breaking change for clients still using the old protobuf definitions that have the old field for the given protobuf number.
   The field in the internal version can be removed.

2. Unit test has to be added to make sure that a new field does not reuse the already reserved protobuf tag.

Example of field removal can be found in https://github.com/gardener/gardener/pull/6972.

## Component configuration APIs

Most Gardener components have a component configuration that follows similar principles to the Gardener API.
Those component configurations are defined in `pkg/{controllermanager,gardenlet,scheduler},pkg/apis/config`.
Hence, the above checklist also applies for changes to those APIs.
However, since these APIs are only used internally and only during the deployment of Gardener the guidelines with respect to changes and backwards-compatibility are slightly relaxed.
If necessary then it is allowed to remove fields without a proper deprecation period if the release note uses the `breaking operator` keywords.

In addition to the above checklist:

1. If necessary then adapt the Helm chart of Gardener defined in `charts/gardener`. Adapt the `values.yaml` file as well as the manifest templates.
