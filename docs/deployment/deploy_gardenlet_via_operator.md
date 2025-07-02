# Deploy a gardenlet Via `gardener-operator`

The gardenlet can automatically be deployed by `gardener-operator` into existing Kubernetes clusters in order to register them as seeds.

## Prerequisites

Using this method only works when [`gardener-operator`](../concepts/operator.md) is managing the garden cluster.
If you have used the [`gardener/controlplane` Helm chart](../../charts/gardener/controlplane) for the deployment of the Gardener control plane, please refer to [this document](deploy_gardenlet_manually.md).

> [!TIP]
> The initial seed cluster can be the garden cluster itself, but for better separation of concerns, it is recommended to only register other clusters as seeds.

## Deployment of gardenlets

Using this method, `gardener-operator` is only taking care of the very first deployment of gardenlet.
Once running, the gardenlet leverages the [self-upgrade](deploy_gardenlet_manually.md#self-upgrades) strategy in order to keep itself up-to-date.
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
          credentialsRef:
            apiVersion: v1
            kind: Secret
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
> [!IMPORTANT]
> After successful deployment of gardenlet, `gardener-operator` will delete the `remote-cluster-kubeconfig` `Secret` and set `.spec.kubeconfigSecretRef` to `nil`.
> This is because the kubeconfig will never ever be needed anymore (`gardener-operator` is only responsible for initial deployment, and gardenlet updates itself with an in-cluster kubeconfig).
> In case your landscape is managed via a GitOps approach, you might want to reflect this change in your repository.

### Forceful Re-Deployment

In certain scenarios, it might be necessary to forcefully re-deploy the gardenlet.
For example, in case the gardenlet client certificate has been expired or is "lost", or the gardenlet `Deployment` has been "accidentally" (😉) deleted from the seed cluster.

You can trigger the forceful re-deployment by annotating the `Gardenlet` with

```
gardener.cloud/operation=force-redeploy
```

> [!TIP]
> Do not forget to create the kubeconfig `Secret` and re-add the `.spec.kubeconfigSecretRef` to the `Gardenlet` specification if this is a remote cluster.

`gardener-operator` will remove the operation annotation after it's done.
Just like after the initial deployment, it'll also delete the kubeconfig `Secret` and set `.spec.kubeconfigSecretRef` to `nil`, see above.

### Configuring the connection to garden cluster
The garden cluster connection of your seeds are configured automatically by `gardener-operator`.
You could also specify the `gardenClusterAddress` and `gardenClusterCACert` in the `Gardenlet` resource manually, but this is not recommended.

If `GardenClusterAddress` is unset `gardener-operator` will determine the address automatically based on the `Garden` resource.
It is set to `"api." + garden.spec.virtualCluster.dns.domains[0]` which should cover most use cases since this is the immutable address of the garden cluster.  
If the runtime cluster is used as a seed cluster and `IstioTLSTermination` feature is not active, `gardenlet` overwrites the address with the internal service address of the garden cluster at runtime.
This happens for this single seed cluster only, so any managed seed running on this seed cluster will still use the default address of the garden cluster.

`gardenClusterCACert` is deprecated and should not be set. In this case, `gardenlet` will update the garden cluster CA certificate automatically from the garden cluster.

If a seed managed by a `Gardenlet` resource loses permanent access to the garden cluster for some reason, you can re-establish the connection by using the [Forceful Re-Deployment](#forceful-re-deployment) feature.
