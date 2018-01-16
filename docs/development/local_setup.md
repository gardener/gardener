# Preparing the Local Development Setup (Mac OS X)

Conceptionally, the Gardener is designated to run in containers within a Pod inside an Kubernetes cluster. It extends the API via the user-aggregated API server concepts. However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time. That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

## Installing Golang environment
Install the latest version of Golang (at least `v1.9.2` is required) by using [Homebrew](https://brew.sh/):

```bash
$ brew install golang
```

Make sure to set your `$GOPATH` environment variable properly (conventionally, it points to `$HOME/go`).

For your convenience, you can add the `bin` directory of the `$GOPATH` to your `$PATH`: `PATH=$PATH:$GOPATH/bin`, but it is not necessarily required.

In order to perform linting on the Go source code, please install [Golint](https://github.com/golang/lint):

```bash
$ go get -u github.com/golang/lint/golint
```

In order to perform tests on the Go source code, please install [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](http://onsi.github.io/gomega/). Please make yourself familiar with both frameworks and read their introductions after installation:

```bash
$ go get -u github.com/onsi/ginkgo/ginkgo
$ go get -u github.com/onsi/gomega
```

We use [Dep](https://github.com/golang/dep) for managing Golang package dependencies. Please install it:

```bash
$ brew install dep
```

## Installing `kubectl` and `helm`
As already mentioned in the introduction, the communication with the Gardener happens via the Kubernetes (Garden) cluster it is targetting. To interact with that cluster, you need to install `kubectl`:

```bash
$ brew install kubernetes-cli
```

You may also need to develop Helm charts or interact with Tiller using the [Helm](https://github.com/kubernetes/helm) CLI:

```bash
$ brew install kubernetes-helm
```

## Installing `git`
We use `git` as VCS which you need to install:

```bash
$ brew install git
```

## Installing Docker (Optional)
In case you want to build Docker images for the Gardener you have to install Docker itself. We recommend using [Docker for Mac OS X](https://docs.docker.com/docker-for-mac/) which can be downloaded from [here](https://download.docker.com/mac/stable/Docker.dmg).

## Installing `gcloud` SDK (Optional)
In case you have to create a new release or a new hotfix of the Gardener you have to push the resulting Docker image into a Docker registry. Currently, we are using the Google Container Registry (this could change in the future). Please follow the offical installation instructions [from Google](https://cloud.google.com/sdk/downloads#mac).


# Installing the Gardener locally
Clone the repository from GitHub into your `$GOPATH`.

```bash
$ mkdir -p $GOPATH/src/github.com/gardener
$ cd $GOPATH/src/github.com/gardener
$ git clone git@github.com:gardener/gardener.git
$ cd gardener
```

# Local development

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of the Garden](https://github.com/gardener/documentation/wiki/Architecture), and what the various clusters are used for.

The development of the Gardener could happen by targetting any cluster. You basically need a Garden cluster (e.g., a [Minikube](https://github.com/kubernetes/minikube) cluster) and one Seed cluster per cloud provider and per data center/region. You can configure the Garden controller manager to watch **all namespaces** for Shoot manifests or to only watch **one single** namespace.

In order to ensure that a specific Seed cluster will be chosen, add the `.spec.cloud.seed` field (see [here](../../example/shoot-azure.yaml#L10) for an example Shoot manifest).

## Get started

Please take a look at the [example manifests folder](../../example) to see which resource objects you need to install into your Garden cluster.

For your convenience, we have created a rule in the `Makefile` which will automatically start the Garden controller manager with development settings:

```bash
$ make dev
time="2017-12-05T15:58:48+01:00" level=info msg="Starting Garden controller manager..."
[...]
time="2017-12-05T15:58:55+01:00" level=info msg="Watching only namespace 'johndoe' for Shoot resources..."
time="2017-12-05T15:58:55+01:00" level=info msg="Garden controller manager (version 0.28.0) initialized successfully."
```

:information_source: Your username is inferred from the user you are logged in with on your machine. The version is incremented based on the content of the `VERSION` file. The version is important for the Gardener in order to identify which Gardener version has last operated a Shoot cluster.

The Gardener should now be ready to operate on Shoot resources associated to your namespace.
