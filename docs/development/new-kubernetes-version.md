# Adding Support For a New Kubernetes Version

This document describes the steps needed to perform in order to confidently add support for a new Kubernetes **minor** version.

## Kubernetes Release Responsible Plan

**Tasks:**
- Ensure Gardener and extensions are updated to support the new Kubernetes version ([example](https://github.com/gardener/gardener/issues/11020))
- Bump Golang dependencies for `k8s.io/*` and `sigs.k8s.io/controller-runtime` ([example](https://github.com/gardener/gardener/issues/11186)) [[ref](#bump-golang-dependencies-for-k8sio-and-sigsk8siocontroller-runtime)]
- Prepare a short (~30-60 min) session for a special edition of Gardener's Review Meeting on new key features and changes in the release
- Check whether old versions can be dropped, and if yes, drive the needed adaptations ([example](https://github.com/gardener/gardener/pull/10664))

Version | Expected Release Date | Release Responsibles                                                                         |
--------|-----------------------|----------------------------------------------------------------------------------------------|
v1.32   | December 11, 2024     | [@marc1404](https://github.com/marc1404), [@LucaBernstein](https://github.com/LucaBernstein) |
v1.33   | April 23, 2025        | [@Kostov6](https://github.com/Kostov6), [@plkokanov](https://github.com/plkokanov)           |
v1.34   | August ?, 2025        | [@tobschli](https://github.com/tobschli), [@ScheererJ](https://github.com/ScheererJ)         |
v1.35   | December ?, 2025      | [@timuthy](https://github.com/timuthy), [@rfranzke](https://github.com/rfranzke)             |
v1.36   | April ?, 2026         | [@oliver-goetz](https://github.com/oliver-goetz), [@ary1992](https://github.com/ary1992)     |
v1.37   | August ?, 2026        | [@acumino](https://github.com/acumino), [@shafeeqes](https://github.com/shafeeqes)           |
v1.38   | December ?, 2026      | [@ialidzhikov](https://github.com/ialidzhikov), TBD                                          |

<details>
  <summary>Click to expand the archived release responsible associations!</summary>

  Version | Expected Release Date | Release Responsibles                                                                         |
  --------|-----------------------|----------------------------------------------------------------------------------------------|
  v1.18   | March 25, 2020        | [@rfranzke](https://github.com/rfranzke)                                                     |
  v1.19   | August 20, 2020       | [@rfranzke](https://github.com/rfranzke)                                                     |
  v1.20   | December 08, 2020     | [@rfranzke](https://github.com/rfranzke)                                                     |
  v1.21   | April 08, 2021        | [@rfranzke](https://github.com/rfranzke)                                                     |
  v1.22   | August 04, 2021       | [@timuthy](https://github.com/timuthy)                                                       |
  v1.23   | December 07, 2021     | [@rfranzke](https://github.com/rfranzke), [@BeckerMax](https://github.com/BeckerMax)         |
  v1.24   | May 03, 2022          | [@acumino](https://github.com/acumino)                                                       |
  v1.25   | August 23, 2022       | [@shafeeqes](https://github.com/shafeeqes)                                                   |
  v1.26   | December 09, 2022     | [@ialidzhikov](https://github.com/ialidzhikov)                                               |
  v1.27   | April 11, 2023        | [@ary1992](https://github.com/ary1992)                                                       |
  v1.28   | August 15, 2023       | [@oliver-goetz](https://github.com/oliver-goetz)                                             |
  v1.29   | December 13, 2023     | [@acumino](https://github.com/acumino)                                                       |
  v1.30   | April 17, 2024        | [@shafeeqes](https://github.com/shafeeqes)                                                   |
  v1.31   | August 13, 2024       | [@ialidzhikov](https://github.com/ialidzhikov)                                               |
</details>

## Introduction

> ⚠️ Typically, once a minor Kubernetes version `vX.Y` is supported by Gardener, then all patch versions `vX.Y.Z` are also automatically supported without any required action.
This is because patch versions do not introduce any new feature or API changes, so there is nothing that needs to be adapted in `gardener/gardener` code.

The Kubernetes community release a new minor version roughly every 4 months.
Please refer to the [official documentation](https://kubernetes.io/releases/release/) about their release cycles for any additional information.

Shortly before a new release, an "umbrella" issue should be opened which is used to collect the required adaptations and to track the work items.
For example, [#5102](https://github.com/gardener/gardener/issues/5102) can be used as a template for the issue description.
As you can see, the task of supporting a new Kubernetes version also includes the provider extensions maintained in the `gardener` GitHub organization and is not restricted to `gardener/gardener` only.

Generally, the work items can be split into two groups:
The first group contains tasks specific to the changes in the given Kubernetes release, the second group contains Kubernetes release-independent tasks.

> ℹ️ Upgrading the `k8s.io/*` and `sigs.k8s.io/controller-runtime` Golang dependencies is typically tracked and worked on separately (see e.g. [#4772](https://github.com/gardener/gardener/issues/4772) or [#5282](https://github.com/gardener/gardener/issues/5282)), however, it is in the responsibility of the Kubernetes Release Responsibles to make sure the libraries get updated (see [the respective section below](#bump-golang-dependencies-for-k8sio-and-sigsk8siocontroller-runtime)).

## Preparation

Once a new Kubernetes version is released, two issues should be created and assigned according to the [release responsible plan](#kubernetes-release-responsible-plan).
Use the most recent previous issue as a template for the new issue:
* [☂️-Issue for "Support for Kubernetes v1.xx" ](https://github.com/gardener/gardener/issues?q=sort%3Acreated-desc%20is%3Aissue%20%E2%98%82%EF%B8%8F-Issue%20for%20%22Support%20for%20Kubernetes%20v1.%22%20)
* [Upgrade `k8s.io/*` to `v0.xx`, `sigs.k8s.io/controller-runtime` to `v0.xx`](https://github.com/gardener/gardener/issues?q=sort%3Acreated-desc%20is%3Aissue%20Upgrade%20%60k8s.io%2F*%60%20to%20%60v0.%60%2C%20%60sigs.k8s.io%2Fcontroller-runtime%60%20to%20%60v0.%60)

## Deriving Release-Specific Tasks

Most new minor Kubernetes releases incorporate API changes, deprecations, or new features.
The community announces them via their [change logs](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/).
In order to derive the release-specific tasks, the respective change log for the new version `vX.Y` has to be read and understood (for example, [the changelog](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.24.md) for `v1.24`).

As already mentioned, typical changes to watch out for are:

- API version promotions or deprecations
- Feature gate promotions or deprecations
- CLI flag changes for Kubernetes components
- New default values in resources
- New available fields in resources
- New features potentially relevant for the Gardener system
- Changes of labels or annotations Gardener relies on
- ...

Obviously, this requires a certain experience and understanding of the Gardener project so that all "relevant changes" can be identified.
While reading the change log, add the tasks (along with the respective PR in `kubernetes/kubernetes` to the umbrella issue).

> ℹ️ Some of the changes might be specific to certain cloud providers. Pay attention to those as well and add related tasks to the issue.

## List Of Release-Independent Tasks

The following paragraphs describe recurring tasks that need to be performed for each new release.

### Make Sure a New `hyperkube` Image Is Released

The [`gardener/hyperkube`](https://github.com/gardener/hyperkube) repository is used to release container images consisting of the `kubectl` and `kubelet` binaries.

There is a CI/CD job that runs periodically and releases a new `hyperkube` image when there is a new Kubernetes release. Before proceeding with the next steps, make sure that a new `hyperkube` image is released for the corresponding new Kubernetes minor version. Make sure that container image is present in GCR.

### Adapting Gardener

<!-- // TODO(marc1404): Reference `compare-k8s-feature-gates.sh` script once it has been fixed (https://github.com/gardener/gardener/issues/11198). -->

- Allow instantiation of a Kubernetes client for the new minor version and update the `README.md`:
  - See [this](https://github.com/gardener/gardener/pull/5255/commits/63bdae022f1cb1c9cbd1cd49b557545dca2ec32a) example commit.
  - The list of supported versions is meanwhile maintained [here](../../pkg/utils/validation/kubernetesversion/version.go) in the `SupportedVersions` variable.
- Maintain the Kubernetes feature gates used for validation of `Shoot` resources:
  - The feature gates are maintained in [this](../../pkg/utils/validation/features/featuregates.go) file.
  - To maintain this list for new Kubernetes versions follow this guide:
    - **Alpha & Beta Feature Gates:**
      - Open: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-alpha-or-beta-features
      - Search the page for the new Kubernetes version, e.g. "1.32".
      - Add new alpha feature gates that have been added "Since" the new Kubernetes version.
      - Change the `Default` for Beta feature gates that have been promoted "Since" the new Kubernetes version.
    - **Graduated & Deprecated Feature Gates:**
      - Open: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-graduated-or-deprecated-features
      - Search the page for the new Kubernetes version, e.g. "1.32".
      - Change `LockedToDefaultInVersion` for GA and Deprecated feature gates that have been graduated/deprecated "Since" the new Kubernetes version.
    - **Removed Feature Gates:**
      - Open: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates-removed/#feature-gates-that-are-removed
      - Search the page for the **current** Kubernetes version, e.g. if the new version is "1.32", search for "1.31".
      - Set `RemovedInVersion` to the **new** Kubernetes version for feature gates that have been removed after the **current** Kubernetes version according to the "To" column. 
  - See [this](https://github.com/gardener/gardener/pull/5255/commits/97923b0604300ff805def8eae981ed388d5e4a83) example commit.
- Maintain the Kubernetes `kube-apiserver` admission plugins used for validation of `Shoot` resources:
  - The admission plugins are maintained in [this](../../pkg/utils/validation/admissionplugins/admissionplugins.go) file.
  - To maintain this list for new Kubernetes versions, run `hack/compare-k8s-admission-plugins.sh <old-version> <new-version>` (e.g. `hack/compare-k8s-admission-plugins.sh 1.26 1.27`).
  - It will present 2 lists of admission plugins: those added and those removed in `<new-version>` compared to `<old-version>`.
  - Add all added admission plugins to the `admissionPluginsVersionRanges` map with `<new-version>` as `AddedInVersion` and no `RemovedInVersion`.
  - For any removed admission plugins, add `<new-version>` as `RemovedInVersion` to the already existing admission plugin in the map.
  - Flag any admission plugins that are required (plugins that must not be disabled in the `Shoot` spec) by setting the `Required` boolean variable to true for the admission plugin in the map.
  - Flag any admission plugins that are forbidden by setting the `Forbidden` boolean variable to true for the admission plugin in the map.
- Maintain the Kubernetes `kube-apiserver` API groups used for validation of `Shoot` resources:
  - The API groups are maintained in [this](../../pkg/utils/validation/apigroups/apigroups.go) file.
  - To maintain this list for new Kubernetes versions, run `hack/compare-k8s-api-groups.sh <old-version> <new-version>` (e.g. `hack/compare-k8s-api-groups.sh 1.26 1.27`).
  - It will present 2 lists of API GroupVersions and 2 lists of API GroupVersionResources: those added and those removed in `<new-version>` compared to `<old-version>`.
  - Add all added group versions to the `apiGroupVersionRanges` map and group version resources to the `apiGVRVersionRanges` map with `<new-version>` as `AddedInVersion` and no `RemovedInVersion`.
  - For any removed APIs, add `<new-version>` as `RemovedInVersion` to the already existing API in the corresponding map.
  - Flag any APIs that are required (APIs that must not be disabled in the `Shoot` spec) by setting the `Required` boolean variable to true for the API in the `apiGVRVersionRanges` map. If this API also should not be disabled for [Workerless Shoots](../usage/shoot/shoot_workerless.md), then set `RequiredForWorkerless` boolean variable also to true. If the API is required for both Shoot types, then both of these booleans need to be set to true. If the whole API Group is required, then mark it correspondingly in the `apiGroupVersionRanges` map.
- Maintain the Kubernetes `kube-controller-manager` controllers for each API group used in deploying required KCM controllers based on active APIs:
  - The API groups are maintained in [this](../../pkg/utils/kubernetes/controllers.go) file.
  - To maintain this list for new Kubernetes versions, run `hack/compute-k8s-controllers.sh <old-version> <new-version>` (e.g. `hack/compute-k8s-controllers.sh 1.28 1.29`).
  - If it complains that the path for the controller is not present in the map, check the release branch of the new Kubernetes version and find the correct path for the missing/wrong controller. You can do so by checking the file `cmd/kube-controller-manager/app/controllermanager.go` and where the controller is initialized from. As of now, there is no straight-forward way to map each controller to its file. If this has improved, please enhance the script.
  - If the paths are correct, it will present 2 lists of controllers: those added and those removed for each API group in `<new-version>` compared to `<old-version>`.
  - Add all added controllers to the `APIGroupControllerMap` map and under the corresponding API group with `<new-version>` as `AddedInVersion` and no `RemovedInVersion`.
  - For any removed controllers, add `<new-version>` as `RemovedInVersion` to the already existing controller in the corresponding API group map. If you are unable to find the removed controller name, then check for its alias. Either in the `staging/src/k8s.io/cloud-provider/names/controller_names.go` file ([example](https://github.com/kubernetes/kubernetes/blob/9fd8f568fe06a154e15cd4919ad2a7f6c6917b9f/staging/src/k8s.io/cloud-provider/names/controller_names.go#L60)) or in the `cmd/kube-controller-manager/app/*` files ([example for apps API group](https://github.com/kubernetes/kubernetes/blob/b584b87a94d6ff5256624bbf83dd5f758dff6eb2/cmd/kube-controller-manager/app/apps.go#L39)). This is because for kubernetes versions starting from `v1.28`, we don't maintain the aliases in the controller, but the controller names itself since some controllers can be initialized without aliases as well ([example](https://github.com/kubernetes/kubernetes/blob/b584b87a94d6ff5256624bbf83dd5f758dff6eb2/cmd/kube-controller-manager/app/networking.go#L32-L39)). The old alias should still be working since it should be backwards compatible as explained [here](https://github.com/kubernetes/kubernetes/blob/9fd8f568fe06a154e15cd4919ad2a7f6c6917b9f/staging/src/k8s.io/cloud-provider/names/controller_names.go#L26-L31). Once the support for kubernetes version < `v1.28` is dropped, we can drop the usages of these aliases and move completely to controller names.
  - Make sure that the API groups in [this](../../pkg/utils/validation/apigroups/apigroups.go) file are in sync with the groups in [this](../../pkg/utils/kubernetes/controllers.go) file. For example, `core/v1` is replaced by the script as `v1` and `apiserverinternal` as `internal`. This is because the API groups registered by the apiserver ([example](https://github.com/kubernetes/kubernetes/blob/8a9b209cb11943f4d53a0d840b55cf92ebfbe004/staging/src/k8s.io/api/apiserverinternal/v1alpha1/register.go#L26)) and the file path imported by the controllers ([example](https://github.com/kubernetes/kubernetes/blob/8a9b209cb11943f4d53a0d840b55cf92ebfbe004/pkg/controller/storageversiongc/gc_controller.go#L24)) might be slightly different in some cases.
- Maintain the `ServiceAccount` names for the controllers part of `kube-controller-manager`:
  - The names are maintained in [this](../../pkg/component/shoot/system/system.go) file.
  - To maintain this list for new Kubernetes versions, run `hack/compare-k8s-controllers.sh <old-version> <new-version>` (e.g. `hack/compare-k8s-controllers.sh 1.26 1.27`).
  - It will present 2 lists of controllers: those added and those removed in `<new-version>` compared to `<old-version>`.
  - Double check whether such `ServiceAccount` indeed appears in the `kube-system` namespace when creating a cluster with `<new-version>`. Note that it sometimes might be hidden behind a default-off feature gate. You can create a local cluster with the new version using the [local provider](getting_started_locally.md). It could so happen that the name of the controller is used in the form of a constant and not a string, see [example](https://github.com/kubernetes/kubernetes/blob/de506ce7ac9981c8253b2f818478bb4093fb7bb6/cmd/kube-controller-manager/app/validatingadmissionpolicystatus.go#L56), In that case not the value of the constant separately. You could also cross-check the names with the result of the `compute-k8s-controllers.sh` script used in the previous step.
  - If it appears, add all added controllers to the list based on the Kubernetes version ([example](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/shootsystem/shootsystem.go#L253-L318)).
  - For any removed controllers, add them only to the Kubernetes version if it is low enough.
- Maintain the names of controllers used for workerless Shoots, [here](https://github.com/gardener/gardener/blob/6988da80bae6ba827d63535655f28885d91b0a23/pkg/component/kubernetes/controllermanager/controllermanager.go#L744-L766) after carefully evaluating whether they are needed if there are no workers.
- Maintain copies of the `DaemonSet` controller's scheduling logic:
  - `gardener-resource-manager`'s [`Node` controller](../concepts/resource-manager.md#node-controller) uses a copy of parts of the `DaemonSet` controller's logic for determining whether a specific `Node` should run a daemon pod of a given `DaemonSet`: see [this file](../../pkg/resourcemanager/controller/node/criticalcomponents/helper/daemon_controller.go).
  - Check the referenced upstream files for changes to the `DaemonSet` controller's logic and adapt our copies accordingly. This might include introducing version-specific checks in our codebase to handle different shoot cluster versions.
- Maintain version specific defaulting logic in shoot admission plugin:
  - Sometimes default values for shoots are intentionally changed with the introduction of a new Kubernetes version.
  - The final Kubernetes version for a shoot is determined in the [Shoot Validator Admission Plugin](https://github.com/gardener/gardener/blob/17dfefaffed6c5e125e35b6614c8dcad801839f1/plugin/pkg/shoot/validator/admission.go).
  - Any defaulting logic that depends on the version should be placed in this admission plugin ([example](https://github.com/gardener/gardener/blob/f754c071e6cf8e45f7ac7bc5924acaf81b96dc06/plugin/pkg/shoot/validator/admission.go#L782)).
- Ensure that [maintenance-controller](../../pkg/controllermanager/controller/shoot/maintenance) is able to auto-update shoots to the new Kubernetes version. Changes to the shoot spec required for the Kubernetes update should be enforced in such cases ([examples](https://github.com/gardener/gardener/blob/bdfc06dc5cb4e5764800fd31ba1dd07727ad78bf/pkg/controllermanager/controller/shoot/maintenance/reconciler.go#L146-L162)).
- Maintain the [Shoot Kubernetes Minor Version Upgrades](../usage/shoot/shoot_kubernetes_versions.md) documentation file.
  - Add breaking and/or behavioral changes introduced with the new Kubernetes version.
- Add the new Kubernetes version to the CloudProfile in local setup.
  - See [this](https://github.com/gardener/gardener/pull/9689/commits/b067a468285a570d5950b62dd99d679ffa4a8bae) example commit.
- In the next Gardener release, file a PR that bumps the used Kubernetes version for local e2e test.
  - This step must be performed in a PR that targets the next Gardener release because of the e2e upgrade tests. The e2e upgrade tests deploy the previous Gardener version where the new Kubernetes version is not present in the CloudProfile. If the e2e tests are adapted in the same PR that adds the support for the Kubernetes version, then the e2e upgrade tests for that PR will fail because the newly added Kubernetes version in missing in the local CloudProfile from the old release.
  - See [this](https://github.com/gardener/gardener/pull/9745) example commit PR.

#### Filing the Pull Request

Work on all the tasks you have collected and validate them using the [local provider](getting_started_locally.md).
Execute the e2e tests and if everything looks good, then go ahead and file the PR ([example PR](https://github.com/gardener/gardener/pull/5255)).
Generally, it is great if you add the PRs also to the umbrella issue so that they can be tracked more easily.

### Adapting Provider Extensions

After the PR in `gardener/gardener` for the support of the new version has been merged, you can go ahead and work on the provider extensions.

> Actually, you can already start even if the PR is not yet merged and use the branch of your fork.

- Update the `github.com/gardener/gardener` dependency in the extension and update the `README.md`.
- Work on release-specific tasks related to this provider.

#### Maintaining the `cloud-controller-manager` Images

Provider extensions are using upstream `cloud-controller-manager` images.
Make sure to adopt the new `cloud-controller-manager` release for the new Kubernetes minor version ([example PR](https://github.com/gardener/gardener-extension-provider-aws/pull/1055)).

Some of the cloud providers are not using upstream `cloud-controller-manager` images for some of the supported Kubernetes versions.
Instead, we build and maintain the images ourselves:

- [cloud-provider-gcp](https://github.com/gardener/cloud-provider-gcp)

Use the instructions below in case you need to maintain a release branch for such `cloud-controller-manager` image:

<details>

<summary>Expand the instructions!</summary>

Until we switch to upstream images, you need to update the Kubernetes dependencies and release a new image.
The required steps are as follows:

- Checkout the `legacy-cloud-provider` branch of the respective repository
- Bump the versions in the `Dockerfile` ([example commit](https://github.com/gardener/cloud-provider-gcp/commit/b7eb3f56b252aaf29adc78406672574b1bc17495)).
- Update the `VERSION` to `vX.Y.Z-dev` where `Z` is the latest available Kubernetes patch version for the `vX.Y` minor version.
- Update the `k8s.io/*` dependencies in the `go.mod` file to `vX.Y.Z` and run `go mod tidy` ([example commit](https://github.com/gardener/cloud-provider-gcp/commit/d41cc9f035bcc4893b40d90a4f617c4d436c5d62)).
- Checkout a new `release-vX.Y` branch and release it ([example](https://github.com/gardener/cloud-provider-gcp/commits/release-v1.23))

> As you are already on it, it is great if you also bump the `k8s.io/*` dependencies for the last three minor releases as well.
In this case, you need to check out the `release-vX.{Y-{1,2,3}}` branches and only perform the last three steps ([example branch](https://github.com/gardener/cloud-provider-gcp/commits/release-v1.20), [example commit](https://github.com/gardener/cloud-provider-gcp/commit/372aa43fbacdeb76b3da9f6fad6cfd924d916227)).

Now you need to update the new releases in the `imagevector/images.yaml` of the respective provider extension so that they are used (see this [example commit](https://github.com/gardener/gardener-extension-provider-aws/pull/942/commits/7e5c0d95ff95d65459d13ae7f79a030049322c71) for reference).

</details>

#### Maintaining Additional Images

Provider extensions might also deploy additional images other than `cloud-controller-manager` that are specific for a given Kubernetes minor version.

Make sure to use a new image for the following components:
- The `ecr-credential-provider` image for the provider-aws extension.

  We are building the `ecr-credential-provider` image ourselves because the upstream community does not provide an OCI image for the corresponding component. For more details, see this [upstream issue](https://github.com/kubernetes/cloud-provider-aws/issues/823).

  Use the following steps to prepare a release of the `ecr-credential-provider` image for the new Kubernetes minor version:
  - Update the `VERSION` file in the [gardener/ecr-credential-provider](https://github.com/gardener/ecr-credential-provider) repository ([example PR](https://github.com/gardener/ecr-credential-provider/pull/2)).
  - Once the PR is merged, trigger a new release from the CI/CD.

- The `csi-driver-cinder` and `csi-driver-manila` images for the provider-openstack extension.

  The upstream community is providing `csi-driver-cinder` and `csi-driver-manila` releases per Kubernetes minor version. Make sure to adopt the new `csi-driver-cinder` and `csi-driver-manila` releases for the new Kubernetes minor version ([example PR](https://github.com/gardener/gardener-extension-provider-openstack/pull/856)).

#### Filing the Pull Request

Again, work on all the tasks you have collected.
This time, you cannot use the local provider for validation but should create real clusters on the various infrastructures.
Typically, the following validations should be performed:

- Create new clusters with versions < `vX.Y`
- Create new clusters with version = `vX.Y`
- Upgrade old clusters from version `vX.{Y-1}` to version `vX.Y`
- Delete clusters with versions < `vX.Y`
- Delete clusters with version = `vX.Y`

If everything looks good, then go ahead and file the PR ([example PR](https://github.com/gardener/gardener-extension-provider-aws/pull/480)).
Generally, it is again great if you add the PRs also to the umbrella issue so that they can be tracked more easily.

### Bump Golang dependencies for `k8s.io/*` and `sigs.k8s.io/controller-runtime`

The versions of the upstream Go dependencies `k8s.io/*` and `sigs.k8s.io/controller-runtime` should be updated to the latest version that is compatible with the new Kubernetes version.
This is handled via a separate issue ([example](https://github.com/gardener/gardener/issues/11186)) and pull request ([example](https://github.com/gardener/gardener/pull/11418)).

* Bump the `k8s.io/*` dependencies in the `go.mod` file to the latest version that is compatible with the new Kubernetes version.
* Bump the `sigs.k8s.io/controller-runtime` dependency in the `go.mod` file to the latest version that is compatible with the new Kubernetes version.
  * Ensure that the major version of `gomodules.xyz/jsonpatch/v2` that we use ([go.mod](https://github.com/gardener/gardener/blob/master/go.mod)) matches the major version of the same dependency in the updated version of `sigs.k8s.io/controller-runtime` ([go.mod](https://github.com/kubernetes-sigs/controller-runtime/blob/main/go.mod)).
