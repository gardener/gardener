# Static Manifest Propagation From Seed To Shoots

## Overview

Static manifest propagation is a mechanism that allows operators to distribute predefined Kubernetes resources across all Shoot clusters automatically.
By placing labeled `Secret`s in the seed cluster's `garden` namespace, operators can ensure that specific manifests (such as RBAC rules, quotas, config policies, or compliance objects) are consistently deployed to every Shoot cluster without manual intervention.

## Why Use Static Manifests?

Static manifest propagation provides several benefits for cluster operators:

- **Centralized Management**: Update manifests in one location (the seed's `garden` namespace) and have changes automatically propagate to all Shoots
- **Consistency**: Ensure all Shoot clusters have the same baseline configurations, policies, or resources
- **Simplified Operations**: Eliminate the need for manual per-Shoot provisioning or custom controllers
- **Generic Distribution**: Works independently of cloud provider or extension logic
- **Compliance & Governance**: Easily enforce organization-wide policies, quotas, or security configurations across all clusters

Common use cases include:

- Deploying RBAC rules or `ClusterRole`s
- Setting `ResourceQuota`s or `LimitRange`s
- Distributing `NetworkPolicy`s
- Injecting compliance or audit configurations
- Providing common `ConfigMap`s or monitoring agents

## How It Works

During Shoot reconciliation, the `gardenlet` performs the following steps:

1. Scans the seed cluster's `garden` namespace for `Secret`s labeled with `shoot.gardener.cloud/static-manifests=true`.
2. Copies all matching `Secret`s into each Shoot namespace.
3. Creates a single `ManagedResource` that references these `Secret`s.
4. The `ManagedResource` ensures the manifests are applied to the Shoot cluster.

This process happens automatically during every Shoot reconciliation, ensuring manifests stay synchronized.

## How to Propagate Static Manifests

### Step 1: Prepare Your Manifests

Create a YAML file containing the Kubernetes resources you want to deploy to all Shoot clusters. For example:

```yaml
# my-manifests.yaml
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: default-quota
  namespace: default
spec:
  hard:
    requests.cpu: "100"
    requests.memory: 200Gi
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: org-viewer
rules:
- apiGroups: [""]
  resources: ["pods", "services"]
  verbs: ["get", "list", "watch"]
```

### Step 2: Create a Secret in the Seed Cluster

Create a `Secret` in the seed cluster's `garden` namespace containing your manifests.
The `Secret` must be labeled with `shoot.gardener.cloud/static-manifests=true`.

```bash
# Create the Secret with the required label
kubectl create secret generic my-static-manifests \
  --from-file=manifests.yaml=my-manifests.yaml \
  --namespace=garden \
  --dry-run=client -o yaml | \
  kubectl label --local -f - shoot.gardener.cloud/static-manifests=true --dry-run=client -o yaml | \
  kubectl apply -f -
```

Or create it declaratively:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-static-manifests
  namespace: garden
  labels:
    shoot.gardener.cloud/static-manifests: "true"
type: Opaque
data:
  manifests.yaml: <base64-encoded-yaml-content>
```

### Step 3: Verify Propagation

After the next Shoot reconciliation, verify that the manifests have been propagated:

1. **Check the Shoot namespace** in the seed cluster for the copied `Secret`:
   ```bash
   kubectl get secret my-static-manifests -n shoot--<project>--<shoot>
   ```

2. **Check the `ManagedResource`** referencing your `Secret`:
   ```bash
   kubectl get managedresource static-manifests-from-seed -n shoot--<project>--<shoot>
   ```

3. **Check the Shoot cluster** to confirm resources are applied:
   ```bash
   # Using the Shoot cluster kubeconfig
   kubectl get resourcequota default-quota -n default
   kubectl get clusterrole org-viewer
   ```

## Updating Static Manifests

To update manifests across all Shoot clusters:

1. Update the `Secret` in the seed's `garden` namespace with the new manifest content.
2. Wait for the next Shoot reconciliation cycle, or manually trigger reconciliation.
3. The `gardenlet` will detect the change and update the `ManagedResource` in each Shoot namespace.
4. The updated manifests will be applied to all Shoot clusters.

## Removing Static Manifests

To stop propagating manifests to Shoot clusters:

1. Delete the `Secret` from the seed's `garden` namespace:
   ```bash
   kubectl delete secret my-static-manifests -n garden
   ```

2. During the next reconciliation, the `gardenlet` will remove the `Secret` from Shoot namespaces.
3. The associated resources will be deleted from the Shoot clusters via the `ManagedResource` cleanup.

## Important Considerations

- **No Templating or Dynamic Logic**: Manifests must be completely static. No templating, variable substitution, or dynamic logic is supported. The same exact manifests are deployed to all Shoot clusters without modification. If you need per-Shoot customization, Shoot-specific values, or sophisticated logic (e.g., conditional deployment, templating based on Shoot properties), you must write a Gardener extension instead.
- **Namespace Scoping**: Ensure manifests use appropriate namespaces. Resources without a namespace will be created in the default namespace of the Shoot cluster.
- **Resource Conflicts**: Avoid creating resources that might conflict with Gardener-managed resources or Shoot-specific configurations.
- **Secret Naming**: Use descriptive names for `Secret`s to distinguish between different sets of manifests.
- **Multiple Secrets**: You can create multiple labeled `Secret`s in the `garden` namespace; all will be propagated.
- **Label Requirement**: The label `shoot.gardener.cloud/static-manifests=true` is mandatory. `Secret`s without this label will not be propagated.
- **Reconciliation Timing**: Changes may take time to propagate depending on the Shoot reconciliation schedule.
