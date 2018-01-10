# Testing
We follow the BDD-style testing principles and are leveraging the [Ginkgo](https://onsi.github.io/ginkgo/) framework along with [Gomega](http://onsi.github.io/gomega/) as matcher library. In order to execute the existing tests, you can use

```bash
$ make test
```

There is an additional command for analyzing the code coverage of the tests. Ginkgo will generate standard Golang cover profiles which will be translated into a HTML file by the [Go Cover Tool](https://blog.golang.org/cover). Another command helps you to clean up the filesystem from the temporary cover profile files and the HTML report:

```bash
$ make test-cov
$ open gardener.coverage.html
$ make test-clean
```

## Dependency management
We are using [Dep](https://github.com/golang/dep) as depedency management tool.
In order to add a new package dependency to the project, you can perform `dep ensure -add <PACKAGE>` or edit the `Gopkg.toml` file and append the package along with the version you want to use as a new `[[constraint]]`.

### Updating dependencies
The `Makefile` contains a rule called `revendor` which performs a `dep ensure -update` and a `dep prune` command. This updates all the dependencies to its latest versions (respecting the constraints specified in the `Gopkg.toml` file). The command also installs the packages which do not yet exist in the `vendor` folder but are specified in the `Gopkg.toml` (in case you have added new ones).

```bash
$ make revendor
```

The depencendies are installed into the `vendor` folder which **should be added** to the VCS.

:warning: Make sure that you test the code after you have updated the dependencies!
