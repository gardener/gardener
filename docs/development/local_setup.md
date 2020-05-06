# Preparing the Setup

Conceptually, all Gardener components are designated to run inside as a Pod inside a Kubernetes cluster.
The API server extends the Kubernetes API via the user-aggregated API server concepts.
However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time.
That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details could be found in

1. [Principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [Kubernetes Development Guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
1. [Architecture of Gardener](https://github.com/gardener/documentation/wiki/Architecture)

This setup is based on [minikube](https://github.com/kubernetes/minikube), a Kubernetes cluster running on a single node. Docker for Desktop and [kind](https://github.com/kubernetes-sigs/kind) are also supported.

## Installing Golang environment

Install latest version of Golang. For MacOS you could use [Homebrew](https://brew.sh/):

```bash
brew install golang
```

For other OS, please check [Go installation documentation](https://golang.org/doc/install).

## Installing kubectl and helm

As already mentioned in the introduction, the communication with the Gardener happens via the Kubernetes (Garden) cluster it is targeting. To interact with that cluster, you need to install `kubectl`. Please make sure that the version of `kubectl` is at least `v1.11.x`.

On MacOS run

```bash
brew install kubernetes-cli
```

Please check the [kubectl installation documentation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) for other OS.

You may also need to develop Helm charts or interact with Tiller using the [Helm](https://github.com/kubernetes/helm) CLI:

On MacOS run

```bash
brew install kubernetes-helm
```

On other OS please check the [Helm installation documentation](https://github.com/kubernetes/helm/blob/master/docs/install.md).

## Installing git

We use `git` as VCS which you need to install.

On MacOS run

```bash
brew install git
```

On other OS, please check the [Git installation documentation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

## Installing openvpn

We use `OpenVPN` to establish network connectivity from the control plane running in the Seed cluster to the Shoot's worker nodes running in private networks.
To harden the security we need to generate another secret to encrypt the network traffic ([details](https://openvpn.net/index.php/open-source/documentation/howto.html#security)).
Please install the `openvpn` binary. On MacOS run

```bash
brew install openvpn
export PATH=$(brew --prefix openvpn)/sbin:$PATH
```

On other OS, please check the [OpenVPN downloads page](https://openvpn.net/index.php/open-source/downloads.html).

## Installing Minikube

You'll need to have [minikube](https://github.com/kubernetes/minikube#installation) installed and running.
> Note: Gardener is working only with self-contained kubeconfig files because of [security issue](https://banzaicloud.com/blog/kubeconfig-security/). You can configure your minikube to create self-contained kubeconfig files via:
> ```bash
> minikube config set embed-certs true
> ```

Alternatively, you can also install Docker for Desktop and [kind](https://github.com/kubernetes-sigs/kind).

In case you want to use the "Docker for Mac Kubernetes" or if you want to build Docker images for the Gardener you have to install Docker itself. On MacOS, please use [Docker for MacOS](https://docs.docker.com/docker-for-mac/) which can be downloaded [here](https://download.docker.com/mac/stable/Docker.dmg).

On other OS, please check the [Docker installation documentation](https://docs.docker.com/install/).

## Installing iproute2

`iproute2` provides a collection of utilities for network administration and configuration.

On MacOS run

```bash
brew install iproute2mac
```

## Installing yaml2json and jq

```bash
go get -u github.com/bronze1man/yaml2json
brew install jq
```

## [MacOS only] Install GNU core utilities

When running on MacOS you have to install the GNU core utilities:

```bash
brew install coreutils gnu-sed
```

This will create symbolic links for the GNU utilities with `g` prefix in `/usr/local/bin`, e.g., `gsed` or `gbase64`. To allow using them without the `g` prefix please put `/usr/local/opt/coreutils/libexec/gnubin` at the beginning of your `PATH` environment variable, e.g., `export PATH=/usr/local/opt/coreutils/libexec/gnubin:$PATH`.

## [Optional] Installing gcloud SDK

In case you have to create a new release or a new hotfix of the Gardener you have to push the resulting Docker image into a Docker registry. Currently, we are using the Google Container Registry (this could change in the future). Please follow the official [installation instructions from Google](https://cloud.google.com/sdk/downloads).

## Local Gardener setup

This setup is only meant to be used for developing purposes, which means that only the control plane of the Gardener cluster is running on your machine.

### Get the sources

Clone the repository from GitHub.

```bash
git clone git@github.com:gardener/gardener.git
cd gardener
```

### Start the Gardener

:warning: Before you start developing, please ensure to comply with the following requirements:

1. You have understood the [principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/), what their purpose is and how they interact with each other.
1. You have understood the [architecture of Gardener](https://github.com/gardener/documentation/wiki/Architecture), and what the various clusters are used for.

#### Start a local kubernetes cluster

For the development of Gardener you need some kind of Kubernetes cluster, which can be used as a "garden" cluster.
I.e. you need a Kubernetes API server on which you can register a `APIService` Gardener's own Extension API Server.  
For this you can use a standard tool from the community to setup a local cluster like minikube, kind or the Kubernetes Cluster feature in Docker for Desktop.

However, if you develop and run Gardener's components locally, you don't actually a fully fledged Kubernetes Cluster,
i.e. you don't actually need to run Pods on it. If you want to use a more lightweight approach for development purposes,
you can use the "nodeless Garden cluster setup" residing in `hack/local-garden`. This is the easiest way to get your
Gardener development setup up and running.

**Using the nodeless cluster setup**

Setting up a local nodeless Garden cluster is quite simple. The only prerequisite is a running docker daemon.
Just use the provided Makefile rules to start your local Garden:
```bash
make local-garden-up
[...]
Starting gardener-dev kube-etcd cluster..!
Starting gardener-dev kube-apiserver..!
Starting gardener-dev kube-controller-manager..!
Starting gardener-dev gardener-etcd cluster..!
namespace/garden created
clusterrole.rbac.authorization.k8s.io/gardener.cloud:admin created
clusterrolebinding.rbac.authorization.k8s.io/front-proxy-client created
[...]
```

This will start all minimally required components of a Kubernetes cluster (`etcd`, `kube-apiserver`, `kube-controller-manager`)
and an `etcd` Instance for the `gardener-apiserver` as Docker containers.

To tear down the local Garden cluster and remove the Docker containers, simply run:
```bash
make local-garden-down
```

**Using minikube**

Alternatively, spin up a cluster with minikube with this command:

```bash
minikube start --embed-certs #  `--embed-certs` can be omitted if minikube has already been set to create self-contained kubeconfig files.
😄  minikube v1.8.2 on Darwin 10.15.3
🔥  Creating virtualbox VM (CPUs=2, Memory=2048MB, Disk=20000MB) ...
[...]
🏄  Done! Thank you for using minikube!
```

#### Prepare the Gardener

Now, that you have started your local cluster, we can go ahead and register the Gardener API Server.
Just point your `KUBECONFIG` environment variable to the local cluster you created in the previous step and run:

```bash
make dev-setup
Found Minikube ...
namespace/garden created
namespace/garden-dev created
deployment.apps/etcd created
service/etcd created
service/gardener-apiserver created
service/gardener-controller-manager created
endpoints/gardener-apiserver created
endpoints/gardener-controller-manager created
apiservice.apiregistration.k8s.io/v1alpha1.core.gardener.cloud created
apiservice.apiregistration.k8s.io/v1beta1.core.gardener.cloud created
validatingwebhookconfiguration.admissionregistration.k8s.io/gardener-controller-manager created
```

Optionally, you can switch off the `Logging` feature gate of Gardenlet to save resources:

```bash
sed -i -e 's/Logging: true/Logging: false/g' dev/20-componentconfig-gardenlet.yaml
```

The Gardener exposes the API servers of Shoot clusters via Kubernetes services of type `LoadBalancer`.
In order to establish stable endpoints (robust against changes of the load balancer address), it creates DNS records pointing to these load balancer addresses. They are used internally and by all cluster components to communicate.
You need to have control over a domain (or subdomain) for which these records will be created.
Please provide an *internal domain secret* (see [this](../../example/10-secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found [here](../usage/configuration.md).

```bash
kubectl apply -f example/10-secret-internal-domain-unmanaged.yaml
secret/internal-domain-unmanaged created
```

#### Run the Gardener

Next, run the Gardener API Server, the Gardener Controller Manager (optionally), the Gardener Scheduler (optionally), and the Gardenlet in different terminal windows/panes using rules in the `Makefile`.

```bash
make start-apiserver
Found Minikube ...
I0306 15:23:51.044421   74536 plugins.go:84] Registered admission plugin "ResourceReferenceManager"
I0306 15:23:51.044523   74536 plugins.go:84] Registered admission plugin "DeletionConfirmation"
[...]
I0306 15:23:51.626836   74536 secure_serving.go:116] Serving securely on [::]:8443
[...]
```

(Optional) Now you are ready to launch the Gardener Controller Manager.

```bash
make start-controller-manager
time="2019-03-06T15:24:17+02:00" level=info msg="Starting Gardener controller manager..."
time="2019-03-06T15:24:17+02:00" level=info msg="Feature Gates: "
time="2019-03-06T15:24:17+02:00" level=info msg="Starting HTTP server on 0.0.0.0:2718"
time="2019-03-06T15:24:17+02:00" level=info msg="Acquired leadership, starting controllers."
time="2019-03-06T15:24:18+02:00" level=info msg="Starting HTTPS server on 0.0.0.0:2719"
time="2019-03-06T15:24:18+02:00" level=info msg="Found internal domain secret internal-domain-unmanaged for domain nip.io."
time="2019-03-06T15:24:18+02:00" level=info msg="Successfully bootstrapped the Garden cluster."
time="2019-03-06T15:24:18+02:00" level=info msg="Gardener controller manager (version 1.0.0-dev) initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="ControllerRegistration controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="SecretBinding controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Project controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="Quota controller initialized."
time="2019-03-06T15:24:18+02:00" level=info msg="CloudProfile controller initialized."
[...]
```

(Optional) Now you are ready to launch the Gardener Scheduler.

```bash
make start-scheduler
time="2019-05-02T16:31:50+02:00" level=info msg="Starting Gardener scheduler ..."
time="2019-05-02T16:31:50+02:00" level=info msg="Starting HTTP server on 0.0.0.0:10251"
time="2019-05-02T16:31:50+02:00" level=info msg="Acquired leadership, starting scheduler."
time="2019-05-02T16:31:50+02:00" level=info msg="Gardener scheduler initialized (with Strategy: SameRegion)"
time="2019-05-02T16:31:50+02:00" level=info msg="Scheduler controller initialized."
[...]
```

(Optional) Now you are ready to launch the Gardenlet.

```bash
make start-gardenlet
time="2019-11-06T15:24:17+02:00" level=info msg="Starting Gardenlet..."
time="2019-11-06T15:24:17+02:00" level=info msg="Feature Gates: HVPA=true, Logging=true"
time="2019-11-06T15:24:17+02:00" level=info msg="Acquired leadership, starting controllers."
time="2019-11-06T15:24:18+02:00" level=info msg="Found internal domain secret internal-domain-unmanaged for domain nip.io."
time="2019-11-06T15:24:18+02:00" level=info msg="Gardenlet (version 1.0.0-dev) initialized."
time="2019-11-06T15:24:18+02:00" level=info msg="ControllerInstallation controller initialized."
time="2019-11-06T15:24:18+02:00" level=info msg="Shoot controller initialized."
time="2019-11-06T15:24:18+02:00" level=info msg="Seed controller initialized."
[...]
```

:warning: The Gardenlet will handle all your seeds for this development scenario, although, for productive usage it is recommended to run it once per seed, see [this document](../concepts/gardenlet.md) for more information.

Please checkout the [Gardener Extensions Manager](https://github.com/gardener/gem) to install extension controllers - make sure that you install all of them required for your local development.
Also, please refer to [this document](../extensions/controllerregistration.md) for further information about how extensions are registered in case you want to use other versions than the latest releases.

The Gardener should now be ready to operate on Shoot resources. You can use

```bash
kubectl get shoots
No resources found.
```

to operate against your local running Gardener API Server.

> Note: It may take several seconds until the `minikube` cluster recognizes that the Gardener API server has been started and is available. `No resources found` is the expected result of our initial development setup.

#### Limitations of local development setup

You can run Gardener (API server, controller manager, scheduler, gardenlet) against any local Kubernetes cluster, however, your seed and shoot clusters must be deployed to a "real" provider.
Currently, it is not possible to run Gardener entirely isolated from any cloud provider.
We are planning to support such a setup based on KubeVirt (see [this for details](https://github.com/gardener/gardener/issues/827)), however, it does not yet exist.
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

To make sure that a specific Seed cluster will be chosen, specify the `.spec.seedName` field (see [here](../../example/90-shoot.yaml#L265-L266) for an example Shoot manifest).

Please take a look at the [example manifests folder](../../example) to see which resource objects you need to install into your Garden cluster.
