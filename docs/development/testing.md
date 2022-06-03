# Testing

## Unit Tests

We follow the BDD-style testing principles and are leveraging the [Ginkgo](https://onsi.github.io/ginkgo/) framework along with [Gomega](http://onsi.github.io/gomega/) as matcher library. In order to execute the existing tests, you can use

```bash
make test         # runs tests
make verify       # runs static code checks and tests (unit and integration)
```

There is an additional command for analyzing the code coverage of the tests. Ginkgo will generate standard Go cover profiles which will be translated into an HTML file by the [Go Cover Tool](https://blog.golang.org/cover). Another command helps you to clean up the filesystem from the temporary cover profile files and the HTML report:

```bash
make test-cov
open gardener.coverage.html
make test-cov-clean
```

## Integration Tests (envtests)

Integration tests in Gardener use the `sigs.k8s.io/controller-runtime/pkg/envtest` package.
It sets up a temporary control plane (etcd + kube-apiserver) and runs the test against it.
The `test-integration` make rule prepares the environment automatically by downloading the respective binaries (if not yet present) and sets the necessary environment variables.

```bash
make test-integration
```

If you want to run a specific set of integration tests, you can also execute them using `./hack/test-integration.sh` directly instead of using the `test-integration` rule. For example:

```bash
./hack/test-integration.sh ./test/integration/resourcemanager/tokenrequestor
```

The script takes care of preparing the environment for you.
If you want to execute the test suites directly via `go test` or `ginkgo`, you have to point the `KUBEBUILDER_ASSETS` environment variable to the path that contains the etcd and kube-apiserver binaries. Alternatively, you can install the binaries to `/usr/local/kubebuilder/bin`.

### Debugging Integration Tests

You can configure envtest to use an existing cluster instead of starting a temporary control plane for your test.
This can be helpful for debugging integration tests, because you can easily inspect what is going on in your test cluster.
For example:

```bash
make kind-up
export KUBECONFIG=$PWD/example/gardener-local/kind/kubeconfig
export USE_EXISTING_CLUSTER=true

# run test with verbose output
./hack/test-integration.sh -v ./test/integration/resourcemanager/health -ginkgo.v
```

## End-to-end Tests (using provider-local)

We run a suite of e2e tests on every pull request and periodically on the `master` branch.
It uses a [KinD cluster](https://kind.sigs.k8s.io/) and [skaffold](https://skaffold.dev/) to boostrap a full installation of Gardener based on the current revision, including [provider-local](../extensions/provider-local.md).
This allows us to run e2e tests in an isolated test environment and fully locally without any infrastructure interaction.
The tests perform a set of operations on Shoot clusters, e.g. creating, deleting, hibernating and waking up.

These tests are executed in our prow instance at [prow.gardener.cloud](https://prow.gardener.cloud/), see [job definition](https://github.com/gardener/ci-infra/blob/e324cb79c39c013d7f253c33690b7fcc92c001d8/config/jobs/gardener/gardener-e2e-kind.yaml) and [job history](https://prow.gardener.cloud/?repo=gardener%2Fgardener&job=*gardener-e2e-kind).

You can also run these tests on your development machine, using the following commands:

```bash
make kind-up
export KUBECONFIG=$PWD/example/gardener-local/kind/kubeconfig
make gardener-up
make test-e2e-local  # alternatively: make test-e2e-local-fast
```

If you want to run a specific set of e2e test cases, you can also execute them using `./hack/test-e2e-local.sh` directly in combination with [ginkgo label filters](https://onsi.github.io/ginkgo/#spec-labels). For example:

```bash
./hack/test-e2e-local.sh --label-filter "Shoot && credentials-rotation"
```

If you want to use an existing shoot instead of creating a new one for the test case and deleting it afterwards, you can specify the existing shoot via the following flags.
This can be useful to speed of the development of e2e tests.

```bash
./hack/test-e2e-local.sh --label-filter "Shoot && credentials-rotation" -- -project-namespace=garden-local -existing-shoot-name=local
```

Also see: [developing Gardener locally](getting_started_locally.md) and [deploying Gardener locally](../deployment/getting_started_locally.md).

## Test Machinery Tests

Please see [Test Machinery Tests](testmachinery_tests.md).
