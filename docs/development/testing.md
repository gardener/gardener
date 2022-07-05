# Testing Strategy and Developer Guideline

This document walks you through

- what kind of tests we have in Gardener
- how to run each of them
- what purpose each kind of test serves
- how to best write tests that are correct, stable, fast and maintainable
- how to debug tests that are not working as expected

The document is aimed towards developers that want to contribute code and need to write tests, as well as maintainers and reviewers that review test code.
It serves as a common guide that we commit to follow in our project to ensure consistency in our tests, good coverage for high confidence and good maintainability.

The guidelines are not meant to be absolute rules.
Always apply common sense and adapt the guideline if it doesn't make much sense for some cases.
If in doubt, don't hesitate to ask questions during PR review (as author but also as reviewer).
Add new learnings as soon as we make them!

Generally speaking, **tests are a strict requirement for contributing new code**.
If you touch code that is currently untested, you need to add tests for the new cases that you introduce as a minimum.
Ideally though, you would add the missing test cases for the current code as well (**boy scout rule** -- "always leave the campground cleaner than you found it").

## Writing Tests (Relevant for All Kinds)

- we follow BDD-style testing principles and use [Ginkgo](https://onsi.github.io/ginkgo/) along with [Gomega](http://onsi.github.io/gomega/)
  - make sure to check out their extensive guides for more information and how to best leverage all of their features
- use `By` to structure test cases with multiple steps, so that steps are easy to follow in the logs: [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/pkg/client/kubernetes/clientmap/internal/generic_clientmap_test.go#L122-L138)
- call `defer GinkgoRecover()` if making assertions in goroutines: [doc](https://pkg.go.dev/github.com/onsi/ginkgo#GinkgoRecover), [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/scheduler/scheduler_test.go#L65-L68)
- use `DeferCleanup` instead of cleaning up manually (or use custom coding from the test framework): [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/resourcemanager/health/health_suite_test.go#L102-L105), [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/resourcemanager/health/health_test.go#L385-L390)
  - `DeferCleanup` makes sure to run the cleanup code in the right point in time, e.g., a `DeferCleanup` added in `BeforeEach` is executed with `AfterEach`
- test failures should point to an exact location, so that failures in CI aren't too difficult to debug/fix
  - use `ExpectWithOffset` for making assertions in helper funcs like `expectSomethingWasCreated`: [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/extensions/pkg/controller/controlplane/genericactuator/actuator_test.go#L732-L736)
  - make sure to add additional descriptions to Gomega matchers if necessary (e.g. in a loop): [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/e2e/shoot/internal/rotation/certificate_authorities.go#L89-L93)
- introduce helper functions for assertions to make test more readable where applicable: [example test](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/test/integration/gardenlet/shootsecret/controller_test.go#L323-L331)
- introduce custom matchers to make tests more readable where applicable: [example matcher](https://github.com/gardener/gardener/blob/2eb54485231408cbdbabaa49812572a07124364f/pkg/utils/test/matchers/matchers.go#L51-L57)
- don't rely on accurate timing of `time.Sleep` and friends
  - if doing so, CPU throttling in CI will make tests flaky, [example flake](https://github.com/gardener/gardener/issues/5410)
  - use fake clocks instead, [example PR](https://github.com/gardener/gardener/pull/4569) 
- use the same client schemes that are also used by production code to avoid subtle bugs/regressions: [example PR](https://github.com/gardener/gardener/pull/5469/commits/b0d9e2baeaee246c6eb466a181df35adca4a6ada)
- make sure, your test is actually asserting the right thing and it doesn't pass if the exact bug is introduced that you want to prevent
  - use specific error matchers instead of asserting any error has happened, make sure that the corresponding branch in the code is tested, e.g., prefer
    ```go
    Expect(err).To(MatchError("foo"))
    ```
    over
    ```go
    Expect(err).To(HaveOccurred())
    ```
  - if you're unsure about your test's behavior, attaching the debugger can sometimes be helpful to make sure your test is correct
- about overwriting global variables
  - this is a common pattern (or hack?) in go for faking calls to external functions
  - however, this can lead to races, when the global variable is used from a goroutine (e.g., the function is called)
  - alternatively, set fields on structs (passed via parameter or set directly): this is not racy, as struct values are typically (and should be) only used for a single test case
  - alternative to dealing with function variables and fields:
    - add an interface, which your code depends on
    - write a fake and a real implementation (similar to `clock.Clock.Sleep`)
    - the real implementation calls the actual function (`clock.RealClock.Sleep` calls `time.Sleep`)
    - the fake implementation does whatever you want it to do for your test (`clock.FakeClock.Sleep` waits until the test code advanced the time)
- use constants in test code with care
  - typically, you should not use constants from the same package as the tested code, instead use literals
  - if the constant value is changed, tests using the constant will still pass, although the "specification" is not fulfilled anymore
  - there are cases where it's fine to use constants, but keep this caveat in mind when doing so
- creating sample data for tests can be a high effort
  - if valuable, add a package for generating common sample data, e.g. Shoot/Cluster objects
- make use of the `testdata` directory for storing arbitrary sample data needed by tests (helm charts, YAML manifests, etc.), [example PR](https://github.com/gardener/gardener/pull/2140)
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
# run with same settings like in CI (race dector, timeout, ...)
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
$ stress -p 16 ./pkg/resourcemanager/controller/garbagecollector/garbagecollector.test -ginkgo.focus "should delete the unused resources"
5s: 1077 runs so far, 0 failures
10s: 2160 runs so far, 0 failures
```

`stress` will output a path to a file containing the full failure message, when a test run fails.

### Purpose of Unit Tests

- unit tests prove correctness of a single unit according to the specification of its interface
  - think: is the unit that I introduced doing what it is supposed to do for all cases?
- unit tests protect against regressions caused by adding new functionality to or refactoring of a single unit
  - think: is the unit that was introduced earlier (by someone else) and that I changed still doing what it was supposed to do for all cases?
- example units: functions (conversion, defaulting, validation, helpers), structs (helpers, basic building blocks like the Secrets Manager), predicates, event handlers
- for these purposes, unit tests need to cover all important cases of input for a single unit and cover edge cases / negative paths as well (e.g., errors)
  - because of the possible high dimensionality of test input, unit tests need to be fast to execute: individual test cases should not take more than a few seconds, test suites not more than 2 minutes
  - fuzzing can be used as a technique in addition to usual test cases for covering edge cases
- test coverage can be used as a tool during test development for covering all cases of a unit
- however, test coverage data can be a false safety net
  - full line coverage doesn't mean you have covered all cases of valid input
  - we don't have strict requirements for test coverage, as it doesn't necessarily yield the desired outcome
- unit tests should not test too large components, e.g. entire controller `Reconcile` functions
  - if a function/component does many steps, it's probably better to split it up into multiple functions/components that can be unit tested individually
  - there might be special cases for very small `Reconcile` functions
  - if there are a lot of edge cases, extract dedicated functions that cover them and use unit tests to test them
  - usual-sized controllers should rather be tested in integration tests
  - individual parts (e.g. helper functions) should still be tested in unit test for covering all cases, though
- unit tests are especially easy to run with a debugger and can help in understanding concrete behavior of components

### Writing Unit Tests

- for the sake of execution speed, fake expensive calls/operations, e.g. secret generation: [example test](https://github.com/gardener/gardener/blob/efcc0a9146d3558253b95071f2c652663f916d92/pkg/operation/botanist/component/kubescheduler/kube_scheduler_suite_test.go#L32-L34)
- generally, prefer fakes over mocks, e.g., use controller-runtime fake client over mock clients
  - mocks decrease maintainability because they expect the tested component to follow a certain way to reach the desired goal (e.g., call specific functions with particular arguments), [example consequence](https://github.com/gardener/gardener/pull/4027/commits/111aba2c8e306421f2fa6b27e5d8ed8b2fc52be9#diff-8e61507edf985df2625840a690115c43bca6c032f2ff818389633bd4365c3efdR293-R298)
  - generally, fakes should be used in "result-oriented" test code (e.g., that a certain object was labelled, but the test doesn't care if it was via patch or update as both a valid ways to reach the desired goal)
  - although rare, there are valid use cases for mocks, e.g. if the following aspects are important for correctness:
    - asserting that an exact function is called
    - asserting that functions are called in a specific order
    - asserting that exact parameters/values/... are passed
    - asserting that a certain function was not called
    - many of these can also be verified with fakes, although mocks might be simpler
  - only use mocks if the tested code directly calls the mock; never if the tested code only calls the mock indirectly (e.g., through a helper package/function)
  - keep in mind the maintenance implications of using mocks:
    - can you make a valid non-behavioral change in the code without breaking the test or dependent tests?
  - it's valid to mix fakes and mocks in the same test or between test cases
- generally, use the go test package, i.e., declare `package <production_package>_test`
  - helps in avoiding cyclic dependencies between production, test and helper packages
  - also forces you to distinguish between the public (exported) API surface of your code and internal state that might not be of interest to tests
  - it might be valid to use the same package as the tested code if you want to test unexported functions
    - alternatively, an [`internal` package](https://go.dev/doc/go1.4#internalpackages) can be used to host "internal" helpers: [example package](https://github.com/gardener/gardener/tree/2eb54485231408cbdbabaa49812572a07124364f/pkg/client/kubernetes/clientmap)
  - helpers can also be exported if no one is supposed to import the containing package (e.g. controller package)

## Integration Tests (envtests)

Integration tests in Gardener use the `sigs.k8s.io/controller-runtime/pkg/envtest` package.
It sets up a temporary control plane (etcd + kube-apiserver) and runs the test against it.

Historically, [test machinery tests](#test-machinery-tests) have also been called "integration tests".
However, test machinery does not perform integration testing but rather executes a form of end-to-end tests against a real landscape.
Hence, we tried to sharpen the terminology that we use to distinguish between "real" integration tests and test machinery tests but you might still find "integration tests" referring to test machinery tests in old issues or outdated documents.

### Running Integration Tests

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
This can be helpful for debugging integration tests, because you can easily inspect what is going on in your test cluster with `kubectl`.
For example:

```bash
make kind-up
export KUBECONFIG=$PWD/example/gardener-local/kind/kubeconfig
export USE_EXISTING_CLUSTER=true

# run test with verbose output
./hack/test-integration.sh -v ./test/integration/resourcemanager/health -ginkgo.v

# watch test objects
k get managedresource -A -w
```

Similar to [debugging unit tests](#debugging-unit-tests), the `stress` tool can help hunting flakes in integration tests.
Though, you might need to run less tests in parallel though (specified via `-p`) and have a bit more patience.

### Purpose of Integration Tests

- integration tests prove that multiple units are correctly integrated into a fully-functional component of the system
- example component with multiple units: a controller with its reconciler, watches, predicates, event handlers, queues, etc.
- integration tests set up a full component (including used libraries) and run it against a test environment close to the actual setup
  - e.g., start controllers against a real Kubernetes control plane to catch bugs that can only happen when talking to a real API server
  - integration tests are generally more expensive to run (e.g., in terms of execution time)
- integration tests should not cover each and every detailed case
  - rather cover a good portion of the "usual" cases that components will face during normal operation (positive and negative test cases)
  - but don't cover all failure cases or all cases of predicates -> they should be covered in unit tests already
  - generally, not supposed to "generate test coverage" but to provide confidence that components work well
- as integration tests typically test only one component (or a cohesive set of components) isolated from others, they cannot catch bugs that occur when multiple controllers interact (could be discovered by e2e tests, though)
- rule of thumb: a new integration tests should be added for each new controller (an integration test doesn't replace unit tests though)

### Writing Integration Tests

- make sure to have a clean test environment on both test suite and test case level:
  - set up dedicated test environments (envtest instances) per test suite
  - use dedicated namespaces per test suite, use `GenerateName` with a test-specific prefix: [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/secret/secret_suite_test.go#L94-L105)
    - this allows running a test in parallel against the same existing cluster for deflaking and stress testing: [example PR](https://github.com/gardener/gardener/pull/5953)
  - use dedicated test resources for each test case, use `GenerateName` ([example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/health/health_test.go#L38-L48)) or checksum of `CurrentSpecReport().LeafNodeLocation.String()` ([example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/managedresource/resource_test.go#L61-L67))
    - this avoids cascading failures of test cases and distracting from the actual root failure
  - don't tolerate already existing resources (~dirty test environment), code smell: ignoring already exist errors
- don't use a cached client in test code (e.g., the one from a controller-runtime manager), always construct a dedicated test client (uncached): [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/resourcemanager/managedresource/resource_suite_test.go#L96-L97)
- use [asynchronous assertions](https://onsi.github.io/gomega/#making-asynchronous-assertions): `Eventually` and `Consistently`
  - never `Expect` anything to happen synchronously (immediately)
  - don't use retry or wait until functions -> use `Eventually`, `Consistently` instead: [example test](https://github.com/gardener/gardener/blob/ee3e50387fc7e6298908242f59894a7ea6f91fa7/test/integration/controllermanager/shootmaintenance/utils_test.go#L36-L48)
  - this allows to override the interval/timeout values from outside instead of hard-coding this in the test (see `hack/test-integration.sh`): [example PR](https://github.com/gardener/gardener/pull/5938#discussion_r869155906)
  - beware of the default `Eventually` / `Consistently` timeouts / poll intervals: [docs](https://onsi.github.io/gomega/#eventually)
  - don't set custom (high) timeouts and intervals in test code: [example PR](https://github.com/gardener/gardener/pull/4983)
    - instead, shorten sync period of controllers, overwrite intervals of the tested code, or use fake clocks: [example test](https://github.com/gardener/gardener/blob/7c4031a57836de20758f32e1015c8a0f6c754d0f/test/integration/resourcemanager/managedresource/resource_suite_test.go#L137-L139)
  - pass `g Gomega` to `Eventually`/`Consistently` and use `g.Expect` in it: [docs](https://onsi.github.io/gomega/#category-3-making-assertions-eminem-the-function-passed-into-codeeventuallycode), [example test](https://github.com/gardener/gardener/blob/708f65c279276abd3a770c2f84a89e02876b3c38/test/e2e/shoot/internal/rotation/certificate_authorities.go#L111-L122), [example PR](https://github.com/gardener/gardener/pull/4936)
  - don't forget to call `{Eventually,Consistently}.Should()`, otherwise the assertions always silently succeeds without errors: [onsi/gomega#561](https://github.com/onsi/gomega/issues/561)

## End-to-end (e2e) Tests (using provider-local)

We run a suite of e2e tests on every pull request and periodically on the `master` branch.
It uses a [KinD cluster](https://kind.sigs.k8s.io/) and [skaffold](https://skaffold.dev/) to boostrap a full installation of Gardener based on the current revision, including [provider-local](../extensions/provider-local.md).
This allows us to run e2e tests in an isolated test environment and fully locally without any infrastructure interaction.
The tests perform a set of operations on Shoot clusters, e.g. creating, deleting, hibernating and waking up.

These tests are executed in our prow instance at [prow.gardener.cloud](https://prow.gardener.cloud/), see [job definition](https://github.com/gardener/ci-infra/blob/e324cb79c39c013d7f253c33690b7fcc92c001d8/config/jobs/gardener/gardener-e2e-kind.yaml) and [job history](https://prow.gardener.cloud/?repo=gardener%2Fgardener&job=*gardener-e2e-kind).

### Running e2e Tests

You can also run these tests on your development machine, using the following commands:

```bash
make kind-up
export KUBECONFIG=$PWD/example/gardener-local/kind/kubeconfig
make gardener-up
make test-e2e-local  # alternatively: make test-e2e-local-simple
```

If you want to run a specific set of e2e test cases, you can also execute them using `./hack/test-e2e-local.sh` directly in combination with [ginkgo label filters](https://onsi.github.io/ginkgo/#spec-labels). For example:

```bash
./hack/test-e2e-local.sh --label-filter "Shoot && credentials-rotation"
```

If you want to use an existing shoot instead of creating a new one for the test case and deleting it afterwards, you can specify the existing shoot via the following flags.
This can be useful to speed of the development of e2e tests.

```bash
./hack/test-e2e-local.sh --label-filter "Shoot && credentials-rotation" -- --project-namespace=garden-local --existing-shoot-name=local
```

Also see: [developing Gardener locally](getting_started_locally.md) and [deploying Gardener locally](../deployment/getting_started_locally.md).

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

- e2e tests provide a high level of confidence that our code runs as expected by users when deployed to production
- they are supposed to catch bugs resulting from interaction between multiple components
- test cases should be as close as possible to real usage by endusers
  - should test "from the perspective of the user" (or operator)
  - example: I create a Shoot and expect to be able to connect to it via the provided kubeconfig
  - accordingly, don't assert details of the system
    - e.g., the user also wouldn't expect that there is a kube-apiserver deployment in the seed, they rather expect that they can talk to it no matter how it is deployed
    - only assert details of the system if the tested feature is not fully visible to the end-user and there is no other way of ensuring that the feature works reliably
    - e.g., the Shoot CA rotation is not fully visible to the user but is assertable by looking at the secrets in the Seed.
- pro: can be executed by developers and users without any real infrastructure (provider-local)
- con: they currently cannot be executed with real infrastructure (e.g., provider-aws), we will work on this as part of [#6016](https://github.com/gardener/gardener/issues/6016)
- keep in mind that the tested scenario is still artificial in a sense of using default configuration, only a few objects, only a few config/settings combinations are covered
  - we will never be able to cover the full "test matrix" and this should not be our goal
  - bugs will still be released and will still happen in production; we can't avoid it
  - instead, we should add test cases for preventing bugs in features or settings that were frequently regressed: [example PR](https://github.com/gardener/gardener/pull/5725)
- usually e2e tests cover the "straight-forward cases"
  - however, negative test cases can also be included, especially if they are important from the user's perspective

### Writing e2e Tests

- always wrap API calls and similar things in `Eventually` blocks: [example test](https://github.com/gardener/gardener/blob/a66b8ec47995561393bf1ad9a817463089a0255e/test/e2e/shoot/internal/rotation/observability.go#L46-L55)
  - at this point, we are pretty much working with a distributed system and failures can happen anytime
  - wrapping calls in `Eventually` makes tests more stable and more realistic (usually, you wouldn't call the system broken if a single API call fails because of a short connectivity issue)
- most of the points from [writing integration tests](#writing-integration-tests) are relevant for e2e tests as well (especially the points about asynchronous assertions)

## Test Machinery Tests

Please see [Test Machinery Tests](testmachinery_tests.md).

### Purpose of Test Machinery Tests

- test machinery tests have to be executed against full-blown Gardener installations
- they can provide a very high level of confidence that an installation is functional in its current state, this includes: all Gardener components, Extensions, the used Cloud Infrastructure, all relevant settings/configuration
- this brings the following benefits:
  - they test more realistic scenarios than e2e tests (real configuration, real infrastructure, etc.)
  - tests run "where the users are"
- however, this also brings significant drawbacks:
  - tests are difficult to develop and maintain
  - tests require a full Gardener installation and cannot be executed in CI (on PR-level or against master)
  - tests require real infrastructure (think cloud provider credentials, cost)
  - using `TestDefinitions` under `.test-defs` requires a full test machinery installation
  - accordingly, tests are heavyweight and expensive to run
  - testing against real infrastructure can cause flakes sometimes (e.g., in outage situations)
  - failures are hard to debug, because clusters are deleted after the test (for obvious cost reasons)
  - bugs can only be caught, once it's "too late", i.e., when code is merged and deployed
- today, test machinery tests cover a bigger "test matrix" (e.g., Shoot creation across infrastructures, kubernetes versions, machine image versions, etc.)
- test machinery also runs Kubernetes conformance tests
- however, because of the listed drawbacks, we should rather focus on augmenting our e2e tests, as we can run them locally and in CI in order to catch bugs before they get merged
- it's still a good idea to add test machinery tests if a feature needs to be tested that is depending on some installation-specific configuration

### Writing Test Machinery Tests

- generally speaking, most points from [writing integration tests](#writing-integration-tests) and [writing e2e tests](#writing-e2e-tests) apply here as well
- however, test machinery tests contain a lot of technical debt and existing code doesn't follow these best practices
- as test machinery tests are out of our general focus, we don't intend on reworking the tests soon or providing more guidance on how to write new ones

## Manual Tests

- manual tests can be useful when the cost of trying to automatically test certain functionality are too high
- useful for PR verification, if a reviewer wants to verify that all cases are properly tested by automated tests
- currently, it's the simplest option for testing upgrade scenarios
  - e.g. migration coding is probably best tested manually, as it's a high effort to write an automated test for little benefit
- obviously, the need for manual tests should be kept at a bare minimum
  - instead, we should add e2e tests wherever sensible/valuable
  - we want to implement some form of general upgrade tests as part of [#6016](https://github.com/gardener/gardener/issues/6016)
