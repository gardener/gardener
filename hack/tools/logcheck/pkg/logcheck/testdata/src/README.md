# logcheck testdata

This directory contains go source files for testing the `logcheck` tool using [golang.org/x/tools/go/analysis/analysistest](https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest).

The following paths are symlinks to the respective `vendor` directory:

```
./github.com/go-logr/logr
./sigs.k8s.io/controller-runtime/pkg/log
```

This is done because `analysistest` expects test file dependencies to be placed in `testdata/src` as well under their package import paths.
Symlinks are used in order to test the `logcheck` tool against the currently used dependency versions of the main module. This prevents inconsistencies between the tool test files and the actually linted source files.
