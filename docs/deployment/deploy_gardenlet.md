# Deploy a Gardenlet

This document describes how to manually deploy a Gardenlet into a Kubernetes cluster and how to automatically register this cluster as a Seed.

## When is this useful

- When deploying the first Gardenlet into a Gardener installation (this is done automatically by [garden-setup](https://github.com/gardener/garden-setup))
- To register a Kubernetes cluster as a `Seed` cluster that is in a "fenced" environment, such as a private datacenter or behind a firewall.

Elaborating on the latter, the Gardenlet only requires network connectivity from the Gardenlet to the Garden cluster (not the other way round), so it can be used to register Kubernetes clusters with no public endpoint. 
When registering a Seed cluster in such a "fenced" environment, the [automatic Gardenlet deployment into Shooted Seed clusters](https://github.com/gardener/gardener/blob/master/docs/usage/shooted_seed.md) cannot be used (see [self-deployment](#self-deployment)).

Additionally, the Gardenlet can automatically register a Kubernetes cluster as a Seed cluster with the Gardener based on a pre-defined Seed specification (see [automatically register as a Seed cluster](#automatically-register-as-a-seed-cluster)).

## Self deployment

The Gardenlet can **automatically** deploy itself into Shoot clusters and register this cluster as a `Seed` (these clusters are called `Shooted Seeds` in the Gardener terminology). 
This is the preferred way to add additional Seed clusters as Shoots already come with production-grade qualities that are also demanded for seeds.

This works by registering an initial cluster as a Seed cluster (you can use the Garden cluster itself!)  - that has a Gardenlet already installed (e.g by following this guide). 
This initial cluster can be used as the Seed cluster for other `Shooted Seed` clusters.  
Hence, the initial cluster hosts the control planes of the other Seed clusters.

The Gardenlet deployed in the initial cluster, can deploy itself into the `Shooted Seed` clusters.  
The advantage of this approach is, that there is only one initial Gardenlet installation required - every other `Shooted Seed` has a Gardenlet deployed automatically.

Overall this allows the Gardener system to dynamically scale - also scenarios such as "Seed autoscaling" are possible.

## Prerequisites

- **Kubernetes cluster that should be registered as a Seed cluster**
    
    - Verify that the cluster has a [supported Kubernetes version](../usage/supported_k8s_versions.md).
    - Determine the nodes, pods and services CIDR of the Cluster. This needs to be configured in the `Seed` configuration. Gardener uses this information to check that the Shoot cluster is not created with overlapping CIDR ranges.
    - A current prerequisite of Kubernetes clusters that are used as Seed clusters is to have a pre-deployed Ingress controller. This is pre-installed for `Shooted Seed` clusters.  
        An ingress controller is required to:
        - configure the `Seed` cluster resource (tells the Gardenlet which ingress domain it can use to deploy ingress resources for seed components like grafana and prometheus).
            
        - handle ingress resources that are deployed during the Seed bootstrap by the Gardenlet.
            
            :warning:  
            There should exist a DNS record `*.ingress.<SEED-CLUSTER-DOMAIN>` where `<SEED-CLUSTER-DOMAIN>` is the value of the `.dns.ingressDomain` field of [a Seed cluster resource](../../example/50-seed.yaml) (or the [respective Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml#L84-L85)).
            
            **This is how it could be done for the Nginx ingress controller**
            
            Deploy nginx into the `kube-system` namespace in the Kubernetes cluster that should be registered as a `Seed`.
            
            Nginx will on most cloud providers create the service with type `LoadBalancer` with an external ip.
            
            ```
            NAME                        TYPE           CLUSTER-IP    EXTERNAL-IP
            nginx-ingress-controller    LoadBalancer   10.0.15.46    34.200.30.30
            ```
            
            Create a wildcard `A` record (e.g *.ingress.sweet-seed.<my-domain>. IN A 34.200.30.30) with your DNS provider and point it to the external ip of the ingress service. This ingress domain is later required to register the `Seed` cluster.
            
- **Kubeconfig for the Seed cluster**
    - Required to deploy the Gardenlet Helm chart to the Seed cluster. 
      Requires admin privileges. 
      The helm chart contains a service account `gardenlet` that the Gardenlet deployment uses by default to talk to the Seed API server.
      - If the Gardenlet is not deployed in the Seed cluster, the Gardenlet can be configured to use a kubeconfig (also need full admin rights) from a mounted directory. 
        This can be configured by `seedClientConnection.kubeconfig` in the [Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml). 
        As this document describes the Helm-based setup on the Seed cluster, this configuration option is not used in the following. 

## Prepare the Garden cluster

1.  **Create a bootstrap token secret in the `kube-system` namespace of the garden cluster**
    The Gardenlet needs to talk to the [Gardener Extension API server](../concepts/apiserver.md) residing in the Garden cluster. 
    
    The Gardenlet can either be configured with an already existing Garden cluster kubeconfig 
     - by specifying `gardenClientConnection.kubeconfig` in the [Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml) 
     - or supplying the environment variable `GARDEN_KUBECONFIG` pointing to a mounted kubeconfig file)
    
    The preferred way however, is to use the Gardenlets ability to request a signed certificate for the Garden cluster by leveraging [Kubernetes Certificate Signing Requests](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/).
    The Gardenlet performs a TLS bootstrapping process that is very similar to the [Kubelet TLS Bootstrapping](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/).
    Make sure that the API server of the Garden cluster has [bootstrap token authentication](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/#enabling-bootstrap-token-authentication) enabled.
    
    The client credentials required for the Gardenlets TLS bootstrapping process, need to be either `token` or `certificate` (e.g. OIDC is not supported) and have permissions to create a Certificate Signing Request ([CSR](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/)).
    It is recommended to use [Bootstrap tokens](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) (also see [here](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/#bootstrap-tokens)) due to their desirable security properties (such as a limited token lifetime).
    
    Therefore, first create a Bootstrap token secret for the Garden cluster: 
    
    ``` yaml
    apiVersion: v1
    kind: Secret
    metadata:
      # Name MUST be of form "bootstrap-token-<token id>"
      name: bootstrap-token-sweetseed
      namespace: kube-system
    
    # Type MUST be 'bootstrap.kubernetes.io/token'
    type: bootstrap.kubernetes.io/token
    stringData:
      # Human readable description. Optional.
      description: "Token to be used by the Gardenlet for Seed `sweet-seed`."
    
      # Token ID and secret. Required.
      token-id: 07401b # 6 characters
      token-secret: f395accd246ae52d # 16 characters
    
      # Expiration. Optional.
      # expiration: 2017-03-10T03:22:11Z
    
      # Allowed usages.
      usage-bootstrap-authentication: "true"
      usage-bootstrap-signing: "true"
    ```
    
   [In a later step](#prepare-the-gardenlet-helm-chart) a kubeconfig based on this token will be made accessible to the Gardenlet upon deployment.
    
2.  **Create RBAC roles for the Gardenlet to allow bootstrapping in the garden cluster**
    
    This step is only required if this is the first Gardenlet in the Gardener installation.
    Additionally, when using the [controlplane chart](../../charts/gardener/controlplane), the following resources are already contained in the Helm chart, i.e., if you use it you can skip these steps as the needed RBAC roles should already exist. 
    
    The Gardenlet uses the configured bootstrap kubeconfig in `gardenClientConnection.bootstrapKubeconfig` to request a signed certificate for the user `gardener.cloud:system:seed:<seed-name>` in the group `gardener.cloud:system:seeds`.
    
    Create a `ClusterRole` and `ClusterRoleBinding` that grant full admin permissions to authenticated Gardenlets.  
    There is already an [issue open](https://github.com/gardener/gardener/issues/1724) to improve the scope of these permissions.
    
    Create the following resources in the Garden cluster:
    
    ```yaml
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: gardener.cloud:system:seeds
    rules:
      - apiGroups:
          - '*'
        resources:
          - '*'
        verbs:
          - '*'
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: gardener.cloud:system:seeds
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: gardener.cloud:system:seeds
    subjects:
      - kind: Group
        name: gardener.cloud:system:seeds
        apiGroup: rbac.authorization.k8s.io
    ```

## Prepare the Gardenlet helm chart

This only describes the minimal configuration. For more configuration options [please take a look at the configuration values](../../charts/gardener/gardenlet/values.yaml).

1.  Create a `gardenlet-values.yaml` based on [this template](https://github.com/gardener/gardener/blob/master/charts/gardener/gardenlet/values.yaml).
    
2.  Create a bootstrap kubeconfig based on the bootstrap token created in the Garden cluster.

    The <bootstrap-token> should be substituted with `token-id.token-secret` (from our example above: `07401b.f395accd246ae52d`) from the bootstrap token secret.
    
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
3.  Provide this bootstrap kubeconfig together with a desired name and namespace to the Gardenlet Helm chart values [here](../../charts/gardener/gardenlet/values.yaml#L31-L35):
    
    ```yaml
    gardenClientConnection:
      bootstrapKubeconfig:
        name: gardenlet-kubeconfig-bootstrap
        namespace: garden
        kubeconfig: |
          <bootstrap-kubeconfig>  # will be base64 encoded by helm
    ```
    
    The bootstrap kubeconfig will be stored in the specified secret.
    
4.  Define a name and namespace where the Gardenlet shall store the real kubeconfig it creates during the bootstrap process [here](../../charts/gardener/gardenlet/values.yaml#L31-L35):
    
    This secret will be created by the Gardenlet in case it does not exist.
    
    ```yaml
    gardenClientConnection:
      kubeconfigSecret:
        name: gardenlet-kubeconfig
        namespace: garden
    ```

## Automatically register as a Seed cluster

A Seed cluster can either be registered by manually creating the [`Seed` resource](../../example/50-seed.yaml) or automatically by the Gardenlet.  
This functionality is particularly useful for `Shooted Seed` clusters, as the Gardenlet in the Garden cluster deploys a copy of itself into the cluster with automatic registration of the `Seed` configured.  
However, it can also be used to have a streamlined Seed registration process when manually deploying the Gardenlet.

Please note that this document does not describe all the possible configurations for the `Seed` resource. For more information take a look at the [example Seed resource](../../example/50-seed.yaml) and the configurable [Seed settings](../usage/seed_settings.md).

**Adjust the Gardenlet component configuration**

Next, supply the `Seed` resource in the Gardenlet configuration (`seedConfig`).

Note that with the `seedConfig` supplied, the Gardenlet is only responsible to create and reconcile this one configured seed (in the example above: `sweet-seed`). 
The Gardenlet can also be configured to reconcile (but not create!) multiple Seeds [based on a label selector](../concepts/gardenlet.md#seed-config-vs-seed-selector), but this is only recommended [for a development setup](../development/local_setup.md#appendix).

Add the `seedConfig` to the helm chart `gardenlet-values.yaml`.
The field `seedConfig.spec.provider.type` specifies the infrastructure provider type (e.g. `aws`) of the Seed cluster.
For all supported infrastructure providers please [take a look here](https://github.com/gardener/gardener/blob/master/extensions/README.md#known-extension-implementations).   

```yaml
....
seedConfig:
  metadata:
    name: sweet-seed
  spec:
    dns:
      ingressDomain: ingress.sweet-seed.<my-domain> # see prerequisites
    networks: # see prerequisites
      nodes: 10.240.0.0/16
      pods: 100.244.0.0/16
      services: 100.32.0.0/13
      shootDefaults: # optional: non-overlapping default CIDRs for Shoot clusters of that Seed
        pods: 100.96.0.0/11
        services: 100.64.0.0/13
    provider:
      region: eu-west-1
      type: <provider>
```

### Optional: Enable backup and restore

The Seed cluster can be setup with backup and restore for the main etcds of Shoot clusters. For more information please [see the documentation](../concepts/backup-restore.md).

Gardener uses [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) that [integrates with different storage providers](https://github.com/gardener/etcd-backup-restore/blob/master/doc/usage/getting_started.md#usage) to store the Shoots' main etcd backups.
Make sure to obtain client credentials that have [sufficient permissions with the chosen storage provider](#TODO).

Create a secret in the garden cluster with client credentials for the storage provider.
The format of the secret is cloud provider specific and can be found in the repository of the respective Gardener extension. 
For example, the secret for AWS S3 can be found in the AWS provider extension [here](https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-etcd-backup-secret.yaml).

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

Configure the `Seed` resource in the Gardenlet configuration (`seedConfig`) to use backup and restore:

```yaml
...
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

## Deploy the Gardenlet

The Gardenlet does not have to run in the same Kubernetes cluster as the Seed it is registering and reconciling, but it is in most cases advantageous to use in-cluster communication to talk to the Seed API server. Running a Gardenlet outside of the cluster is mostly used for local development.

For your reference, the `gardenlet-values.yaml` should look something like this (with automatic Seed registration and backup for Shoot clusters enabled):

```yaml
global:
  # Gardenlet configuration values
  gardenlet:
    enabled: true
    ...
    <default config>
    ...
    config:
      gardenClientConnection:
        ...
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
            ....
            
        kubeconfigSecret:
          name: gardenlet-kubeconfig
          namespace: garden
      ...
      <default config>
      ...
      seedConfig:
        metadata:
          name: sweet-seed
        spec:
          dns:
            ingressDomain: ingress.sweet-seed.<my-domain>
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

Deploy the Gardenlet Helm chart to the Kubernetes cluster.

```bash
helm install gardenlet charts/gardener/gardenlet \
  --namespace garden \
  -f gardenlet-values.yaml \
  --wait
```

This will create most importantly

- A service account `gardenlet` that the Gardenlet can use to talk to the Seed API server.
- RBAC roles for the service account (full admin rights at the moment).
- The secret (`garden`/`gardenlet-bootstrap-kubeconfig`) containing the bootstrap kubeconfig.
- The gardenlet deployment in the `garden` namespace.

## Check that the Gardenlet is successfully deployed

1.  **Check that the Gardenlets certificate bootstrap was successful**
    
    The secret `gardenlet-kubeconfig` in the namespace `garden` in the Seed cluster should be created and contain a kubeconfig with a valid certificate.
    
    Get the kubeconfig from the created secret.
    
    ```
    $ kubectl -n garden get secret gardenlet-kubeconfig -o json | jq -r .data.kubeconfig | base64 -d
    ```
    
    Possibly test against the Garden cluster and verify it is working.
    
    Extract the `client-certificate-data` from the user `gardenlet`.
    
    View the certificate:
    
    ```
     $ openssl x509 -in ./gardenlet-cert -noout -text
    ```
    
    Make sure that the certificate is valid for one year.
    
2.  **Make sure the bootstrap secret has been deleted from the Seed cluster**
    
    Check that the secret `gardenlet-bootstrap-kubeconfig` in the namespace `garden` has been deleted.
    
3.  **Check that the Seed is registered and `READY` in the Garden cluster**
    
    Check that the Seed `sweet-seed` exists and all conditions indicate availability.
    This indicates that the [Gardenlet is sending regular heartbeats](../concepts/gardenlet.md##heartbeats) and the [Seed bootstrapping](../usage/seed_bootstrapping.md) was successful.
    
    The conditions on the `Seed` resource should include the following:
    
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
        "message": "Seed cluster has been bootstrapped successfully.",
        "reason": "BootstrappingSucceeded",
        "status": "True",
        "type": "Bootstrapped"
      }
    ]
    ```