# Gardener Resource Manager

Initially, the gardener-resource-manager was a project similar to the [kube-addon-manager](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/addon-manager).
It manages Kubernetes resources in a target cluster which means that it creates, updates, and deletes them.
Also, it makes sure that manual modifications to these resources are reconciled back to the desired state.

In the Gardener project we were using the kube-addon-manager since more than two years.
While we have progressed with our [extensibility story](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md) (moving cloud providers out-of-tree) we had decided that the kube-addon-manager is no longer suitable for this use-case.
The problem with it is that it needs to have its managed resources on its file system.
This requires storing the resources in `ConfigMap`s or `Secret`s and mounting them to the kube-addon-manager pod during deployment time.
The gardener-resource-manager uses `CustomResourceDefinition`s which allows to dynamically add, change, and remove resources with immediate action and without the need to reconfigure the volume mounts/restarting the pod.

Meanwhile, the `gardener-resource-manager` has evolved to a more generic component comprising several controllers and webhook handlers.
It is deployed by gardenlet once per seed (in the `garden` namespace) and once per shoot (in the respective shoot namespaces in the seed).

## Controllers

### `ManagedResource` controller

This controller watches custom objects called `ManagedResource`s in the `resources.gardener.cloud/v1alpha1` API group.
These objects contain references to secrets which itself contain the resources to be managed.
The reason why a `Secret` is used to store the resources is that they could contain confidential information like credentials.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: managedresource-example1
  namespace: default
type: Opaque
data:
  objects.yaml: YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnTWFwCm1ldGFkYXRhOgogIG5hbWU6IHRlc3QtMTIzNAogIG5hbWVzcGFjZTogZGVmYXVsdAotLS0KYXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnTWFwCm1ldGFkYXRhOgogIG5hbWU6IHRlc3QtNTY3OAogIG5hbWVzcGFjZTogZGVmYXVsdAo=
    # apiVersion: v1
    # kind: ConfigMap
    # metadata:
    #   name: test-1234
    #   namespace: default
    # ---
    # apiVersion: v1
    # kind: ConfigMap
    # metadata:
    #   name: test-5678
    #   namespace: default
---
apiVersion: resources.gardener.cloud/v1alpha1
kind: ManagedResource
metadata:
  name: example
  namespace: default
spec:
  secretRefs:
  - name: managedresource-example1
```

In the above example, the controller creates two `ConfigMap`s in the `default` namespace.
When a user is manually modifying them they will be reconciled back to the desired state stored in the `managedresource-example` secret.

It is also possible to inject labels into all the resources:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: managedresource-example2
  namespace: default
type: Opaque
data:
  other-objects.yaml: YXBpVmVyc2lvbjogYXBwcy92MSAjIGZvciB2ZXJzaW9ucyBiZWZvcmUgMS45LjAgdXNlIGFwcHMvdjFiZXRhMgpraW5kOiBEZXBsb3ltZW50Cm1ldGFkYXRhOgogIG5hbWU6IG5naW54LWRlcGxveW1lbnQKc3BlYzoKICBzZWxlY3RvcjoKICAgIG1hdGNoTGFiZWxzOgogICAgICBhcHA6IG5naW54CiAgcmVwbGljYXM6IDIgIyB0ZWxscyBkZXBsb3ltZW50IHRvIHJ1biAyIHBvZHMgbWF0Y2hpbmcgdGhlIHRlbXBsYXRlCiAgdGVtcGxhdGU6CiAgICBtZXRhZGF0YToKICAgICAgbGFiZWxzOgogICAgICAgIGFwcDogbmdpbngKICAgIHNwZWM6CiAgICAgIGNvbnRhaW5lcnM6CiAgICAgIC0gbmFtZTogbmdpbngKICAgICAgICBpbWFnZTogbmdpbng6MS43LjkKICAgICAgICBwb3J0czoKICAgICAgICAtIGNvbnRhaW5lclBvcnQ6IDgwCg==
    # apiVersion: apps/v1
    # kind: Deployment
    # metadata:
    #   name: nginx-deployment
    # spec:
    #   selector:
    #     matchLabels:
    #       app: nginx
    #   replicas: 2 # tells deployment to run 2 pods matching the template
    #   template:
    #     metadata:
    #       labels:
    #         app: nginx
    #     spec:
    #       containers:
    #       - name: nginx
    #         image: nginx:1.7.9
    #         ports:
    #         - containerPort: 80

---
apiVersion: resources.gardener.cloud/v1alpha1
kind: ManagedResource
metadata:
  name: example
  namespace: default
spec:
  secretRefs:
  - name: managedresource-example2
  injectLabels:
    foo: bar
```

In this example the label `foo=bar` will be injected into the `Deployment` as well as into all created `ReplicaSet`s and `Pod`s.

#### Modes

The gardener-resource-manager can manage a resource in different modes. The supported modes are:
- `Ignore`
    - The corresponding resource is removed from the ManagedResource status (`.status.resources`). No action is performed on the cluster - the resource is no longer "managed" (updated or deleted).
    - The primary use case is a migration of a resource from one ManagedResource to another one.

The mode for a resource can be specified with the `resources.gardener.cloud/mode` annotation. The annotation should be specified in the encoded resource manifest in the Secret that is referenced by the ManagedResource.

#### Resource Class

By default, gardener-resource-manager controller watches for ManagedResources in all namespaces. `--namespace` flag can be specified to gardener-resource-manager binary to restrict the watch to ManagedResources in a single namespace.
A ManagedResource has an optional `.spec.class` field that allows to indicate that it belongs to given class of resources. `--resource-class` flag can be specified to gardener-resource-manager binary to restrict the watch to ManagedResources with the given `.spec.class`. A default class is assumed if no class is specified.

#### Conditions

A ManagedResource has a ManagedResourceStatus, which has an array of Conditions. Conditions currently include:

| Condition          | Description                                               |
| ------------------ | --------------------------------------------------------- |
| `ResourcesApplied` | `True` if all resources are applied to the target cluster |
| `ResourcesHealthy` | `True` if all resources are present and healthy           |

`ResourcesApplied` may be `False` when:
- the resource `apiVersion` is not known to the target cluster
- the resource spec is invalid (for example the label value does not match the required regex for it)
- ...

`ResourcesHealthy` may be `False` when:
- the resource is not found
- the resource is a Deployment and the Deployment does not have the minimum availability.
- ...

Each Kubernetes resources has different notion for being healthy. For example, a Deployment is considered healthy if the controller observed its current revision and if the number of updated replicas is equal to the number of replicas.

The following section describes a healthy ManagedResource:

```json
"conditions": [
  {
    "type": "ResourcesApplied",
    "status": "True",
    "reason": "ApplySucceeded",
    "message": "All resources are applied.",
    "lastUpdateTime": "2019-09-09T11:31:21Z",
    "lastTransitionTime": "2019-09-08T19:53:23Z"
  },
  {
    "type": "ResourcesHealthy",
    "status": "True",
    "reason": "ResourcesHealthy",
    "message": "All resources are healthy.",
    "lastUpdateTime": "2019-09-09T11:31:21Z",
    "lastTransitionTime": "2019-09-09T11:31:21Z"
  }
]  
```

#### Ignoring Updates

In some cases it is not desirable to update or re-apply some of the cluster components (for example, if customization is required or needs to be applied by the end-user).
For these resources, the annotation "resources.gardener.cloud/ignore" needs to be set to "true" or a truthy value (Truthy values are "1", "t", "T", "true", "TRUE", "True") in the corresponding managed resource secrets,
this can be done from the components that create the managed resource secrets, for example Gardener extensions or Gardener. Once this is done, the resource will be initially created and later ignored during reconciliation.

#### Origin

All the objects managed by the resource manager get a dedicated annotation
`resources.gardener.cloud/origin` describing the `ManagedResource` object that describes
this object.

By default this is in this format &lt;namespace&gt;/&lt;objectname&gt;.
In multi-cluster scenarios (the `ManagedResource` objects are maintained in a
cluster different from the one the described objects are managed), it might
be useful to include the cluster identity, as well.

This can be enforced by setting the `--cluster-id` option. Here, several
possibilities are supported:
- given a direct value: use this as id for the source cluster
- `<cluster>`: read the cluster identity from a `cluster-identity` config map
  in the `kube-system` namespace (attribute `cluster-identity`). This is
  automatically maintained in all clusters managed or involved in a gardener landscape.
- `<default>`: try to read the cluster identity from the config map. If not found,
  no identity is used
- empty string: no cluster identity is used (completely cluster local scenarios)

The format of the origin annotation with a cluster id is &lt;cluster id&gt;:&lt;namespace&gt;/&lt;objectname&gt;.

The default for the cluster id is the empty value (do not use cluster id).

### Garbage Collector For Immutable `ConfigMap`s/`Secret`s

In Kubernetes, workload resources (e.g., `Pod`s) can mount `ConfigMap`s or `Secret`s or reference them via environment variables in containers.
Typically, when the content of such `ConfigMap`/`Secret` gets changed then the respective workload is usually not dynamically reloading the configuration, i.e., a restart is required.
The most commonly used approach is probably having so-called [checksum annotations in the pod template](https://helm.sh/docs/howto/charts_tips_and_tricks/#automatically-roll-deployments) which makes Kubernetes to recreate the pod if the checksum changes.
However, it has the downside that old, still running versions of the workload might not be able to properly work with the already updated content in the `ConfigMap`/`Secret`, potentially causing application outages.

In order to protect users from such outages (and to also improve the performance of the cluster), the Kubernetes community provides the ["immutable `ConfigMap`s/`Secret`s feature"](https://kubernetes.io/docs/concepts/configuration/configmap/#configmap-immutable).
Enabling immutability requires `ConfigMap`s/`Secret`s to have unique names.
Having unique names requires the client to delete `ConfigMap`s`/`Secret`s no longer in use.

In order to provide a similarly lightweight experience for clients (compared to the well-established checksum annotation approach), the Gardener Resource Manager features an optional garbage collector controller (disabled by default).
The purpose of this controller is cleaning up such immutable `ConfigMap`s/`Secret`s if they are no longer in use.

#### How does the garbage collector work?

The following algorithm is implemented in the GC controller:

1. List all `ConfigMap`s and `Secret`s labeled with `resources.gardener.cloud/garbage-collectable-reference=true`.
1. List all `Deployment`s, `StatefulSet`s, `DaemonSet`s, `Job`s, `CronJob`s, `Pod`s and for each of them
    1. iterate over the `.metadata.annotations` and for each of them
        1. If the annotation key follows the `reference.resources.gardener.cloud/{configmap,secret}-<hash>` scheme and the value equals `<name>` then consider it as "in-use".
1. Delete all `ConfigMap`s and `Secret`s not considered as "in-use".

Consequently, clients need to

1. Create immutable `ConfigMap`s/`Secret`s with unique names (e.g., a checksum suffix based on the `.data`).
1. Label such `ConfigMap`s/`Secret`s with `resources.gardener.cloud/garbage-collectable-reference=true`.
1. Annotate their workload resources with `reference.resources.gardener.cloud/{configmap,secret}-<hash>=<name>` for all `ConfigMap`s/`Secret`s used by the containers of the respective `Pod`s.

   ⚠️ Add such annotations to `.metadata.annotations` as well as to all templates of other resources (e.g., `.spec.template.metadata.annotations` in `Deployment`s or `.spec.jobTemplate.metadata.annotations` and `.spec.jobTemplate.spec.template.metadata.annotations` for `CronJob`s.
   This ensures that the GC controller does not unintentionally consider `ConfigMap`s/`Secret`s as "not in use" just because there isn't a `Pod` referencing them anymore (e.g., they could still be used by a `Deployment` scaled down to `0`).

ℹ️ For the last step, there is a helper function `InjectAnnotations` in the `pkg/controller/garbagecollector/references` which you can use for your convenience.

**Example:**

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-1234
  namespace: default
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-5678
  namespace: default
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
---
apiVersion: v1
kind: Pod
metadata:
  name: example
  namespace: default
  annotations:
    reference.resources.gardener.cloud/configmap-82a3537f: test-5678
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
    terminationGracePeriodSeconds: 2
```

The GC controller would delete the `ConfigMap/test-1234` because it is considered as not "in-use".

ℹ️ If the GC controller is activated then the `ManagedResource` controller will no longer delete `ConfigMap`s/`Secret`s having the above label.

#### How to activate the garbage collector?

The GC controller can be activated by providing the `--garbage-collector-sync-period` flag with a value larger than `0` (e.g., `1h`) to the Gardener Resource Manager.
