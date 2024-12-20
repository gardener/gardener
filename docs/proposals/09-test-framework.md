# Gardener Integration Test Framework

## Motivation
As we want to improve our code coverage in the next months, we will need a simple and easy to use test framework.
The current testframework already contains a lot of general test functions that ease the work for writing new tests.
However, there are multiple disadvantages with the current structure of the tests and the testframework:
1. Every new test is an own testsuite and therefore needs its own [`TestDef`](../../.test-defs). With this approach there will be hundreds of test definitions, growing with every new test (or at least every new test suite).
  But in most cases, new tests do not need their own special `TestDef`: it's just the wrong scope for the testmachinery and will result in unnecessary complex testruns and configurations. In addition, it would result in additional maintenance for a huge number of `TestDefs`.
2. The testsuites currently have their own specific interface/configuration that they need in order to be executed correctly (see [K8s Update test](../../.test-defs/ShootKubernetesUpdateTest.yaml#L14)).
  Consequently, the configuration has to be defined in the testruns which result in one step per test with their very own configuration, which means that the testmachinery cannot simply select testdefinitions by label.
  As the testmachinery cannot make use of its ability to run labeled tests (e.g. run all tests labeled `default`), the testflow size increases with every new tests and the testruns have to be manually adjusted with every new test.
3. The current Gardener test framework contains multiple test operations where some are just used for specific tests (e.g. `plant_operations`) and some are more general (`garden_operation`). Also, the functions offered by the operations vary in their specialization as some are really specific to just one test, e.g. shoot test operation with `WaitUntilGuestbookAppIsAvailable`, whereas others are more general, like `WaitUntilPodIsRunning`.<br>
  This structure makes it hard for developers to find commonly used functions and also hard to integrate, as the common framework grows with specialized functions.

## Goals
In order to clean the testframework, make it easier for new developers to write tests and easier to add and maintain test execution within the testmachinery, the following goals are defined:
- Have a small number of test suites (Gardener shoots, see [test flavors](#test_flavors)) to only maintain a fixed number of testdefinitions.
- Use ginkgo test labels (inspired by the k8s e2e tests) to differentiate test behavior, test execution, and test importance.
- Use standardized configuration for all tests (differ depending on the test suite) but provide better tooling to dynamically read additional configuration from configuration files like the `cloudprofile`.
- Clean the testframework to only contain general functionality and keep specific functions inside the tests.


## Proposal
The proposed new test framework consists of the following changes to tackle the above described goals.
​
### Test Flavors
Reducing the number of test definitions is done by ​combining the current specified test suites into the following 3 general ones:
- _System test suite_
  - e.g. create-shoot, delete-shoot, hibernate
  - Need their own testdef because they have a special meaning in the context of the testmachinery
- _Gardener test suite_
  - e.g. RBAC, scheduler
  - All tests that only need a Gardener installation but no shoot cluster
  - Possible functions/environment:
    - New project for test suite (copy secret binding, cleanup)?
- _Shoot test suite_
  - e.g. shoot app, network
  - Test that require a running shoot
  - Possible functions:
    - Namespace per test
    - Cleanup of ns


As inspired by the k8s e2e tests, test labels are used to differentiate the tests by their behavior, their execution, and their importance.
Test labels means that tests are described using predefined labels in the test's text (e.g `ginkgo.It("[BETA] this is a test")`).
With this labeling strategy, it is also possible to see the test properties directly in the code and promoting a test can be done via a pullrequest and will then be automatically recognized by the testmachinery with the next release.

Using ginkgo focus to only run desired tests and combined testsuites, an example test definition will look like the following.
```yaml
apiVersion: testmachinery.sapcloud.io
kind: TestDefinition
metadata:
  name: gardener-beta-suite
spec:
  description: Test suite that runs all gardener tests that are labeled as beta
  activeDeadlineSeconds: 7200
  labels: ["gardener", "beta"]
​
  command: [bash, -c]
  args:
  - >-
    go test -timeout=0 ./test/integration/suite
    --v -ginkgo.v -ginkgo.progress -ginkgo.no-color
    -ginkgo.focus="[GARDENER] [BETA]"
```
Using this approach, the overall number of testsuites is then reduced to a fixed number (excluding the system steps) of `test suites * labelCombinations`.

### Framework
The new framework will consist of a common framework, a Gardener framework (integrating the common framework), and a shoot framework (integrating the Gardener framework).

All of these frameworks will have their own configuration that is exposed via commandline flags so that, for example, the shoot test framework can be executed by `go test -timeout=0 ./test/integration/suite --v -ginkgo.v -ginkgo.focus="[SHOOT]" --kubecfg=/path/to/config --shoot-name=xx`.

The available test labels should be declared in the code with predefined values and in a predefined order, so that everyone is aware about possible labels and the tests are labeled similarly across all integration tests. This approach is somehow similar to what Kubernetes is doing in their e2e test suite but with some more restrictions (compare [example k8s e2e test](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/apps/deployment.go#L84)).<br>
A possible solution to have consistent labeling would be to define them with every new `ginkgo.It` definition: `f.Beta().Flaky().It("my test")`, which internally orders them and would produce a ginkgo test with the text: `[BETA] [FLAKY] my test`.


**General Functions**
The test framework should include some general functions that can and will be reused by every test.
These general functions may include:
​
- Logging
- State Dump
- Detailed test output (status, duration, etc..)
- Cleanup handling per test (`It`)
- General easy to use functions like `WaitUntilDeploymentCompleted`, `GetLogs`, `ExecCommand`, `AvailableCloudprofiles`, etc..
​
#### Example
A possible test with the new test framework would look like:
```go
var _ = ginkgo.Describe("Shoot network testing", func() {
  // the testframework registers some cleanup handling for a state dump on failure and maybe cleanup of created namespaces
  f := framework.NewShootFramework()
  f.CAfterEach(func(ctx context.Context) {
    ginkgo.By("cleanup network test daemonset")
    err := f.ShootClient.Client().Delete(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
    if err != nil {
      if !apierrors.IsNotFound(err) {
        Expect(err).To(HaveOccurred())
      }
    }
  }, FinalizationTimeout)
  f.Release().Default().CIt("should reach all webservers on all nodes", func(ctx context.Context) {
    ginkgo.By("Deploy the net test daemon set")
    templateFilepath := filepath.Join(f.ResourcesDir, "templates", nginxTemplateName)
    err := f.RenderAndDeployTemplate(f.Namespace(), templateFilepath)
    Expect(err).ToNot(HaveOccurred())
    err = f.WaitUntilDaemonSetIsRunning(ctx, f.ShootClient.Client(), name, namespace)
    Expect(err).NotTo(HaveOccurred())
    pods := &corev1.PodList{}
    err = f.ShootClient.Client().List(ctx, pods, client.MatchingLabels{"app": "net-nginx"})
    Expect(err).NotTo(HaveOccurred())
    // check if all webservers can be reached from all nodes
    ginkgo.By("test connectivity to webservers")
    shootRESTConfig := f.ShootClient.RESTConfig()
    var res error
    for _, from := range pods.Items {
      for _, to := range pods.Items {
        // test pods
        f.Logger.Infof("%s to %s: %s", from.GetName(), to.GetName(), data)
      }
    }
    Expect(res).ToNot(HaveOccurred())
  }, NetworkTestTimeout)
})
```

## Future Plans

### Ownership
When the test coverage is increased and there will be more tests, we will need to track ownership for tests.
At the beginning, the ownership will be shared across all maintainers of the residing repository but this is not suitable anymore as tests will grow and get more complex.

Therefore, the test ownership should be tracked via subgroups (in Kubernetes this would be a SIG (comp. [sig apps e2e test](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/apps/framework.go#L22))). These subgroups will then be tracked via labels and the members of these groups will then be notified if tests fail.