module github.com/gardener/gardener/hack/tools/logcheck

// Version must be kept in sync with Go version of https://github.com/golangci/golangci-lint.
go 1.22.1

// This is a separate go module to decouple the gardener codebase and production binaries from dependencies that are
// only needed to build the logcheck tool
require (
	golang.org/x/exp v0.0.0-20240103183307-be819d1f06fc
	// this has to be kept in sync with the used golangci-lint version
	// use go version -m hack/tools/bin/<<architecture>>/golangci-lint to detect the dependency versions
	golang.org/x/tools v0.27.0
)

require (
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/sync v0.9.0 // indirect
)
