# Deploying the Gardener into a Minikube with Vagrant Provider

## Prerequisites

Make sure that you have completed the following steps:

- [Installing Golang environment](../development/local_setup.md#installing-golang-environment)
- [Installing `kubectl` and `helm`](../development/local_setup.md#installing-kubectl-and-helm)
- [Installing Minikube](../development/local_setup.md#installing-minikube)
- [Installing iproute2](../development/local_setup.md#installing-iproute2)
- [Installing `Vagrant`](../development/local_setup.md#installing-vagrant)
- [Installing `Virtualbox`](../development/local_setup.md#installing-virtualbox)
- [Get the sources](../development/local_setup.md#get-the-sources)

## Running

Then in a terminal execute:

```bash
# Minikube requires extra memory for this setup
$ minikube start --cpus=3 --memory=4096 --extra-config=apiserver.Authorization.Mode=RBAC

# Allow Tiller and Dashboard to run in RBAC mode
$ kubectl create clusterrolebinding add-on-cluster-admin --clusterrole=cluster-admin --serviceaccount=kube-system:default

# Start Helm's Tiller
$ helm init

# Deploy Gardener with vagrant specific configuration
$ helm install charts/gardener \
  --name gardener \
  --namespace garden \
  --values=charts/gardener/values.yaml \
  --values=charts/gardener/vagrant-local-values.yaml

# Check that everything is deployed successfuly and running without a problem
$ kubectl -n garden get pods
NAME                                           READY     STATUS    RESTARTS   AGE
gardener-apiserver-d5989f856-swgbg             2/2       Running   0          32s
gardener-controller-manager-6f7bd556d6-p98fx   1/1       Running   0          32s

$ make dev-setup-vagrant
namespace "garden-dev" created
cloudprofile "vagrant" created
[..]
```

You can now continue with [Check Vagrant Setup](../development/local_setup.md#check-vagrant-setup)
