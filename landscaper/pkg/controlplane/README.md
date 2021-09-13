# Prerequisites

1) A kubeconfig for a Kubernetes cluster (`runtime` cluster) to run the Gardener control plane pods (Gardener Extension API server, Gardener Controller Manager, Gardener Scheduler, Gardener Admission Controller)

2) A kubeconfig for a Kubernetes cluster with a Kubernetes API server set up for API aggregation.
  - The Gardener API server extends this API server and serves the Gardener resource groups.
  - For more information how to configure the aggregation layer see the [Kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer).
  - This can be the `runtime` cluster.

3) An existing etcd cluster that is accessible from the runtime cluster to be used by the GardenerExtension API server
  - if client authentication is enabled on etcd: need to obtain client credentials (x509 certificate and key) trusted by the etcd cluster.
  
# Deployment Models

The Gardener control plane can be set up in two different ways.

### 1) Extending the Runtime Cluster

In this setup, 
- the `runtime` cluster hosts the Gardener control plane pods
- the API server of the `runtime` cluster is extended by the Gardener Extension API server, thus serves the Gardener resource groups (Shoot, Seed, ...).
- the Gardener control plane pods are configured against the API server of the `runtime` cluster

Consider this option if
- you can directly **configure and upgrade the API server** itself (e.g. to set up API aggregation) - **this is typically not the case on managed Kubernetes offerings**.
- there are no performance concerns considering the `runtime` cluster's API server will serve both the Gardener API plus hosting other workload. 

### 2) Virtual Garden

**Prerequisites**
- An Kubernetes API server deployment configured for API aggregation in the `runtime` cluster.
  This component does not deploy the such API server. It must already exist in the runtime cluster
  and setup for [API aggregation](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer).
  This is the case when the [virtual garden component](https://github.com/gardener/virtual-garden) of the Landscaper has already been deployed to the `runtime` cluster.

In this setup,
- the `runtime` cluster hosts the Gardener control plane pods
- a dedicated Kubernetes API server deployed in the `runtime` cluster is extended by the Gardener Extension API server (`virtual-garden` API server).
  - the `virtual-garden` API server serves the Gardener API
- the Gardener control plane pods are configured against the `virtual-garden` API server

Consider this option if
- you want to use a dedicated etcd only for Gardener resource groups
- the etcd of the `runtime` cluster is not under you own control (like on managed Kubernetes offerings)
  - you might want to set up the etcd deployment for scale, deploy an automatic backup solution, etc. (This should be already done when using the [virtual garden component](https://github.com/gardener/virtual-garden))
- scalability of the API server of the runtime cluster is a concern
  - deployed as a dedicated deployment, the virtual Garden API server can scale independently of the `runtime` cluster's API server


Please note that all resources will be installed in the `garden` namespace.

# Mandatory configuration

The control plane component deploys the control plane helm  chart found in the [charts directory](../../../charts/gardener/controlplane/).
Also, the configuration of the component roughly corresponds to the configurable helm chart values.
Hence, most of the configuration of the Gardener API server is defaulted through the helm chart [values](../../../charts/gardener/controlplane/values.yaml).
On top of that, missing CA bundles and certificates for the control plane will be generated and exported.

Below describes the few required configuration options that cannot be defaulted.
There is an example of a minimal control plane import configuration [at the end of this document](#minimal-example-configuration).

## Runtime cluster

Configures the Kubernetes cluster that runs the Gardener control plane.
**Note**: The Kubernetes API server must be set up for API aggregation when **not** using the [`virtual-garden` deployment model](#2-virtual-garden).

``` yaml
runtimeCluster: 
  apiVersion: landscaper.gardener.cloud/v1alpha1
  kind: Target
  spec:
    type: landscaper.gardener.cloud/kubernetes-cluster
    config:
      kubeconfig: |
        ---
        apiVersion:...
        # here goes the kubeconfig of the runtime cluster

```

## Virtual Garden

Optionally, configure the `virtual-garden` setup option of Gardener.

``` yaml
virtualGarden:
  enabled: true
  kubeconfig: 
    apiVersion: landscaper.gardener.cloud/v1alpha1
    kind: Target
    spec:
      type: landscaper.gardener.cloud/kubernetes-cluster
      config:
        kubeconfig: |
          ---
          apiVersion:...
          # here goes the kubeconfig of the virtual-garden API server

```

## DNS Setup

Gardener manages DNS records for the communication between control planes of a Shoot cluster residing on the Seed and the data plane in the Shoot.
Therefore, a DNS provider with credentials and a domain has to be configured.
For instance, providing `example.test` as internal domain, Gardener creates an A record in the configured DNS provider
for each Shoot cluster in the form of `api.<shoot-name>.<project-name>.internal.shoot.example.test`.

Optionally, a `default domain`  can be configure to be used in the kubeconfig generated for 
Shoot cluster administrators.
For instance, providing `example.test` as default domain, Gardener creates Kubeconfigs for Shoot clusters with the domain
`api.<shoot-name>.<project-name>.shoot.example.test`

``` yaml
internalDomain:
  domain: "my.domain.com"
  provider: "aws-route53 / alicloud-dns / azure-dns / google-clouddns / openstack-designate / cloudflare-dns"
  credentials:
    # Example for AWS Route53 credentials 
    AWS_ACCESS_KEY_ID: abc
    AWS_SECRET_ACCESS_KEY: dbc

defaultDomain:
   # same configuration as for the internal domain
```

## Configure etcd for the Gardener API server

At least the URL of the etcd cluster must be provided.
- If the etcd is deployed in-cluster, the URL should be of the form `k8s-service-name:port`
- If the etcd serves TLS (configurable via flag `--cert-file` on etcd), this URL can use the HTTPS schema.

``` yaml
gardenerAPIserver:
  componentConfiguration:
    etcd:
      url: "virtual-garden-etcd-main-client.garden.svc:2379"
```

It is recommended to provide a PEM encoded CA bundle of the TLS serving certificate of etcd.
Used by the Gardener API server to verify that the TLS serving certificate of etcd is signed by this CA (when using TLS).
- Configures the flag `--etcd-cafile` on the Gardener API server


``` yaml
gardenerAPIserver:
  componentConfiguration:
    etcd:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
```

Provide client credentials, if the etcd cluster requires client authentication.
- This is the case when etcd flags `--client-cert-auth` and `--trusted-ca-file` are set.
Make sure that the client credentials are signed by the CA provided to etcd via the flag `--trusted-ca-file`

``` yaml
gardenerAPIserver:
  componentConfiguration:
    etcd:
      clientCrt: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      clientKey: |
        -----BEGIN RSA PRIVATE KEY-----
        ...
        -----END RSA PRIVATE KEY-----
```


# Optional configuration

## Set a Gardener identity

The Gardener cluster identity is a string that uniquely identifies the Gardener installation.
It can be any string that uniquely identifies the landscape.
If not provided, sets a default identity.

``` yaml
clusterIdentity: my-company-landscape-dev
```

## Custom Certificates and Secrets

## Secret for VPN

The VPN bridge from a Shoot's control plane running in the Seed cluster to the worker nodes of the Shoots is based
on OpenVPN. It requires a Diffie Hellman key.
If no such key is explicitly provided then the Gardener will use a default one (not recommended, but useful for local development).
The key is used for all Shoots.

Can be generated by `openssl dhparam -out dh2048.pem 2048`

```
openVPNDiffieHellmanKey: |
#   my-key generated by `openssl dhparam -out dh2048.pem 2048`
```

## Gardener API server

A valid PEM encoded x509 certificate and key to serve the TLS endpoints on the Gardener Extension API server can be provided.

``` yaml
gardenerAPIserver:
  componentConfiguration:
    tls:
      crt: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      key: |
        -----BEGIN RSA PRIVATE KEY-----
        ...
        -----END RSA PRIVATE KEY-----
```

Also, it is recommended to provide the PEM encoded CA bundle that signed the Gardener Extension API server's TLS serving certificates(configured above).
This CA bundle is set to the `APIService` resources for the Gardener resource groups in the to-be aggregated API server.
This is how the to be-aggregated Kubernetes API server is able to validate the Gardener Extension API server's TLS serving certificate.
For more information, please consult the [documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-aggregation-layer/#contacting-the-extension-apiserver).

``` yaml
gardenerAPIserver:
  componentConfiguration:
    caBundle:
      -----BEGIN CERTIFICATE-----
       ...
      -----END CERTIFICATE-----
```

## Gardener Controller Manager

Optionally, provide a PEM encoded x509 certificate and key for serving metrics over TLS.
Per default, http is used for the `/healthz` and metrics endpoint.

``` yaml
gardenerControllerManager:
  componentConfiguration:
    tls:
      crt: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      key: |
        -----BEGIN RSA PRIVATE KEY-----
        ...
        -----END RSA PRIVATE KEY-----
```

## Gardener Admission Controller

The Gardener Admission controller is deployed per default. To disable it configure the following:

```
gardenerAdmissionController:
  enabled: false:
```

If the Admission Controller shall be used, you can provide a custom CA bundle as well as TLS serving certificates.
The CA bundle is a PEM encoded CA bundle which will be used by the Gardener API server
to validate the TLS serving certificate of the Gardener Admission Webhook server of the Gardener Admission Controller.
The CA bundle is put into the `MutatingWebhookConfiguration` and `ValidatingWebhookConfiguration` resources when registering the Webhooks.
The TLS serving certificate of the Gardener Admission Webhook server has to be signed by this CA.


``` yaml
gardenerAdmissionController:
  componentConfiguration:
    caBundle:
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls:
      crt: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      key: |
        -----BEGIN RSA PRIVATE KEY-----
        ...
        -----END RSA PRIVATE KEY-----
```

### Seed Authorizer

The Seed Authorizer is a special-purpose authorization plugin that specifically authorizes API requests made by the gardenlets 
in the Garden cluster. Please see [here](https://github.com/gardener/gardener/blob/master/docs/deployment/gardenlet_api_access.md)
for more information.

**Prerequisite**: 

The Seed Authorizer must be already configured on the to-be extended API server (runtime cluster or virtual-garden).
This is already done when using the [virtual-garden component](https://github.com/gardener/virtual-garden).

The following configuration is required:


``` yaml
rbac:
  seedAuthorizer:
    enabled: true
```

This has the effect that the Gardenlet authenticating as `gardener.cloud:system:seeds` does NOT have
admin access to all resources in the Garden cluster (the RBAC rolebindings are not deployed for the Gardenlet).
Instead, the authorization decision is delegated via webhook from the `virtual-garden` /`runtime` cluster API Server
to the Seed Authorizer running as a webhook in the Gardener Admission Controller.

### Seed Restriction Plugin

The Seed restriction plugin can be enabled to provide an extra layer of security.
For more information, please see [here](https://github.com/gardener/gardener/blob/master/docs/deployment/gardenlet_api_access.md#seedrestriction-admission-webhook-enablement).

**Please note**: 
The Seed Restriction Plugin and the Seed Authorizer should be enabled together. 
If only one is enabled, then you are missing a piece of the security pie.
If the Seed Authorizer is enabled already, the Seed Restriction Plugin will be enabled per default.

The following configuration is required:

``` yaml
gardenerAdmissionController:
  seedRestriction:
    enabled: true
```
This sets up a `ValidatingWebhookConfiguration` pointing to the Gardener Admission Controller serving the
Seed restriction webhook.

## Custom deployment configurations
Each component has a set of common configuration values configuring its Kubernetes deployment.
Below is an example for the GCM.

```
gardenerControllerManager:
  deploymentConfiguration:
    replicaCount: 1
    serviceAccountName: gardener-controller-manager
    resources:
      requests:
        cpu: 100m
        memory: 100Mi
      limits:
        cpu: 750m
        memory: 512Mi
    podLabels:
      foo: bar
    podAnnotations:
      foo: bar
    vpa: true
```

Depending on the component, there are additional configuration options such 
as specifying additional volume mounts and environment variables. 
Please check with the [import configuration types](apis/imports) of each component.

## Custom component configuration

The component configuration is the config file for each Gardener control plane component.
You can find example configurations for each control plane component [in the example directory](../../../example).

### Component configuration for the Gardener Controller Manager

Specifying a GCM component configuration is optional, as default values will be provided.
If you want to overwrite the default component configuration values, please see the [example configuration](../../../example/20-componentconfig-gardener-controller-manager.yaml ).

```
gardenerControllerManager:
  componentConfiguration:
    config:
      apiVersion: controllermanager.config.gardener.cloud/v1alpha1
      kind: ControllerManagerConfiguration
      ... 
      please see example/20-componentconfig-gardener-controller-manager.yaml for what
	  can be configured here.
	  ...
```

### Component configuration for the Gardener Admission Controller

The component configuration of the Gardener Admission Controller is optional, as default values will be provided.
To overwrite the default values, please see the [example configuration](../../../example/20-componentconfig-gardener-admission-controller.yaml).

```
gardenerAdmissionController:
  componentConfiguration:
    config:
      apiVersion: admissioncontroller.config.gardener.cloud/v1alpha1
      kind: AdmissionControllerConfiguration
      ... 
      please see example/20-componentconfig-gardener-admission-controller.yaml for what
	  can be configured here.
      ...
```

### Component configuration for the Gardener Scheduler

Specifying a configuration for the Gardener scheduler is optional, as default values will be provided.
To overwrite the default configuration, please see the [example configuration](../../../example/20-componentconfig-gardener-scheduler.yaml).

```
gardenerScheduler:
  componentConfiguration:
    config:
      apiVersion: scheduler.config.gardener.cloud/v1alpha1
      kind: SchedulerConfiguration
      ... 
      please see example/20-componentconfig-gardener-scheduler.yaml for what
	  can be configured here.
      ...
```

# Minimal example configuration

Below is a minimal example configuration to deploy a Gardener control plane.

**What needs to be filled in is:**
- The kubeconfig of the `runtime` cluster
- DNS provider credentials (below example is for Route53).
- The URL to the etcd cluster running in the `runtime` cluster

**Optionally provide:**
- The kubeconfig of the `virtual-garden` cluster when using the `virtual-garden` deployment model.
- The CA bundle of the etcd cluster.
- If the etcd cluster has client authentication enabled: the client credentials which are signed by the etcd CA.

```
apiVersion: controlplane.gardener.landscaper.gardener.cloud/v1alpha1
kind: Imports
runtimeCluster:
  apiVersion: landscaper.gardener.cloud/v1alpha1
  kind: Target
  spec:
    type: landscaper.gardener.cloud/kubernetes-cluster
    config:
      kubeconfig: |
        ---
        apiVersion:...
        # here goes the kubeconfig of the runtime cluster


virtualGarden:
  enabled: false
  # specify when virtual garden option is enabled
  #kubeconfig:
  #  apiVersion: landscaper.gardener.cloud/v1alpha1
  #  kind: Target
  #  spec:
  #    type: landscaper.gardener.cloud/kubernetes-cluster
  #    config:
  #      kubeconfig: |
  #        ---
  #        apiVersion:...
  #        # here goes the kubeconfig of the virtual-garden cluster

internalDomain:
  provider: aws-route53
  domain: internal.test.domain
  credentials:
    AWS_ACCESS_KEY_ID: <very-secret>
    AWS_SECRET_ACCESS_KEY: <very-secret>


gardenerAPIserver:
  componentConfiguration:
    etcd:
      url: "virtual-garden-etcd-main-client.garden.svc:2379"
      # recommended to set CA of Etcd cluster
#      caBundle: |
#        -----BEGIN CERTIFICATE-----
#        ...
#        -----END CERTIFICATE-----
      # This configuration is required if the etcd has client authentication enabled
      # client credentials have been signed by the etcd CA
#      tls:
#        crt: |
#          -----BEGIN CERTIFICATE-----
#          ...
#          -----END CERTIFICATE-----
#        key: |
#          -----BEGIN RSA PRIVATE KEY-----
#          ...
#          -----END RSA PRIVATE KEY-----

gardenerAdmissionController:
  enabled: true