# Preparing the setup

Conceptionally, Gardener is designated to run inside as a Pod inside an Kubernetes cluster. It extends the Kubernetes API via the user-aggregated API server concepts. However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time. That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details could be found in

1. [Principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [Kubernetes Development Guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
1. [Architecture of the Garden](https://github.com/gardener/documentation/wiki/Architecture)

This setup is based on [minikube](https://github.com/kubernetes/minikube), a Kubernetes cluster running on a single node. Docker Desktop Edge and [kind](https://github.com/kubernetes-sigs/kind) are also supported.

## Installing Golang environment

Install latest version of Golang. For Mac OS you could use [Homebrew](https://brew.sh/):

```bash
$ brew install golang
```

For other OS, please check [Go installation documentation](https://golang.org/doc/install).

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

## Local Gardener setup

This setup is only meant to be used for developing purposes, which means that only the control plane of the Gardener cluster is running on your machine.

### Get the sources

Clone the repository from GitHub.

```bash
$ git clone git@github.com:gardener/gardener.git
$ cd gardener
```

### Start the Gardener

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of the Garden](https://github.com/gardener/documentation/wiki/Architecture), and what the various clusters are used for.

The development of the Gardener could happen by targeting any cluster. You basically need a Garden cluster (e.g., a [Minikube](https://github.com/kubernetes/minikube) cluster) and one Seed cluster per cloud provider and per data center/region. You can configure the Gardener controller manager to watch **all namespaces** for Shoot manifests or to only watch **one single** namespace.

The commands below will configure your `minikube` with the absolute minimum resources to launch Gardener API Server and Gardener Controller Manager on your local machine.

#### Start minikube

```bash
$ minikube start
😄  minikube v1.0.1 on darwin (amd64)
🔥  Creating virtualbox VM (CPUs=2, Memory=2048MB, Disk=20000MB) ...
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
Please provide an *internal domain secret* (see [this](../../example/10-secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found [here](../concepts/configuration.md).

```bash
$ kubectl apply -f example/10-secret-internal-domain-unmanaged.yaml
secret/internal-domain-unmanaged created
```

#### Run the Gardener API Server and the Gardener Controller Manager

Next, run the Gardener API Server, the Gardener Controller Manager and the Gardener Scheduler (optionally) in different terminals using rules in the `Makefile`.

```bash
$ make start-apiserver
Found Minikube ...
I0306 15:23:51.044421   74536 plugins.go:84] Registered admission plugin "ResourceReferenceManager"
I0306 15:23:51.044523   74536 plugins.go:84] Registered admission plugin "DeletionConfirmation"
[...]
I0306 15:23:51.626836   74536 secure_serving.go:116] Serving securely on [::]:8443
[...]
```

Now you are ready to launch the Gardener Controller Manager

```bash
$ make start-controller-manager
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

Optionally, you can also launch the Gardener Scheduler

```bash
$ make start-scheduler
time="2019-05-02T16:31:50+02:00" level=info msg="Starting Gardener scheduler ..."
time="2019-05-02T16:31:50+02:00" level=info msg="Starting HTTP server on 0.0.0.0:10251"
time="2019-05-02T16:31:50+02:00" level=info msg="Acquired leadership, starting scheduler."
time="2019-05-02T16:31:50+02:00" level=info msg="Gardener scheduler initialized (with Strategy: SameRegion)"
time="2019-05-02T16:31:50+02:00" level=info msg="Scheduler controller initialized."
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

#### Limitations of local development setup

You can run Gardener (API server and controller manager) against any local Kubernetes cluster, however, your seed and shoot clusters must be deployed to a "real" provider.
Currently, it is not possible to run Gardener entirely isolated from any cloud provider. We are planning to support such a setup based on KubeVirt (see [this for details](https://github.com/gardener/gardener/issues/827)), however, it does not yet exist.
This means that - after you have setup Gardener - you need to register an external seed cluster (e.g., one created in AWS).
Only after that step you can start creating shoot clusters with your locally running Gardener.

Some time ago, we had a local setup based on VirtualBox/Vagrant.
However, as we have progressed with the [Extensibility epic](https://github.com/gardener/gardener/issues/308) we noticed that this implementation/setup does no longer fit into how we envision external providers to be.
Moreover, it hid too many things and came with a bunch of limitations, making the development scenario too "artificial":

- No integration with machine-controller-manager.
- The Shoot API Server is exposed via a NodePort. In a cloud setup a LoadBalancer would be used.
- It was not possible to create Shoot clusters consisting of more than one worker node. Cluster auto-scaling therefore is not supported.
- It was not possible to create two or more Shoot clusters in parallel.
- The communication between the Seed and the Shoot Clusters uses VPN tunnel. In this setup tunnels are not needed since all components run on localhost.

## Additional information

In order to ensure that a specific Seed cluster will be chosen, add the `.spec.cloud.seed` field (see [here](../../example/90-shoot-azure.yaml#L10) for an example Shoot manifest).

Please take a look at the [example manifests folder](../../example) to see which resource objects you need to install into your Garden cluster.
