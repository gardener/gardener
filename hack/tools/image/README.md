# `golang-test` image

## Motivation

There were many flakes in our tests when downloads of binaries like `yq` failed from https://github.com.
Thus, we created the `golang-test` image which contains the content of `./hack/tools/bin` directory. It already contains tools like `yq`, `skaffold`, `helm` and Go binaries.
This slightly speeds up the tests, because required binaries do not have to be downloaded and a compilation of Go based test tools is not required anymore on every test run.

## Usage

`golang-test` installs a couple of debian tools and binaries of the `gardener/gardener` toolchain.

Before running tests, binaries could be imported by `make import-tools-bin`. If the source directory is not available, the step will be skipped. The default source directory is `/gardenertools` and can be changed by setting the `TOOLS_BIN_SOURCE_DIR` variable accordingly. 

In case some binaries are updated and not part of the `golang-test` image yet, the tests can still succeed. Each binary has a `.version_...` file in `./hack/tools/bin` directory. If the version file is outdated or does not exist yet, the respective binary is downloaded anyway while the test is executed.

`golang-test` image is used to run our unit and integration tests. Additionally, it is the base image of our [krte](https://github.com/gardener/ci-infra/tree/master/images/krte) image, we use to run e2e tests with. When a new `golang-test` image was built, prow automatically builds the corresponding `krte` image.
