# `golang-test` image

## Motivation

There are lots flakes in our tests because the download of binaries like `yq` from github.com fails. Thus, we created the `golang-test` image which contains the content of `./hack/tools/bin` directory. These are downloaded binaries like `yq`, `skaffold`, `helm` and Go binaries. This will also speed up the tests a bit, because the binaries does not have to be downloaded anymore and the Go based test tools does not have to be compiled on every test run.

## Usage

`golang-test` installs a couple of debian tools and binaries of the `gardener/gardener` toolchain.

Before running tests, binaries could be imported by `make import-tools-bin`. If the source directory is not available, the step will be skipped. The default source directory is `/gardenertools` and can be changed by setting the `TOOLS_BIN_SOURCE_DIR` variable accordingly. 

In case some binaries are updated and not part of the `golang-test` image yet, the tests can still succeed. Each binary has a `.version_...` file in `./hack/tools/bin` directory. If the version file is outdated or does not exist yet, the respective binary is downloaded anyway while the test is executed.

`golang-test` image is used to run our unit and integration tests. Additionally, it is the base image of our [krte](https://github.com/gardener/ci-infra/tree/master/images/krte) image, we use to run our e2e tests. When a new `golang-test` image was built, prow automatically builds the corresponding `krte` image.
