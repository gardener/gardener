module github.com/gardener/gardener/hack/tools/logcheck

// Version must be kept in sync with Go version of https://github.com/golangci/golangci-lint.
go 1.26.0

// This is a separate go module to decouple the gardener codebase and production binaries from dependencies that are
// only needed to build the logcheck tool
require (
	golang.org/x/exp v0.0.0-20260527015227-08cc5374adb3
	// this has to be kept in sync with the used golangci-lint version
	// use go version -m hack/tools/bin/<<architecture>>/golangci-lint to detect the dependency versions
	// or, with `go work` enabled, use `go work sync`
	golang.org/x/tools v0.46.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
)
