# Deploy a gardenlet Manually

Manually deploying a gardenlet is usually only required if the Kubernetes cluster to be registered as a seed cluster is managed via third-party tooling (i.e., the Kubernetes cluster is not a shoot cluster, so [Deploy a gardenlet Automatically](deploy_gardenlet_automatically.md) cannot be used).
In this case, `gardenlet` needs to be deployed manually, meaning that its [Helm chart](../../charts/gardener/gardenlet) must be installed.

> [!TIP]
> Once you've deployed a gardenlet manually, you can deploy new gardenlets automatically.
> The manually deployed gardenlet is then used as a template for the new gardenlets.
> For more information, see [Deploy a gardenlet Automatically](deploy_gardenlet_automatically.md).

## Prerequisites

### Kubernetes Cluster that Should Be Registered as a Seed Cluster

- Verify that the cluster has a [supported Kubernetes version](../usage/shoot-operations/supported_k8s_versions.md).

- Determine the nodes, pods, and services CIDR of the cluster.
  You need to configure this information in the `Seed` configuration.
  Gardener uses this information to check that the shoot cluster isn't created with overlapping CIDR ranges.

- Every seed cluster needs an Ingress controller which distributes external requests to internal components like Plutono and Prometheus.
  For this, configure the following lines in your [Seed resource](../../example/50-seed.yaml):
  ```yaml
  spec:
    dns:
      provider:
        type: aws-route53
        secretRef:
          name: ingress-secret
          namespace: garden
    ingress:
      domain: ingress.my-seed.example.com
      controller:
        kind: nginx
        providerConfig:
          <some-optional-provider-specific-config-for-the-ingressController>
  ```

## Procedure Overview

1. Prepare the garden cluster:
    1. [Create a bootstrap token secret in the `kube-system` namespace of the garden cluster](#create-a-bootstrap-token-secret-in-the-kube-system-namespace-of-the-garden-cluster)
    2. [Create RBAC roles for the gardenlet to allow bootstrapping in the garden cluster](#create-rbac-roles-for-the-gardenlet-to-allow-bootstrapping-in-the-garden-cluster)
2. [Prepare the gardenlet Helm chart](#prepare-the-gardenlet-helm-chart).
3. [Automatically register shoot cluster as a seed cluster](#automatically-register-shoot-cluster-as-a-seed-cluster).
4. [Deploy the gardenlet](#deploy-the-gardenlet)
5. [Check that the gardenlet is successfully deployed](#check-that-the-gardenlet-is-successfully-deployed)

## Create a Bootstrap Token Secret in the `kube-system` Namespace of the Garden Cluster

The gardenlet needs to talk to the [Gardener API server](../concepts/apiserver.md) residing in the garden cluster.

Use gardenlet's ability to request a signed certificate for the garden cluster by leveraging [Kubernetes Certificate Signing Requests](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/).
The gardenlet performs a TLS bootstrapping process that is similar to the [Kubelet TLS Bootstrapping](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/).
Make sure that the API server of the garden cluster has [bootstrap token authentication](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/#enabling-bootstrap-token-authentication) enabled.

The client credentials required for the gardenlet's TLS bootstrapping process need to be either `token` or `certificate` (OIDC isn't supported) and have permissions to create a Certificate Signing Request ([CSR](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/)).
It's recommended to use [bootstrap tokens](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) due to their desirable security properties (such as a limited token lifetime).

Therefore, first create a bootstrap token secret for the garden cluster:

``` yaml
apiVersion: v1
kind: Secret
metadata:
  # Name MUST be of form "bootstrap-token-<token id>"
  name: bootstrap-token-07401b
  namespace: kube-system

# Type MUST be 'bootstrap.kubernetes.io/token'
type: bootstrap.kubernetes.io/token
stringData:
  # Human readable description. Optional.
  description: "Token to be used by the gardenlet for Seed `sweet-seed`."

  # Token ID and secret. Required.
  token-id: 07401b # 6 characters
  token-secret: f395accd246ae52d # 16 characters

  # Expiration. Optional.
  # expiration: 2017-03-10T03:22:11Z

  # Allowed usages.
  usage-bootstrap-authentication: "true"
  usage-bootstrap-signing: "true"
```

When you later prepare the gardenlet Helm chart, a `kubeconfig` based on this token is shared with the gardenlet upon deployment.

## Prepare the gardenlet Helm Chart

This section only describes the minimal configuration, using the global configuration values of the gardenlet Helm chart.
For an overview over all values, see the [configuration values](../../charts/gardener/gardenlet/values.yaml).
We refer to the global configuration values as _gardenlet configuration_ in the following procedure.

1. Create a gardenlet configuration `gardenlet-values.yaml` based on [this template](../../charts/gardener/gardenlet/values.yaml).

2. Create a bootstrap `kubeconfig` based on the bootstrap token created in the garden cluster.

   Replace the `<bootstrap-token>` with `token-id.token-secret` (from our previous example: `07401b.f395accd246ae52d`) from the bootstrap token secret.

   ```yaml
   apiVersion: v1
   kind: Config
   current-context: gardenlet-bootstrap@default
   clusters:
   - cluster:
       certificate-authority-data: <ca-of-garden-cluster>
       server: https://<endpoint-of-garden-cluster>
     name: default
   contexts:
   - context:
       cluster: default
       user: gardenlet-bootstrap
     name: gardenlet-bootstrap@default
   users:
   - name: gardenlet-bootstrap
     user:
       token: <bootstrap-token>
   ```

3. In the `gardenClientConnection.bootstrapKubeconfig` section of your gardenlet configuration, provide the bootstrap `kubeconfig` together with a name and namespace to the gardenlet Helm chart.

    ```yaml
    gardenClientConnection:
      bootstrapKubeconfig:
        name: gardenlet-kubeconfig-bootstrap
        namespace: garden
        kubeconfig: |
          <bootstrap-kubeconfig>  # will be base64 encoded by helm
    ```

    The bootstrap `kubeconfig` is stored in the specified secret.

4. In the `gardenClientConnection.kubeconfigSecret` section of your gardenlet configuration, define a name and a namespace where the gardenlet stores the real `kubeconfig` that it creates during the bootstrap process.
   If the secret doesn't exist, the gardenlet creates it for you.

    ```yaml
    gardenClientConnection:
      kubeconfigSecret:
        name: gardenlet-kubeconfig
        namespace: garden
    ```

### Updating the Garden Cluster CA

The kubeconfig created by the gardenlet in step 4 will not be recreated as long as it exists, even if a new bootstrap kubeconfig is provided.
To enable rotation of the garden cluster CA certificate, a new bundle can be provided via the `gardenClientConnection.gardenClusterCACert` field.
If the provided bundle differs from the one currently in the gardenlet's kubeconfig secret then it will be updated.
To remove the CA completely (e.g. when switching to a publicly trusted endpoint), this field can be set to either `none` or `null`.

## Prepare Seed Specification

When gardenlet  starts, it tries to register a `Seed` resource in the garden cluster based on the specification provided in `seedConfig` in its configuration.

> This procedure doesn't describe all the possible configurations for the `Seed` resource.
> For more information, see:
> - [Example Seed resource](../../example/50-seed.yaml)
> - [Configurable Seed settings](../operations/seed_settings.md)

1. Supply the `Seed` resource in the `seedConfig` section of your gardenlet configuration `gardenlet-values.yaml`.
1. Add the `seedConfig` to your gardenlet configuration `gardenlet-values.yaml`.
The field `seedConfig.spec.provider.type` specifies the infrastructure provider type (for example, `aws`) of the seed cluster.
For all supported infrastructure providers, see [Known Extension Implementations](../../extensions/README.md#known-extension-implementations).

    ```yaml
    # ...
    seedConfig:
      metadata:
        name: sweet-seed
        labels:
          environment: evaluation
        annotations:
          custom.gardener.cloud/option: special
      spec:
        dns:
          provider:
            type: <provider>
            secretRef:
              name: ingress-secret
              namespace: garden
        ingress: # see prerequisites
          domain: ingress.dev.my-seed.example.com
          controller:
            kind: nginx
        networks: # see prerequisites
          nodes: 10.240.0.0/16
          pods: 100.244.0.0/16
          services: 100.32.0.0/13
          shootDefaults: # optional: non-overlapping default CIDRs for shoot clusters of that Seed
            pods: 100.96.0.0/11
            services: 100.64.0.0/13
        provider:
          region: eu-west-1
          type: <provider>
    ```

Apart from the seed's name, `seedConfig.metadata` can optionally contain `labels` and `annotations`.
gardenlet will set the labels of the registered `Seed` object to the labels given in the `seedConfig` plus `gardener.cloud/role=seed`.
Any custom labels on the `Seed` object will be removed on the next restart of gardenlet.
If a label is removed from the `seedConfig` it is removed from the `Seed` object as well.
In contrast to labels, annotations in the `seedConfig` are added to existing annotations on the `Seed` object.
Thus, custom annotations that are added to the `Seed` object during runtime are not removed by gardenlet on restarts.
Furthermore, if an annotation is removed from the `seedConfig`, gardenlet does **not** remove it from the `Seed` object.

### Optional: Enable HA Mode

You may consider running `gardenlet` with multiple replicas, especially if the seed cluster is configured to host [HA shoot control planes](../usage/high-availability/shoot_high_availability.md).
Therefore, the following Helm chart values define the degree of high availability you want to achieve for the `gardenlet` deployment.

```yaml
replicaCount: 2 # or more if a higher failure tolerance is required.
failureToleranceType: zone # One of `zone` or `node` - defines how replicas are spread.
```

### Optional: Enable Backup and Restore

The seed cluster can be set up with backup and restore for the main `etcds` of shoot clusters.

Gardener uses [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) that [integrates with different storage providers](https://github.com/gardener/etcd-backup-restore/blob/master/docs/deployment/getting_started.md) to store the shoot cluster's main `etcd` backups.
Make sure to obtain client credentials that have sufficient permissions with the chosen storage provider.

Create a secret in the garden cluster with client credentials for the storage provider.
The format of the secret is cloud provider specific and can be found in the repository of the respective Gardener extension.
For example, the secret for AWS S3 can be found in the AWS provider extension ([30-etcd-backup-secret.yaml](https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-etcd-backup-secret.yaml)).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sweet-seed-backup
  namespace: garden
type: Opaque
data:
  # client credentials format is provider specific
```

Configure the `Seed` resource in the `seedConfig` section of your gardenlet configuration to use backup and restore:

```yaml
# ...
seedConfig:
  metadata:
    name: sweet-seed
  spec:
    backup:
      provider: <provider>
      secretRef:
        name: sweet-seed-backup
        namespace: garden
```

### Optional: Enable Self-Upgrades

In order to take off the continuous task of deploying gardenlet's Helm chart in case you want to upgrade its version, it supports self-upgrades.
The way this works is that it pulls information (its configuration and deployment values) from a `seedmanagement.gardener.cloud/v1alpha1.Gardenlet` resource in the garden cluster.
This resource must be in the `garden` namespace and must have the same name as the `Seed` the gardenlet is responsible for.
For more information, see [this section](#self-upgrades).

In order to make gardenlet automatically create a corresponding `seedmanagement.gardener.cloud/v1alpha1.Gardenlet` resource, you must provide

```yaml
selfUpgrade:
  deployment:
    helm:
      ociRepository:
        ref: <url-to-oci-repository-containing-gardenlet-helm-chart>
```

in your `gardenlet-values.yaml` file.
Please replace the `ref` placeholder with the URL to the OCI repository containing the gardenlet Helm chart you are installing.

> [!NOTE]
> If you don't configure this `selfUpgrade` section in the initial deployment, you can also do it later, or you directly create the corresponding `seedmanagement.gardener.cloud/v1alpha1.Gardenlet` resource in the garden cluster.

## Deploy the gardenlet

The `gardenlet-values.yaml` looks something like this (with backup for shoot clusters enabled):

```yaml
# <default config>
# ...
config:
  gardenClientConnection:
    # ...
    bootstrapKubeconfig:
      name: gardenlet-bootstrap-kubeconfig
      namespace: garden
      kubeconfig: |
        apiVersion: v1
        clusters:
        - cluster:
            certificate-authority-data: <dummy>
            server: <my-garden-cluster-endpoint>
          name: my-kubernetes-cluster
        # ...

    kubeconfigSecret:
      name: gardenlet-kubeconfig
      namespace: garden
  # ...
  # <default config>
  # ...
  seedConfig:
    metadata:
      name: sweet-seed
    spec:
      dns:
        provider:
          type: <provider>
          secretRef:
            name: ingress-secret
            namespace: garden
      ingress: # see prerequisites
        domain: ingress.dev.my-seed.example.com
        controller:
          kind: nginx
      networks:
        nodes: 10.240.0.0/16
        pods: 100.244.0.0/16
        services: 100.32.0.0/13
        shootDefaults:
          pods: 100.96.0.0/11
          services: 100.64.0.0/13
      provider:
        region: eu-west-1
        type: <provider>
      backup:
        provider: <provider>
        secretRef:
          name: sweet-seed-backup
          namespace: garden
```

Deploy the gardenlet Helm chart to the Kubernetes cluster:

```bash
helm install gardenlet charts/gardener/gardenlet \
  --namespace garden \
  -f gardenlet-values.yaml \
  --wait
```

This Helm chart creates:

- A service account `gardenlet` that the gardenlet can use to talk to the Seed API server.
- RBAC roles for the service account (full admin rights at the moment).
- The secret (`garden`/`gardenlet-bootstrap-kubeconfig`) containing the bootstrap `kubeconfig`.
- The gardenlet deployment in the `garden` namespace.

## Check that the gardenlet Is Successfully Deployed

1. Check that the gardenlets certificate bootstrap was successful.

   Check if the secret `gardenlet-kubeconfig` in the namespace `garden` in the seed cluster
   is created and contains a `kubeconfig` with a valid certificate.

   1. Get the `kubeconfig` from the created secret.

       ```
       $ kubectl -n garden get secret gardenlet-kubeconfig -o json | jq -r .data.kubeconfig | base64 -d
       ```

   2. Test against the garden cluster and verify it's working.

   3. Extract the `client-certificate-data` from the user `gardenlet`.

   4. View the certificate:

      ```
      $ openssl x509 -in ./gardenlet-cert -noout -text
      ```

2. Check that the bootstrap secret `gardenlet-bootstrap-kubeconfig` has been deleted from the seed cluster in namespace `garden`.

3. Check that the seed cluster is registered and `READY` in the garden cluster.

   Check that the seed cluster `sweet-seed` exists and all conditions indicate that it's available.
   If so, the [Gardenlet is sending regular heartbeats](../concepts/gardenlet.md#heartbeats) and the [seed bootstrapping](../operations/seed_bootstrapping.md) was successful.

   Check that the conditions on the `Seed` resource look similar to the following:

   ```bash
   $ kubectl get seed sweet-seed -o json | jq .status.conditions
   [
     {
       "lastTransitionTime": "2020-07-17T09:17:29Z",
       "lastUpdateTime": "2020-07-17T09:17:29Z",
       "message": "Gardenlet is posting ready status.",
       "reason": "GardenletReady",
       "status": "True",
       "type": "GardenletReady"
     },
     {
       "lastTransitionTime": "2020-07-17T09:17:49Z",
       "lastUpdateTime": "2020-07-17T09:53:17Z",
       "message": "Backup Buckets are available.",
       "reason": "BackupBucketsAvailable",
       "status": "True",
       "type": "BackupBucketsReady"
     }
   ]
   ```

## Self Upgrades

In order to keep your gardenlets in such "unmanaged seeds" up-to-date (i.e., in seeds which are no shoot clusters), its Helm chart must be regularly deployed.
This requires network connectivity to such clusters which can be challenging if they reside behind a firewall or in restricted environments.
It is much simpler if gardenlet could keep itself up-to-date, based on configuration read from the garden cluster.
This approach greatly reduces operational complexity.

gardenlet runs [a controller](../concepts/gardenlet.md#gardenlet-controller) which watches for `seedmanagement.gardener.cloud/v1alpha1.Gardenlet` resources in the garden cluster in the `garden` namespace having the same name as the `Seed` the gardenlet is responsible for.
Such resources contain its component configuration and deployment values.
Most notably, a URL to an OCI repository containing gardenlet's Helm chart is included.

An example `Gardenlet` resource looks like this:

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
    gardenClientConnection:
      kubeconfigSecret:
        name: gardenlet-kubeconfig
        namespace: garden
    controllers:
      shoot:
        reconcileInMaintenanceOnly: true
        respectSyncPeriodOverwrite: true
      shootState:
        concurrentSyncs: 0
    featureGates:
      DefaultSeccompProfile: true
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

On reconciliation, gardenlet downloads the Helm chart, renders it with the provided values, and then applies it to its own cluster.
Hence, in order to keep a gardenlet up-to-date, it is enough to update the tag/digest of the OCI repository ref for the Helm chart:

```yaml
spec:
  deployment:
    helm:
      ociRepository:
        ref: <url-to-gardenlet-chart-repository>:v1.97.0
```

This way, network connectivity to the cluster in which gardenlet runs is not required at all (at least for deployment purposes).

When you delete this resource, nothing happens: gardenlet remains running with the configuration as before.
However, self-upgrades are obviously not possible anymore.
In order to upgrade it, you have to either recreate the `Gardenlet` object, or redeploy the Helm chart.

## Related Links

- [Issue #1724: Harden Gardenlet RBAC privileges](https://github.com/gardener/gardener/issues/1724).
- [Backup and Restore](../concepts/backup-restore.md).
