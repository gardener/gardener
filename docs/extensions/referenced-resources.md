# Referenced Resources

`Shoot`s, `Seed`s and `Garden`s can include a list of resources (usually secrets) that can be referenced by name in the extension `providerConfig` and other Shoot sections, for example:

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

Gardener expects these referenced resources to be located in the project namespace (e.g., `garden-dev`) for `Shoot`s and in the `garden` namespace for `Seed`s and `Garden`s.
In the case of `Secret`s and `ConfigMap`s, the Seed resources are copied to the `garden` namespace in the seed cluster, while `Shoot` resources are copied to the control-plane namespace of the shoot cluster.
When the referenced resource is of kind `WorkloadIdentity`, a Kubernetes secret with the workload identity config and token data is provisioned in the seed cluster, respectively in the `garden` namespace for `Seed` resources and in the control plane namespace for `Shoot` resources.
`Garden`s are not allowed to refer to `WorkloadIdentity` because they are not available in the `Runtime` cluster.
To avoid conflicts with other resources in the shoot, all resources in the seed are prefixed with a static value.
And to avoid conflict between referenced `Secret` and `WorkloadIdentity`, the workload identity secret is using own prefix.

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
