# Referenced Resources

The Shoot resource can include a list of resources (usually secrets) that can be referenced by name in the extension `providerConfig` and other Shoot sections, for example:

```yaml
kind: Shoot
apiVersion: core.gardener.cloud/v1beta1
metadata:
  name: crazy-botany
  namespace: garden-dev
  ...
spec:
  ...
  extensions:
  - type: foobar
    providerConfig:
      apiVersion: foobar.extensions.gardener.cloud/v1alpha1
      kind: FooBarConfig
      foo: bar
      secretRef: foobar-secret
  resources:
  - name: foobar-secret
    resourceRef:
      apiVersion: v1
      kind: Secret
      name: my-foobar-secret
```

Gardener expects to find these referenced resources in the project namespace (e.g., `garden-dev`) and will copy them to the Shoot namespace in the Seed cluster when reconciling a Shoot, adding a prefix to their names to avoid naming collisions with Gardener's own resources.

Extension controllers can resolve the references to these resources by accessing the Shoot via the `Cluster` resource. To properly read a referenced resources, extension controllers should use the utility function `GetObjectByReference` from the `extensions/pkg/controller` package, for example:

```go
    ...
    ref = &autoscalingv1.CrossVersionObjectReference{
        APIVersion: "v1",
        Kind:       "Secret",
        Name:       "foo",
    }
    secret := &corev1.Secret{}
    if err := controller.GetObjectByReference(ctx, client, ref, "shoot--test--foo", secret); err != nil {
        return err
    }
    // Use secret
    ...
```
