# Overview

Conceptually, all Gardener components are designed to run as a Pod inside a Kubernetes cluster.
The Gardener API server extends the Kubernetes API via the user-aggregated API server concepts.
However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time.
That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details can be found in

1. [Principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [Kubernetes Development Guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
1. [Architecture of Gardener](https://github.com/gardener/documentation/wiki/Architecture)

This guide is split into two main parts:

* [Preparing your setup by installing all dependencies and tools](#preparing-the-setup)
* [Getting the Gardener source code locally](#get-the-sources)

# Preparing the Setup

## [macOS only] Installing homebrew

The copy-paste instructions in this guide are designed for macOS and use the package manager [Homebrew](https://brew.sh/).

On macOS run

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

## [macOS only] Installing GNU bash

Built-in apple-darwin bash is missing some features that could cause shell scripts to fail locally.

```bash
brew install bash
```

## Installing git

We use `git` as VCS which you need to install. On macOS run

```bash
brew install git
```

For other OS, please check the [Git installation documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

## Installing Go

Install the latest version of Go. On macOS run

```bash
brew install go
```

For other OS, please check [Go installation documentation](https://golang.org/doc/install).

## Installing kubectl

Install `kubectl`. Please make sure that the version of `kubectl` is at least `v1.27.x`. On macOS run

```bash
brew install kubernetes-cli
```

For other OS, please check the [kubectl installation documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/).

## Installing Docker

You need to have docker installed and running. On macOS run

```bash
brew install --cask docker
```

For other OS please check the [docker installation documentation](https://docs.docker.com/get-docker/).

## Installing iproute2

`iproute2` provides a collection of utilities for network administration and configuration. On macOS run

```bash
brew install iproute2mac
```

## Installing jq

[jq](https://jqlang.github.io/jq/) is a lightweight and flexible command-line JSON processor. On macOS run

```bash
brew install jq
```

## Installing yq

[yq](https://mikefarah.gitbook.io/yq) is a lightweight and portable command-line YAML processor. On macOS run

```bash
brew install yq
```

## Installing GNU Parallel

[GNU Parallel](https://www.gnu.org/software/parallel/) is a shell tool for executing jobs in parallel, used by the code generation scripts (`make generate`). On macOS run

```bash
brew install parallel
```

## [macOS only] Install GNU Core Utilities

When running on macOS, install the GNU core utilities and friends:

```bash
brew install coreutils gnu-sed gnu-tar grep gzip
```

This will create symbolic links for the GNU utilities with `g` prefix on your `PATH`, e.g., `gsed` or `gbase64`.
To allow using them without the `g` prefix, add the `gnubin` directories to the beginning of your `PATH` environment variable (`brew install` and `brew info` will print out instructions for each formula):

```bash
export PATH=$(brew --prefix)/opt/coreutils/libexec/gnubin:$PATH
export PATH=$(brew --prefix)/opt/gnu-sed/libexec/gnubin:$PATH
export PATH=$(brew --prefix)/opt/gnu-tar/libexec/gnubin:$PATH
export PATH=$(brew --prefix)/opt/grep/libexec/gnubin:$PATH
export PATH=$(brew --prefix)/opt/gzip/bin:$PATH
```

## [Windows Only] WSL2

Apart from Linux distributions and macOS, the local gardener setup can also run on the Windows Subsystem for Linux 2.

While WSL1, plain docker for Windows and various Linux distributions and local Kubernetes environments may be supported, this setup was verified with:

* [WSL2](https://docs.microsoft.com/en-us/windows/wsl/wsl2-index)
* [Docker Desktop WSL2 Engine](https://docs.docker.com/docker-for-windows/wsl/)
* [Ubuntu 18.04 LTS on WSL2](https://ubuntu.com/blog/ubuntu-on-wsl-2-is-generally-available)
* Nodeless local garden (see below)

The Gardener repository and all the above-mentioned tools (git, golang, kubectl, ...) should be installed in your WSL2 distro, according to the distribution-specific Linux installation instructions.

# Get the Sources

Clone the repository from GitHub into your `$GOPATH`.

```bash
mkdir -p $(go env GOPATH)/src/github.com/gardener
cd $(go env GOPATH)/src/github.com/gardener
git clone git@github.com:gardener/gardener.git
cd gardener
```

> Note: Gardener is using Go modules and cloning the repository into `$GOPATH` is not a hard requirement. However it is still recommended to clone into `$GOPATH` because `k8s.io/code-generator` does not work yet outside of `$GOPATH` - [kubernetes/kubernetes#86753](https://github.com/kubernetes/kubernetes/issues/86753).

# Start the Gardener

Please see [getting_started_locally.md](../deployment/getting_started_locally.md) how to build and deploy Gardener from your local sources.
