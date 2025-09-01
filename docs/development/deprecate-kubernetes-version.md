# Deprecating a Kubernetes version in Gardener

This guide describes the typical steps to deprecate a Kubernetes version in the Gardener codebase. Use the referenced PRs for implementation details and examples:
- https://github.com/gardener/gardener/pull/10664
- https://github.com/gardener/gardener/pull/12486

## Notes
- The exact files and field names can vary between Gardener releases. Use the repository search to find the locations in the codebase (examples below).

## Prerequisites
- Adaptation of a new Kubernetes version in Gardener.

## Tasks
- Create an umbrella issue and include a list of all repositories that must be checked out and potentially be modified in order to deprecate the Kubernetes version.
    - example: https://github.com/gardener/gardener/issues/12409
- Research all the possible deprecations that could be related to the deprecation of the version.
- Search for the version in codebase. Include all its variants - if it's 1.33, search for both "1.33" and "133".
- Replace the version in the code search findings with the highest version supporting the surrounding logic. 
- In documentation you can usually replace the version with a newer one or just remove the version (for example in README.md files).
- Adapt examples to use a newer version, preferably one of the newest ones.
- For provider-extensions remove the images for the version in `imagevector/images.yaml` and also check files containing Cloud Profiles.
- Adapt charts, for example:
```
    {{- if semverCompare ">= <version>" .Values.kubernetesVersion }}
    - --provide-node-service=false
    {{- end }}
```
- Adapt featuregates that were added or removed in <= \<version>







