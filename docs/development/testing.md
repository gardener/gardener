# Testing Strategy and Developer Guideline

This document walks you through:

- What kind of tests we have in Gardener
- How to run each of them
- What purpose each kind of test serves
- How to best write tests that are correct, stable, fast and maintainable
- How to debug tests that are not working as expected

The document is aimed towards developers that want to contribute code and need to write tests, as well as maintainers and reviewers that review test code.
It serves as a common guide that we commit to follow in our project to ensure consistency in our tests, good coverage for high confidence, and good maintainability.

The guidelines are not meant to be absolute rules.
Always apply common sense and adapt the guideline if it doesn't make much sense for some cases.
If in doubt, don't hesitate to ask questions during a PR review (as an author, but also as a reviewer).
Add new learnings as soon as we make them!

Generally speaking, **tests are a strict requirement for contributing new code**.
If you touch code that is currently untested, you need to add tests for the new cases that you introduce as a minimum.
Ideally though, you would add the missing test cases for the current code as well (**boy scout rule** -- "always leave the campground cleaner than you found it").

## Writing Tests (Relevant for All Kinds)

- We follow BDD (behavior-driven development) testing principles and use [Ginkgo](https://onsi.github.io/ginkgo/), along with [Gomega](http://onsi.github.io/gomega/).
  - Make sure to check out their extensive guides for more information and how to best leverage all of their features
- Use `By` to structure test cases with multiple steps, so that steps are easy to follow in the logs: [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/pkg/client/kubernetes/clientmap/internal/generic_clientmap_test.go#L122-L138)
- Call `defer GinkgoRecover()` if making assertions in goroutines: [doc](https://pkg.go.dev/github.com/onsi/ginkgo#GinkgoRecover), [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/scheduler/scheduler_test.go#L65-L68)
- Use `DeferCleanup` instead of cleaning up manually (or use custom coding from the test framework): [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/resourcemanager/health/health_suite_test.go#L102-L105), [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/resourcemanager/health/health_test.go#L385-L390)
  - `DeferCleanup` makes sure to run the cleanup code in the right point in time, e.g., a `DeferCleanup` added in `BeforeEach` is executed with `AfterEach`.
- Test results should point to locations that cause the failures, so that the CI output isn't too difficult to debug/fix.
  - Consider using `ExpectWithOffset` if the test uses assertions made in a helper function, among other assertions defined directly in the test (e.g. `expectSomethingWasCreated`): [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/extensions/pkg/controller/controlplane/genericactuator/actuator_test.go#L732-L736)
  - Make sure to add additional descriptions to Gomega matchers if necessary (e.g. in a loop): [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/e2e/shoot/internal/rotation/certificate_authorities.go#L89-L93)
- Introduce helper functions for assertions to make test more readable where applicable: [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/gardenlet/shootsecret/controller_test.go#L323-L331)
- Keep test code and output readable:
  - Introduce custom matchers where applicable: [example matcher](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/pkg/utils/test/matchers/matchers.go#L51-L57)
  - Prevent [gstruct matchers](https://pkg.go.dev/github.com/onsi/gomega/gstruct) on larger object list: [example test](https://github.com/gardener/gardener/blob/882c00c2c5835324f41a0ebde8c81f5ec6050074/test/integration/operator/garden/garden/garden_test.go#L415-L465). The failure output is often truncated and unclear.
- Don't rely on accurate timing of `time.Sleep` and friends.
  - If doing so, CPU throttling in CI will make tests flaky, [example flake](https://github.com/gardener/gardener/issues/5410)
  - Use fake clocks instead, [example PR](https://github.com/gardener/gardener/pull/4569)
- Use the same client schemes that are also used by production code to avoid subtle bugs/regressions: [example PR](https://github.com/gardener/gardener/pull/5469), [production schemes](https://github.com/gardener/gardener/blob/2de823d0a457beb9d680260243032c95fa47dc72/pkg/resourcemanager/cmd/source.go#L34-L43), [usage in test](https://github.com/gardener/gardener/blob/2de823d0a457beb9d680260243032c95fa47dc72/test/integration/resourcemanager/health/health_suite_test.go#L108-L109)
- Make sure that your test is actually asserting the right thing and it doesn't pass if the exact bug is introduced that you want to prevent.
  - Use specific error matchers instead of asserting any error has happened, make sure that the corresponding branch in the code is tested, e.g., prefer
    ```go
    Expect(err).To(MatchError("foo"))
    ```
    over
    ```go
    Expect(err).To(HaveOccurred())
    ```
  - If you're unsure about your test's behavior, attaching the debugger can sometimes be helpful to make sure your test is correct.
- About overwriting global variables:
  - This is a common pattern (or hack?) in go for faking calls to external functions.
  - However, this can lead to races, when the global variable is used from a goroutine (e.g., the function is called).
  - Alternatively, set fields on structs (passed via parameter or set directly): this is not racy, as struct values are typically (and should be) only used for a single test case.
  - An alternative to dealing with function variables and fields:
    - Add an interface which your code depends on
    - Write a fake and a real implementation (similar to `clock.Clock.Sleep`)
    - The real implementation calls the actual function (`clock.RealClock.Sleep` calls `time.Sleep`)
    - The fake implementation does whatever you want it to do for your test (`clock.FakeClock.Sleep` waits until the test code advanced the time)
- Use constants in test code with care.
  - Typically, you should not use constants from the same package as the tested code, instead use literals.
  - If the constant value is changed, tests using the constant will still pass, although the "specification" is not fulfilled anymore.
  - There are cases where it's fine to use constants, but keep this caveat in mind when doing so.
- Creating sample data for tests can be a high effort.
  - If valuable, add a package for generating common sample data, e.g. Shoot/Cluster objects.
- Make use of the `testdata` directory for storing arbitrary sample data needed by tests (helm charts, YAML manifests, etc.), [example PR](https://github.com/gardener/gardener/pull/2140)
  - From https://pkg.go.dev/cmd/go/internal/test:
    > The go tool will ignore a directory named "testdata", making it available to hold ancillary data needed by the tests.

## Unit Tests

### Running Unit Tests

Run all unit tests:

```bash
make test
```

Run all unit tests with test coverage:

```bash
make test-cov
open test.coverage.html
make test-cov-clean
```

Run unit tests of specific packages:

```bash
# run with same settings like in CI (race detector, timeout, ...)
./hack/test.sh ./pkg/resourcemanager/controller/... ./pkg/utils/secrets/...

# freestyle
go test ./pkg/resourcemanager/controller/... ./pkg/utils/secrets/...
ginkgo run ./pkg/resourcemanager/controller/... ./pkg/utils/secrets/...
```

### Debugging Unit Tests

Use ginkgo to focus on (a set of) test specs via [code](https://onsi.github.io/ginkgo/#focused-specs) or via [CLI flags](https://onsi.github.io/ginkgo/#description-based-filtering).
Remember to unfocus specs before contributing code, otherwise your PR tests will fail.

```bash
$ ginkgo run --focus "should delete the unused resources" ./pkg/resourcemanager/controller/garbagecollector
...
Will run 1 of 3 specs
SS•

Ran 1 of 3 Specs in 0.003 seconds
SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 2 Skipped
PASS
```

Use ginkgo to run tests until they fail:

```bash
$ ginkgo run --until-it-fails ./pkg/resourcemanager/controller/garbagecollector
...
Ran 3 of 3 Specs in 0.004 seconds
SUCCESS! -- 3 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS

All tests passed...
Will keep running them until they fail.
This was attempt #58
No, seriously... you can probably stop now.
```

Use the [`stress` tool](https://pkg.go.dev/golang.org/x/tools/cmd/stress) for deflaking tests that fail sporadically in CI, e.g., due resource contention (CPU throttling):

```bash
# get the stress tool
go install golang.org/x/tools/cmd/stress@latest

# build a test binary
ginkgo build ./pkg/resourcemanager/controller/garbagecollector
# alternatively
go test -c ./pkg/resourcemanager/controller/garbagecollector

# run the test in parallel and report any failures
stress -p 16 ./pkg/resourcemanager/controller/garbagecollector/garbagecollector.test -ginkgo.focus "should delete the unused resources"
5s: 1077 runs so far, 0 failures
10s: 2160 runs so far, 0 failures
```

`stress` will output a path to a file containing the full failure message when a test run fails.

### Purpose of Unit Tests

- Unit tests prove the correctness of a single unit according to the specification of its interface.
  - Think: Is the unit that I introduced doing what it is supposed to do for all cases?
- Unit tests protect against regressions caused by adding new functionality to or refactoring of a single unit.
  - Think: Is the unit that was introduced earlier (by someone else) and that I changed still doing what it was supposed to do for all cases?
- Example units: functions (conversion, defaulting, validation, helpers), structs (helpers, basic building blocks like the Secrets Manager), predicates, event handlers.
- For these purposes, unit tests need to cover all important cases of input for a single unit and cover edge cases / negative paths as well (e.g., errors).
  - Because of the possible high dimensionality of test input, unit tests need to be fast to execute: individual test cases should not take more than a few seconds, test suites not more than 2 minutes.
  - Fuzzing can be used as a technique in addition to usual test cases for covering edge cases.
- Test coverage can be used as a tool during test development for covering all cases of a unit.
- However, test coverage data can be a false safety net.
  - Full line coverage doesn't mean you have covered all cases of valid input.
  - We don't have strict requirements for test coverage, as it doesn't necessarily yield the desired outcome.
- Unit tests should not test too large components, e.g. entire controller `Reconcile` functions.
  - If a function/component does many steps, it's probably better to split it up into multiple functions/components that can be unit tested individually
  - There might be special cases for very small `Reconcile` functions.
  - If there are a lot of edge cases, extract dedicated functions that cover them and use unit tests to test them.
  - Usual-sized controllers should rather be tested in integration tests.
  - Individual parts (e.g. helper functions) should still be tested in unit test for covering all cases, though.
- Unit tests are especially easy to run with a debugger and can help in understanding concrete behavior of components.

### Writing Unit Tests

- For the sake of execution speed, fake expensive calls/operations, e.g. secret generation: [example test](https://github.com/gardener/gardener/blob/b0de7db96ad436fe32c25daae5e8cb552dac351f/pkg/component/kubescheduler/kube_scheduler_suite_test.go#L32-L34)
- Generally, prefer fakes over mocks, e.g., use controller-runtime fake client over mock clients.
  - Mocks decrease maintainability because they expect the tested component to follow a certain way to reach the desired goal (e.g., call specific functions with particular arguments), [example consequence](https://github.com/gardener/gardener/pull/4027/commits/111aba2c8e306421f2fa6b27e5d8ed8b2fc52be9#diff-8e61507edf985df2625840a690115c43bca6c032f2ff818389633bd4365c3efdR293-R298)
  - Generally, fakes should be used in "result-oriented" test code (e.g., that a certain object was labelled, but the test doesn't care if it was via patch or update as both a valid ways to reach the desired goal).
  - Although rare, there are valid use cases for mocks, e.g. if the following aspects are important for correctness:
    - Asserting that an exact function is called
    - Asserting that functions are called in a specific order
    - Asserting that exact parameters/values/... are passed
    - Asserting that a certain function was not called
    - Many of these can also be verified with fakes, although mocks might be simpler
  - Only use mocks if the tested code directly calls the mock; never if the tested code only calls the mock indirectly (e.g., through a helper package/function).
  - Keep in mind the maintenance implications of using mocks:
    - Can you make a valid non-behavioral change in the code without breaking the test or dependent tests?
  - It's valid to mix fakes and mocks in the same test or between test cases.
- Generally, use the go test package, i.e., declare `package <production_package>_test`:
  - Helps in avoiding cyclic dependencies between production, test and helper packages
  - Also forces you to distinguish between the public (exported) API surface of your code and internal state that might not be of interest to tests
  - It might be valid to use the same package as the tested code if you want to test unexported functions.
    - Alternatively, an [`internal` package](https://go.dev/doc/go1.4#internalpackages) can be used to host "internal" helpers: [example package](https://github.com/gardener/gardener/tree/2eb54485231408cbdbabaa49812572a07124364f/pkg/client/kubernetes/clientmap)
  - Helpers can also be exported if no one is supposed to import the containing package (e.g. controller package).

## Integration Tests (envtests)

Integration tests in Gardener use the `sigs.k8s.io/controller-runtime/pkg/envtest` package.
It sets up a temporary control plane (etcd + kube-apiserver) and runs the test against it.
The test suites start their individual `envtest` environment before running the tested controller/webhook and executing test cases.
Before exiting, the test suites tear down the temporary test environment.

Package `github.com/gardener/gardener/test/envtest` augments the controller-runtime's `envtest` package by starting and registering `gardener-apiserver`.
This is used to test controllers that act on resources in the Gardener APIs (aggregated APIs).

Historically, [test machinery tests](#test-machinery-tests) have also been called "integration tests".
However, test machinery does not perform integration testing but rather executes a form of end-to-end tests against a real landscape.
Hence, we tried to sharpen the terminology that we use to distinguish between "real" integration tests and test machinery tests but you might still find "integration tests" referring to test machinery tests in old issues or outdated documents.

### Running Integration Tests

The `test-integration` make rule prepares the environment automatically by downloading the respective binaries (if not yet present) and setting the necessary environment variables.

```bash
make test-integration
```

If you want to run a specific set of integration tests, you can also execute them using `./hack/test-integration.sh` directly instead of using the `test-integration` rule. Prior to execution, the `PATH` environment variable needs to be set to also included the tools binary directory. For example:

```bash
export PATH="$PWD/hack/tools/bin/$(go env GOOS)-$(go env GOARCH):$PATH"

source ./hack/test-integration.env
./hack/test-integration.sh ./test/integration/resourcemanager/tokenrequestor
```

The script takes care of preparing the environment for you.
If you want to execute the test suites directly via `go test` or `ginkgo`, you have to point the `KUBEBUILDER_ASSETS` environment variable to the path that contains the etcd and kube-apiserver binaries. Alternatively, you can install the binaries to `/usr/local/kubebuilder/bin`. Additionally, the environment variables from `hack/test-integration.env` should be sourced.

### Debugging Integration Tests

You can configure `envtest` to use an existing cluster or control plane instead of starting a temporary control plane that is torn down immediately after executing the test.
This can be helpful for debugging integration tests because you can easily inspect what is going on in your test environment with `kubectl`.

While you can use an existing cluster (e.g., `kind`), some test suites expect that no controllers and no nodes are running in the test environment (as it is the case in `envtest` test environments).
Hence, using a full-blown cluster with controllers and nodes might sometimes be impractical, as you would need to stop cluster components for the tests to work.

You can use `make start-envtest` to start an `envtest` test environment that is managed separately from individual test suites.
This allows you to keep the test environment running for as long as you want, and to debug integration tests by executing multiple test runs in parallel or inspecting test runs using `kubectl`.
When you are finished, just hit `CTRL-C` for tearing down the test environment.
The kubeconfig for the test environment is placed in `dev/envtest-kubeconfig.yaml`.

`make start-envtest` brings up an `envtest` environment using the default configuration.
If your test suite requires a different control plane configuration (e.g., disabled admission plugins or enabled feature gates), feel free to locally modify the configuration in [`test/start-envtest`](../../test/start-envtest) while debugging.

Run an `envtest` suite (not using `gardener-apiserver`) against an existing test environment:

```bash
make start-envtest

# in another terminal session:
export KUBECONFIG=$PWD/dev/envtest-kubeconfig.yaml
export USE_EXISTING_CLUSTER=true

# run test with verbose output
./hack/test-integration.sh -v ./test/integration/resourcemanager/health -ginkgo.v

# in another terminal session:
export KUBECONFIG=$PWD/dev/envtest-kubeconfig.yaml
# watch test objects
k get managedresource -A -w
```

Run a `gardenerenvtest` suite (using `gardener-apiserver`) against an existing test environment:

```bash
# modify GardenerTestEnvironment{} in test/start-envtest to disable admission plugins and enable feature gates like in test suite...

make start-envtest ENVTEST_TYPE=gardener

# in another terminal session:
export KUBECONFIG=$PWD/dev/envtest-kubeconfig.yaml
export USE_EXISTING_GARDENER=true

# run test with verbose output
./hack/test-integration.sh -v ./test/integration/controllermanager/bastion -ginkgo.v

# in another terminal session:
export KUBECONFIG=$PWD/dev/envtest-kubeconfig.yaml
# watch test objects
k get bastion -A -w
```

Similar to [debugging unit tests](#debugging-unit-tests), the `stress` tool can help hunting flakes in integration tests.
Though, you might need to run less tests in parallel though (specified via `-p`) and have a bit more patience.
Generally, reproducing flakes in integration tests is easier when stress-testing against an existing test environment instead of starting temporary individual control planes per test run.

Stress-test an `envtest` suite (not using `gardener-apiserver`):

```bash
# build a test binary
ginkgo build ./test/integration/resourcemanager/health

# prepare a test environment to run the test against
make start-envtest

# in another terminal session:
export KUBECONFIG=$PWD/dev/envtest-kubeconfig.yaml
export USE_EXISTING_CLUSTER=true

# use same timeout settings like in CI
source ./hack/test-integration.env

# switch to test package directory like `go test`
cd ./test/integration/resourcemanager/health

# run the test in parallel and report any failures
stress -ignore "unable to grab random port" -p 16 ./health.test
...
```

Stress-test a `gardenerenvtest` suite (using `gardener-apiserver`):

```bash
# modify test/start-envtest to disable admission plugins and enable feature gates like in test suite...

# build a test binary
ginkgo build ./test/integration/controllermanager/bastion

# prepare a test environment including gardener-apiserver to run the test against
make start-envtest ENVTEST_TYPE=gardener

# in another terminal session:
export KUBECONFIG=$PWD/dev/envtest-kubeconfig.yaml
export USE_EXISTING_GARDENER=true

# use same timeout settings like in CI
source ./hack/test-integration.env

# switch to test package directory like `go test`
cd ./test/integration/controllermanager/bastion

# run the test in parallel and report any failures
stress -ignore "unable to grab random port" -p 16 ./bastion.test
...
```

### Purpose of Integration Tests

- Integration tests prove that multiple units are correctly integrated into a fully-functional component of the system.
- Example components with multiple units:
  - A controller with its reconciler, watches, predicates, event handlers, queues, etc.
  - A webhook with its server, handler, decoder, and webhook configuration.
- Integration tests set up a full component (including used libraries) and run it against a test environment close to the actual setup.
  - e.g., start controllers against a real Kubernetes control plane to catch bugs that can only happen when talking to a real API server.
  - Integration tests are generally more expensive to run (e.g., in terms of execution time).
- Integration tests should not cover each and every detailed case.
  - Rather than that, cover a good portion of the "usual" cases that components will face during normal operation (positive and negative test cases).
  - Also, there is no need to cover all failure cases or all cases of predicates -> they should be covered in unit tests already.
  - Generally, not supposed to "generate test coverage" but to provide confidence that components work well.
- As integration tests typically test only one component (or a cohesive set of components) isolated from others, they cannot catch bugs that occur when multiple controllers interact (could be discovered by e2e tests, though).
- Rule of thumb: a new integration tests should be added for each new controller (an integration test doesn't replace unit tests though).

### Writing Integration Tests

- Make sure to have a clean test environment on both test suite and test case level:
  - Set up dedicated test environments (envtest instances) per test suite.
  - Use dedicated namespaces per test suite:
    - Use `GenerateName` with a test-specific prefix: [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/secret/secret_suite_test.go#L94-L105)
    - Restrict the controller-runtime manager to the test namespace by setting `manager.Options.Namespace`: [example test](https://github.com/gardener/gardener/blob/d9b00b574182094c5d03ab16dfc2d20515e9b6ed/test/integration/controllermanager/cloudprofile/cloudprofile_suite_test.go#L104)
    - Alternatively, use a test-specific prefix with a random suffix determined upfront: [example test](https://github.com/gardener/gardener/blob/ae5cb871291bb46e5113c5a738c7458fe6141a81/test/integration/resourcemanager/seccompprofile/seccompprofile_suite_test.go#L70)
      - This can be used to restrict webhooks to a dedicated test namespace: [example test](https://github.com/gardener/gardener/blob/b60abf22a5a7d5d1382fdab4b2fba80372f0b9a2/test/integration/resourcemanager/endpointslicehints/endpointslicehints_suite_test.go#L75)
    - This allows running a test in parallel against the same existing cluster for deflaking and stress testing: [example PR](https://github.com/gardener/gardener/pull/5953)
  - If the controller works on cluster-scoped resources:
    - Label the resources with a label specific to the test run, e.g. the test namespace's name: [example test](https://github.com/gardener/gardener/blob/b01239edfd594b09ecd44dd77fba7a05a74820e8/test/integration/controllermanager/cloudprofile/cloudprofile_test.go#L38)
    - Restrict the manager's cache for these objects with a corresponding label selector: [example test](https://github.com/gardener/gardener/blob/b01239edfd594b09ecd44dd77fba7a05a74820e8/test/integration/controllermanager/cloudprofile/cloudprofile_suite_test.go#L110-L116)
    - Alternatively, use a checksum of a random UUID using `uuid.NewUUID()` function: [example test](https://github.com/gardener/gardener/blob/ae5cb871291bb46e5113c5a738c7458fe6141a81/test/integration/resourcemanager/seccompprofile/seccompprofile_suite_test.go#L69-L70)
    - This allows running a test in parallel against the same existing cluster for deflaking and stress testing, even if it works with cluster-scoped resources that are visible to all parallel test runs: [example PR](https://github.com/gardener/gardener/pull/6527)
  - Use dedicated test resources for each test case:
    - Use `GenerateName`: [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/health/health_test.go#L38-L48)
    - Alternatively, use a checksum of a random UUID using `uuid.NewUUID()` function: [example test](https://github.com/gardener/gardener/blob/ae5cb871291bb46e5113c5a738c7458fe6141a81/test/integration/resourcemanager/seccompprofile/seccompprofile_suite_test.go#L69-L70)
    - Logging the created object names is generally a good idea to support debugging failing or flaky tests: [example test](https://github.com/gardener/gardener/blob/50f92c5dc35160fe05da9002a79e7ce4a9cf3509/test/integration/controllermanager/cloudprofile/cloudprofile_test.go#L94-L96)
    - Always delete all resources after the test case (e.g., via `DeferCleanup`) that were created for the test case
    - This avoids conflicts between test cases and cascading failures which distract from the actual root failures
  - Don't tolerate already existing resources (~dirty test environment), code smell: ignoring already exist errors
- Don't use a cached client in test code (e.g., the one from a controller-runtime manager), always construct a dedicated test client (uncached): [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/managedresource/resource_suite_test.go#L96-L97)
- When creating/updating an object with `runtime.RawExtension` field against a real cluster (not fake or mocked client), pass the field definition in the `Raw` field of the `runtime.RawExtension`. The `Object` field of `runtime.RawExtension` doesn't have a protobuf tag, and the `Raw` field does, which allows it to be serialized.
- Use [asynchronous assertions](https://onsi.github.io/gomega/#making-asynchronous-assertions): `Eventually` and `Consistently`.
  - Never `Expect` anything to happen synchronously (immediately).
  - Don't use retry or wait until functions -> use `Eventually`, `Consistently` instead: [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/controllermanager/shootmaintenance/utils_test.go#L36-L48)
  - This allows to override the interval/timeout values from outside instead of hard-coding this in the test (see `hack/test-integration.sh`): [example PR](https://github.com/gardener/gardener/pull/5938#discussion_r869155906)
  - Beware of the default `Eventually` / `Consistently` timeouts / poll intervals: [docs](https://onsi.github.io/gomega/#eventually)
  - Don't set custom (high) timeouts and intervals in test code: [example PR](https://github.com/gardener/gardener/pull/4983)
    - iInstead, shorten sync period of controllers, overwrite intervals of the tested code, or use fake clocks: [example test](https://github.com/gardener/gardener/blob/7c4031a57836de20758f32e1015c8a0f6c754d0f/test/integration/resourcemanager/managedresource/resource_suite_test.go#L137-L139)
  - Pass `g Gomega` to `Eventually`/`Consistently` and use `g.Expect` in it: [docs](https://onsi.github.io/gomega/#category-3-making-assertions-eminem-the-function-passed-into-codeeventuallycode), [example test](https://github.com/gardener/gardener/blob/708f65c279276abd3a770c2f84a89e02876b3c38/test/e2e/shoot/internal/rotation/certificate_authorities.go#L111-L122), [example PR](https://github.com/gardener/gardener/pull/4936)
  - Don't forget to call `{Eventually,Consistently}.Should()`, otherwise the assertions always silently succeeds without errors: [onsi/gomega#561](https://github.com/onsi/gomega/issues/561)
- When using Gardener's envtest (`envtest.GardenerTestEnvironment`):
  - Disable gardener-apiserver's admission plugins that are not relevant to the integration test itself by passing `--disable-admission-plugins`: [example test](https://github.com/gardener/gardener/blob/50f92c5dc35160fe05da9002a79e7ce4a9cf3509/test/integration/controllermanager/shoot/maintenance/maintenance_suite_test.go#L61-L67)
  - This makes setup / teardown code simpler and ensures to only test code relevant to the tested component itself (but not the entire set of admission plugins)
  - e.g., you can disable the `ShootValidator` plugin to create `Shoots` that reference non-existing `SecretBindings` or disable the `DeletionConfirmation` plugin to delete Gardener resources without adding a deletion confirmation first.
- Use a custom rate limiter for controllers in integration tests: [example test](https://github.com/gardener/gardener/blob/3dd6b111d677eb4bceadaa8fc469877097660577/test/integration/controllermanager/exposureclass/exposureclass_suite_test.go#L130-L131)
  - This can be used for limiting exponential backoff to shorten wait times.
  - Otherwise, if using the default rate limiter, exponential backoff might exceed the timeout of `Eventually` calls and cause flakes.

## End-to-End (e2e) Tests (Using provider-local)

We run a suite of e2e tests on every pull request and periodically on the `master` branch.
It uses a [KinD cluster](https://kind.sigs.k8s.io/) and [skaffold](https://skaffold.dev/) to bootstrap a full installation of Gardener based on the current revision, including [provider-local](../extensions/provider-local.md).
This allows us to run e2e tests in an isolated test environment and fully locally without any infrastructure interaction.
The tests perform a set of operations on Shoot clusters, e.g. creating, deleting, hibernating and waking up.

These tests are executed in our prow instance at [prow.gardener.cloud](https://prow.gardener.cloud/), see [job definition](https://github.com/gardener/ci-infra/blob/e324cb79c39c013d7f253c33690b7fcc92c001d8/config/jobs/gardener/gardener-e2e-kind.yaml) and [job history](https://prow.gardener.cloud/?repo=gardener%2Fgardener&job=*gardener-e2e-kind).

### Running e2e Tests

You can also run these tests on your development machine, using the following commands:

```bash
make kind-up
export KUBECONFIG=$PWD/example/gardener-local/kind/local/kubeconfig
make gardener-up
make test-e2e-local  # alternatively: make test-e2e-local-simple
```

If you want to run a specific set of e2e test cases, you can also execute them using `./hack/test-e2e-local.sh` directly in combination with [ginkgo label filters](https://onsi.github.io/ginkgo/#spec-labels). For example:

```bash
./hack/test-e2e-local.sh --label-filter "Shoot && credentials-rotation" ./test/e2e/gardener/...
```

If you want to use an existing shoot instead of creating a new one for the test case and deleting it afterwards, you can specify the existing shoot via the following flags.
This can be useful to speed up the development of e2e tests.

```bash
./hack/test-e2e-local.sh --label-filter "Shoot && credentials-rotation" ./test/e2e/gardener/... -- --project-namespace=garden-local --existing-shoot-name=local
```

For more information, see [Developing Gardener Locally](getting_started_locally.md) and [Deploying Gardener Locally](../deployment/getting_started_locally.md).

### Debugging e2e Tests

When debugging e2e test failures in CI, logs of the cluster components can be very helpful.
Our e2e test jobs export logs of all containers running in the kind cluster to prow's artifacts storage.
You can find them by clicking the `Artifacts` link in the top bar in prow's job view and navigating to `artifacts`.
This directory will contain all cluster component logs grouped by node.

Pull all artifacts using [`gsutil`](https://cloud.google.com/storage/docs/gsutil) for searching and filtering the logs locally (use the path displayed in the artifacts view):

```bash
gsutil cp -r gs://gardener-prow/pr-logs/pull/gardener_gardener/6136/pull-gardener-e2e-kind/1542030416616099840/artifacts/gardener-local-control-plane /tmp
```

### Purpose of e2e Tests

- e2e tests provide a high level of confidence that our code runs as expected by users when deployed to production.
- They are supposed to catch bugs resulting from interaction between multiple components.
- Test cases should be as close as possible to real usage by end users:
  - You should test "from the perspective of the user" (or operator).
  - Example: I create a Shoot and expect to be able to connect to it via the provided kubeconfig.
  - Accordingly, don't assert details of the system.
    - e.g., the user also wouldn't expect that there is a kube-apiserver deployment in the seed, they rather expect that they can talk to it no matter how it is deployed
    - Only assert details of the system if the tested feature is not fully visible to the end-user and there is no other way of ensuring that the feature works reliably
    - e.g., the Shoot CA rotation is not fully visible to the user but is assertable by looking at the secrets in the Seed.
- Pro: can be executed by developers and users without any real infrastructure (provider-local).
- Con: they currently cannot be executed with real infrastructure (e.g., provider-aws), we will work on this as part of [#6016](https://github.com/gardener/gardener/issues/6016).
- Keep in mind that the tested scenario is still artificial in a sense of using default configuration, only a few objects, only a few config/settings combinations are covered.
  - We will never be able to cover the full "test matrix" and this should not be our goal.
  - Bugs will still be released and will still happen in production; we can't avoid it.
  - Instead, we should add test cases for preventing bugs in features or settings that were frequently regressed: [example PR](https://github.com/gardener/gardener/pull/5725)
- Usually e2e tests cover the "straight-forward cases".
  - However, negative test cases can also be included, especially if they are important from the user's perspective.

### Writing e2e Tests

- Tests must always use the `Ordered` decorator
- Separate individual steps from each other with `It` statements
  - This makes debugging of the test and flake detection much easier
  - Make sure to always utilize the `SpecContext` when dealing with contexts inside of `It` statements, [like here](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/shoot/create_delete_unprivileged.go#L52)
- Use the `TestContext` to store the current state of the test ([Reference](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/context.go#L30))
  -  Do not share test contexts between test cases
- Whenever possible, use the type-specific test contexts like: `ShootContext`, `SeedContext`
  - [Example Shoot](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/shoot/create_delete_unprivileged.go#L45)
  - [Example Seed](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/seed/renew_garden_access_secrets.go#L61)
- Common steps should be implemented in dedicated helper functions, which accept the `TestContext` or its type-specific derivatives
  - [Example](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/shoot/shoot.go#L53)
- Use `BeforeTestSetup` to initialize the `TestContext` or its type-specific derivatives
  - [Example](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/shoot/create_force-delete.go#L25)
- Always wrap API calls and similar things in `Eventually` blocks: [example test](https://github.com/gardener/gardener/blob/3206df77c64b3a7d4c899bce184b532a5bad6c96/test/e2e/gardener/shoot/create_delete_unprivileged.go#L76)
  - At this point, we are pretty much working with a distributed system and failures can happen anytime.
  - Wrapping calls in `Eventually` makes tests more stable and more realistic (usually, you wouldn't call the system broken if a single API call fails because of a short connectivity issue).
- Most of the points from [writing integration tests](#writing-integration-tests) are relevant for e2e tests as well (especially the points about asynchronous assertions).
- In contrast to integration tests, in e2e tests, it might make sense to specify higher timeouts for `Eventually` calls, e.g., when waiting for a `Shoot` to be reconciled.
  - Generally, try to use the default settings for `Eventually` specified via the environment variables.
  - Only set higher timeouts if waiting for long-running reconciliations to be finished.

## Gardener Upgrade Tests (Using provider-local)

Gardener upgrade tests setup a kind cluster and deploy Gardener version `vX.X.X` before upgrading it to a given version `vY.Y.Y`.

This allows verifying whether the current (unreleased) revision/branch (or a specific release) is compatible with the latest (or a specific other) release. The `GARDENER_PREVIOUS_RELEASE` and `GARDENER_NEXT_RELEASE` environment variables are used to specify the respective versions.

This helps understanding what happens or how the system reacts when Gardener upgrades from versions `vX.X.X` to `vY.Y.Y` for existing shoots in different states (`creation`/`hibernation`/`wakeup`/`deletion`). Gardener upgrade tests also help qualifying releases for all flavors (**non-HA** or **HA** with failure tolerance `node`/`zone`).

Just like E2E tests, upgrade tests also use a [KinD cluster](https://kind.sigs.k8s.io/) and [skaffold](https://skaffold.dev/) for bootstrapping a full Gardener installation based on the current revision/branch, including [provider-local](../extensions/provider-local.md).
This allows running e2e tests in an isolated test environment, fully locally without any infrastructure interaction.
The tests perform a set of operations on Shoot clusters, e.g. create, delete, hibernate and wake up.

Below is a sequence describing how the tests are performed.

- Create a `kind` cluster.
- Install Gardener version `vX.X.X`.
- Run gardener pre-upgrade tests which are labeled with `pre-upgrade`.
- Upgrade Gardener version from `vX.X.X` to `vY.Y.Y`.
- Run gardener post-upgrade tests which are labeled with `post-upgrade`
- Tear down seed and kind cluster.

### How to Run Upgrade Tests Between Two Gardener Releases

Sometimes, we need to verify/qualify two Gardener releases when we upgrade from one version to another.
This can performed by fetching the two Gardener versions from the  **[GitHub Gardener release page](https://github.com/gardener/gardener/releases/latest)** and setting appropriate env variables `GARDENER_PREVIOUS_RELEASE`, `GARDENER_NEXT_RELEASE`.

>**`GARDENER_PREVIOUS_RELEASE`** -- This env variable refers to a source revision/branch (or a specific release) which has to be installed first and then upgraded to version **`GARDENER_NEXT_RELEASE`**. By default, it fetches the latest release version from **[GitHub Gardener release page](https://github.com/gardener/gardener/releases/latest)**.

>**`GARDENER_NEXT_RELEASE`** -- This env variable refers to the target revision/branch (or a specific release) to be upgraded to after successful installation of **`GARDENER_PREVIOUS_RELEASE`**. By default, it considers the local HEAD revision, builds code, and installs Gardener from the current revision where the Gardener upgrade tests triggered.

- `make ci-e2e-kind-upgrade GARDENER_PREVIOUS_RELEASE=v1.60.0 GARDENER_NEXT_RELEASE=v1.61.0`
- `make ci-e2e-kind-ha-single-zone-upgrade GARDENER_PREVIOUS_RELEASE=v1.60.0 GARDENER_NEXT_RELEASE=v1.61.0`
- `make ci-e2e-kind-ha-multi-zone-upgrade GARDENER_PREVIOUS_RELEASE=v1.60.0 GARDENER_NEXT_RELEASE=v1.61.0`

### Purpose of Upgrade Tests

- Tests will ensure that shoot clusters reconciled with the previous version of Gardener work as expected even with the next Gardener version.
- This will reproduce or catch actual issues faced by end users.
- One of the test cases ensures no downtime is faced by the end-users for shoots while upgrading Gardener if the shoot's control-plane is configured as HA.

### Writing Upgrade Tests

- Tests are divided into two parts and labeled with `pre-upgrade` and `post-upgrade` labels.
- An example test case which ensures a shoot which was `hibernated` in a previous Gardener release should `wakeup` as expected in next release:
  - Creating a shoot and hibernating a shoot is pre-upgrade test case which should be labeled `pre-upgrade` label.
  - Then wakeup a shoot and delete a shoot is post-upgrade test case which should be labeled `post-upgrade` label.

## Test Machinery Tests

Please see [Test Machinery Tests](testmachinery_tests.md).

### Purpose of Test Machinery Tests

- Test machinery tests have to be executed against full-blown Gardener installations.
- They can provide a very high level of confidence that an installation is functional in its current state, this includes: all Gardener components, Extensions, the used Cloud Infrastructure, all relevant settings/configuration.
- This brings the following benefits:
  - They test more realistic scenarios than e2e tests (real configuration, real infrastructure, etc.).
  - Tests run "where the users are".
- However, this also brings significant drawbacks:
  - Tests are difficult to develop and maintain.
  - Tests require a full Gardener installation and cannot be executed in CI (on PR-level or against master).
  - Tests require real infrastructure (think cloud provider credentials, cost).
  - Using `TestDefinitions` under `.test-defs` requires a full test machinery installation.
  - Accordingly, tests are heavyweight and expensive to run.
  - Testing against real infrastructure can cause flakes sometimes (e.g., in outage situations).
  - Failures are hard to debug, because clusters are deleted after the test (for obvious cost reasons).
  - Bugs can only be caught, once it's "too late", i.e., when code is merged and deployed.
- Today, test machinery tests cover a bigger "test matrix" (e.g., Shoot creation across infrastructures, kubernetes versions, machine image versions).
- Test machinery also runs Kubernetes conformance tests.
- However, because of the listed drawbacks, we should rather focus on augmenting our e2e tests, as we can run them locally and in CI in order to catch bugs before they get merged.
- It's still a good idea to add test machinery tests if a feature that is depending on some installation-specific configuration needs to be tested.

### Writing Test Machinery Tests

- Generally speaking, most points from [writing integration tests](#writing-integration-tests) and [writing e2e tests](#writing-e2e-tests) apply here as well.
- However, test machinery tests contain a lot of technical debt and existing code doesn't follow these best practices.
- As test machinery tests are out of our general focus, we don't intend on reworking the tests soon or providing more guidance on how to write new ones.

## Manual Tests

- Manual tests can be useful when the cost of trying to automatically test certain functionality are too high.
- Useful for PR verification, if a reviewer wants to verify that all cases are properly tested by automated tests.
- Currently, it's the simplest option for testing upgrade scenarios.
  - e.g. migration coding is probably best tested manually, as it's a high effort to write an automated test for little benefit
- Obviously, the need for manual tests should be kept at a bare minimum.
  - Instead, we should add e2e tests wherever sensible/valuable.
  - We want to implement some form of general upgrade tests as part of [#6016](https://github.com/gardener/gardener/issues/6016).
