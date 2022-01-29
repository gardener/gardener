# Dependency Management

We are using [go modules](https://github.com/golang/go/wiki/Modules) for depedency management.
In order to add a new package dependency to the project, you can perform `go get <PACKAGE>@<VERSION>` or edit the `go.mod` file and append the package along with the version you want to use.

## Updating Dependencies

The `Makefile` contains a rule called `revendor` which performs `go mod vendor` and `go mod tidy`.
`go mod vendor` resets the main module's vendor directory to include all packages needed to build and test all the main module's packages. It does not include test code for vendored packages.
`go mod tidy` makes sure go.mod matches the source code in the module. It adds any missing modules necessary to build the current module's packages and dependencies, and it removes unused modules that don't provide any relevant packages.

```bash
make revendor
```

The dependencies are installed into the `vendor` folder which **should be added** to the VCS.

:warning: Make sure that you test the code after you have updated the dependencies!
