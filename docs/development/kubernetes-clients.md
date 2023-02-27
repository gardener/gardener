# Kubernetes Clients in Gardener

This document aims at providing a general developer guideline on different aspects of using Kubernetes clients in a large-scale distributed system and project like Gardener.
The points included here are not meant to be consulted as absolute rules, but rather as general rules of thumb that allow developers to get a better feeling about certain gotchas and caveats.
It should be updated with lessons learned from maintaining the project and running Gardener in production.

## Prerequisites:

Please familiarize yourself with the following basic Kubernetes API concepts first, if you're new to Kubernetes. A good understanding of these basics will help you better comprehend the following document.

- [Kubernetes API Concepts](https://kubernetes.io/docs/reference/using-api/api-concepts/) (including terminology, watch basics, etc.)
- [Extending the Kubernetes API](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/) (including Custom Resources and aggregation layer / extension API servers)
- [Extend the Kubernetes API with CustomResourceDefinitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [Working with Kubernetes Objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/)
- [Sample Controller](https://github.com/kubernetes/sample-controller/blob/master/docs/controller-client-go.md) (the diagram helps to build an understanding of an controller's basic structure)

## Client Types: Client-Go, Generated, Controller-Runtime

For historical reasons, you will find different kinds of Kubernetes clients in Gardener:

### Client-Go Clients

[client-go](https://github.com/kubernetes/client-go) is the default/official client for talking to the Kubernetes API in Golang.
It features the so called ["client sets"](https://github.com/kubernetes/client-go/blob/release-1.21/kubernetes/clientset.go#L72) for all built-in Kubernetes API groups and versions (e.g. `v1` (aka `core/v1`), `apps/v1`).
client-go clients are generated from the built-in API types using [client-gen](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/generating-clientset.md) and are composed of interfaces for every known API GroupVersionKind.
A typical client-go usage looks like this:  
```go
var (
  ctx        context.Context
  c          kubernetes.Interface // "k8s.io/client-go/kubernetes"
  deployment *appsv1.Deployment   // "k8s.io/api/apps/v1"
)

updatedDeployment, err := c.AppsV1().Deployments("default").Update(ctx, deployment, metav1.UpdateOptions{})
```

_Important characteristics of client-go clients:_

- clients are specific to a given API GroupVersionKind, i.e., clients are hard-coded to corresponding API-paths (don't need to use the discovery API to map GVK to a REST endpoint path).
- client's don't modify the passed in-memory object (e.g. `deployment` in the above example). Instead, they return a new in-memory object.  
  This means that controllers have to continue working with the new in-memory object or overwrite the shared object to not lose any state updates.

### Generated Client Sets for Gardener APIs

Gardener's APIs extend the Kubernetes API by registering an extension API server (in the garden cluster) and `CustomResourceDefinition`s (on Seed clusters), meaning that the Kubernetes API will expose additional REST endpoints to manage Gardener resources in addition to the built-in API resources.
In order to talk to these extended APIs in our controllers and components, client-gen is used to generate client-go-style clients to [`pkg/client/{core,extensions,seedmanagement,...}`](../../pkg/client).

Usage of these clients is equivalent to `client-go` clients, and the same characteristics apply. For example:

```go
var (
  ctx   context.Context
  c     gardencoreclientset.Interface // "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
  shoot *gardencorev1beta1.Shoot      // "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

updatedShoot, err := c.CoreV1beta1().Shoots("garden-my-project").Update(ctx, shoot, metav1.UpdateOptions{})
```

### Controller-Runtime Clients

[controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) is a Kubernetes community project ([kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) subproject) for building controllers and operators for custom resources.
Therefore, it features a generic client that follows a different approach and does not rely on generated client sets. Instead, the client can be used for managing any Kubernetes resources (built-in or custom) homogeneously.
For example:

```go
var (
  ctx        context.Context
  c          client.Client            // "sigs.k8s.io/controller-runtime/pkg/client"
  deployment *appsv1.Deployment       // "k8s.io/api/apps/v1"
  shoot      *gardencorev1beta1.Shoot // "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

err := c.Update(ctx, deployment)
// or
err = c.Update(ctx, shoot)
```

A brief introduction to controller-runtime and its basic constructs can be found at the [official Go documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime).

_Important characteristics of controller-runtime clients:_

- The client functions take a generic `client.Object` or `client.ObjectList` value. These interfaces are implemented by all Golang types, that represent Kubernetes API objects or lists respectively which can be interacted with via usual API requests. [1]
- The client first consults a `runtime.Scheme` (configured during client creation) for recognizing the object's `GroupVersionKind` (this happens on the client-side only).  
  A `runtime.Scheme` is basically a registry for Golang API types, defaulting and conversion functions. Schemes are usually provided per `GroupVersion` (see [this example](https://github.com/kubernetes/api/blob/release-1.21/apps/v1/register.go) for `apps/v1`) and can be combined to one single scheme for further usage ([example](https://github.com/gardener/gardener/blob/v1.29.0/pkg/client/kubernetes/types.go#L96)). In controller-runtime clients, schemes are used only for mapping a typed API object to its `GroupVersionKind`.
- It then consults a `meta.RESTMapper` (also configured during client creation) for mapping the `GroupVersionKind` to a `RESTMapping`, which contains the `GroupVersionResource` and `Scope` (namespaced or cluster-scoped). From these values, the client can unambiguously determine the REST endpoint path of the corresponding API resource. For instance: `appsv1.DeploymentList` is available at `/apis/apps/v1/deployments` or `/apis/apps/v1/namespaces/<namespace>/deployments` respectively.
  - There are different `RESTMapper` implementations, but generally they are talking to the API server's discovery API for retrieving `RESTMappings` for all API resources known to the API server (either built-in, registered via API extension or `CustomResourceDefinition`s).
  - The default implementation of a controller-runtime (which Gardener uses as well) is the [dynamic `RESTMapper`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.9.0/pkg/client/apiutil/dynamicrestmapper.go#L77). It caches discovery results (i.e. `RESTMappings`) in-memory and only re-discovers resources from the API server when a client tries to use an unknown `GroupVersionKind`, i.e., when it encounters a `No{Kind,Resource}MatchError`.
- The client writes back results from the API server into the passed in-memory object.
  - This means that controllers don't have to worry about copying back the results and should just continue to work on the given in-memory object.
  - This is a nice and flexible pattern, and helper functions should try to follow it wherever applicable. Meaning, if possible accept an object param, pass it down to clients and keep working on the same in-memory object instead of creating a new one in your helper function.
  - The benefit is that you don't lose updates to the API object and always have the last-known state in memory. Therefore, you don't have to read it again, e.g., for getting the current `resourceVersion` when working with [optimistic locking](#conflicts-concurrency-control-and-optimistic-locking), and thus minimize the chances for running into conflicts.
  - However, controllers *must not* use the same in-memory object concurrently in multiple goroutines. For example, decoding results from the API server in multiple goroutines into the same maps (e.g., labels, annotations) will cause panics because of "concurrent map writes". Also, reading from an in-memory API object in one goroutine while decoding into it in another goroutine will yield non-atomic reads, meaning data might be corrupt and represent a non-valid/non-existing API object.
  - Therefore, if you need to use the same in-memory object in multiple goroutines concurrently (e.g., shared state), remember to leverage proper synchronization techniques like channels, mutexes, `atomic.Value` and/or copy the object prior to use. The average controller however, will not need to share in-memory API objects between goroutines, and it's typically an indicator that the controller's design should be improved.
- The client decoder erases the object's `TypeMeta` (`apiVersion` and `kind` fields) after retrieval from the API server, see [kubernetes/kubernetes#80609](https://github.com/kubernetes/kubernetes/issues/80609), [kubernetes-sigs/controller-runtime#1517](https://github.com/kubernetes-sigs/controller-runtime/issues/1517).
  Unstructured and metadata-only requests objects are an exception to this because the contained `TypeMeta` is the only way to identify the object's type.
  Because of this behavior, `obj.GetObjectKind().GroupVersionKind()` is likely to return an empty `GroupVersionKind`.
  I.e., you must not rely on `TypeMeta` being set or `GetObjectKind()` to return something usable.  
  If you need to identify an object's `GroupVersionKind`, use a scheme and its `ObjectKinds` function instead (or the helper function `apiutil.GVKForObject`).
  This is not specific to controller-runtime clients and applies to client-go clients as well.

[1] Other lower level, config or internal API types (e.g., such as [`AdmissionReview`](https://github.com/kubernetes/api/blob/release-1.21/admission/v1/types.go#L29)) don't implement `client.Object`. However, you also can't interact with such objects via the Kubernetes API and thus also not via a client, so this can be disregarded at this point.

### Metadata-Only Clients

Additionally, controller-runtime clients can be used to easily retrieve metadata-only objects or lists.
This is useful for efficiently checking if at least one object of a given kind exists, or retrieving metadata of an object, if one is not interested in the rest (e.g., spec/status).  
The `Accept` header sent to the API server then contains `application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1`, which makes the API server only return metadata of the retrieved object(s).
This saves network traffic and CPU/memory load on the API server and client side.
If the client fully lists all objects of a given kind including their spec/status, the resulting list can be quite large and easily exceed the controllers available memory.
That's why it's important to carefully check if a full list is actually needed, or if metadata-only list can be used instead.

For example:

```go
var (
  ctx       context.Context
  c         client.Client                         // "sigs.k8s.io/controller-runtime/pkg/client"
  shootList = &metav1.PartialObjectMetadataList{} // "k8s.io/apimachinery/pkg/apis/meta/v1"
)
shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))

if err := c.List(ctx, shootList, client.InNamespace("garden-my-project"), client.Limit(1)); err != nil {
  return err
}

if len(shootList.Items) > 0 {
  // project has at least one shoot
} else {
  // project doesn't have any shoots
}
```

### Gardener's Client Collection, ClientMaps

The Gardener codebase has a collection of clients ([`kubernetes.Interface`](https://github.com/gardener/gardener/blob/v1.29.0/pkg/client/kubernetes/types.go#L149)), which can return all the above mentioned client types.
Additionally, it contains helpers for rendering and applying helm charts (`ChartRender`, `ChartApplier`) and retrieving the API server's version (`Version`).  
Client sets are managed by so called `ClientMap`s, which are a form of registry for all client set for a given type of cluster, i.e., Garden, Seed and Shoot.
ClientMaps manage the whole lifecycle of clients: they take care of creating them if they don't exist already, running their caches, refreshing their cached server version and invalidating them when they are no longer needed.

```go
var (
  ctx   context.Context
  cm    clientmap.ClientMap // "github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
  shoot *gardencorev1beta1.Shoot
)

cs, err := cm.GetClient(ctx, keys.ForShoot(shoot)) // kubernetes.Interface
if err != nil {
  return err
}

c := cs.Client() // client.Client
```

The client collection mainly exist for historical reasons (there used to be a lot of code using the client-go style clients).
However, Gardener is in the process of moving more towards controller-runtime and only using their clients, as they provide many benefits and are much easier to use.
Also, [gardener/gardener#4251](https://github.com/gardener/gardener/issues/4251) aims at refactoring our controller and admission components to native controller-runtime components.

> ⚠️ Please always prefer controller-runtime clients over other clients when writing new code or refactoring existing code.

## Cache Types: Informers, Listers, Controller-Runtime Caches

Similar to the different types of client(set)s, there are also different kinds of Kubernetes client caches.
However, all of them are based on the same concept: `Informer`s.
An `Informer` is a watch-based cache implementation, meaning it opens [watch connections](https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes) to the API server and continuously updates cached objects based on the received watch events (`ADDED`, `MODIFIED`, `DELETED`).
`Informer`s offer to add indices to the cache for efficient object lookup (e.g., by name or labels) and to add `EventHandler`s for the watch events.
The latter is used by controllers to fill queues with objects that should be reconciled on watch events.

Informers are used in and created via several higher-level constructs:

### SharedInformerFactories, Listers

The generated clients (built-in as well as extended) feature a `SharedInformerFactory` for every API group, which can be used to create and retrieve `Informers` for all GroupVersionKinds.
Similarly, it can be used to retrieve `Listers` that allow getting and listing objects from the `Informer`'s cache.
However, both of these constructs are only used for historical reasons, and we are in the process of migrating away from them in favor of cached controller-runtime clients (see [gardener/gardener#2414](https://github.com/gardener/gardener/issues/2414), [gardener/gardener#2822](https://github.com/gardener/gardener/issues/2822)). Thus, they are described only briefly here.

_Important characteristics of Listers:_

- Objects read from Informers and Listers can always be slightly out-out-date (i.e., stale) because the client has to first observe changes to API objects via watch events (which can intermittently lag behind by a second or even more).
- Thus, don't make any decisions based on data read from Listers if the consequences of deciding wrongfully based on stale state might be catastrophic (e.g. leaking infrastructure resources). In such cases, read directly from the API server via a client instead.
- Objects retrieved from Informers or Listers are pointers to the cached objects, so they must not be modified without copying them first, otherwise the objects in the cache are also modified.

### Controller-Runtime Caches

controller-runtime features a cache implementation that can be used equivalently as their clients. In fact, it implements a subset of the `client.Client` interface containing the `Get` and `List` functions.
Under the hood, a `cache.Cache` dynamically creates `Informers` (i.e., opens watches) for every object GroupVersionKind that is being retrieved from it.

Note that the underlying Informers of a controller-runtime cache (`cache.Cache`) and the ones of a `SharedInformerFactory` (client-go) are not related in any way.
Both create `Informers` and watch objects on the API server individually.
This means that if you read the same object from different cache implementations, you may receive different versions of the object because the watch connections of the individual Informers are not synced.

> ⚠️ Because of this, controllers/reconcilers should get the object from the same cache in the reconcile loop, where the `EventHandler` was also added to set up the controller. For example, if a `SharedInformerFactory` is used for setting up the controller then read the object in the reconciler from the `Lister` instead of from a cached controller-runtime client.

By default, the `client.Client` created by a controller-runtime `Manager` is a `DelegatingClient`. It delegates `Get` and `List` calls to a `Cache`, and all other calls to a client that talks directly to the API server. Exceptions are requests with `*unstructured.Unstructured` objects and object kinds that were configured to be excluded from the cache in the `DelegatingClient`.

> ℹ️
> `kubernetes.Interface.Client()` returns a `DelegatingClient` that uses the cache returned from `kubernetes.Interface.Cache()` under the hood. This means that all `Client()` usages need to be ready for cached clients and should be able to cater with stale cache reads.

_Important characteristics of cached controller-runtime clients:_

- Like for Listers, objects read from a controller-runtime cache can always be slightly out of date. Hence, don't base any important decisions on data read from the cache (see above).
- In contrast to Listers, controller-runtime caches fill the passed in-memory object with the state of the object in the cache (i.e., they perform something like a "deep copy into"). This means that objects read from a controller-runtime cache can safely be modified without unintended side effects.
- Reading from a controller-runtime cache or a cached controller-runtime client implicitly starts a watch for the given object kind under the hood. This has important consequences:
  - Reading a given object kind from the cache for the first time can take up to a few seconds depending on size and amount of objects as well as API server latency. This is because the cache has to do a full list operation and wait for an initial watch sync before returning results.
  - ⚠️ Controllers need appropriate RBAC permissions for the object kinds they retrieve via cached clients (i.e., `list` and `watch`).
  - ⚠️ By default, watches started by a controller-runtime cache are cluster-scoped, meaning it watches and caches objects across all namespaces. Thus, be careful which objects to read from the cache as it might significantly increase the controller's memory footprint.
- There is no interaction with the cache on writing calls (`Create`, `Update`, `Patch` and `Delete`), see below.

**Uncached objects, filtered caches, `APIReader`s:**

In order to allow more granular control over which object kinds should be cached and which calls should bypass the cache, controller-runtime offers a few mechanisms to further tweak the client/cache behavior:

- When creating a `DelegatingClient`, certain object kinds can be configured to always be read directly from the API instead of from the cache. Note that this does not prevent starting a new Informer when retrieving them directly from the cache.
- Watches can be restricted to a given (set of) namespace(s) by using `cache.MultiNamespacedCacheBuilder` or setting `cache.Options.Namespace`.
- Watches can be filtered (e.g., by label) per object kind by configuring `cache.Options.SelectorsByObject` on creation of the cache.
- Retrieving metadata-only objects or lists from a cache results in a metadata-only watch/cache for that object kind.
- The `APIReader` can be used to always talk directly to the API server for a given `Get` or `List` call (use with care and only as a last resort!).

### To Cache or Not to Cache

Although watch-based caches are an important factor for the immense scalability of Kubernetes, it definitely comes at a price (mainly in terms of memory consumption).
Thus, developers need to be careful when introducing new API calls and caching new object kinds.
Here are some general guidelines on choosing whether to read from a cache or not:

- Always try to use the cache wherever possible and make your controller able to tolerate stale reads.
  - Leverage optimistic locking: use deterministic naming for objects you create (this is what the `Deployment` controller does [2]).
  - Leverage optimistic locking / concurrency control of the API server: send updates/patches with the last-known `resourceVersion` from the cache (see below). This will make the request fail, if there were concurrent updates to the object (conflict error), which indicates that we have operated on stale data and might have made wrong decisions. In this case, let the controller handle the error with exponential backoff. This will make the controller eventually consistent.
  - Track the actions you took, e.g., when creating objects with `generateName` (this is what the `ReplicaSet` controller does [3]). The actions can be tracked in memory and repeated if the expected watch events don't occur after a given amount of time.
  - Always try to write controllers with the assumption that data will only be eventually correct and can be slightly out of date (even if read directly from the API server!).
  - If there is already some other code that needs a cache (e.g., a controller watch), reuse it instead of doing extra direct reads.
  - Don't read an object again if you just sent a write request. Write requests (`Create`, `Update`, `Patch` and `Delete`) don't interact with the cache. Hence, use the current state that the API server returned (filled into the passed in-memory object), which is basically a "free direct read" instead of reading the object again from a cache, because this will probably set back the object to an older `resourceVersion`.
- If you are concerned about the impact of the resulting cache, try to minimize that by using filtered or metadata-only watches.
- If watching and caching an object type is not feasible, for example because there will be a lot of updates, and you are only interested in the object every ~5m, or because it will blow up the controllers memory footprint, fallback to a direct read. This can either be done by disabling caching the object type generally or doing a single request via an `APIReader`. In any case, please bear in mind that every direct API call results in a [quorum read from etcd](https://kubernetes.io/docs/reference/using-api/api-concepts/#the-resourceversion-parameter), which can be costly in a heavily-utilized cluster and impose significant scalability limits. Thus, always try to minimize the impact of direct calls by filtering results by namespace or labels, limiting the number of results and/or using metadata-only calls.

[2] The `Deployment` controller uses the pattern `<deployment-name>-<podtemplate-hash>` for naming `ReplicaSets`. This means, the name of a `ReplicaSet` it tries to create/update/delete at any given time is deterministically calculated based on the `Deployment` object. By this, it is insusceptible to stale reads from its `ReplicaSets` cache.

[3] In simple terms, the `ReplicaSet` controller tracks its `CREATE pod` actions as follows: when creating new `Pods`, it increases a counter of expected `ADDED` watch events for the corresponding `ReplicaSet`. As soon as such events arrive, it decreases the counter accordingly. It only creates new `Pods` for a given `ReplicaSet` once all expected events occurred (counter is back to zero) or a timeout has occurred. This way, it prevents creating more `Pods` than desired because of stale cache reads and makes the controller eventually consistent.

## Conflicts, Concurrency Control, and Optimistic Locking

Every Kubernetes API object contains the `metadata.resourceVersion` field, which identifies an object's version in the backing data store, i.e., etcd. Every write to an object in etcd results in a newer `resourceVersion`.
This field is mainly used for concurrency control on the API server in an optimistic locking fashion, but also for efficient resumption of interrupted watch connections.

Optimistic locking in the Kubernetes API sense means that when a client wants to update an API object, then it includes the object's `resourceVersion` in the request to indicate the object's version the modifications are based on.
If the `resourceVersion` in etcd has not changed in the meantime, the update request is accepted by the API server and the updated object is written to etcd.
If the `resourceVersion` sent by the client does not match the one of the object stored in etcd, there were concurrent modifications to the object. Consequently, the request is rejected with a conflict error (status code `409`, API reason `Conflict`), for example:

```json
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Failure",
  "message": "Operation cannot be fulfilled on configmaps \"foo\": the object has been modified; please apply your changes to the latest version and try again",
  "reason": "Conflict",
  "details": {
    "name": "foo",
    "kind": "configmaps"
  },
  "code": 409
}
```

This concurrency control is an important mechanism in Kubernetes as there are typically multiple clients acting on API objects at the same time (humans, different controllers, etc.). If a client receives a conflict error, it should read the object's latest version from the API server, make the modifications based on the newest changes, and retry the update.
The reasoning behind this is that a client might choose to make different decisions based on the concurrent changes made by other actors compared to the outdated version that it operated on.

_Important points about concurrency control and conflicts:_

- The `resourceVersion` field carries a string value and clients must not assume numeric values (the type and structure of versions depend on the backing data store). This means clients may compare `resourceVersion` values to detect whether objects were changed. But they must not compare `resourceVersion`s to figure out which one is newer/older, i.e., no greater/less-than comparisons are allowed.
- By default, update calls (e.g. via client-go and controller-runtime clients) use optimistic locking as the passed in-memory usually object contains the latest `resourceVersion` known to the controller, which is then also sent to the API server.
- API servers can also choose to accept update calls without optimistic locking (i.e., without a `resourceVersion` in the object's metadata) for any given resource. However, sending update requests without optimistic locking is strongly discouraged, as doing so overwrites the entire object, discarding any concurrent changes made to it.
- On the other side, patch requests can always be executed either with or without optimistic locking, by (not) including the `resourceVersion` in the patched object's metadata. Sending patch requests without optimistic locking might be safe and even desirable as a patch typically updates only a specific section of the object. However, there are also situations where patching without optimistic locking is not safe (see below).

### Don’t Retry on Conflict

Similar to how a human would typically handle a conflict error, there are helper functions implementing `RetryOnConflict`-semantics, i.e., try an update call, then re-read the object if a conflict occurs, apply the modification again and retry the update.
However, controllers should generally *not* use `RetryOnConflict`-semantics. Instead, controllers should abort their current reconciliation run and let the queue handle the conflict error with exponential backoff.
The reasoning behind this is that a conflict error indicates that the controller has operated on stale data and might have made wrong decisions earlier on in the reconciliation.
When using a helper function that implements `RetryOnConflict`-semantics, the controller doesn't check which fields were changed and doesn't revise its previous decisions accordingly.
Instead, retrying on conflict basically just ignores any conflict error and blindly applies the modification.

To properly solve the conflict situation, controllers should immediately return with the error from the update call. This will cause retries with exponential backoff so that the cache has a chance to observe the latest changes to the object.
In a later run, the controller will then make correct decisions based on the newest version of the object, not run into conflict errors, and will then be able to successfully reconcile the object. This way, the controller becomes eventually consistent.

The other way to solve the situation is to modify objects without optimistic locking in order to avoid running into a conflict in the first place (only if this is safe).
This can be a preferable solution for controllers with long-running reconciliations (which is actually an anti-pattern but quite unavoidable in some of Gardener's controllers).
Aborting the entire reconciliation run is rather undesirable in such cases, as it will add a lot of unnecessary waiting time for end users and overhead in terms of compute and network usage.

However, in any case, retrying on conflict is probably not the right option to solve the situation (there are some correct use cases for it, though, they are very rare). Hence, don't retry on conflict.

### To Lock or Not to Lock

As explained before, conflicts are actually important and prevent clients from doing wrongful concurrent updates. This means that conflicts are not something we generally want to avoid or ignore.
However, in many cases controllers are exclusive owners of the fields they want to update and thus it might be safe to run without optimistic locking.

For example, the gardenlet is the exclusive owner of the `spec` section of the Extension resources it creates on behalf of a Shoot (e.g., the `Infrastructure` resource for creating VPC). Meaning, it knows the exact desired state and no other actor is supposed to update the Infrastructure's `spec` fields.
When the gardenlet now updates the Infrastructures `spec` section as part of the Shoot reconciliation, it can simply issue a `PATCH` request that only updates the `spec` and runs without optimistic locking.
If another controller concurrently updated the object in the meantime (e.g., the `status` section), the `resourceVersion` got changed, which would cause a conflict error if running with optimistic locking.
However, concurrent `status` updates would not change the gardenlet's mind on the desired `spec` of the Infrastructure resource as it is determined only by looking at the Shoot's specification.
If the `spec` section was changed concurrently, it's still fine to overwrite it because the gardenlet should reconcile the `spec` back to its desired state.

Generally speaking, if a controller is the exclusive owner of a given set of fields and they are independent of concurrent changes to other fields in that object, it can patch these fields without optimistic locking.
This might ignore concurrent changes to other fields or blindly overwrite changes to the same fields, but this is fine if the mentioned conditions apply.
Obviously, this applies only to patch requests that modify only a specific set of fields but not to update requests that replace the entire object.

In such cases, it's even desirable to run without optimistic locking as it will be more performant and save retries.
If certain requests are made with high frequency and have a good chance of causing conflicts, retries because of optimistic locking can cause a lot of additional network traffic in a large-scale Gardener installation. 

## Updates, Patches, Server-Side Apply

There are different ways of modifying Kubernetes API objects.
The following snippet demonstrates how to do a given modification with the most frequently used options using a controller-runtime client: 

```go
var (
  ctx   context.Context
  c     client.Client
  shoot *gardencorev1beta1.Shoot
)

// update
shoot.Spec.Kubernetes.Version = "1.22"
err := c.Update(ctx, shoot)

// json merge patch
patch := client.MergeFrom(shoot.DeepCopy())
shoot.Spec.Kubernetes.Version = "1.22"
err = c.Patch(ctx, shoot, patch)

// strategic merge patch
patch = client.StrategicMergeFrom(shoot.DeepCopy())
shoot.Spec.Kubernetes.Version = "1.22"
err = c.Patch(ctx, shoot, patch)
```

_Important characteristics of the shown request types:_

- Update requests always send the entire object to the API server and update all fields accordingly. By default, optimistic locking is used (`resourceVersion` is included).
- Both patch types run without optimistic locking by default. However, it can be enabled explicitly if needed:
    ```go
    // json merge patch + optimistic locking
    patch := client.MergeFromWithOptions(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
    // ...
  
    // strategic merge patch + optimistic locking
    patch = client.StrategicMergeFrom(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
    // ...
    ```
- Patch requests only contain the changes made to the in-memory object between the copy passed to `client.*MergeFrom` and the object passed to `Client.Patch()`. The diff is calculated on the client-side based on the in-memory objects only. This means that if in the meantime some fields were changed on the API server to a different value than the one on the client-side, the fields will not be changed back as long as they are not changed on the client-side as well (there will be no diff in memory).
- Thus, if you want to ensure a given state using patch requests, always read the object first before patching it, as there will be no diff otherwise, meaning the patch will be empty. For more information, see [gardener/gardener#4057](https://github.com/gardener/gardener/pull/4057) and the comments in [gardener/gardener#4027](https://github.com/gardener/gardener/pull/4027).
- Also, always send updates and patch requests even if your controller hasn't made any changes to the current state on the API server. I.e., don't make any optimization for preventing empty patches or no-op updates. There might be mutating webhooks in the system that will modify the object and that rely on update/patch requests being sent (even if they are no-op). Gardener's extension concept makes heavy use of mutating webhooks, so it's important to keep this in mind.
- JSON merge patches always replace lists as a whole and don't merge them. Keep this in mind when operating on lists with merge patch requests. If the controller is the exclusive owner of the entire list, it's safe to run without optimistic locking. Though, if you want to prevent overwriting concurrent changes to the list or its items made by other actors (e.g., additions/removals to the `metadata.finalizers` list), enable optimistic locking.
- Strategic merge patches are able to make more granular modifications to lists and their elements without replacing the entire list. It uses Golang struct tags of the API types to determine which and how lists should be merged. See [Update API Objects in Place Using kubectl patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/) or the [strategic merge patch documentation](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md) for more in-depth explanations and comparison with JSON merge patches.
  With this, controllers *might* be able to issue patch requests for individual list items without optimistic locking, even if they are not exclusive owners of the entire list. Remember to check the `patchStrategy` and `patchMergeKey` struct tags of the fields you want to modify before blindly adding patch requests without optimistic locking.
- Strategic merge patches are only supported by built-in Kubernetes resources and custom resources served by Extension API servers. Strategic merge patches are not supported by custom resources defined by `CustomResourceDefinition`s (see [this comparison](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#advanced-features-and-flexibility)). In that case, fallback to JSON merge patches.
- [Server-side Apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) is yet another mechanism to modify API objects, which is supported by all API resources (in newer Kubernetes versions). However, it has a few problems and more caveats preventing us from using it in Gardener at the time of writing. See [gardener/gardener#4122](https://github.com/gardener/gardener/issues/4122) for more details.

> Generally speaking, patches are often the better option compared to update requests because they can save network traffic, encoding/decoding effort, and avoid conflicts under the presented conditions.
> If choosing a patch type, consider which type is supported by the resource you're modifying and what will happen in case of a conflict. Consider whether your modification is safe to run without optimistic locking.
> However, there is no simple rule of thumb on which patch type to choose.

## On Helper Functions

Here is a note on some helper functions, that should be avoided and why:

`controllerutil.CreateOrUpdate` does a basic get, mutate and create or update call chain, which is often used in controllers. We should avoid using this helper function in Gardener, because it is likely to cause conflicts for cached clients and doesn't send no-op requests if nothing was changed, which can cause problems because of the heavy use of webhooks in Gardener extensions (see above).
That's why usage of this function was completely replaced in [gardener/gardener#4227](https://github.com/gardener/gardener/pull/4227) and similar PRs.

`controllerutil.CreateOrPatch` is similar to `CreateOrUpdate` but does a patch request instead of an update request. It has the same drawback as `CreateOrUpdate` regarding no-op updates.
Also, controllers can't use optimistic locking or strategic merge patches when using `CreateOrPatch`.
Another reason for avoiding use of this function is that it also implicitly patches the status section if it was changed, which is confusing for others reading the code. To accomplish this, the func does some back and forth conversion, comparison and checks, which are unnecessary in most of our cases and simply wasted CPU cycles and complexity we want to avoid.

There were some `Try{Update,UpdateStatus,Patch,PatchStatus}` helper functions in Gardener that were already removed by [gardener/gardener#4378](https://github.com/gardener/gardener/pull/4378) but are still used in some extension code at the time of writing.
The reason for eliminating these functions is that they implement `RetryOnConflict`-semantics. Meaning, they first get the object, mutate it, then try to update and retry if a conflict error occurs.
As explained above, retrying on conflict is a controller anti-pattern and should be avoided in almost every situation.
The other problem with these functions is that they read the object first from the API server (always do a direct call), although in most cases we already have a recent version of the object at hand. So, using this function generally does unnecessary API calls and therefore causes unwanted compute and network load.

For the reasons explained above, there are similar helper functions that accomplish similar things but address the mentioned drawbacks: `controllerutils.{GetAndCreateOrMergePatch,GetAndCreateOrStrategicMergePatch}`.
These can be safely used as replacements for the aforementioned helper funcs.
If they are not fitting for your use case, for example because you need to use optimistic locking, just do the appropriate calls in the controller directly.

## Related Links

- [Kubernetes Client usage in Gardener](https://www.youtube.com/watch?v=RPsUo925PUA&t=40s) (Community Meeting talk, 2020-06-26)

These resources are only partially related to the topics covered in this doc, but might still be interesting for developer seeking a deeper understanding of Kubernetes API machinery, architecture and foundational concepts.

- [API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [The Kubernetes Resource Model](https://github.com/kubernetes/design-proposals-archive/blob/main/architecture/resource-management.md)
