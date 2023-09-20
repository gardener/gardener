# `gardenertools` image

## Motivation

There are lots flakes in our tests because the download of binaries like `yq` from github.com fails. Thus, we created the `gardenertools` image which contains the content of `./hack/tools/bin` directory. These are downloaded binaries like `yq`, `skaffold`, `helm` and Go binaries. This will also speed up the tests a bit, because the binaries does not have to be downloaded anymore and the Go based test tools does not have to be compiled on every test run.

## Usage

`gardenertools` does not have an entrypoint. It is intended to be a base image for test-images only.

Before running tests, binaries could be imported by `make import-tools-bin`. If the source directory is not available, the step will be skipped. The default source directory is `/gardenertools` and can be changed by setting the `TOOLS_BIN_SOURCE_DIR` variable accordingly. 

The scenario runs like this:
- `gardenertools` image will be [built by prow](https://github.com/gardener/ci-infra/blob/bdf1542fb74296b005a69b395779bb89dbdbe703/config/jobs/gardener/gardener-build-dev-images.yaml#L56-L100) every time the content of `./hack/tools.mk` or `./hack/tools/image` changes.
- When prow notices a new `gardenertools` image it build a new version of [golang-test](https://github.com/gardener/ci-infra/tree/master/images/golang-test) and [krte](https://github.com/gardener/ci-infra/tree/master/images/krte) images which include these binaries.
- `import-tools-bin` make target is added as first target to each prow job.

In case some binaries are updated and not part of the `gardenertools` image yet, the tests can still succeed. Each binary has a `.version_...` file in `./hack/tools/bin` directory. If the version file is outdated or does not exist yet, the respective binary is downloaded anyway while the test is executed.
