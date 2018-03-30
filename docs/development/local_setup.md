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

### Ginkgo and Gomega

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

## Installing `Vagrant`

In case you want to run the `gardener-local-provider` and test the creation of Shoot clusters on your machine you have to [install](https://www.vagrantup.com/downloads.html) Vagrant.

Please make sure that the executable `bsdtar` is available on your system.

## Installing `Virtualbox`

In this local setup a virtualizer is needed. Here, [`Virtualbox`](https://www.virtualbox.org) is used. However, Vagrant supports other virtualizers as well. Please check the [`Vagrant` documentation](https://www.vagrantup.com/docs/index.html) for further details.

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

The commands below will configure your `minikube` with the absolute minimum resources to launch Gardener API Server and Gardener Controller Manager on a local machine.

#### Start `minikube`

First, start `minikube` with at least Kubernetes v1.9.x, e.g. via `minikube --kubernetes-version=v1.9.0`

```bash
$ minikube start --kubernetes-version=v1.9.0
Starting local Kubernetes v1.9.0 cluster...
[...]
kubectl is now configured to use the cluster.
```

#### Prepare the Gardener

The Gardener exposes the API servers of Shoot clusters via Kubernetes services of type `LoadBalancer`. In order to establish stable endpoints (robust against changes of the load balancer address), it creates DNS records pointing to these load balancer addresses. They are used internally and by all cluster components to communicate.
You need to have control over a domain (or subdomain) for which these records will be created.
Please provide an *internal domain secret* (see [this](../../example/secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found [here](../deployment/configuration.md).

```bash
$ make dev-setup
namespace "garden" created
namespace "garden-dev" created
secret "internal-domain-unmanaged" created
deployment "etcd" created
service "etcd" created
service "gardener-apiserver" created
endpoints "gardener-apiserver" created
apiservice "v1beta1.garden.sapcloud.io" created
```

#### Run the Gardener API Server and the Gardener Controller Manager

Next, you need to run the Gardener API Server and the Gardener Controller Manager using rules in the `Makefile`.

```bash
$ make start-api
[restful] 2018/02/01 15:39:43 log.go:33: [restful/swagger] listing is available at https:///swaggerapi
[restful] 2018/02/01 15:39:43 log.go:33: [restful/swagger] https:///swaggerui/ is mapped to folder /swagger-ui/
I0201 15:39:43.750573   84958 serve.go:89] Serving securely on [::]:8443
[...]
```

In another terminal, launch the Gardener Controller Manager

```bash
$ make start
time="2018-02-20T13:24:39+02:00" level=info msg="Starting Gardener controller manager..."
time="2018-02-20T13:24:39+02:00" level=info msg="Gardener controller manager HTTP server started (serving on 0.0.0.0:2718)"
time="2018-02-20T13:24:39+02:00" level=info msg="Found internal domain secret internal-domain-unmanaged for domain nip.io."
time="2018-02-20T13:24:39+02:00" level=info msg="Successfully bootstrapped the Garden cluster."
time="2018-02-20T13:24:39+02:00" level=info msg="Gardener controller manager (version 0.2.0) initialized."
time="2018-02-20T13:24:39+02:00" level=info msg="Quota controller initialized."
time="2018-02-20T13:24:39+02:00" level=info msg="CloudProfile controller initialized."
time="2018-02-20T13:24:39+02:00" level=info msg="SecretBinding controller initialized."
time="2018-02-20T13:24:39+02:00" level=info msg="Watching all namespaces for Shoot resources..."
time="2018-02-20T13:24:39+02:00" level=info msg="Shoot controller initialized."
time="2018-02-20T13:24:39+02:00" level=info msg="Seed controller initialized."
[...]
```

:information_source: Your username is inferred from the user you are logged in with on your machine. The version is incremented based on the content of the `VERSION` file. The version is important for the Gardener in order to identify which Gardener version has last operated a Shoot cluster.

The Gardener should now be ready to operate on Shoot resources. You can use

```bash
$ kubectl get shoots
No resources found.
```

to operate against your local running Gardener API server.

> Note: It may take several seconds until the `minikube` cluster recognizes that the Gardener API server has been started and is available. `No resources found` is the expected result of our initial development setup.

#### Configure `minikube` to act as Gardener and Seed Cluster

The Gardener Local Provider gives you the ability to create Shoot clusters on your local machine without the need to have an account on a Cloud Provider. Please make sure that Vagrant is installed (see section [Installing Vagrant](#installing-vagrant))

Make sure that you already run `make dev-setup` and that the Gardener API server and the Gardener controller manager are running via `make start-api` and `make start` as described before.

Next, you need to configure `minikube` to work as the Gardener and as the Seed cluster in such a way that it uses the local Vagrant installation to create the Shoot clusters.

```bash
$ make dev-setup-local
cloudprofile "local" created
secret "dev-local" created
secretbinding "core-local" created
Cluster "gardener-dev" set.
User "gardener-dev" set.
Context "gardener-dev" modified.
Switched to context "gardener-dev".
secret "seed-local-dev" created
seed "local-dev" created
```

#### Check Vagrant setup

To be sure that the Vagrant has been successfuly installed and configured, test your setup:

```bash
$ cd vagrant
$ vagrant up
Bringing machine 'core-01' up with 'virtualbox' provider...
==> core-01: Importing base box 'coreos-stable'...
==> core-01: Configuring Ignition Config Drive
==> core-01: Matching MAC address for NAT networking...
[...]
```

If successful, delete your machine before continuing:

```bash
$ vagrant destroy --force
==> core-01: Forcing shutdown of VM...
==> core-01: Destroying VM and associated drives...

$ cd $GOPATH/src/github.com/gardener/gardener
```

#### Start the Gardener Local Provider

The Seed cluster provides the possibility to create Shoot clusters on several cloud provider. The Gardener Provider implements a common interface to all supported cloud providers. Here, the corresponding Gardener Provider for Local is used.

By executing

```bash
$ make start-local
2018/02/14 10:53:34 Listening on :3777
2018/02/14 10:53:34 Vagrant directory /Users/foo/go/src/github.com/gardener/gardener/vagrant
2018/02/14 10:53:34 user-data path /Users/foo/git/go/src/github.com/gardener/gardener/dev/user-data
```

the Gardener Local Provider is started.

At this point three processes should run in an individual terminal, the Gardener API server, the Gardener controller manager and finally the Gardener Local Provider.

#### Create, access and delete a Shoot Cluster

Now, you can create a Shoot cluster by running

```bash
$ kubectl apply -f dev/shoot-local.yaml
shoot "local" created
```

When the Shoot API server is created you can download the `kubeconfig` for it and access it:

```bash
$ kubectl --namespace shoot-dev-local get secret kubecfg -o jsonpath="{.data.kubeconfig}" | base64 --decode > dev/shoot-kubeconfig

# Depending on your Internet speed, it can take some time, before your node reports a READY status.
$ kubectl --kubeconfig dev/shoot-kubeconfig get nodes
NAME                    STATUS    ROLES     AGE       VERSION
192.168.99.201.nip.io   Ready     node      1m        v1.9.1
```

> Note: It is required that your minikube has network connectivity to the nodes created by Vagrant.

For additional debugging on your Vagrant node you can `ssh` into it

```bash
$ cd vagrant
$ vagrant ssh
```

To delete the Shoot cluster

```bash
$ ./hack/delete-shoot local garden-dev
shoot "local" deleted
shoot "local" patched
```

#### Limitations

Currently, there are some limitations in the local Shoot setup which need to be considered. Please keep in mind that this setup is intended to be used by Gardener developers.

- The cloud provider allows to choose from a various list of different machine types. This flexibility is not available in this setup on a single local machine. However, it is possible to specify the Shoot nodes resources (cpu and memory) used by Vagrant in this [configuration file](../../vagrant/Vagrantfile). In the Shoot creation process the Machine Controller Manager plays a central role. Due to the limitation in this setup this component is not used.
- It is not yet possible to create Shoot clusters consisting of more than one worker node. Cluster Autoscaling therefore is not supported
- It is not yet possible to create two or more Shoot clusters in parallel
- The Shoot API Server is exposed via a NodePort. In a cloud setup a LoadBalancer would be used
- The communication between the Seed and the Shoot Clusters uses VPN tunnel. In this setup tunnels are not needed since all components run on localhost

## Additional information

In order to ensure that a specific Seed cluster will be chosen, add the `.spec.cloud.seed` field (see [here](../../example/shoot-azure.yaml#L10) for an example Shoot manifest).

Please take a look at the [example manifests folder](../../example) to see which resource objects you need to install into your Garden cluster.
