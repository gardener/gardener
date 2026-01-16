# Removing Support For a Kubernetes Version

This guide describes the typical steps to remove support for a Kubernetes version in the Gardener codebase. Use the referenced PRs for implementation details and examples:
- https://github.com/gardener/gardener/pull/10664
- https://github.com/gardener/gardener/pull/12486

## Notes

- The exact files and field names can vary between Gardener releases. Use the repository search to find the locations in the codebase (examples below).

## Prerequisites

- As mentioned in [Adding Support For a New Kubernetes Version](new-kubernetes-version.md) - adding support for a new Kubernetes version is a prerequisite for dropping support for versions older than 4 Kubernetes minor versions.
- A Kubernetes version has to be supported for at least 14 months after its initial support date. Check the [Supported Kubernetes Versions](../usage/shoot-operations/supported_k8s_versions.md) page for details.

## Tasks

- Create an umbrella issue and include a list of all repositories that must be checked out and potentially be modified in order to remove support for the Kubernetes version. ([example](https://github.com/gardener/gardener/issues/12409))
- Research all the deprecations (API, functions, etc) that could be related to the removal of support of the version. ([example](https://github.com/gardener/gardener/pull/10664#:~:text=What%20this%20PR,config%20(ProtectKernelDefaults%20etc.)))
- Adapt the `README.md` file (remove conformance test results from the table). ([example](https://github.com/gardener/gardener/pull/12486/commits/16fa4cf56cdf85cf645c3ca1b739607b515669cb))
- Update the supported Kubernetes versions in the [`SupportedVersions` variable](../../pkg/utils/validation/kubernetesversion/version.go) and in the [Supported Kubernetes Versions documentation](../usage/shoot-operations/supported_k8s_versions.md). ([example](https://github.com/gardener/gardener/pull/12486/commits/16fa4cf56cdf85cf645c3ca1b739607b515669cb))
- Search for the version in codebase. Include all its variants - if it's 1.33, search for both "1.33" and "133". If there's a version-specific logic, adapt it according to the supported versions.
    - The version usages in unit tests and documentation generally should not be adapted unless they contain version-specific logic or features, as per [this discussion](https://github.com/gardener/gardener/pull/12486#pullrequestreview-3151223952).
    - The `AddedInVersion` field in [Kubernetes feature gate version ranges map](https://github.com/gardener/gardener/blob/f78a04e2dfbf9fc833a56b3f6ee69c4b7cf0bfee/pkg/utils/validation/features/featuregates.go#L24), [Kubernetes API groups version ranges](https://github.com/gardener/gardener/blob/f78a04e2dfbf9fc833a56b3f6ee69c4b7cf0bfee/pkg/utils/validation/apigroups/apigroups.go#L21), [Kubernetes API groups to controllers map](https://github.com/gardener/gardener/blob/f78a04e2dfbf9fc833a56b3f6ee69c4b7cf0bfee/pkg/utils/kubernetes/controllers.go#L10) and [Kubernetes admission plugin version ranges map](https://github.com/gardener/gardener/blob/f78a04e2dfbf9fc833a56b3f6ee69c4b7cf0bfee/pkg/utils/validation/admissionplugins/admissionplugins.go#L38) should not be removed, as it provides valuable historical information about when feature gates, admission plugins, and controllers were introduced.
- Remove the images for the version in `imagevector/containers.yaml`. ([example](https://github.com/gardener/gardener/pull/12486/commits/3a4e7d00fedbbd242288fda54cfea4dac2486696))
- Adapt charts. ([example](https://github.com/gardener/gardener/pull/10664/commits/a919cec969476d5fa942a84599e536575bf47c93))
- Remove upstream conformance test results from kubernetes/test-infra. ([example](https://github.com/kubernetes/test-infra/pull/35396)).
    - This should be done only when the corresponding PRs for removal of the version are already merged and released.