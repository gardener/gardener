# Features, Hotfixes, and Releases

This document describes how to contribute features or hotfixes, and how new Gardener releases are usually scheduled, validated, etc.

## Contributing new Features or Fixes

Please refer to the [Gardener contributor guide](https://github.com/gardener/documentation/blob/master/CONTRIBUTING.md).
Besides a lot of a general information, it also provides a checklist for newly created pull requests that may help you to prepare your changes for an efficient review process.

:warning: Please ensure that your modifications pass the verification checks (linting, formatting, static code checks, tests, etc.) by executing

```bash
make verify
```

before filing your pull request.

The guide applies for both changes to the `master` and to any `release-*` branch.
All changes must be submitted via a pull request and be reviewed and approved by at least one code owner.

## Releases

There is no fixed schedule for new releases of the `gardener/gardener` component.
The [@gardener-maintainers](https://github.com/orgs/gardener/teams/gardener-maintainers) are trying to provide a new release roughly every other week (depending on their capacity and the stability/robustness of the `master` branch).

Hotfixes are usually only maintained for the latest minor release as well as the minor release before that.

The validation process for new releases usually takes a couple of days and includes the following steps:

1. `master` (or latest `release-*` branch) is deployed to a development landscape that already hosts some existing seed and shoot clusters.
1. An extended test suite is triggered by the "release responsible" which
   1. executes the Gardener integration tests for different Kubernetes versions, infrastructures, and `Shoot` settings.
   1. executes the Kubernetes conformance tests.
   1. executes further tests like Kubernetes/OS patch/minor version upgrades.
1. Additionally, every four hours (or on demand) more tests (e.g., including the Kubernetes e2e test suite) are executed for different infrastructures.
1. The "release responsible" is verifying new features or other notable changes (derived of the draft release notes) in this development system.

If all tests are green, all checks were successful, and the release responsible has performed all of the planned verifications then the release is triggered.
