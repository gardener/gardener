# Overview

Conceptually, all Gardener components are designed to run as a Pod inside a Kubernetes cluster.
The Gardener API server extends the Kubernetes API via the user-aggregated API server concepts.
However, if you want to develop it, you may want to work locally with the Gardener without building a Docker image and deploying it to a cluster each and every time.
That means that the Gardener runs outside a Kubernetes cluster which requires providing a [Kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/authenticate-across-clusters-kubeconfig/) in your local filesystem and point the Gardener to it when starting it (see below).

Further details can be found in

1. [Principles of Kubernetes](https://kubernetes.io/docs/concepts/), and its [components](https://kubernetes.io/docs/concepts/overview/components/)
1. [Kubernetes Development Guide](https://github.com/kubernetes/community/tree/master/contributors/devel)
1. [Architecture of Gardener](https://github.com/gardener/documentation/wiki/Architecture)

This guide is split into three main parts:
* [Preparing your setup by installing all dependencies and tools](#preparing-the-setup)
* [Building and starting Gardener components locally](#start-gardener-locally)
* [Using your local Gardener setup to create a Shoot](#create-a-shoot)

# Preparing the Setup

## [macOS only] Installing homebrew

The copy-paste instructions in this guide are designed for macOS and use the package manager [Homebrew](https://brew.sh/).

On macOS run
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
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

Install `kubectl`. Please make sure that the version of `kubectl` is at least `v1.20.x`. On macOS run

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

```bash
brew install jq
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

# Start Gardener Locally

## Get the Sources

Clone the repository from GitHub into your `$GOPATH`.

```bash
mkdir -p $(go env GOPATH)/src/github.com/gardener
cd $(go env GOPATH)/src/github.com/gardener
git clone git@github.com:gardener/gardener.git
cd gardener
```

> Note: Gardener is using Go modules and cloning the repository into `$GOPATH` is not a hard requirement. However it is still recommended to clone into `$GOPATH` because `k8s.io/code-generator` does not work yet outside of `$GOPATH` - [kubernetes/kubernetes#86753](https://github.com/kubernetes/kubernetes/issues/86753).

## Start the Gardener

ℹ️ In the following guide, you have to define the configuration (`CloudProfile`s, `SecretBinding`s, `Seed`s, etc.) manually for the infrastructure environment you want to develop against.
Additionally, you have to register the respective Gardener extensions manually.
If you are rather looking for a quick start guide to develop entirely locally on your machine (no real cloud provider or infrastructure involved), then you should rather follow [this guide](getting_started_locally.md).

### Start a Local Kubernetes Cluster

For the development of Gardener you need a Kubernetes API server on which you can register Gardener's own Extension API Server as `APIService`. This cluster doesn't need any worker nodes to run pods, though, therefore, you can use the "nodeless Garden cluster setup" residing in `hack/local-garden`. This will start all minimally required components of a Kubernetes cluster (`etcd`, `kube-apiserver`, `kube-controller-manager`)
and an `etcd` Instance for the `gardener-apiserver` as Docker containers. This is the easiest way to get your
Gardener development setup up and running.

**Using the nodeless cluster setup**

Use the provided Makefile rules to start your local Garden:
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

ℹ️ [Optional] If you want to develop the `SeedAuthorization` feature then you have to run `make ACTIVATE_SEEDAUTHORIZER=true local-garden-up`. However, please note that this forces you to start the `gardener-admission-controller` via `make start-admission-controller`.

To tear down the local Garden cluster and remove the Docker containers, simply run:
```bash
make local-garden-down
```

<details>
  <summary><b>Alternative: Using a local Kubernetes cluster</b></summary>

  Instead of starting a Kubernetes API server and etcd as docker containers, you can also opt for running a local Kubernetes cluster, provided by e.g. [minikube](https://minikube.sigs.k8s.io/docs/start/), [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) or docker desktop.

  > Note: Gardener requires self-contained kubeconfig files because of a [security issue](https://banzaicloud.com/blog/kubeconfig-security/). You can configure your minikube to create self-contained kubeconfig files via:
  > ```bash
  > minikube config set embed-certs true
  > ```
  > or when starting the local cluster
  > ```bash
  > minikube start --embed-certs
  > ```

</details>

<details>
  <summary><b>Alternative: Using a remote Kubernetes cluster</b></summary>

For some testing scenarios, you may want to use a remote cluster instead of a local one as your Garden cluster.
To do this, you can use the "remote Garden cluster setup" residing in `hack/remote-garden`. This will start an `etcd` instance for the `gardener-apiserver` as a Docker container, and open tunnels for accessing local gardener components from the remote cluster.

To avoid mistakes, the remote cluster must have a `garden` namespace labeled with `gardener.cloud/purpose=remote-garden`.
You must create the `garden` namespace and label it manually before running `make remote-garden-up` as described below.

Use the provided `Makefile` rules to bootstrap your remote Garden:

```bash
export KUBECONFIG=<path to kubeconfig>
make remote-garden-up
[...]
# Start gardener etcd used to store gardener resources (e.g., seeds, shoots)
Starting gardener-dev-remote gardener-etcd cluster!
[...]
# Open tunnels for accessing local gardener components from the remote cluster
[...]
```

To close the tunnels and remove the locally-running Docker containers, run:

```bash
make remote-garden-down
```

ℹ️ [Optional] If you want to use the remote Garden cluster setup with the `SeedAuthorization` feature, you have to adapt the `kube-apiserver` process of your remote Garden cluster. To do this, perform the following steps after running `make remote-garden-up`:

* Create an [authorization webhook configuration file](https://kubernetes.io/docs/reference/access-authn-authz/webhook/#configuration-file-format) using the IP of the `garden/quic-server` pod running in your remote Garden cluster and port 10444 that tunnels to your locally running `gardener-admission-controller` process.

  ```yaml
  apiVersion: v1
  kind: Config
  current-context: seedauthorizer
  clusters:
  - name: gardener-admission-controller
    cluster:
      insecure-skip-tls-verify: true
      server: https://<quic-server-pod-ip>:10444/webhooks/auth/seed
  users:
  - name: kube-apiserver
    user: {}
  contexts:
  - name: seedauthorizer
    context:
      cluster: gardener-admission-controller
      user: kube-apiserver
  ```
* Change or add the following command line parameters to your `kube-apiserver` process:
  - `--authorization-mode=<...>,Webhook`
  - `--authorization-webhook-config-file=<path to config file>`
  - `--authorization-webhook-cache-authorized-ttl=0`
  - `--authorization-webhook-cache-unauthorized-ttl=0`
* Delete the cluster role and rolebinding `gardener.cloud:system:seeds` from your remote Garden cluster.

If your remote Garden cluster is a Gardener shoot, and you can access the seed on which this shoot is scheduled, you can automate the above steps by running the [`enable-seed-authorizer` script](../../hack/local-development/remote-garden/enable-seed-authorizer) and passing the kubeconfig of the seed cluster and the shoot namespace as parameters:

```bash
hack/local-development/remote-garden/enable-seed-authorizer <seed kubeconfig> <namespace>
```

> Note: This script is not working anymore, as the `ReversedVPN` feature can't be disabled. The annotation `alpha.featuregates.shoot.gardener.cloud/reversed-vpn` on `Shoot`s is no longer respected.

To prevent Gardener from reconciling the shoot and overwriting your changes, add the annotation `shoot.gardener.cloud/ignore: 'true'` to the remote Garden shoot. Note that this annotation takes effect only if it is enabled via the `constollers.shoot.respectSyncPeriodOverwrite: true` option in the `gardenlet` configuration.

To disable the seed authorizer again, run the same script with `-d` as a third parameter:

```bash
hack/local-development/remote-garden/enable-seed-authorizer <seed kubeconfig> <namespace> -d
```

If the seed authorizer is enabled, you also have to start the `gardener-admission-controller` via `make start-admission-controller`.

> ⚠️ In the remote garden setup all Gardener components run with administrative permissions, i.e., there is no fine-grained access control via RBAC (as opposed to productive installations of Gardener).

</details>

### Prepare the Gardener

Now, that you have started your local cluster, we can go ahead and register the Gardener API Server.
Just point your `KUBECONFIG` environment variable to the cluster you created in the previous step and run:

```bash
make dev-setup
[...]
namespace/garden created
namespace/garden-dev created
deployment.apps/etcd created
service/etcd created
service/gardener-apiserver created
service/gardener-admission-controller created
endpoints/gardener-apiserver created
endpoints/gardener-admission-controller created
apiservice.apiregistration.k8s.io/v1beta1.core.gardener.cloud created
apiservice.apiregistration.k8s.io/v1alpha1.seedmanagement.gardener.cloud created
apiservice.apiregistration.k8s.io/v1alpha1.settings.gardener.cloud created
```

ℹ️ [Optional] If you want to enable logging, in the gardenlet configuration add:
```yaml
logging:
  enabled: true
```

The Gardener exposes the API servers of Shoot clusters via Kubernetes services of type `LoadBalancer`.
In order to establish stable endpoints (robust against changes of the load balancer address), it creates DNS records pointing to these load balancer addresses. They are used internally and by all cluster components to communicate.
You need to have control over a domain (or subdomain) for which these records will be created.
Please provide an *internal domain secret* (see [this](../../example/10-secret-internal-domain.yaml) for an example) which contains credentials with the proper privileges. Further information can be found in [Gardener Configuration and Usage](../usage/configuration.md).

```bash
kubectl apply -f example/10-secret-internal-domain-unmanaged.yaml
secret/internal-domain-unmanaged created
```

### Run the Gardener

Next, run the Gardener API Server, the Gardener Controller Manager (optionally), the Gardener Scheduler (optionally), and the gardenlet in different terminal windows/panes using rules in the `Makefile`.

```bash
make start-apiserver
[...]
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

The Gardener should now be ready to operate on Shoot resources. You can use

```bash
kubectl get shoots
No resources found.
```

to operate against your local running Gardener API Server.

> Note: It may take several seconds until the Gardener API server has been started and is available. `No resources found` is the expected result of our initial development setup.

# Create a Shoot

The steps below describe the general process of creating a Shoot. Have in mind that the steps do not provide full example manifests. The reader needs to check the provider documentation and adapt the manifests accordingly.

#### 1. Copy the Example Manifests

The next steps require modifications of the example manifests. These modifications are part of local setup and should not be `git push`-ed. To do not interfere with git, let's copy the example manifests to `dev/` which is ignored by git.

```bash
cp example/*.yaml dev/
```

#### 2. Create a Project

Every Shoot is associated with a Project. Check the corresponding example manifests `dev/00-namespace-garden-dev.yaml` and `dev/05-project-dev.yaml`. Adapt them and create them.

```bash
kubectl apply -f dev/00-namespace-garden-dev.yaml
kubectl apply -f dev/05-project-dev.yaml
```

Make sure that the Project is successfully reconciled:

```bash
$ kubectl get project dev
NAME   NAMESPACE    STATUS   OWNER                  CREATOR            AGE
dev    garden-dev   Ready    john.doe@example.com   kubernetes-admin   6s
```

#### 3. Create a CloudProfile

The `CloudProfile` resource is provider specific and describes the underlying cloud provider (available machine types, regions, machine images, etc.). Check the corresponding example manifest `dev/30-cloudprofile.yaml`. Check also the documentation and example manifests of the provider extension. Adapt `dev/30-cloudprofile.yaml` and apply it.

```bash
kubectl apply -f dev/30-cloudprofile.yaml
```

#### 4. Install Necessary Gardener Extensions

The [Known Extension Implementations](../../extensions/README.md#known-extension-implementations) section contains a list of available extension implementations. You need to create a ControllerRegistration and ControllerDeployment for:
* at least one infrastructure provider
* a DNS provider (if the DNS for the Seed is not disabled)
* at least one operating system extension
* at least one network plugin extension

As a convention, the example ControllerRegistration manifest (containing also the necessary ControllerDeployment) for an extension is located under `example/controller-registration.yaml` in the corresponding repository (for example for AWS the ControllerRegistration can be found [here](https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/controller-registration.yaml)). An example creation for provider-aws (make sure to replace `<version>` with the newest released version tag):

```bash
kubectl apply -f https://raw.githubusercontent.com/gardener/gardener-extension-provider-aws/<version>/example/controller-registration.yaml
```

Instead of updating extensions manually you can use [Gardener Extensions Manager](https://github.com/gardener/gem) to install and update extension controllers. This is especially useful if you want to keep and maintain your development setup for a longer time.
Also, please refer to [Registering Extension Controllers](../extensions/controllerregistration.md) for further information about how extensions are registered in case you want to use other versions than the latest releases.

#### 5. Register a Seed

Shoot controlplanes run in seed clusters, so we need to create our first Seed now.

Check the corresponding example manifest `dev/40-secret-seed.yaml` and `dev/50-seed.yaml`. Update `dev/40-secret-seed.yaml` with base64 encoded kubeconfig of the cluster that will be used as Seed (the scope of the permissions should be identical to the kubeconfig that the gardenlet creates during bootstrapping - for now, `cluster-admin` privileges are recommended).

```bash
kubectl apply -f dev/40-secret-seed.yaml
```

Adapt `dev/50-seed.yaml` - adjust `.spec.secretRef` to refer the newly created Secret, adjust `.spec.provider` with the Seed cluster provider and revise the other fields.

```bash
kubectl apply -f dev/50-seed.yaml
```

#### 6. Start the gardenlet

Once the Seed is created, start the gardenlet to reconcile it. The `make start-gardenlet` command will automatically configure the local gardenlet process to use the Seed and its kubeconfig. If you have multiple Seeds, you have to specify which to use by setting the `SEED_NAME` environment variable like in `make start-gardenlet SEED_NAME=my-first-seed`.

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

The gardenlet will now reconcile the Seed. Check the progess from time to time until it's `Ready`:

```bash
kubectl get seed
NAME       STATUS    PROVIDER    REGION      AGE    VERSION       K8S VERSION
seed-aws   Ready     aws         eu-west-1   4m     v1.61.0-dev   v1.24.8
```

#### 7. Create a Shoot

A Shoot requires a SecretBinding. The SecretBinding refers to a Secret that contains the cloud provider credentials. The Secret data keys are provider specific and you need to check the documentation of the provider to find out which data keys are expected (for example for AWS the related documentation can be found at [Provider Secret Data](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage-as-end-user.md#provider-secret-data)). Adapt `dev/70-secret-provider.yaml` and `dev/80-secretbinding.yaml` and apply them.

```bash
kubectl apply -f dev/70-secret-provider.yaml
kubectl apply -f dev/80-secretbinding.yaml
```

After the SecretBinding creation, you are ready to proceed with the Shoot creation. You need to check the documentation of the provider to find out the expected configuration (for example for AWS the related documentation and example Shoot manifest can be found at [Using the AWS provider extension with Gardener as end-user](https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage-as-end-user.md)). Adapt `dev/90-shoot.yaml` and apply it.

To make sure that a specific Seed cluster will be chosen or to skip the scheduling (the sheduling requires Gardener Scheduler to be running), specify the `.spec.seedName` field (see [here](../../example/90-shoot.yaml#L317-L318)).

```bash
kubectl apply -f dev/90-shoot.yaml
```

Watch the progress of the operation and make sure that the Shoot will be successfully created.

```bash
watch kubectl get shoot --all-namespaces
```
