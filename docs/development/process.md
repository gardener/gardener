# Releases, Features, Hotfixes

This document describes how to contribute features or hotfixes, and how new Gardener releases are usually scheduled, validated, etc.

- [Releases](#releases)
- [Contributing new Features or Fixes](#contributing-new-features-or-fixes)
- [Cherry Picks](#cherry-picks)

## Releases

The [@gardener-maintainers](https://github.com/orgs/gardener/teams/gardener-maintainers) are trying to provide a new release roughly every other week (depending on their capacity and the stability/robustness of the `master` branch).

Hotfixes are usually maintained for the latest three minor releases, though, there are no fixed release dates.

### Release Responsible Plan

Version | Week No     | Begin Validation Phase | Due Date           | Release Responsible |
------- | ----------- | ---------------------- | -------------------| ------------------- |
v1.32   | Week 37-38  | September 13, 2021     | September 26, 2021 | [@vpnachev](https://github.com/vpnachev)           |
v1.33   | Week 39-40  | September 27, 2021     | October 10, 2021   | [@voelzmo](https://github.com/voelzmo)             |
v1.34   | Week 41-42  | October 11, 2021       | October 24, 2021   | [@plkokanov](https://github.com/plkokanov)         |
v1.35   | Week 43-44  | October 25, 2021       | November 7, 2021   | [@kris94](https://github.com/kris94)               |
v1.36   | Week 45-46  | November 8, 2021       | November 21, 2021  | [@timebertt](https://github.com/timebertt)         |
v1.37   | Week 47-48  | November 22, 2021      | December 5, 2021   | [@danielfoehrKn](https://github.com/danielfoehrKn) |
v1.38   | Week 49-50  | December 6, 2021       | December 19, 2021  | [@rfranzke](https://github.com/rfranzke)           |
v1.39   | Week 01-02  | January 3, 2022        | January 16, 2022   | [@ialidzhikov](https://github.com/ialidzhikov)     |
v1.40   | Week 03-04  | January 17, 2022       | January 30, 2022   | [@timuthy](https://github.com/timuthy)             |
v1.41   | Week 05-06  | January 31, 2022       | February 13, 2022  | [@BeckerMax](https://github.com/BeckerMax)         |
v1.42   | Week 06-07  | February 14, 2022      | February 27, 2022  | [@stoyanr](https://github.com/stoyanr)             |
v1.43   | Week 08-09  | February 28, 2022      | March 13, 2022     | [@vanjiii](https://github.com/vanjiii)             |
v1.44   | Week 10-11  | March 14, 2022         | March 27, 2022     | [@voelzmo](https://github.com/voelzmo)             |

Apart from the release of the next version, the release responsible is also taking care of potential hotfix releases of the last three minor versions.
The release responsible is the main contact person for coordinating new feature PRs for the next minor versions or cherry-pick PRs for the last three minor versions.

<details>
  <summary>Click to expand the archived release responsible associations!</summary>

  Version | Week No     | Begin Validation Phase | Due Date           | Release Responsible |
  ------- | ----------- | ---------------------- | -------------------| ------------------- |
  v1.17   | Week 07-08  | February 15, 2021      | February 28, 2021  | [@rfranzke](https://github.com/rfranzke)           |
  v1.18   | Week 09-10  | March 1, 2021          | March 14, 2021     | [@danielfoehrKn](https://github.com/danielfoehrKn) |
  v1.19   | Week 11-12  | March 15, 2021         | March 28, 2021     | [@timebertt](https://github.com/timebertt)         |
  v1.20   | Week 13-14  | March 29, 2021         | April 11, 2021     | [@vpnachev](https://github.com/vpnachev)           |
  v1.21   | Week 15-16  | April 12, 2021         | April 25, 2021     | [@timuthy](https://github.com/timuthy)             |
  v1.22   | Week 17-18  | April 26, 2021         | May 9, 2021        | [@BeckerMax](https://github.com/BeckerMax)         |
  v1.23   | Week 19-20  | May 10, 2021           | May 23, 2021       | [@ialidzhikov](https://github.com/ialidzhikov)     |
  v1.24   | Week 21-22  | May 24, 2021           | June 5, 2021       | [@stoyanr](https://github.com/stoyanr)             |
  v1.25   | Week 23-24  | June 7, 2021           | June 20, 2021      | [@rfranzke](https://github.com/rfranzke)           |
  v1.26   | Week 25-26  | June 21, 2021          | July 4, 2021       | [@danielfoehrKn](https://github.com/danielfoehrKn) |
  v1.27   | Week 27-28  | July 5, 2021           | July 18, 2021      | [@timebertt](https://github.com/timebertt)         |
  v1.28   | Week 29-30  | July 19, 2021          | August 1, 2021     | [@ialidzhikov](https://github.com/ialidzhikov)     |
  v1.29   | Week 31-32  | August 2, 2021         | August 15, 2021    | [@timuthy](https://github.com/timuthy)             |
  v1.30   | Week 33-34  | August 16, 2021        | August 29, 2021    | [@BeckerMax](https://github.com/BeckerMax)         |
  v1.31   | Week 35-36  | August 30, 2021        | September 12, 2021 | [@stoyanr](https://github.com/stoyanr)             |
</details>

### Release Validation

The release phase for a new minor version lasts two weeks.
Typically, the first week is used for the validation of the release.
This phase includes the following steps:

1. `master` (or latest `release-*` branch) is deployed to a development landscape that already hosts some existing seed and shoot clusters.
1. An extended test suite is triggered by the "release responsible" which
   1. executes the Gardener integration tests for different Kubernetes versions, infrastructures, and `Shoot` settings.
   1. executes the Kubernetes conformance tests.
   1. executes further tests like Kubernetes/OS patch/minor version upgrades.
1. Additionally, every four hours (or on demand) more tests (e.g., including the Kubernetes e2e test suite) are executed for different infrastructures.
1. The "release responsible" is verifying new features or other notable changes (derived of the draft release notes) in this development system.

Usually, the new release is triggered in the beginning of the second week if all tests are green, all checks were successful, and if all of the planned verifications were performed by the release responsible.

## Contributing new Features or Fixes

Please refer to the [Gardener contributor guide](https://gardener.cloud/docs/contribute/).
Besides a lot of a general information, it also provides a checklist for newly created pull requests that may help you to prepare your changes for an efficient review process.
If you are contributing a fix or major improvement, please take care to open cherry-pick PRs to all affected and still supported versions once the change is approved and merged in the `master` branch.

:warning: Please ensure that your modifications pass the verification checks (linting, formatting, static code checks, tests, etc.) by executing

```bash
make verify
```

before filing your pull request.

The guide applies for both changes to the `master` and to any `release-*` branch.
All changes must be submitted via a pull request and be reviewed and approved by at least one code owner.

## Cherry Picks

This section explains how to initiate cherry picks on release branches within the `gardener/gardener` repository.

- [Prerequisites](#prerequisites)
- [Initiate a Cherry Pick](#initiate-a-cherry-pick)

### Prerequisites

Before you initiate a cherry pick, make sure that the following prerequisites are accomplished.

- A pull request merged against the `master` branch.
- The release branch exists (example: [`release-v1.18`](https://github.com/gardener/gardener/tree/release-v1.18))
- Have the `gardener/gardener` repository cloned as follows:
  - the `origin` remote should point to your fork (alternatively this can be overwritten by passing `FORK_REMOTE=<fork-remote>`)
  - the `upstream` remote should point to the Gardener github org (alternatively this can be overwritten by passing `UPSTREAM_REMOTE=<upstream-remote>`)
- Have `hub` installed, which is most easily installed via
  `go get github.com/github/hub` assuming you have a standard golang
  development environment.
- A github token which has permissions to create a PR in an upstream branch.

### Initiate a Cherry Pick

- Run the [cherry pick script][cherry-pick-script]

  This example applies a master branch PR #3632 to the remote branch
  `upstream/release-v3.14`:

  ```shell
  GITHUB_USER=<your-user> hack/cherry-pick-pull.sh upstream/release-v3.14 3632
  ```

  - Be aware the cherry pick script assumes you have a git remote called
    `upstream` that points at the Gardener github org.

  - You will need to run the cherry pick script separately for each patch
    release you want to cherry pick to. Cherry picks should be applied to all
    active release branches where the fix is applicable.

  - When asked for your github password, provide the created github token
    rather than your actual github password.
    Refer [https://github.com/github/hub/issues/2655#issuecomment-735836048](https://github.com/github/hub/issues/2655#issuecomment-735836048)

[cherry-pick-script]: https://github.com/gardener/gardener/blob/master/hack/cherry-pick-pull.sh
