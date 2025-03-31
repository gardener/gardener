# Deploy a gardenlet Via `gardener-operator`

The gardenlet can automatically be deployed by `gardener-operator` into existing Kubernetes clusters in order to register them as seeds.

## Prerequisites

Using this method only works when [`gardener-operator`](../concepts/operator.md) is managing the garden cluster.
If you have used the [`gardener/controlplane` Helm chart](../../charts/gardener/controlplane) for the deployment of the Gardener control plane, please refer to [this document](deploy_gardenlet_manually.md).

> [!TIP]
> The initial seed cluster can be the garden cluster itself, but for better separation of concerns, it is recommended to only register other clusters as seeds.

## Deployment of gardenlets

Using this method, `gardener-operator` is only taking care of the very first deployment of gardenlet.
Once running, the gardenlet leverages the [self upgrade](deploy_gardenlet_manually.md#self-upgrades) strategy in order to keep itself up-to-date.
Concretely, `gardener-operator` only acts when there is no respective `Seed` resource yet.

In order to request a gardenlet deployment, create following resource in the (virtual) garden cluster:

```yaml
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: Gardenlet
metadata:
  name: local
  namespace: garden
spec:
  deployment:
    replicaCount: 1
    revisionHistoryLimit: 2
    helm:
      ociRepository:
        ref: <url-to-gardenlet-chart-repository>:v1.97.0
  config:
    apiVersion: gardenlet.config.gardener.cloud/v1alpha1
    kind: GardenletConfiguration
    controllers:
      shoot:
        reconcileInMaintenanceOnly: true
        respectSyncPeriodOverwrite: true
      shootState:
        concurrentSyncs: 0
    logging:
      enabled: true
      vali:
        enabled: true
      shootNodeLogging:
        shootPurposes:
        - infrastructure
        - production
        - development
        - evaluation
    seedConfig:
      apiVersion: core.gardener.cloud/v1beta1
      kind: Seed
      metadata:
        labels:
          base: kind
      spec:
        backup:
          provider: local
          region: local
          secretRef:
            name: backup-local
            namespace: garden
        dns:
          provider:
            secretRef:
              name: internal-domain-internal-local-gardener-cloud
              namespace: garden
            type: local
        ingress:
          controller:
            kind: nginx
          domain: ingress.local.seed.local.gardener.cloud
        networks:
          nodes: 172.18.0.0/16
          pods: 10.1.0.0/16
          services: 10.2.0.0/16
          shootDefaults:
            pods: 10.3.0.0/16
            services: 10.4.0.0/16
        provider:
          region: local
          type: local
          zones:
          - "0"
        settings:
          excessCapacityReservation:
            enabled: false
          scheduling:
            visible: true
          verticalPodAutoscaler:
            enabled: true
```

This causes `gardener-operator` to deploy gardenlet to the same cluster where it is running.
Once it comes up, gardenlet will create a `Seed` resource with the same name and uses the `Gardenlet` resource for self-upgrades (see [this document](deploy_gardenlet_manually.md#self-upgrades)).

### Remote Clusters

If you want `gardener-operator` to deploy gardenlet into some other cluster, create a kubeconfig `Secret` and reference it in the `Gardenlet` resource:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: remote-cluster-kubeconfig
  namespace: garden
type: Opaque
data:
  kubeconfig: base64(kubeconfig-to-remote-cluster)
---
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: Gardenlet
metadata:
  name: local
  namespace: garden
spec:
  kubeconfigSecretRef:
    name: remote-cluster-kubeconfig
# ...
```

After successful deployment of gardenlet, `gardener-operator` will delete the `remote-cluster-kubeconfig` `Secret` and set `.spec.kubeconfigSecretRef` to `nil`.
This is because the kubeconfig will never ever be needed anymore (`gardener-operator` is only responsible for initial deployment, and gardenlet updates itself with an in-cluster kubeconfig).
