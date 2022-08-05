# `gomegacheck` tool

## Description and Motivation

This directory contains a tool for checking test assertions using [gomega](https://github.com/onsi/gomega).

It checks that `Expect`, `Eventually`, `Consistently` and friends have a corresponding `Should`, `ShouldNot` (or similar) call.
It's easy to forget `.Should(Succeed())` for these assertions, see [onsi/gomega#561](https://github.com/onsi/gomega/issues/561).
Forgetting these calls is a dangerous trap, as it basically makes the test code itself erroneous.
The tool also checks that only a single `GomegaMatcher` is passed to `Should` and friends.

As we have seen this happening in a lot of tests in our project, we have added a tool to prevent such programmer-level errors.

You can also check out our [testing guideline](../../../docs/development/testing.md).

## Implementation Details

The tool is implemented using the [`golang.org/x/tools/go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis) package, which is a "linter framework" used by `go vet` and `golangci-lint` as well.
If you are looking for a better understanding of how the tool is implemented, you can take a look at [this tutorial](https://disaev.me/p/writing-useful-go-analysis-linter/) on writing custom linters, which explains all the fundamental building blocks.

## Installation und Usage

There are two options for installing and using the `gomegacheck` tool:

- standalone binary: simple, but limited
- [`golangci-lint`](https://golangci-lint.run/) plugin (recommended): specific build requirements, but comes with caching, nice output, and flexible configuration options (e.g., excludes and `//nolint` comments) out of the box

### Standalone Binary

Install the tool with:

```bash
go install github.com/gardener/gardener/hack/tools/gomegacheck
```

Use it with:
```bash
gomegacheck package [package...]
```

### [`golangci-lint`](https://golangci-lint.run/) Plugin (Recommended)

[`golangci-lint`](https://golangci-lint.run/) features a mechanism for loading [custom linters](https://golangci-lint.run/contributing/new-linters/#how-to-add-a-private-linter-to-golangci-lint) using Go's plugin library.

Build the plugin with:

```bash
CGO_ENABLED=1 go build -o gomegacheck.so -buildmode=plugin github.com/gardener/gardener/hack/tools/gomegacheck/plugin
```

> ⚠️ Both the `golangci-lint` binary and the `gomegacheck` plugin have to be built with `CGO_ENABLED=1` for the plugin mechanism to work. See [golangci/golangci-lint#1276](https://github.com/golangci/golangci-lint/issues/1276#issuecomment-665903858) for more details.
> This means, you can't use released binaries from GitHub or Homebrew for example, but have to build the binary yourself.

Load the plugin by adding the following to your `.golangci.yaml` configuration file:

```yaml
linters-settings:
  custom:
    gomegacheck:
      path: hack/tools/bin/gomegacheck.so
      description: Check test assertions using gomega
      original-url: github.com/gardener/gardener/hack/tools/gomegacheck
```

> ℹ️ In order for the plugin mechanism to work, all versions of plugin dependencies that overlap with `golangci-lint` must be set to the same version. You can see the versions by running `go version -m golangci-lint`.

Now, `gomegacheck` is executed along with all other linters:

```bash
golangci-lint run ./...
```

This repository is configured to run `gomegacheck` as a `golangci-lint` plugin along with all other linters on:

```bash
make check
```
