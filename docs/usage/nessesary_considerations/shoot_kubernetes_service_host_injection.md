---
title: KUBERNETES_SERVICE_HOST Environment Variable Injection
weight: 4
---

# `KUBERNETES_SERVICE_HOST` Environment Variable Injection

In each Shoot cluster's `kube-system` namespace a `DaemonSet` called `apiserver-proxy` is deployed. It routes traffic to the upstream Shoot Kube APIServer. See the [APIServer SNI GEP](../proposals/08-shoot-apiserver-via-sni.md) for more details.

To skip this extra network hop, a [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) called `apiserver-proxy.networking.gardener.cloud` is deployed next to the API server in the Seed. It adds a `KUBERNETES_SERVICE_HOST` environment variable to each container and init container that do not specify it. See the webhook [repository](https://github.com/gardener/apiserver-proxy/) for more information.

## Opt-Out of Pod Injection

In some cases it's desirable to opt-out of Pod injection:

- DNS is disabled on that individual Pod, but it still needs to talk to the kube-apiserver.
- Want to test the `kube-proxy` and `kubelet` in-cluster discovery.

### Opt-Out of Pod Injection for Specific Pods

To opt out of the injection, the Pod should be labeled with `apiserver-proxy.networking.gardener.cloud/inject: disable`, e.g.:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
        apiserver-proxy.networking.gardener.cloud/inject: disable
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
```

### Opt-Out of Pod Injection on Namespace Level

To opt out of the injection of **all** Pods in a namespace, you should label your namespace with `apiserver-proxy.networking.gardener.cloud/inject: disable`, e.g.:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    apiserver-proxy.networking.gardener.cloud/inject: disable
  name: my-namespace
```

or via `kubectl` for existing namespace:

```console
kubectl label namespace my-namespace apiserver-proxy.networking.gardener.cloud/inject=disable
```

> **Note:** Please be aware that it's not possible to disable injection on a namespace level and enable it for individual pods in it.

### Opt-Out of Pod Injection for the Entire Cluster

If the injection is causing problems for different workloads and ignoring individual pods or namespaces is not possible, then the feature could be disabled for the entire cluster with the `alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector` annotation with value `disable` on the `Shoot` resource itself:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  annotations:
    alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector: 'disable'
  name: my-cluster
```

or via `kubectl` for existing shoot cluster:

```console
kubectl label shoot my-cluster alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector=disable
```

> **Note:** Please be aware that it's not possible to disable injection on a cluster level and enable it for individual pods in it.
