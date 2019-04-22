# Preparing the setup

Conceptionally, Gardener is designated to run inside as a Pod inside an Kubernetes cluster. It extends the Kubernetes API via the user-aggregated API server concepts. However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time. That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details could be found in

1. [Principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [Kubernetes Development Guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
1. [Architecture of the Garden](https://github.com/gardener/documentation/wiki/Architecture)

This setup is based on [minikube](https://github.com/kubernetes/minikube), a Kubernetes cluster running on a single node. Docker Desktop Edge is also supported.

## Installing Golang environment

Install latest version of Golang. For Mac OS you could use [Homebrew](https://brew.sh/):

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
$ go get -u golang.org/x/lint/golint
```

### Ginkgo and Gomega

In order to perform tests on the Go source code, please install [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](http://onsi.github.io/gomega/). Please make yourself familiar with both frameworks and read their introductions after installation:

```bash
$ go get -u github.com/onsi/ginkgo/ginkgo
$ go get -u github.com/onsi/gomega
```

## Installing kubectl and helm

As already mentioned in the introduction, the communication with the Gardener happens via the Kubernetes (Garden) cluster it is targeting. To interact with that cluster, you need to install `kubectl`. Please make sure that the version of `kubectl` is at least `v1.11.x`.

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

## Installing git

We use `git` as VCS which you need to install.

On Mac OS run

```bash
$ brew install git
```

On other OS, please check the [Git installation documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

## Installing openvpn

We use `OpenVPN` to establish network connectivity from the control plane running in the Seed cluster to the Shoot's worker nodes running in private networks.
To harden the security we need to generate another secret to encrypt the network traffic ([details](https://openvpn.net/index.php/open-source/documentation/howto.html#security)).
Please install the `openvpn` binary. On Mac OS run

```bash
$ brew install openvpn
$ export PATH=$(brew --prefix openvpn)/sbin:$PATH
```

On other OS, please check the [OpenVPN downloads page](https://openvpn.net/index.php/open-source/downloads.html).

## Installing Minikube

You'll need to have [minikube](https://github.com/kubernetes/minikube#installation) installed and running.

## Installing iproute2

`iproute2` provides a collection of utilities for network administration and configuration.

On Mac OS run

```bash
$ brew install iproute2mac
```

## Installing yaml2json and jq

```bash
$ go get -u github.com/bronze1man/yaml2json
$ brew install jq
```

## [Mac OS X only] Install GNU core utilities

When running on Mac OS X you have to install the GNU core utilities:
```bash
$ brew install coreutils gnu-sed
```

This will create symlinks for the GNU utilities with `g` prefix in `/usr/local/bin`, e.g., `gsed` or `gbase64`. To allow using them without the `g` prefix please put `/usr/local/opt/coreutils/libexec/gnubin` at the beginning of your `PATH` environment variable, e.g., `export PATH=/usr/local/opt/coreutils/libexec/gnubin:$PATH`.

## [Optional] Installing Docker

In case you want to use the "Docker for Mac Kubernetes" or if you want to build Docker images for the Gardener you have to install Docker itself. On Mac OS X, please use [Docker for Mac OS X](https://docs.docker.com/docker-for-mac/) which can be downloaded [here](https://download.docker.com/mac/stable/Docker.dmg).

On other OS, please check the [Docker installation documentation](https://docs.docker.com/install/).

## [Optional] Installing gcloud SDK

In case you have to create a new release or a new hotfix of the Gardener you have to push the resulting Docker image into a Docker registry. Currently, we are using the Google Container Registry (this could change in the future). Please follow the official [installation instructions from Google](https://cloud.google.com/sdk/downloads).

## Installing Vagrant

In case you want to run the `gardener-local-provider` and test the creation of Shoot clusters on your machine you have to [install](https://www.vagrantup.com/downloads.html) Vagrant.

Please make sure that the executable `bsdtar` is available on your system.

## Installing Virtualbox

In this local setup a virtualizer is needed. Here, [`Virtualbox`](https://www.virtualbox.org) is used. However, Vagrant supports other virtualizers as well. Please check the [`Vagrant` documentation](https://www.vagrantup.com/docs/index.html) for further details.

## Test nip.io

`nip.io` is used as an unmanaged DNS implementation for the local setup. Some ISPs don't handle `nip.io` very well. Test NS resolution:

```bash
nslookup 192.168.99.201.nip.io
Server:         8.8.8.8
Address:        8.8.8.8#53

Non-authoritative answer:
Name:   192.168.99.201.nip.io
Address: 192.168.99.201
```

If there is an error, switch your DNS server to `8.8.8.8` / `8.8.4.4` or `1.1.1.1`.

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

#### Start minikube

First, start `minikube` with at least Kubernetes v1.11.x. The default CPU and memory settings of the minikube machine are not sufficient to host the control plane of a shoot cluster, thus the minimal resources should be 3 CPUs and 4096 MB memory, while the recommended are 4 CPUs and 8192 MB memory.

```bash
$ minikube start --cpus=4 --memory=8192
😄  minikube v1.0.0 on darwin (amd64)
🔥  Creating virtualbox VM (CPUs=4, Memory=8192MB, Disk=20000MB) ...
[...]
🏄  Done! Thank you for using minikube!
```

#### Prepare the Gardener

```bash
$ make dev-setup
Found Minikube ...
namespace/garden created
namespace/garden-dev created
deployment.apps/etcd created
service/etcd created
service/gardener-apiserver created
service/gardener-controller-manager created
endpoints/gardener-apiserver created
endpoints/gardener-controller-manager created
apiservice.apiregistration.k8s.io/v1beta1.garden.sapcloud.io created
apiservice.apiregistration.k8s.io/v1alpha1.core.gardener.cloud created
validatingwebhookconfiguration.admissionregistration.k8s.io/validate-namespace-deletion created
```

Optionally, you can switch off the `Logging` feature gate of Gardener Controller Manager to save resources:

```bash
$ sed -i -e 's/Logging: true/Logging: false/g' dev/20-componentconfig-gardener-controller-manager.yaml
```

The Gardener exposes the API servers of Shoot clusters via Kubernetes services of type `LoadBalancer`. In order to establish stable endpoints (robust against changes of the load balancer address), it creates DNS records pointing to these load balancer addresses. They are used internally and by all cluster components to communicate.
You need to have control over a domain (or subdomain) for which these records will be created.
Please provide an *internal domain secret* (see [this](../../example/10-secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found [here](../deployment/configuration.md).

```bash
$ kubectl apply -f example/10-secret-internal-domain-unmanaged.yaml
secret/internal-domain-unmanaged created
```

#### Run the Gardener API Server and the Gardener Controller Manager

Next, run the Gardener API Server and the Gardener Controller Manager in different terminals using rules in the `Makefile`.

```bash
$ make start-api
Found Minikube ...
I0306 15:23:51.044421   74536 plugins.go:84] Registered admission plugin "ResourceReferenceManager"
I0306 15:23:51.044523   74536 plugins.go:84] Registered admission plugin "DeletionConfirmation"
[...]
I0306 15:23:51.626836   74536 secure_serving.go:116] Serving securely on [::]:8443
[...]
```

Now you are ready to launch the Gardener Controller Manager

```bash
$ make start
time="2019-03-06T15:24:17+02:00" level=info msg="Starting Gardener controller manager..."
time="2019-03-06T15:24:17+02:00" level=info msg="Feature Gates: CertificateManagement=false,Logging=true"
time="2019-03-06T15:24:17+02:00" level=info msg="Starting HTTP server on 0.0.0.0:2718"
time="2019-03-06T15:24:17+02:00" level=info msg="Acquired leadership, starting controllers."
time="2019-03-06T15:24:18+02:00" level=info msg="Starting HTTPS server on 0.0.0.0:2719"
time="2019-03-06T15:24:18+02:00" level=info msg="Found internal domain secret internal-domain-unmanaged for domain nip.io."
time="2019-03-06T15:24:18+02:00" level=info msg="Successfully bootstrapped the Garden cluster."
time="2019-03-06T15:24:18+02:00" level=info msg="Gardener controller manager (version 0.19.0-dev) initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="ControllerInstallation controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="ControllerRegistration controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Shoot controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="BackupInfrastructure controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Seed controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="SecretBinding controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Project controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Quota controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="CloudProfile controller initialized."
[...]
```

Run the following command to install extension controllers - make sure that you install all of them required for your local development. Also, please refer to [this document](../extensions/controllerregistration.md) for further information about how extensions are registered in case you want to use other versions than the latest releases.

```bash
$ make dev-setup-extensions

> Found extension 'os-coreos'. Do you want to install it into your local Gardener setup? (y/n)
...
```

The Gardener should now be ready to operate on Shoot resources. You can use

```bash
$ kubectl get shoots
No resources found.
```

to operate against your local running Gardener API Server.

> Note: It may take several seconds until the `minikube` cluster recognizes that the Gardener API server has been started and is available. `No resources found` is the expected result of our initial development setup.

#### Configure minikube to act as Gardener and Seed Cluster

Before continuing, make sure that Vagrant is installed (see section [Installing Vagrant](#installing-vagrant)), that you already ran `make dev-setup`, and that the Gardener API Server and the Gardener Controller Manager are running via `make start-api` and `make start` as described above.

Next, you need to configure `minikube` to work as the Gardener and as the Seed cluster in such a way that it uses the local Vagrant installation to create the Shoot clusters.

```bash
$ make dev-setup-local
namespace/garden-dev unchanged
project.garden.sapcloud.io/dev created
cloudprofile.garden.sapcloud.io/local created
secret/core-local created
secretbinding.garden.sapcloud.io/core-local created
Cluster "gardener-dev" set.
User "gardener-dev" set.
Context "gardener-dev" modified.
Switched to context "gardener-dev".
controllerinstallation.core.gardener.cloud/os-coreos created
secret/seed-local created
seed.garden.sapcloud.io/local created
```

#### Check Vagrant setup

To be sure that the Vagrant has been successfully installed and configured, test your setup:

```bash
$ cd vagrant
$ vagrant up
Bringing machine 'core-01' up with 'virtualbox' provider...
==> core-01: Importing base box 'coreos-stable'...
[...]
```

If successful, delete your machine before continuing:

```bash
$ vagrant destroy --force
==> core-01: Forcing shutdown of VM...
==> core-01: Destroying VM and associated drives...

$ cd ..
```

#### Create, access and delete a Shoot Cluster

Now, you can create a Shoot cluster by running

```bash
$ kubectl apply -f dev/90-shoot-local.yaml
shoot "local" created
```

Wait until the 2 secrets `osc-result-cloud-config-local-*` appear in the Shoot cluster namespace and then copy `cloud_config` from the secret `osc-result-cloud-config-local-xxxxx-downloader` to the local file `dev/user-data`. This file is used to pass the downloader configuration to the Vagrant machine, which triggers the mechanism that configures the machine properly and causes it to join the Shoot cluster as a node.

```bash
$ kubectl get secrets -n shoot--dev--local | grep osc-result-cloud-config-local
osc-result-cloud-config-local-640f6-downloader   Opaque                                1      70s
osc-result-cloud-config-local-640f6-original     Opaque                                1      68s

$ kubectl get secrets osc-result-cloud-config-local-640f6-downloader -n shoot--dev--local -o jsonpath="{.data.cloud_config}" | base64 -d > dev/user-data
```

Manually start the Vagrant machine:

```bash
$ cd vagrant
$ vagrant up
Bringing machine 'core-01' up with 'virtualbox' provider...
==> core-01: Importing base box 'coreos-stable'...
[...]

$ cd ..
```

At this point, you can download the `kubeconfig` for the Shoot cluster and access it:

```bash
$ kubectl --namespace shoot--dev--local get secret kubecfg -o jsonpath="{.data.kubeconfig}" | base64 --decode > dev/shoot-kubeconfig

# Depending on your Internet speed, it can take some time, before your node reports a READY status.
$ kubectl --kubeconfig dev/shoot-kubeconfig get nodes
NAME                    STATUS    ROLES     AGE       VERSION
192.168.99.201.nip.io   Ready     node      1m        v1.12.5
```

> Note: It is required that your minikube has network connectivity to the nodes created by Vagrant.

For additional debugging on your Vagrant node you can `ssh` into it

```bash
$ cd vagrant
$ vagrant ssh
```

To delete the Shoot cluster

```bash
$ ./hack/delete shoot local garden-dev
shoot "local" deleted
shoot "local" patched
```

Manually destroy the Vagrant machine when you no longer need it:

```bash
$ cd vagrant
$ vagrant destroy --force
==> core-01: Discarding saved state of VM...
==> core-01: Destroying VM and associated drives...

$ cd ..
```

#### Limitations of local Shoot setup

Currently, there are some limitations in the local Shoot setup which need to be considered. Please keep in mind that this setup is intended to be used by Gardener developers.

- The cloud provider allows to choose from a various list of different machine types. This flexibility is not available in this setup on a single local machine. However, it is possible to specify the Shoot nodes resources (cpu and memory) used by Vagrant in this [configuration file](../../vagrant/Vagrantfile). In the Shoot creation process the Machine Controller Manager plays a central role. Due to the limitation in this setup this component is not used.
- It is not yet possible to create Shoot clusters consisting of more than one worker node. Cluster auto-scaling therefore is not supported
- It is not yet possible to create two or more Shoot clusters in parallel
- The Shoot API Server is exposed via a NodePort. In a cloud setup a LoadBalancer would be used
- The communication between the Seed and the Shoot Clusters uses VPN tunnel. In this setup tunnels are not needed since all components run on localhost

## Additional information

In order to ensure that a specific Seed cluster will be chosen, add the `.spec.cloud.seed` field (see [here](../../example/90-shoot-azure.yaml#L10) for an example Shoot manifest).

Please take a look at the [example manifests folder](../../example) to see which resource objects you need to install into your Garden cluster.
