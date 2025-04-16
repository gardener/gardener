# Contract: `Extension` Resource

Gardener defines common procedures which must be passed to create a functioning shoot cluster. Well known steps are represented by special resources like `Infrastructure`, `OperatingSystemConfig` or `DNS`. These resources are typically reconciled by dedicated controllers setting up the infrastructure on the hyperscaler or managing DNS entries, etc.

But, some requirements don't match with those special resources or don't depend on being proceeded at a specific step in the creation / deletion flow of the shoot. They require a more generic hook. Therefore, Gardener offers the `Extension` resource.

## What is required to register and support an Extension type?

Gardener creates one `Extension` resource per registered extension type in `ControllerRegistration` per shoot.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: extension-example
spec:
  resources:
  - kind: Extension
    type: example
    autoEnable:
    - shoot
    workerlessSupported: true
```

If `spec.resources[].autoEnable` is set to `shoot`, then the `Extension` resources of the given `type` is created for every shoot cluster. Set to `none` (default), the `Extension` resource is only created if configured in the `Shoot` manifest. In case of workerless `Shoot`, an automatically enabled `Extension` resource is created only if `spec.resources[].workerlessSupported` is also set to `true`. If an extension configured in the spec of a workerless `Shoot` is not supported yet, the admission request will be rejected.
Another valid values is `seed` (-> automatically enabled for all seeds).

The `Extension` resources are created in the shoot namespace of the seed cluster.

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Extension
metadata:
  name: example
  namespace: shoot--foo--bar
spec:
  type: example
  providerConfig: {}
```

Your controller needs to reconcile `extensions.extensions.gardener.cloud`. Since there can exist multiple `Extension` resources per shoot, each one holds a `spec.type` field to let controllers check their responsibility (similar to all other extension resources of Gardener).

## ProviderConfig

It is possible to provide data in the `Shoot` resource which is copied to `spec.providerConfig` of the `Extension` resource.

```yaml
---
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: bar
  namespace: garden-foo
spec:
  extensions:
  - type: example
    providerConfig:
      foo: bar
...
```

results in

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Extension
metadata:
  name: example
  namespace: shoot--foo--bar
spec:
  type: example
  providerConfig:
    foo: bar
```

## Shoot Reconciliation Flow and Extension Status

Gardener creates Extension resources as part of the Shoot reconciliation. Moreover, it is guaranteed that the [Cluster](../cluster.md) resource exists before the `Extension` resource is created. `Extension`s can be reconciled at different stages during Shoot reconciliation depending on the defined extension lifecycle strategy in the respective `ControllerRegistration` resource. Please consult the [Extension Lifecycle](../registration.md#extension-lifecycle) section for more information.

For an `Extension` controller it is crucial to maintain the `Extension`'s status correctly. At the end Gardener checks the status of each `Extension` and only reports a successful shoot reconciliation if the state of the last operation is `Succeeded`.

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Extension
metadata:
  generation: 1
  name: example
  namespace: shoot--foo--bar
spec:
  type: example
status:
  lastOperation:
    state: Succeeded
  observedGeneration: 1
```
