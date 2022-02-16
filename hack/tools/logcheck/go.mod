module github.com/gardener/gardener/hack/tools/logcheck

go 1.16

// This is a separate go module to decouple the gardener codebase and production binaries from dependencies that are
// only needed to build the logcheck tool
require (
	golang.org/x/exp v0.0.0-20220124173137-7a6bfc487013
	// this has to be kept in sync with the used golangci-lint version
	// use go version -m golanci-lint to detect the dependency versions
	golang.org/x/tools v0.1.9-0.20211228192929-ee1ca4ffc4da
)
