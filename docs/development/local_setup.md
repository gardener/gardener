# Preparing the setup

Conceptionally, the Gardener is designated to run in containers within a Pod inside an Kubernetes cluster. It extends the API via the user-aggregated API server concepts. However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time. That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details could be found in

1. [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [architecture of the Garden](https://github.com/gardener/documentation/wiki/Architecture)

This setup is based on [minikube](https://github.com/kubernetes/minikube), a Kubernetes cluster running on a single node.

## Installing Golang environment

Install the latest version of Golang (at least `v1.9.2` is required). For Mac OS you could use [Homebrew](https://brew.sh/):

```bash
$ brew install golang
```

For other OS, please check [Go installation documentation](https://golang.org/doc/install).

Make sure to set your `$GOPATH` environment variable properly (conventionally, it points to `$HOME/go`).

For your convenience, you can add the `bin` directory of the `$GOPATH` to your `$PATH`: `PATH=$PATH:$GOPATH/bin`, but it is not necessarily required.

We use [Dep](https://github.com/golang/dep) for managing Golang package dependencies. Please install it
on Mac OS via

```bash
$ brew install dep
```

On other OS please check the [Dep installation documentation](https://golang.github.io/dep/docs/installation.html) and the [Dep releases page](https://github.com/golang/dep/releases). After downloading the appropriate release in your `$GOPATH/bin` folder you need to make it executable via `chmod +x <dep-release>` and to rename it to dep via `mv dep-<release> dep`.

### Golint

In order to perform linting on the Go source code, please install [Golint](https://github.com/golang/lint):

```bash
$ go get -u github.com/golang/lint/golint
```

### Ginko and Gomega

In order to perform tests on the Go source code, please install [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](http://onsi.github.io/gomega/). Please make yourself familiar with both frameworks and read their introductions after installation:

```bash
$ go get -u github.com/onsi/ginkgo/ginkgo
$ go get -u github.com/onsi/gomega
```

## Installing `kubectl` and `helm`

As already mentioned in the introduction, the communication with the Gardener happens via the Kubernetes (Garden) cluster it is targeting. To interact with that cluster, you need to install `kubectl`.

On Mac OS run

```bash
$ brew install kubernetes-cli
```

Please check the [kubectl installation documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for other OS.

You may also need to develop Helm charts or interact with Tiller using the [Helm](https://github.com/kubernetes/helm) CLI:

On Mac OS run

```bash
$ brew install kubernetes-helm
```

On other OS please check the [Helm installation documentation](https://github.com/kubernetes/helm/blob/master/docs/install.md).

## Installing `git`

We use `git` as VCS which you need to install.

On Mac OS run

```bash
$ brew install git
```

On other OS, please check the [Git installation documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

## Installing Minikube

You'll need to have [minikube](https://github.com/kubernetes/minikube#installation) installed and running.

## Installing iproute2

`iproute2` provides a collection of utilities for network administration and configuration.

On Mac OS run

```bash
$ brew install iproute2mac
```

## [Optional] Installing Docker

In case you want to build Docker images for the Gardener you have to install Docker itself. We recommend using [Docker for Mac OS X](https://docs.docker.com/docker-for-mac/) which can be downloaded from [here](https://download.docker.com/mac/stable/Docker.dmg).

On other OS, please check the [Docker installation documentation](https://docs.docker.com/install/).

## [Optional] Installing `gcloud` SDK

In case you have to create a new release or a new hotfix of the Gardener you have to push the resulting Docker image into a Docker registry. Currently, we are using the Google Container Registry (this could change in the future). Please follow the official [installation instructions from Google](https://cloud.google.com/sdk/downloads).

## Local Gardener setup

This setup is only meant to be used for developing purposes, which means that only the control plane of the Gardener cluster is running on your machine.

### Get the sources

Clone the repository from GitHub into your `$GOPATH`.

```bash
$ mkdir -p $GOPATH/src/github.com/gardener
$ cd $GOPATH/src/github.com/gardener
$ git clone git@github.com:gardener/gardener.git
$ cd gardener
```

### Start the Gardener

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of the Garden](https://github.com/gardener/documentation/wiki/Architecture), and what the various clusters are used for.

The development of the Gardener could happen by targeting any cluster. You basically need a Garden cluster (e.g., a [Minikube](https://github.com/kubernetes/minikube) cluster) and one Seed cluster per cloud provider and per data center/region. You can configure the Gardener controller manager to watch **all namespaces** for Shoot manifests or to only watch **one single** namespace.

The commands below will configure your `minikube` with the absolute minimum resources to launch Gardener API server & controller manager:

```bash
$ minikube start
Starting local Kubernetes v1.8.0 cluster...
[...]
kubectl is now configured to use the cluster.
```

The Gardener exposes the API servers of Shoot clusters via Kubernetes services of type `LoadBalancer`. In order to establish stable endpoints (robust against changes of the load balancer address), it creates DNS records pointing to these load balancer addresses. They are used internally and by all cluster components to communicate.
You need to have control over a domain (or subdomain) for which these records will be created.
Please provide an *internal domain secret* (see [this](../../example/secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found [here](../deployment/configuration.md).

```bash
$ mkdir -p dev

$ kubectl apply -f example/namespace-garden.yaml
namespace "garden" created

$ cp example/secret-internal-domain.yaml dev/secret-internal-domain.yaml
# <Put your credentials in dev/secret-internal-domain.yaml>

$ kubectl apply -f dev/secret-internal-domain.yaml
secret "internal-domain-example-com" created

$ make dev-setup
namespace "garden-development" created
deployment "etcd" created
service "etcd" created
service "gardener-apiserver" created
endpoints "gardener-apiserver" created
apiservice "v1beta1.garden.sapcloud.io" created
```

Next, you need to run the Gardener API server and the Gardener controller manager using rules in the `Makefile`.

```bash
$ make start-api
[restful] 2018/02/01 15:39:43 log.go:33: [restful/swagger] listing is available at https:///swaggerapi
[restful] 2018/02/01 15:39:43 log.go:33: [restful/swagger] https:///swaggerui/ is mapped to folder /swagger-ui/
I0201 15:39:43.750573   84958 serve.go:89] Serving securely on [::]:8443
```

In another terminal, you must launch the Gardener controller manager:

```bash
$ make start
time="2018-01-29T10:18:37+02:00" level=info msg="Starting Gardener controller manager..."
time="2018-01-29T10:18:38+02:00" level=info msg="Gardener controller manager HTTP server started (serving on 0.0.0.0:2718)"
time="2018-01-29T10:18:38+02:00" level=info msg="Found default domain secret default-domain for domain 192.168.99.100.nip.io."
time="2018-01-29T10:18:38+02:00" level=info msg="Found internal domain secret internal-domain-nip-io for domain 192.168.99.100.nip.io."
time="2018-01-29T10:18:38+02:00" level=info msg="Successfully bootstrapped the Gardener cluster."
time="2018-01-29T10:18:38+02:00" level=info msg="Gardener controller manager (version 0.1.0) initialized."
time="2018-01-29T10:18:38+02:00" level=info msg="Watching all namespaces for Shoot resources..."
time="2018-01-29T10:18:38+02:00" level=info msg="Shoot controller initialized."
time="2018-01-29T10:18:38+02:00" level=info msg="Seed controller initialized."
```

:information_source: Your username is inferred from the user you are logged in with on your machine. The version is incremented based on the content of the `VERSION` file. The version is important for the Gardener in order to identify which Gardener version has last operated a Shoot cluster.

The Gardener should now be ready to operate on Shoot resources. You can use

```bash
$ kubectl get shoots
No resources found.
```

to operate against your local running Gardener API server.

> Note: It may take several seconds until the `minikube` cluster recognizes that the Gardener API server has been started and is available. `No resources found` is the expected result of our initial development setup.

## Additional information

In order to ensure that a specific Seed cluster will be chosen, add the `.spec.cloud.seed` field (see [here](../../example/shoot-azure.yaml#L10) for an example Shoot manifest).

Please take a look at the [example manifests folder](../../example) to see which resource objects you need to install into your Garden cluster.
