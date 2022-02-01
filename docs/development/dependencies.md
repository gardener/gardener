# Dependency Management

We are using [go modules](https://github.com/golang/go/wiki/Modules) for depedency management.
In order to add a new package dependency to the project, you can perform `go get <PACKAGE>@<VERSION>` or edit the `go.mod` file and append the package along with the version you want to use.

## Updating Dependencies

The `Makefile` contains a rule called `revendor` which performs `go mod tidy` and `go mod vendor`.
`go mod tidy` makes sure go.mod matches the source code in the module. It adds any missing modules necessary to build the current module's packages and dependencies, and it removes unused modules that don't provide any relevant packages.
`go mod vendor` resets the main module's vendor directory to include all packages needed to build and test all the main module's packages. It does not include test code for vendored packages.

```bash
make revendor
```

The dependencies are installed into the `vendor` folder which **should be added** to the VCS.

:warning: Make sure that you test the code after you have updated the dependencies!

## Exported Packages

This repository contains several packages that could be considered "exported packages", in a sense that they are supposed to be reused in other Go projects.
For example:

- Gardener's API packages: `pkg/apis`
- Library for building Gardener extensions: `extensions`
- Gardener's Test Framework: `test/framework`

There are a few more folders in this repository (non-Go sources) that are reused across projects in the gardener organization:

- GitHub templates: `.github`
- Concourse / cc-utils related helpers: `hack/.ci`
- Development, build and testing helpers: `hack`

These packages feature a dummy `doc.go` file to allow other Go projects to pull them in as go mod dependencies.

These packages are explicitly *not* supposed to be used in other projects (consider them as "non-exported"):

- API validation packages: `pkg/apis/*/*/validation`
- Operation package (main Gardener business logic regarding `Seed` and `Shoot` clusters): `pkg/operation`
- Third party code: `third_party`

Currently, we don't have a mechanism yet for selectively syncing out these exported packages into dedicated repositories like kube's [staging mechanism](https://github.com/kubernetes/kubernetes/tree/master/staging) ([publishing-bot](https://github.com/kubernetes/publishing-bot)).

## Import Restrictions

We want to make sure, that other projects can depend on this repository's "exported" packages without pulling in the entire repository (including "non-exported" packages) or a high number of other unwanted dependencies.
Hence, we have to be careful when adding new imports or references between our packages.

> ℹ️ General rule of thumb: the mentioned "exported" packages should be as self-contained as possible and depend on as few other packages in the repository and other projects as possible.

In order to support that rule and automatically check compliance with that goal, we leverage [import-boss](https://github.com/kubernetes/code-generator/tree/master/cmd/import-boss).
The tool checks all imports of the given packages (including transitive imports) against rules defined in `.import-restrictions` files in each directory.
An import is allowed if it matches at least one allowed prefix and does not match any forbidden prefixes.
Note: `''` (the empty string) is a prefix of everything.
For more details, see: https://github.com/kubernetes/code-generator/tree/master/cmd/import-boss

`import-boss` is executed on every pull request and blocks the PR if it doesn't comply with the defined import restrictions.
You can also run it locally using `make check`.

Import restrictions should be changed in the following situations:

- We spot a new pattern of imports across our packages that was not restricted before but makes it more difficult for other projects to depend on our "exported" packages.
  In that case, the imports should be further restricted to disallow such problematic imports, and the code/package structure should be reworked to comply with the newly given restrictions.
- We want to share code between packages, but existing import restrictions prevent us from doing so.
  In that case, please consider what additional dependencies it will pull in, when loosening existing restrictions.
  Also consider possible alternatives, like code restructurings or extracting shared code into dedicated packages for minimal impact on dependent projects.
