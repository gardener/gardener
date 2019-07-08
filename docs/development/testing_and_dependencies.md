# Testing

We follow the BDD-style testing principles and are leveraging the [Ginkgo](https://onsi.github.io/ginkgo/) framework along with [Gomega](http://onsi.github.io/gomega/) as matcher library. In order to execute the existing tests, you can use

```bash
$ make test         # runs tests
$ make verify       # runs static code checks and test
```

There is an additional command for analyzing the code coverage of the tests. Ginkgo will generate standard Golang cover profiles which will be translated into a HTML file by the [Go Cover Tool](https://blog.golang.org/cover). Another command helps you to clean up the filesystem from the temporary cover profile files and the HTML report:

```bash
$ make test-cov
$ open gardener.coverage.html
$ make test-clean
```

## Dependency management

We are using [go modules](https://github.com/golang/go/wiki/Modules) for depedency management.
In order to add a new package dependency to the project, you can perform `go get <PACKAGE>@<VERSION>` or edit the `go.mod` file and append the package along with the version you want to use.

### Updating dependencies

The `Makefile` contains a rule called `revendor` which performs `go mod vendor` and `go mod tidy`.
`go mod vendor` resets the main module's vendor directory to include all packages needed to build and test all the main module's packages. It does not include test code for vendored packages.
`go mod tidy` makes sure go.mod matches the source code in the module. It adds any missing modules necessary to build the current module's packages and dependencies, and it removes unused modules that don't provide any relevant packages.

```bash
$ make revendor
```

The dependencies are installed into the `vendor` folder which **should be added** to the VCS.

:warning: Make sure that you test the code after you have updated the dependencies!
