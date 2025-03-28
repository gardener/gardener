# Referenced Resources

`Shoot`s and `Seed`s can include a list of resources (usually secrets) that can be referenced by name in the extension `providerConfig` and other Shoot sections, for example:

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

Gardener expects these referenced resources to be located in the project namespace (e.g., `garden-dev`) for `Shoot`s and in the `garden` namespace for `Seed`s.
`Seed` resources are copied to the `garden` namespace in the seed cluster, while `Shoot` resources are copied to the control-plane namespace in the shoot cluster.
To avoid conflicts with other resources in the shoot, all resources in the seed are prefixed with a static value.

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
