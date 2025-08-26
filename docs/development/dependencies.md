# Dependency Management

We are using [go modules](https://github.com/golang/go/wiki/Modules) for dependency management.
In order to add a new package dependency to the project, you can perform `go get <PACKAGE>@<VERSION>` or edit the `go.mod` file and append the package along with the version you want to use.

## Updating Dependencies

The `Makefile` contains a rule called `tidy` which performs `go mod tidy`:

- `go mod tidy` makes sure `go.mod` matches the source code in the module. It adds any missing modules necessary to build the current module's packages and dependencies, and it removes unused modules that don't provide any relevant packages.

```bash
make tidy
```

:warning: Make sure that you test the code after you have updated the dependencies!

## Exported Packages

This repository contains several packages that could be considered "exported packages", in a sense that they are supposed to be reused in other Go projects.
For example:

- Gardener's API packages: `pkg/apis`
- Library for building Gardener extensions: `extensions`
- Gardener's Test Framework: `test/framework`

There are a few more folders in this repository (non-Go sources) that are reused across projects in the Gardener organization:

- GitHub templates: `.github`
- Concourse / cc-utils related helpers: `hack/.ci`
- Development, build and testing helpers: `hack`

These packages feature a dummy `doc.go` file to allow other Go projects to pull them in as go mod dependencies.

These packages are explicitly *not* supposed to be used in other projects (consider them as "non-exported"):

- API validation packages: `pkg/apis/*/*/validation`
- Operation package (main Gardener business logic regarding `Seed` and `Shoot` clusters): `pkg/gardenlet/operation`
- Third party code: `third_party`

Currently, we don't have a mechanism yet for selectively syncing out these exported packages into dedicated repositories like kube's [staging mechanism](https://github.com/kubernetes/kubernetes/tree/master/staging) ([publishing-bot](https://github.com/kubernetes/publishing-bot)).

## Import Restrictions

We want to make sure that other projects can depend on this repository's "exported" packages without pulling in the entire repository (including "non-exported" packages) or a high number of other unwanted dependencies.
Hence, we have to be careful when adding new imports or references between our packages.

> ℹ️ General rule of thumb: the mentioned "exported" packages should be as self-contained as possible and depend on as few other packages in the repository and other projects as possible.

In order to support that rule and automatically check compliance with that goal, we leverage [import-boss](https://github.com/kubernetes/kubernetes/blob/master/cmd/import-boss).
The tool checks all imports of the given packages (including transitive imports) against rules defined in `.import-restrictions` files in each directory.
An import is allowed if it matches at least one allowed prefix and does not match any forbidden prefixes.

> Note: `''` (the empty string) is a prefix of everything.
For more details, see the [import-boss](https://github.com/kubernetes/kubernetes/blob/master/cmd/import-boss/README.md) topic.

`import-boss` is executed on every pull request and blocks the PR if it doesn't comply with the defined import restrictions.
You can also run it locally using `make check`.

Import restrictions should be changed in the following situations:

- We spot a new pattern of imports across our packages that was not restricted before but makes it more difficult for other projects to depend on our "exported" packages.
  In that case, the imports should be further restricted to disallow such problematic imports, and the code/package structure should be reworked to comply with the newly given restrictions.
- We want to share code between packages, but existing import restrictions prevent us from doing so.
  In that case, please consider what additional dependencies it will pull in, when loosening existing restrictions.
  Also consider possible alternatives, like code restructurings or extracting shared code into dedicated packages for minimal impact on dependent projects.

## Updating Go

Go releases twice a year and supports the latest two releases ([ref](https://go.dev/s/release)).
We try to keep up with the latest Go release and update our projects accordingly.
The language version directive in the [`go.mod`](../../go.mod) file should remain on the lowest supported Go version to avoid unnecessary restrictions for consumers of our packages.
Once a new Go version is released, please consider the guidance below for updating the Go version in this repository.

Check the [release notes](https://go.dev/doc/devel/release) to see whether there are any relevant changes that might be useful or affect us in another way.

Maintain the image variants of the [`krte`](https://github.com/gardener/ci-infra/tree/master/images/krte) image used for end-to-end testing in [KinD](https://kind.sigs.k8s.io/) in the following file:
[`hack/tools/image/variants.yaml`](../../hack/tools/image/variants.yaml)  
Remove older Go versions [if they are no longer supported](https://endoflife.date/go) and add the new version ([example](https://github.com/gardener/gardener/pull/12770)).

Check the registry to see when the new image variants are available:
* [europe-docker.pkg.dev/gardener-project/releases/ci-infra/krte](https://console.cloud.google.com/artifacts/docker/gardener-project/europe/releases/ci-infra%2Fkrte)
* [europe-docker.pkg.dev/gardener-project/releases/ci-infra/golang-test](https://console.cloud.google.com/artifacts/docker/gardener-project/europe/releases/ci-infra%2Fgolang-test)

The images used by the CI jobs are maintained in the [ci-infra repository](https://github.com/gardener/ci-infra).
Update the references for the end-to-end tests with the new `krte` image and for unit- and integration tests with the new `golang-test` image ([example](https://github.com/gardener/ci-infra/pull/4338)).
As a courtesy, consider removing references to no longer maintained image variants and updating to newer images wherever possible ([example](https://github.com/gardener/ci-infra/pull/4352)).
Go maintains a [strong backward compatibility promise](https://go.dev/blog/compat), even if a tool has been built for an older Go version try running it with the latest version and update accordingly. 

Finally, update the Go version references inside this repository (mainly GitHub Actions workflows and image references) to the newer version ([example](https://github.com/gardener/gardener/pull/12753)).
In the [`go.mod`](../../go.mod) file, ensure that the language version directive and, if defined, the toolchain directive are set to the lowest supported Go version.
