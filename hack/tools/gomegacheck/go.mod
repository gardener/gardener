module github.com/gardener/gardener/hack/tools/gomegacheck

go 1.20

// This is a separate go module to decouple the gardener codebase and production binaries from dependencies that are
// only needed to build the gomegacheck tool
// this has to be kept in sync with the used golangci-lint version
// use go version -m golanci-lint to detect the dependency versions
require golang.org/x/tools v0.6.0

require (
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
)
