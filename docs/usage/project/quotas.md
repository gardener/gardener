---
description: Limit Shoot infrastructure consumption and cluster lifetime with Quotas
---

# Quotas

Gardener `Quota` resources limit the infrastructure resources consumed by `Shoot` clusters and can optionally limit their lifetime.
They are namespaced resources connected to `Shoot`s through a `CredentialsBinding` or `SecretBinding`.
Creating a `Quota` alone does not attach it to every `Shoot` in a project or automatically discover all uses of a provider credential.

A `Quota` declares upper limits in `spec.metrics`, such as CPU, memory, storage, and load balancers, and an optional `spec.clusterLifetimeDays`.
To use it, create the `Quota` and list it under `quotas` in the binding that a `Shoot` references.
When such a `Shoot` is created or its workers are changed, Gardener adds up the requested resources of all `Shoot`s using that `Quota` and rejects the request if a limit would be exceeded.
If a lifetime is set, Gardener deletes those `Shoot`s automatically after it expires.
The sections below explain scopes, a complete example, how usage is calculated, and the lifetime behavior.

A Gardener `Quota` differs from the native Kubernetes `ResourceQuota` named `gardener` maintained by the [Project controller](../../concepts/controller-manager.md#project-controller).
That resource limits object counts in the project namespace; see its [ResourceQuota reconciler](../../concepts/controller-manager.md#resourcequota-reconciler).
Gardener Quotas instead constrain the infrastructure consumption described below.

## Prerequisites

- Access to the Garden API with `kubectl` and an existing project namespace, here `garden-dev`.
- Authorization to create or manage Quotas, or an operator who provides them, and permission to create bindings and Shoots. Referencing a Quota requires read authorization for that Quota, including when it is in another namespace.
- Existing provider credentials. The examples use a `Secret` named `my-provider-account` in `garden-dev`; create it using your provider's required credential keys before creating the binding.
- A valid provider-specific Shoot configuration, including an available `CloudProfile` or `NamespacedCloudProfile`, region, machine types, images, networking, and provider configuration. Use the [Shoot example manifest](../../../example/90-shoot.yaml) as a starting point and replace its placeholders for your environment.

## Scopes and Aggregation

`spec.scope` is an object reference with `apiVersion` and `kind`, not a string such as `project`.
It is immutable after creation.

| `spec.scope.apiVersion` | `spec.scope.kind` | Aggregation category |
| --- | --- | --- |
| `core.gardener.cloud/v1beta1` | `Project` | Project: Shoots in the requesting Shoot's namespace whose bindings reference this Quota. |
| `v1` | `Secret` | Credentials: Shoots across namespaces whose bindings reference this Quota. |
| `security.gardener.cloud/v1alpha1` | `WorkloadIdentity` | Credentials: Shoots across namespaces whose bindings reference this Quota. |

In each case, Gardener finds applicable Shoots through both binding types referencing the same Quota **name and namespace**.
The scope selects the aggregation category; it does not select a particular credential by name.
To share a credentials quota across projects, reference that same Quota from the relevant bindings in each project.
Other bindings using the same credential without referencing the Quota are not automatically included.

A binding can reference at most one project-scope Quota and one credentials-scope Quota.
`Secret` and `WorkloadIdentity` are the same credentials category, so a binding cannot reference one quota of each of those kinds alongside each other.
Every referenced Quota must pass independently.

## Create and Attach a Quota

### 1. Create the Quota

Save this complete Quota manifest as `quota.yaml`:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Quota
metadata:
  name: project-quota
  namespace: garden-dev
spec:
  scope:
    apiVersion: core.gardener.cloud/v1beta1
    kind: Project
  metrics:
    cpu: "10"
    gpu: "0"
    memory: 40Gi
    storage.standard: 500Gi
    storage.premium: 100Gi
    loadbalancer: "4"
```

Apply it before creating a binding that references it:

```sh
kubectl apply -f quota.yaml
```

To create a credentials-scope Quota instead, use one of the credentials scope pairs in the table and a distinct Quota name.
See also the [Quota example manifest](../../../example/60-quota.yaml) and [Quota API reference](../../api-reference/core.md#quota).

### 2. Create a CredentialsBinding

Save the following as `credentialsbinding.yaml` and replace `<provider-type>` with your provider, for example `aws`, matching the credentials and Shoot configuration.
This is a complete binding manifest once the placeholder is replaced:

```yaml
apiVersion: security.gardener.cloud/v1alpha1
kind: CredentialsBinding
metadata:
  name: my-provider-account
  namespace: garden-dev
provider:
  type: <provider-type>
credentialsRef:
  apiVersion: v1
  kind: Secret
  name: my-provider-account
  namespace: garden-dev
quotas:
- name: project-quota
  namespace: garden-dev
```

`quotas` is a top-level field, not part of `spec` or `credentialsRef`.
For an existing WorkloadIdentity, use `apiVersion: security.gardener.cloud/v1alpha1` and `kind: WorkloadIdentity` in `credentialsRef`, with its name and namespace, as shown in the [CredentialsBinding example](../../../example/80-credentialsbinding.yaml).

```sh
kubectl apply -f credentialsbinding.yaml
```

### 3. Reference the Binding from a Shoot

The following is a **partial Shoot excerpt**, not a complete manifest.
Merge these fields into your provider-specific configuration based on the [Shoot example](../../../example/90-shoot.yaml), saved as `shoot.yaml`:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: quota-demo
  namespace: garden-dev
spec:
  credentialsBindingName: my-provider-account
```

The Shoot and its binding must be in the same namespace.
Apply the completed Shoot manifest after the credentials, Quota, and binding exist:

```sh
kubectl apply -f shoot.yaml
```

### Alternative: SecretBinding

For a Secret-based setup using `SecretBinding`, replace step 2 with this manifest, saved as `secretbinding.yaml`, and replace `<provider-type>` as above:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: SecretBinding
metadata:
  name: my-provider-account
  namespace: garden-dev
provider:
  type: <provider-type>
secretRef:
  name: my-provider-account
  namespace: garden-dev
quotas:
- name: project-quota
  namespace: garden-dev
```

Apply it with `kubectl apply -f secretbinding.yaml` after the Secret and Quota exist.
In step 3, use the following **partial Shoot spec excerpt** in the completed `shoot.yaml` instead:

```yaml
spec:
  secretBindingName: my-provider-account
```

Keep the Shoot in `garden-dev` and remove `spec.credentialsBindingName`: the two Shoot reference fields are mutually exclusive.
Then apply the completed Shoot manifest.
See the [SecretBinding example](../../../example/80-secretbinding.yaml).

## Metrics and Enforcement

Metric values are non-negative Kubernetes quantities, such as `"10"` for CPU or `40Gi` for memory.
The supported keys are:

| Metric | Consumption counted |
| --- | --- |
| `cpu` | Machine CPU multiplied by each worker pool's maximum count. |
| `gpu` | Machine GPU multiplied by each worker pool's maximum count. |
| `memory` | Machine memory multiplied by each worker pool's maximum count. |
| `storage.standard` | Worker volume size multiplied by maximum count for the `standard` volume class. |
| `storage.premium` | Worker volume size multiplied by maximum count for the `premium` volume class. |
| `loadbalancer` | One base load balancer per Shoot, plus one when the nginx ingress addon is enabled. |

Machine resources and volume classes come from the Shoot's `CloudProfile` or `NamespacedCloudProfile`; machine-provided storage is used when applicable.
The calculation uses configured worker **maximum** counts, not current node counts or actual utilization.
Manually created persistent volumes and load balancers from in-cluster Services are not included.
Quota therefore does not account for all cloud-provider billing or enforce in-cluster workload consumption.

For example, three maximum workers on machines with 2 CPU each consume 6 CPU.
With 4 CPU already consumed by other applicable Shoots, the aggregate is 10 CPU, which passes the example Quota's `cpu: "10"` limit.
Increasing the maximum to four workers makes the aggregate 12 CPU and fails.
Equality to a limit is accepted; omitted metric keys are unconstrained by that Quota.
If the binding also references a credentials Quota allowing only 8 CPU, the 10 CPU request fails that quota even though the project quota passes, assuming the same existing aggregate consumption.

The [Shoot quota admission validator](../../concepts/apiserver.md#shoot-quotas) checks Shoot creation and revalidates updates that add worker pools, change worker machine types, maximum counts or volumes, or enable the nginx ingress addon.
It does not continuously enforce measured usage.
An exceeded limit rejects the request with a forbidden error containing `quota limits exceeded. Unable to allocate further` and the affected metric names.
Reduce the requested resources or ask the quota administrator to adjust the applicable limits.
Binding admission also rejects unreadable Quota references and multiple quotas in the same scope category.

## Cluster Lifetime and Automatic Deletion

Optionally add `clusterLifetimeDays: 14` under the Quota's `spec`.
The value must be positive; omit it for no lifetime constraint from that Quota.

> [!WARNING]
> A lifetime constraint causes automatic Shoot deletion after expiration. Use it only for clusters that may be deleted automatically.

When several referenced Quotas configure a lifetime, the smallest value applies.
For example, lifetimes of 14 and 7 days result in an initial expiration seven days after the Shoot's creation time.
The [Shoot quota reconciler](../../concepts/controller-manager.md#quota-reconciler) initially sets the `shoot.gardener.cloud/expiration-timestamp` annotation to that time if the annotation is absent.
It confirms deletion and deletes expired Shoots during reconciliation, so deletion is not guaranteed at the exact expiration instant.

Changing a Quota's lifetime does not reset an existing expiration timestamp.
An authorized update to the expiration annotation must use an RFC 3339 timestamp; admission permits an extension only up to the current time plus the shortest configured lifetime.
If all referenced lifetime constraints are removed, the controller removes the expiration annotation and stops deleting the Shoot based on quota lifetime.
