# Removing Support For a Kubernetes Version

This guide describes the typical steps to deprecate a Kubernetes version in the Gardener codebase. Use the referenced PRs for implementation details and examples:
- https://github.com/gardener/gardener/pull/10664
- https://github.com/gardener/gardener/pull/12486

## Notes

- The exact files and field names can vary between Gardener releases. Use the repository search to find the locations in the codebase (examples below).

## Prerequisites

- As mentioned in [Adding Support For a New Kubernetes Version](new-kubernetes-version.md) - adding support for a new Kubernetes version is a prerequisite for dropping support for versions older than 5 Kubernetes minor versions.

## Tasks

- Create an umbrella issue and include a list of all repositories that must be checked out and potentially be modified in order to deprecate the Kubernetes version.
    - example: https://github.com/gardener/gardener/issues/12409
- Research all the possible deprecations (API, functions, etc) that could be related to the deprecation of the version.
- Adapt the README.md file (remove conformance test results from the table).
- Update the supported Kubernetes versions in `docs/usage/shoot-operations/supported_k8s_versions.md`.
- Search for the version in codebase. Include all its variants - if it's 1.33, search for both "1.33" and "133". If there's a version specific logic, adapt it according to the supported versions.
- Remove upstream conformance test results from kubernetes/test-infra. ([example](https://github.com/kubernetes/test-infra/pull/35396)).
- Remove the images for the version in `imagevector/containers.yaml`.
- Adapt charts, for example:
```
    {{- if semverCompare ">= <version>" .Values.kubernetesVersion }}
    - --provide-node-service=false
    {{- end }}
```
