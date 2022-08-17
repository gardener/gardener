module github.com/gardener/gardener/hack/tools/logcheck

go 1.18

// This is a separate go module to decouple the gardener codebase and production binaries from dependencies that are
// only needed to build the logcheck tool
require (
	golang.org/x/exp v0.0.0-20220124173137-7a6bfc487013
	// this has to be kept in sync with the used golangci-lint version
	// use go version -m golanci-lint to detect the dependency versions
	golang.org/x/tools v0.1.12
)

require (
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
)
