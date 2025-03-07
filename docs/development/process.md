# Releases, Features, Hotfixes

This document describes how to contribute features or hotfixes, and how new Gardener releases are usually scheduled, validated, etc.

- [Releases, Features, Hotfixes](#releases-features-hotfixes)
  - [Releases](#releases)
    - [Release Responsible Plan](#release-responsible-plan)
    - [Release Validation](#release-validation)
  - [Contributing New Features or Fixes](#contributing-new-features-or-fixes)
    - [TODO Statements](#todo-statements)
    - [Deprecations and Backwards-Compatibility](#deprecations-and-backwards-compatibility)
  - [Cherry Picks](#cherry-picks)
    - [Prerequisites](#prerequisites)
    - [Initiate a Cherry Pick](#initiate-a-cherry-pick)

## Releases

The [@gardener-maintainers](https://github.com/orgs/gardener/teams/gardener-maintainers) are trying to provide a new release roughly every other week (depending on their capacity and the stability/robustness of the `master` branch).

Hotfixes are usually maintained for the latest three minor releases, though, there are no fixed release dates.

### Release Responsible Plan

Version | Week No    | Begin Validation Phase | Due Date           | Release Responsible                                |
------- |------------| ---------------------- | -------------------|----------------------------------------------------|
v1.115  | Week 11-12 | March 10, 2025         | March 23, 2025     | [@ialidzhikov](https://github.com/ialidzhikov)     |
v1.116  | Week 13-14 | March 24, 2025         | April 6, 2025      | [@Kostov6](https://github.com/Kostov6)             |
v1.117  | Week 15-16 | April 7, 2025          | April 20, 2025     | [@marc1404](https://github.com/marc1404)           |
v1.118  | Week 17-18 | April 21, 2025         | May 4, 2025        | [@acumino](https://github.com/acumino)             |
v1.119  | Week 19-20 | May 5, 2025            | May 18, 2025       | [@timuthy](https://github.com/timuthy)             |
v1.120  | Week 21-22 | May 19, 2025           | June 1, 2025       | [@LucaBernstein](https://github.com/LucaBernstein) |
v1.121  | Week 23-24 | June 2, 2025           | June 15, 2025      | [@shafeeqes](https://github.com/shafeeqes)         |
v1.122  | Week 25-26 | June 16, 2025          | June 29, 2025      | [@ary1992](https://github.com/ary1992)             |
v1.123  | Week 27-28 | June 30, 2025          | July 13, 2025      | [@ScheererJ](https://github.com/ScheererJ)         |
v1.124  | Week 29-30 | July 14, 2025          | July 27, 2025      | [@oliver-goetz](https://github.com/oliver-goetz)   |
v1.125  | Week 31-32 | July 28, 2025          | August 10, 2025    | [@tobschli](https://github.com/tobschli)           |
v1.126  | Week 33-34 | August 11, 2025        | August 24, 2025    | [@plkokanov](https://github.com/plkokanov)         |
v1.127  | Week 35-36 | August 25, 2025        | September 7, 2025  | [@rfranzke](https://github.com/rfranzke)           |
v1.128  | Week 37-38 | September 8, 2025      | September 21, 2025 | [@ialidzhikov](https://github.com/ialidzhikov)     |
v1.129  | Week 39-40 | September 22, 2025     | October 5, 2025    | [@Kostov6](https://github.com/Kostov6)             |

Apart from the release of the next version, the release responsible is also taking care of potential hotfix releases of the last three minor versions.
The release responsible is the main contact person for coordinating new feature PRs for the next minor versions or cherry-pick PRs for the last three minor versions.

<details>
  <summary>Click to expand the archived release responsible associations!</summary>

  Version | Week No    | Begin Validation Phase | Due Date           | Release Responsible                                                                    |
  ------- | -----------| ---------------------- | -------------------|----------------------------------------------------------------------------------------|
  v1.17   | Week 07-08 | February 15, 2021      | February 28, 2021  | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.18   | Week 09-10 | March 1, 2021          | March 14, 2021     | [@danielfoehrKn](https://github.com/danielfoehrKn)                                     |
  v1.19   | Week 11-12 | March 15, 2021         | March 28, 2021     | [@timebertt](https://github.com/timebertt)                                             |
  v1.20   | Week 13-14 | March 29, 2021         | April 11, 2021     | [@vpnachev](https://github.com/vpnachev)                                               |
  v1.21   | Week 15-16 | April 12, 2021         | April 25, 2021     | [@timuthy](https://github.com/timuthy)                                                 |
  v1.22   | Week 17-18 | April 26, 2021         | May 9, 2021        | [@BeckerMax](https://github.com/BeckerMax)                                             |
  v1.23   | Week 19-20 | May 10, 2021           | May 23, 2021       | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.24   | Week 21-22 | May 24, 2021           | June 5, 2021       | [@stoyanr](https://github.com/stoyanr)                                                 |
  v1.25   | Week 23-24 | June 7, 2021           | June 20, 2021      | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.26   | Week 25-26 | June 21, 2021          | July 4, 2021       | [@danielfoehrKn](https://github.com/danielfoehrKn)                                     |
  v1.27   | Week 27-28 | July 5, 2021           | July 18, 2021      | [@timebertt](https://github.com/timebertt)                                             |
  v1.28   | Week 29-30 | July 19, 2021          | August 1, 2021     | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.29   | Week 31-32 | August 2, 2021         | August 15, 2021    | [@timuthy](https://github.com/timuthy)                                                 |
  v1.30   | Week 33-34 | August 16, 2021        | August 29, 2021    | [@BeckerMax](https://github.com/BeckerMax)                                             |
  v1.31   | Week 35-36 | August 30, 2021        | September 12, 2021 | [@stoyanr](https://github.com/stoyanr)                                                 |
  v1.32   | Week 37-38 | September 13, 2021     | September 26, 2021 | [@vpnachev](https://github.com/vpnachev)                                               |
  v1.33   | Week 39-40 | September 27, 2021     | October 10, 2021   | [@voelzmo](https://github.com/voelzmo)                                                 |
  v1.34   | Week 41-42 | October 11, 2021       | October 24, 2021   | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.35   | Week 43-44 | October 25, 2021       | November 7, 2021   | [@kris94](https://github.com/kris94)                                                   |
  v1.36   | Week 45-46 | November 8, 2021       | November 21, 2021  | [@timebertt](https://github.com/timebertt)                                             |
  v1.37   | Week 47-48 | November 22, 2021      | December 5, 2021   | [@danielfoehrKn](https://github.com/danielfoehrKn)                                     |
  v1.38   | Week 49-50 | December 6, 2021       | December 19, 2021  | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.39   | Week 01-04 | January 3, 2022        | January 30, 2022   | [@ialidzhikov](https://github.com/ialidzhikov), [@timuthy](https://github.com/timuthy) |
  v1.40   | Week 05-06 | January 31, 2022       | February 13, 2022  | [@BeckerMax](https://github.com/BeckerMax)                                             |
  v1.41   | Week 07-08 | February 14, 2022      | February 27, 2022  | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.42   | Week 09-10 | February 28, 2022      | March 13, 2022     | [@kris94](https://github.com/kris94)                                                   |
  v1.43   | Week 11-12 | March 14, 2022         | March 27, 2022     | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.44   | Week 13-14 | March 28, 2022         | April 10, 2022     | [@timebertt](https://github.com/timebertt)                                             |
  v1.45   | Week 15-16 | April 11, 2022         | April 24, 2022     | [@acumino](https://github.com/acumino)                                                 |
  v1.46   | Week 17-18 | April 25, 2022         | May 8, 2022        | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.47   | Week 19-20 | May 9, 2022            | May 22, 2022       | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.48   | Week 21-22 | May 23, 2022           | June 5, 2022       | [@ary1992](https://github.com/ary1992)                                                 |
  v1.49   | Week 23-24 | June 6, 2022           | June 19, 2022      | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.50   | Week 25-26 | June 20, 2022          | July 3, 2022       | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.51   | Week 27-28 | July 4, 2022           | July 17, 2022      | [@timebertt](https://github.com/timebertt)                                             |
  v1.52   | Week 29-30 | July 18, 2022          | July 31, 2022      | [@acumino](https://github.com/acumino)                                                 |
  v1.53   | Week 31-32 | August 1, 2022         | August 14, 2022    | [@kris94](https://github.com/kris94)                                                   |
  v1.54   | Week 33-34 | August 15, 2022        | August 28, 2022    | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.55   | Week 35-36 | August 29, 2022        | September 11, 2022 | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.56   | Week 37-38 | September 12, 2022     | September 25, 2022 | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.57   | Week 39-40 | September 26, 2022     | October 9, 2022    | [@ary1992](https://github.com/ary1992)                                                 |
  v1.58   | Week 41-42 | October 10, 2022       | October 23, 2022   | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.59   | Week 43-44 | October 24, 2022       | November 6, 2022   | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.60   | Week 45-46 | November 7, 2022       | November 20, 2022  | [@acumino](https://github.com/acumino)                                                 |
  v1.61   | Week 47-48 | November 21, 2022      | December 4, 2022   | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.62   | Week 49-50 | December 5, 2022       | December 18, 2022  | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.63   | Week 01-04 | January 2, 2023        | January 29, 2023   | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.64   | Week 05-06 | January 30, 2023       | February 12, 2023  | [@ary1992](https://github.com/ary1992)                                                 |
  v1.65   | Week 07-08 | February 13, 2023      | February 26, 2023  | [@timuthy](https://github.com/timuthy)                                                 |
  v1.66   | Week 09-10 | February 27, 2023      | March 12, 2023     | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.67   | Week 11-12 | March 13, 2023         | March 26, 2023     | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.68   | Week 13-14 | March 27, 2023         | April 9, 2023      | [@acumino](https://github.com/acumino)                                                 |
  v1.69   | Week 15-16 | April 10, 2023         | April 23, 2023     | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.70   | Week 17-18 | April 24, 2023         | May 7, 2023        | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.71   | Week 19-20 | May 8, 2023            | May 21, 2023       | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.72   | Week 21-22 | May 22, 2023           | June 4, 2023       | [@ary1992](https://github.com/ary1992)                                                 |
  v1.73   | Week 23-24 | June 5, 2023           | June 18, 2023      | [@timuthy](https://github.com/timuthy)                                                 |
  v1.74   | Week 25-26 | June 19, 2023          | July 2, 2023       | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.75   | Week 27-28 | July 3, 2023           | July 16, 2023      | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.76   | Week 29-30 | July 17, 2023          | July 30, 2023      | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.77   | Week 31-32 | July 31, 2023          | August 13, 2023    | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.78   | Week 33-34 | August 14, 2023        | August 27, 2023    | [@acumino](https://github.com/acumino)                                                 |
  v1.79   | Week 35-36 | August 28, 2023        | September 10, 2023 | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.80   | Week 37-38 | September 11, 2023     | September 24, 2023 | [@ScheererJ](https://github.com/ScheererJ)                                             |
  v1.81   | Week 39-40 | September 25, 2023     | October 8, 2023    | [@ary1992](https://github.com/ary1992)                                                 |
  v1.82   | Week 41-42 | October 9, 2023        | October 22, 2023   | [@timuthy](https://github.com/timuthy)                                                 |
  v1.83   | Week 43-44 | October 23, 2023       | November 5, 2023   | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.84   | Week 45-46 | November 6, 2023       | November 19, 2023  | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.85   | Week 47-48 | November 20, 2023      | December 3, 2023   | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.86   | Week 49-50 | December 4, 2023       | December 17, 2023  | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.87   | Week 01-04 | January 1, 2024        | January 28, 2024   | [@acumino](https://github.com/acumino)                                                 |
  v1.88   | Week 05-06 | January 29, 2024       | February 11, 2024  | [@timuthy](https://github.com/timuthy)                                                 |
  v1.89   | Week 07-08 | February 12, 2024      | February 25, 2024  | [@ScheererJ](https://github.com/ScheererJ)                                             |
  v1.90   | Week 09-10 | February 26, 2024      | March 10, 2024     | [@ary1992](https://github.com/ary1992)                                                 |
  v1.91   | Week 11-12 | March 11, 2024         | March 24, 2024     | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.92   | Week 13-14 | March 25, 2024         | April 7, 2024      | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.93   | Week 15-16 | April 8, 2024          | April 21, 2024     | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.94   | Week 17-18 | April 22, 2024         | May 5, 2024        | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.95   | Week 19-20 | May 6, 2024            | May 19, 2024       | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.96   | Week 21-22 | May 20, 2024           | June 2, 2024       | [@acumino](https://github.com/acumino)                                                 |
  v1.97   | Week 23-24 | June 3, 2024           | June 16, 2024      | [@timuthy](https://github.com/timuthy)                                                 |
  v1.98   | Week 25-26 | June 17, 2024          | June 30, 2024      | [@ScheererJ](https://github.com/ScheererJ)                                             |
  v1.99   | Week 27-28 | July 1, 2024           | July 14, 2024      | [@ary1992](https://github.com/ary1992)                                                 |
  v1.100  | Week 29-30 | July 15, 2024          | July 28, 2024      | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.101  | Week 31-32 | July 29, 2024          | August 11, 2024    | [@rfranzke](https://github.com/rfranzke)                                               |
  v1.102  | Week 33-34 | August 12, 2024        | August 25, 2024    | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.103  | Week 35-36 | August 26, 2024        | September 8, 2024  | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.104  | Week 37-38 | September 9, 2024      | September 22, 2024 | [@ialidzhikov](https://github.com/ialidzhikov)                                         |
  v1.105  | Week 39-40 | September 23, 2024     | October 6, 2024    | [@acumino](https://github.com/acumino)                                                 |
  v1.106  | Week 41-42 | October 7, 2024        | October 20, 2024   | [@timuthy](https://github.com/timuthy)                                                 |
  v1.107  | Week 43-44 | October 21, 2024       | November 3, 2024   | [@LucaBernstein](https://github.com/LucaBernstein)                                     |
  v1.108  | Week 45-46 | November 4, 2024       | November 17, 2024  | [@shafeeqes](https://github.com/shafeeqes)                                             |
  v1.109  | Week 47-48 | November 18, 2024      | December 1, 2024   | [@ary1992](https://github.com/ary1992)                                                 |
  v1.110  | Week 49-50 | December 2, 2024       | December 15, 2024  | [@ScheererJ](https://github.com/ScheererJ)                                             |
  v1.111  | Week 01-04 | December 30, 2024      | January 26, 2025   | [@oliver-goetz](https://github.com/oliver-goetz)                                       |
  v1.112  | Week 05-06 | January 27, 2025       | February 9, 2025   | [@tobschli](https://github.com/tobschli)                                               |
  v1.113  | Week 07-08 | February 10, 2025      | February 23, 2025  | [@plkokanov](https://github.com/plkokanov)                                             |
  v1.114  | Week 09-10 | February 24, 2025      | March 9, 2025      | [@rfranzke](https://github.com/rfranzke)                                               |
</details>

Click [here](new-kubernetes-version.md#kubernetes-release-responsible-plan) to view the Kubernetes Release Responsible plan.

### Release Validation

The release phase for a new minor version lasts two weeks.
Typically, the first week is used for the validation of the release.
This phase includes the following steps:

1. `master` (or latest `release-*` branch) is deployed to a development landscape that already hosts some existing seed and shoot clusters.
1. An extended test suite is triggered by the "release responsible" which:
   1. executes the Gardener integration tests for different Kubernetes versions, infrastructures, and `Shoot` settings.
   1. executes the Kubernetes conformance tests.
   1. executes further tests like Kubernetes/OS patch/minor version upgrades.
1. Additionally, every four hours (or on demand) more tests (e.g., including the Kubernetes e2e test suite) are executed for different infrastructures.
1. The "release responsible" is verifying new features or other notable changes (derived of the draft release notes) in this development system.

Usually, the new release is triggered in the beginning of the second week if all tests are green, all checks were successful, and if all of the planned verifications were performed by the release responsible.

## Contributing New Features or Fixes

Please refer to the [Gardener contributor guide](https://gardener.cloud/docs/contribute/).
Besides a lot of general information, it also provides a checklist for newly created pull requests that may help you to prepare your changes for an efficient review process.
If you are contributing a fix or major improvement, please take care to open cherry-pick PRs to all affected and still supported versions once the change is approved and merged in the `master` branch.

:warning: Please ensure that your modifications pass the verification checks (linting, formatting, static code checks, tests, etc.) by executing

```bash
make verify
```

before filing your pull request.

The guide applies for both changes to the `master` and to any `release-*` branch.
All changes must be submitted via a pull request and be reviewed and approved by at least one code owner.

### TODO Statements

Sometimes, TODO statements are being introduced when one cannot follow up immediately with certain tasks or when temporary migration code is required.
In order to properly follow-up with such TODOs and to prevent them from piling up without getting attention, the following rules should be followed:

- Each TODO statement should have an associated person and state when it can be removed.
  Example:
  ```golang
  // TODO(<github-username>): Remove this code after v1.75 has been released.
  ```
- When the task depends on a certain implementation, a GitHub issue should be opened and referenced in the statement.
  Example:
  ```golang
  // TODO(<github-username>): Remove this code after https://github.com/gardener/gardener/issues/<issue-number> has been implemented.
  ```
  The associated person should actively drive the implementation of the referenced issue (unless it cannot be done because of third-party dependencies or conditions) so that the TODO statement does not get stale.
- TODO statements without actionable tasks or those that are unlikely to ever be implemented (maybe because of very low priorities) should not be specified in the first place. If a TODO is specified, the associated person should make sure to actively follow-up.

### Deprecations and Backwards-Compatibility

In case you have to remove functionality _relevant to end-users_ (e.g., a field or default value in the `Shoot` API), please **connect it with a Kubernetes minor version upgrade**.
This way, end-users are forced to actively adapt their manifests when they perform their Kubernetes upgrades.
For example, the `.spec.kubernetes.enableStaticTokenKubeconfig` field in the `Shoot` API is no longer allowed to be set for Kubernetes versions `>= 1.27`.

In case you have to remove or change functionality _which cannot be directly connected with a Kubernetes version upgrade_, please consider introducing a feature gate.
This way, landscape operators can announce the planned changes to their users and communicate a timeline when they plan to activate the feature gate.
End-users can then prepare for it accordingly.
For example, the fact that changes to `kubelet.kubeReserved` in the `Shoot` API will lead to a rolling update of the worker nodes (previously, these changes were updated in-place) is controlled via the `NewWorkerPoolHash` feature gate.

In case you have to remove functionality _relevant to Gardener extensions_, please deprecate it first, and add a [TODO statement](#todo-statements) to remove it only after **at least 9 releases**.
Do not forget to write a proper release note as part of your pull request.
This gives extension developers enough time (~18 weeks) to adapt to the changes (and to release a new version of their extension) before Gardener finally removes the functionality.
Examples are removing a field in the `extensions.gardener.cloud/v1alpha1` API group, or removing a controller in the extensions library.

In case you have to run migration code (_which is mostly internal_), please add a [TODO statement](#todo-statements) to remove it only after **3 releases**.
This way, we can ensure that the Gardener version skew policy is not violated.
For example, the migration code for moving the Prometheus instances under management of `prometheus-operator` was running for three releases.

> [!TIP]
> Please revisit the [version skew policy](../deployment/version_skew_policy.md).

## Cherry Picks

This section explains how to initiate cherry picks on release branches within the `gardener/gardener` repository.

- [Prerequisites](#prerequisites)
- [Initiate a Cherry Pick](#initiate-a-cherry-pick)

### Prerequisites

Before you initiate a cherry pick, make sure that the following prerequisites are accomplished.

- A pull request merged against the `master` branch.
- The release branch exists (check in the [branches section](https://github.com/gardener/gardener/branches)).
- Have the `gardener/gardener` repository cloned as follows:
  - the `origin` remote should point to your fork (alternatively this can be overwritten by passing `FORK_REMOTE=<fork-remote>`).
  - the `upstream` remote should point to the Gardener GitHub org (alternatively this can be overwritten by passing `UPSTREAM_REMOTE=<upstream-remote>`).
- Have `hub` installed, which is most easily installed via
  `go get github.com/github/hub` assuming you have a standard golang
  development environment.
- A GitHub token which has permissions to create a PR in an upstream branch.

### Initiate a Cherry Pick

- Run the [cherry pick script][cherry-pick-script].

  This example applies a master branch PR #3632 to the remote branch
  `upstream/release-v3.14`:

  ```shell
  GITHUB_USER=<your-user> hack/cherry-pick-pull.sh upstream/release-v3.14 3632
  ```

  - Be aware the cherry pick script assumes you have a git remote called
    `upstream` that points at the Gardener GitHub org.

  - You will need to run the cherry pick script separately for each patch
    release you want to cherry pick to. Cherry picks should be applied to all
    active release branches where the fix is applicable.

  - When asked for your GitHub password, provide the created GitHub token
    rather than your actual GitHub password.
    Refer [https://github.com/github/hub/issues/2655#issuecomment-735836048](https://github.com/github/hub/issues/2655#issuecomment-735836048)

- [cherry-pick-script](../../hack/cherry-pick-pull.sh)
