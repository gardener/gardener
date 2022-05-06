module github.com/gardener/gardener/hack/tools/logcheck

go 1.18

// This is a separate go module to decouple the gardener codebase and production binaries from dependencies that are
// only needed to build the logcheck tool
require (
	golang.org/x/exp v0.0.0-20220124173137-7a6bfc487013
	// this has to be kept in sync with the used golangci-lint version
	// use go version -m golanci-lint to detect the dependency versions
	golang.org/x/tools v0.1.10
)

require (
	golang.org/x/mod v0.6.0-dev.0.20220106191415-9b9b3d81d5e3 // indirect
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)
