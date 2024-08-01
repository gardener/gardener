---
title: 03 New Core Gardener Cloud APIs
---

# New `core.gardener.cloud/v1beta1` APIs Required to Extract Cloud-Specific/OS-Specific Knowledge Out of Gardener Core

## Table of Contents

- [New `core.gardener.cloud/v1beta1` APIs Required to Extract Cloud-Specific/OS-Specific Knowledge Out of Gardener Core](#new-coregardenercloudv1beta1-apis-required-to-extract-cloud-specificos-specific-knowledge-out-of-gardener-core)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [`CloudProfile` Resource](#cloudprofile-resource)
    - [`Seed` Resource](#seed-resource)
    - [`Project` Resource](#project-resource)
    - [`SecretBinding` resource](#secretbinding-resource)
    - [`Quota` Resource](#quota-resource)
    - [`BackupBucket` Resource](#backupbucket-resource)
    - [`BackupEntry` Resource](#backupentry-resource)
    - [`Shoot` Resource](#shoot-resource)
    - [`Plant` resource](#plant-resource)

## Summary

In [GEP-1](./01-extensibility.md) we have proposed how to (re-)design Gardener to allow providers maintaining their provider-specific knowledge out of the core tree.
Meanwhile, we have progressed a lot and are about to remove the [`CloudBotanist` interface](https://github.com/gardener/gardener/blob/de75a5bfcbedd16ba341ace0eb58be2a87049dcb/pkg/operation/cloudbotanist/types.go) entirely.
The only missing aspect that will allow providers to really maintain their code out of the core is to design new APIs.

This proposal describes how the new `Shoot`, `Seed`, etc., APIs will be re-designed to cope with the changes made with extensibility.
We already have the new `core.gardener.cloud/v1beta1` API group that will be the new default soon.

## Motivation

We want to allow providers to individually maintain their specific knowledge without the necessity to touch the Gardener core code.
In order to achieve the same, we have to provide proper APIs.

### Goals

* Provide proper APIs to allow providers maintaining their code outside of the core codebase.
* Do not complicate the APIs for end-users such that they can easily create, delete, and maintain shoot clusters.

### Non-Goals

* Let's try to not split everything up into too many different resources. Instead, let's try to keep all relevant information in the same resources when possible/appropriate.

## Proposal

In GEP-1 we already have proposed a first version for new `CloudProfile` and `Shoot` resources.
In order to deprecate the existing/old `garden.sapcloud.io/v1beta1` API group (and remove it, eventually), we should move all existing resources to the new `core.gardener.cloud/v1beta1` API group.

### `CloudProfile` Resource

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: cloudprofile1
spec:
  type: <some-provider-name> # {aws,azure,gcp,...}
# Optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.
# An empty list means that all seeds of the same provider type are supported.
# This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.
# seedSelector:
#   matchLabels:
#     foo: bar
  kubernetes:
    versions:
    - version: 1.12.1
    - version: 1.11.0
    - version: 1.10.6
    - version: 1.10.5
      expirationDate: 2020-04-05T01:02:03Z # optional
  machineImages:
  - name: coreos
    versions:
    - version: 2023.5.0
    - version: 1967.5.0
      expirationDate: 2020-04-05T08:00:00Z
  - name: ubuntu
    versions:
    - version: 18.04.201906170
  machineTypes:
  - name: m5.large
    cpu: "2"
    gpu: "0"
    memory: 8Gi
  # storage: 20Gi # optional (not needed in every environment, may only be specified if no volumeTypes have been specified)
    usable: true
  volumeTypes: # optional (not needed in every environment, may only be specified if no machineType has a `storage` field)
  - name: gp2
    class: standard
  - name: io1
    class: premium
  regions:
  - name: europe-central-1
    zones: # optional (not needed in every environment)
    - name: europe-central-1a
    - name: europe-central-1b
    - name: europe-central-1c
    # unavailableMachineTypes: # optional, list of machine types defined above that are not available in this zone
    # - m5.large
    # unavailableVolumeTypes: # optional, list of volume types defined above that are not available in this zone
    # - io1
# CA bundle that will be installed onto every shoot machine that is using this provider profile.
# caBundle: |
#   -----BEGIN CERTIFICATE-----
#   ...
#   -----END CERTIFICATE-----
  providerConfig:
    <some-provider-specific-cloudprofile-config>
    # We don't have concrete examples for every existing provider yet, but these are the proposals:
    #
    # Example for Alicloud:
    #
    # apiVersion: alicloud.provider.extensions.gardener.cloud/v1alpha1
    # kind: CloudProfileConfig
    # machineImages:
    # - name: coreos
    #   version: 2023.5.0
    #   id: coreos_2023_4_0_64_30G_alibase_20190319.vhd
    #
    #
    # Example for AWS:
    #
    # apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
    # kind: CloudProfileConfig
    # machineImages:
    # - name: coreos
    #   version: 1967.5.0
    #   regions:
    #   - name: europe-central-1
    #     ami: ami-0f46c2ed46d8157aa
    #
    #
    # Example for Azure:
    #
    # apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
    # kind: CloudProfileConfig
    # machineImages:
    # - name: coreos
    #   version: 1967.5.0
    #   publisher: CoreOS
    #   offer: CoreOS
    #   sku: Stable
    # countFaultDomains:
    # - region: westeurope
    #   count: 2
    # countUpdateDomains:
    # - region: westeurope
    #   count: 5
    #
    #
    # Example for GCP:
    #
    # apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
    # kind: CloudProfileConfig
    # machineImages:
    # - name: coreos
    #   version: 2023.5.0
    #   image: projects/coreos-cloud/global/images/coreos-stable-2023-5-0-v20190312
    #
    #
    # Example for OpenStack:
    #
    # apiVersion: openstack.provider.extensions.gardener.cloud/v1alpha1
    # kind: CloudProfileConfig
    # machineImages:
    # - name: coreos
    #   version: 2023.5.0
    #   image: coreos-2023.5.0
    # keyStoneURL: https://url-to-keystone/v3/
    # dnsServers:
    # - 10.10.10.10
    # - 10.10.10.11
    # dhcpDomain: foo.bar
    # requestTimeout: 30s
    # constraints:
    #   loadBalancerProviders:
    #   - name: haproxy
    #   floatingPools:
    #   - name: fip1
    #     loadBalancerClasses:
    #     - name: class1
    #       floatingSubnetID: 04eed401-f85f-4610-8041-c4835c4beea6
    #       floatingNetworkID: 23949a30-1cdd-4732-ba47-d03ced950acc
    #       subnetID: ac46c204-9d0d-4a4c-a90d-afefe40cfc35
    #
    #
    # Example for Packet:
    #
    # apiVersion: packet.provider.extensions.gardener.cloud/v1alpha1
    # kind: CloudProfileConfig
    # machineImages:
    # - name: coreos
    #   version: 2079.3.0
    #   id: d61c3912-8422-4daf-835e-854efa0062e4
```

### `Seed` Resource

Special note: The proposal contains fields that are not yet existing in the current `garden.sapcloud.io/v1beta1.Seed` resource, but they should be implemented (open issues that require them are linked).

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: seed-secret
  namespace: garden
type: Opaque
data:
  kubeconfig: base64(kubeconfig-for-seed-cluster)

---
apiVersion: v1
kind: Secret
metadata:
  name: backup-secret
  namespace: garden
type: Opaque
data:
  # <some-provider-specific data keys>
  # https://github.com/gardener/gardener-extension-provider-alicloud/blob/master/example/30-backupbucket.yaml#L9-L11
  # https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-infrastructure.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-azure/blob/master/example/30-backupbucket.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-backupbucket.yaml#L9
  # https://github.com/gardener/gardener-extension-provider-openstack/blob/master/example/30-backupbucket.yaml#L9-L13

---
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
metadata:
  name: seed1
spec:
  provider:
    type: <some-provider-name> # {aws,azure,gcp,...}
    region: europe-central-1
  secretRef:
    name: seed-secret
    namespace: garden
  # Motivation for DNS section: https://github.com/gardener/gardener/issues/201.
  dns:
    provider: <some-provider-name> # {aws-route53, google-clouddns, ...}
    secretName: my-dns-secret # must be in `garden` namespace
    ingressDomain: seed1.dev.example.com
  volume: # optional (introduced to get rid of `persistentvolume.garden.sapcloud.io/minimumSize` and `persistentvolume.garden.sapcloud.io/provider` annotations)
    minimumSize: 20Gi
    providers:
    - name: foo
      purpose: etcd-main
  networks: # Seed and Shoot networks must be disjunct
    nodes: 10.240.0.0/16
    pods: 10.241.128.0/17
    services: 10.241.0.0/17
  # Shoot default networks, see also https://github.com/gardener/gardener/issues/895.
  # shootDefaults:
  #   pods: 100.96.0.0/11
  #   services: 100.64.0.0/13
  taints:
  - key: seed.gardener.cloud/protected
  - key: seed.gardener.cloud/invisible
  blockCIDRs:
  - 169.254.169.254/32
  backup: # See https://github.com/gardener/gardener/blob/master/docs/proposals/02-backupinfra.md
    type: <some-provider-name> # {aws,azure,gcp,...}
  # region: eu-west-1
    secretRef:
      name: backup-secret
      namespace: garden
status:
  conditions:
  - lastTransitionTime: "2020-07-14T19:16:42Z"
    lastUpdateTime: "2020-07-14T19:18:17Z"
    message: all checks passed
    reason: Passed
    status: "True"
    type: Available
  gardener:
    id: 4c9832b3823ee6784064877d3eb10c189fc26e98a1286c0d8a5bc82169ed702c
    name: gardener-controller-manager-7fhn9ikan73n-7jhka
    version: 1.0.0
  observedGeneration: 1
```

### `Project` Resource

Special note: The `members` and `viewers` field of the `garden.sapcloud.io/v1beta1.Project` resource will be merged together into one `members` field.
Every member will have a role that is either `admin` or `viewer`.
This will allow us to add new roles without changing the API.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Project
metadata:
  name: example
spec:
  description: Example project
  members:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: john.doe@example.com
    role: admin
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: joe.doe@example.com
    role: viewer
  namespace: garden-example
  owner:
    apiGroup: rbac.authorization.k8s.io
    kind: User
    name: john.doe@example.com
  purpose: Example project
status:
  observedGeneration: 1
  phase: Ready
```

### `SecretBinding` resource

Special note: No modifications needed compared to the current `garden.sapcloud.io/v1beta1.SecretBinding` resource.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secret1
  namespace: garden-core
type: Opaque
data:
  # <some-provider-specific data keys>
  # https://github.com/gardener/gardener-extension-provider-alicloud/blob/master/example/30-infrastructure.yaml#L14-L15
  # https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-infrastructure.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-azure/blob/master/example/30-infrastructure.yaml#L14-L17
  # https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-infrastructure.yaml#L14
  # https://github.com/gardener/gardener-extension-provider-openstack/blob/master/example/30-infrastructure.yaml#L15-L18
  # https://github.com/gardener/gardener-extension-provider-packet/blob/master/example/30-infrastructure.yaml#L14-L15
  #
  # If you use your own domain (not the default domain of your landscape) then you have to add additional keys to this secret.
  # The reason is that the DNS management is not part of the Gardener core code base but externalized, hence, it might use other
  # key names than Gardener itself.
  # The actual values here depend on the DNS extension that is installed to your landscape.
  # For example, check out https://github.com/gardener/external-dns-management and find a lot of example secret manifests here:
  # https://github.com/gardener/external-dns-management/tree/master/examples

---
apiVersion: core.gardener.cloud/v1beta1
kind: SecretBinding
metadata:
  name: secretbinding1
  namespace: garden-core
secretRef:
  name: secret1
# namespace: namespace-other-than-'garden-core' // optional
quotas: []
# - name: quota-1
# # namespace: namespace-other-than-'garden-core' // optional
```

### `Quota` Resource

Special note: No modifications needed compared to the current `garden.sapcloud.io/v1beta1.Quota` resource.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Quota
metadata:
  name: trial-quota
  namespace: garden-trial
spec:
  scope:
    apiGroup: core.gardener.cloud
    kind: Project
# clusterLifetimeDays: 14
  metrics:
    cpu: "200"
    gpu: "20"
    memory: 4000Gi
    storage.standard: 8000Gi
    storage.premium: 2000Gi
    loadbalancer: "100"
```

### `BackupBucket` Resource

Special note: This new resource is cluster-scoped.

```yaml
# See also: https://github.com/gardener/gardener/blob/master/docs/proposals/02-backupinfra.md.

apiVersion: v1
kind: Secret
metadata:
  name: backup-operator-provider
  namespace: backup-garden
type: Opaque
data:
  # <some-provider-specific data keys>
  # https://github.com/gardener/gardener-extension-provider-alicloud/blob/master/example/30-backupbucket.yaml#L9-L11
  # https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-backupbucket.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-azure/blob/master/example/30-backupbucket.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-backupbucket.yaml#L9
  # https://github.com/gardener/gardener-extension-provider-openstack/blob/master/example/30-backupbucket.yaml#L9-L13

---
apiVersion: core.gardener.cloud/v1beta1
kind: BackupBucket
metadata:
  name: <seed-provider-type>-<region>-<seed-uid>
  ownerReferences:
  - kind: Seed
    name: seed1
spec:
  provider:
    type: <some-provider-name> # {aws,azure,gcp,...}
    region: europe-central-1
  seed: seed1
  secretRef:
    name: backup-operator-provider
    namespace: backup-garden
status:
  lastOperation:
    description: Backup bucket has been successfully reconciled.
    lastUpdateTime: '2020-04-13T14:34:27Z'
    progress: 100
    state: Succeeded
    type: Reconcile
  observedGeneration: 1
```

### `BackupEntry` Resource

Special note: This new resource is cluster-scoped.

```yaml
# See also: https://github.com/gardener/gardener/blob/master/docs/proposals/02-backupinfra.md.

apiVersion: v1
kind: Secret
metadata:
  name: backup-operator-provider
  namespace: backup-garden
type: Opaque
data:
  # <some-provider-specific data keys>
  # https://github.com/gardener/gardener-extension-provider-alicloud/blob/master/example/30-backupbucket.yaml#L9-L11
  # https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-backupbucket.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-azure/blob/master/example/30-backupbucket.yaml#L9-L10
  # https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-backupbucket.yaml#L9
  # https://github.com/gardener/gardener-extension-provider-openstack/blob/master/example/30-backupbucket.yaml#L9-L13

---
apiVersion: core.gardener.cloud/v1beta1
kind: BackupEntry
metadata:
  name: shoot--core--crazy-botany--3ef42
  namespace: garden-core
  ownerReferences:
  - apiVersion: core.gardener.cloud/v1beta1
    blockOwnerDeletion: false
    controller: true
    kind: Shoot
    name: crazy-botany
    uid: 19a9538b-5058-11e9-b5a6-5e696cab3bc8
spec:
  bucketName: cloudprofile1-random[:5]
  seed: seed1
status:
  lastOperation:
    description: Backup entry has been successfully reconciled.
    lastUpdateTime: '2020-04-13T14:34:27Z'
    progress: 100
    state: Succeeded
    type: Reconcile
  observedGeneration: 1
```

### `Shoot` Resource

Special notes:

* `kubelet` configuration in the worker pools may override the default `.spec.kubernetes.kubelet` configuration (that applies for all worker pools if not overridden).
* Moved remaining control plane configuration to new `.spec.provider.controlplane` section.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: crazy-botany
  namespace: garden-core
spec:
  secretBindingName: secretbinding1
  cloudProfileName: cloudprofile1
  region: europe-central-1
# seedName: seed1
  provider:
    type: <some-provider-name> # {aws,azure,gcp,...}
    infrastructureConfig:
      <some-provider-specific-infrastructure-config>
      # https://github.com/gardener/gardener-extension-provider-alicloud/blob/master/example/30-infrastructure.yaml#L56-L64
      # https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-infrastructure.yaml#L43-L53
      # https://github.com/gardener/gardener-extension-provider-azure/blob/master/example/30-infrastructure.yaml#L63-L71
      # https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-infrastructure.yaml#L53-L57
      # https://github.com/gardener/gardener-extension-provider-openstack/blob/master/example/30-infrastructure.yaml#L56-L64
      # https://github.com/gardener/gardener-extension-provider-packet/blob/master/example/30-infrastructure.yaml#L48-L49
    controlPlaneConfig:
      <some-provider-specific-controlplane-config>
      # https://github.com/gardener/gardener-extension-provider-alicloud/blob/master/example/30-controlplane.yaml#L60-L65
      # https://github.com/gardener/gardener-extension-provider-aws/blob/master/example/30-controlplane.yaml#L60-L64
      # https://github.com/gardener/gardener-extension-provider-azure/blob/master/example/30-controlplane.yaml#L61-L66
      # https://github.com/gardener/gardener-extension-provider-gcp/blob/master/example/30-controlplane.yaml#L59-L64
      # https://github.com/gardener/gardener-extension-provider-openstack/blob/master/example/30-controlplane.yaml#L64-L70
      # https://github.com/gardener/gardener-extension-provider-packet/blob/master/example/30-controlplane.yaml#L60-L61
    workers:
    - name: cpu-worker
      minimum: 3
      maximum: 5
    # maxSurge: 1
    # maxUnavailable: 0
      machine:
        type: m5.large
        image:
          name: <some-os-name>
          version: <some-os-version>
        # providerConfig:
        #   <some-os-specific-configuration>
      volume:
        type: gp2
        size: 20Gi
    # providerConfig:
    #   <some-provider-specific-worker-config>
    # labels:
    #   key: value
    # annotations:
    #   key: value
    # taints: # See also https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
    # - key: foo
    #   value: bar
    #   effect: NoSchedule
    # caBundle: <some-ca-bundle-to-be-installed-to-all-nodes-in-this-pool>
    # kubernetes:
    #   kubelet:
    #     cpuCFSQuota: true
    #     cpuManagerPolicy: none
    #     podPidsLimit: 10
    #     featureGates:
    #       SomeKubernetesFeature: true
    # zones: # optional, only relevant if the provider supports availability zones
    # - europe-central-1a
    # - europe-central-1b
  kubernetes:
    version: 1.15.1
  # allowPrivilegedContainers: true # 'true' means that all authenticated users can use the "gardener.privileged" PodSecurityPolicy, allowing full unrestricted access to Pod features.
  # kubeAPIServer:
  #   featureGates:
  #     SomeKubernetesFeature: true
  #   runtimeConfig:
  #     scheduling.k8s.io/v1alpha1: true
  #   oidcConfig:
  #     caBundle: |
  #       -----BEGIN CERTIFICATE-----
  #       Li4u
  #       -----END CERTIFICATE-----
  #     clientID: client-id
  #     groupsClaim: groups-claim
  #     groupsPrefix: groups-prefix
  #     issuerURL: https://identity.example.com
  #     usernameClaim: username-claim
  #     usernamePrefix: username-prefix
  #     signingAlgs: RS256,some-other-algorithm
  #-#-# only usable with Kubernetes >= 1.11
  #     requiredClaims:
  #       key: value
  #-# only usable with Kubernetes >= 1.30
  #   authentication:
  #     structured:
  #       configMapName: name-of-configmap-containing-authenticaion-config
  #   admissionPlugins:
  #   - name: PodNodeSelector
  #     config: |
  #       podNodeSelectorPluginConfig:
  #         clusterDefaultNodeSelector: <node-selectors-labels>
  #         namespace1: <node-selectors-labels>
  #         namespace2: <node-selectors-labels>
  #   auditConfig:
  #     auditPolicy:
  #       configMapRef:
  #         name: auditpolicy
  # kubeControllerManager:
  #   featureGates:
  #     SomeKubernetesFeature: true
  #   horizontalPodAutoscaler:
  #     syncPeriod: 30s
  #     tolerance: 0.1
  #-#-# only usable with Kubernetes < 1.12
  #     downscaleDelay: 15m0s
  #     upscaleDelay: 1m0s
  #-#-# only usable with Kubernetes >= 1.12
  #     downscaleStabilization: 5m0s
  #     initialReadinessDelay: 30s
  #     cpuInitializationPeriod: 5m0s
  # kubeScheduler:
  #   featureGates:
  #     SomeKubernetesFeature: true
  # kubeProxy:
  #   featureGates:
  #     SomeKubernetesFeature: true
  #   mode: IPVS
  # kubelet:
  #   cpuCFSQuota: true
  #   cpuManagerPolicy: none
  #   podPidsLimit: 10
  #   featureGates:
  #     SomeKubernetesFeature: true
  # clusterAutoscaler:
  #   scaleDownUtilizationThreshold: 0.5
  #   scaleDownUnneededTime: 30m
  #   scaleDownDelayAfterAdd: 60m
  #   scaleDownDelayAfterFailure: 10m
  #   scaleDownDelayAfterDelete: 10s
  #   scanInterval: 10s
  dns:
    # When the shoot shall use a cluster domain no domain and no providers need to be provided - Gardener will
    # automatically compute a correct domain.
    domain: crazy-botany.core.my-custom-domain.com
    providers:
    - type: aws-route53
      secretName: my-custom-domain-secret
      domains:
        include:
        - my-custom-domain.com
        - my-other-custom-domain.com
        exclude:
        - yet-another-custom-domain.com
      zones:
        include:
        - zone-id-1
        exclude:
        - zone-id-2
  extensions:
  - type: foobar
  # providerConfig:
  #   apiVersion: foobar.extensions.gardener.cloud/v1alpha1
  #   kind: FooBarConfiguration
  #   foo: bar
  networking:
    type: calico
    pods: 100.96.0.0/11
    services: 100.64.0.0/13
    nodes: 10.250.0.0/16
  # providerConfig:
  #   apiVersion: calico.extensions.gardener.cloud/v1alpha1
  #   kind: NetworkConfig
  #   ipam:
  #     type: host-local
  #     cidr: usePodCIDR
  #   backend: bird
  #   typha:
  #     enabled: true
  # See also: https://github.com/gardener/gardener/blob/master/docs/proposals/03-networking.md
  maintenance:
    timeWindow:
      begin: 220000+0100
      end: 230000+0100
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
# hibernation:
#   enabled: false
#   schedules:
#   - start: "0 20 * * *" # Start hibernation every day at 8PM
#     end: "0 6 * * *"    # Stop hibernation every day at 6AM
#     location: "America/Los_Angeles" # Specify a location for the cron to run in
  addons:
    nginx-ingress:
      enabled: false
    # loadBalancerSourceRanges: []
    kubernetes-dashboard:
      enabled: true
    # authenticationMode: basic # allowed values: basic,token
status:
  conditions:
  - type: APIServerAvailable
    status: 'True'
    lastTransitionTime: '2020-01-30T10:38:15Z'
    lastUpdateTime: '2020-04-13T14:35:21Z'
    reason: HealthzRequestFailed
    message: API server /healthz endpoint responded with success status code. [response_time:3ms]
  - type: ControlPlaneHealthy
    status: 'True'
    lastTransitionTime: '2020-04-02T05:18:58Z'
    lastUpdateTime: '2020-04-13T14:35:21Z'
    reason: ControlPlaneRunning
    message: All control plane components are healthy.
  - type: EveryNodeReady
    status: 'True'
    lastTransitionTime: '2020-04-01T16:27:21Z'
    lastUpdateTime: '2020-04-13T14:35:21Z'
    reason: EveryNodeReady
    message: Every node registered to the cluster is ready.
  - type: SystemComponentsHealthy
    status: 'True'
    lastTransitionTime: '2020-04-03T18:26:28Z'
    lastUpdateTime: '2020-04-13T14:35:21Z'
    reason: SystemComponentsRunning
    message: All system components are healthy.
  gardener:
    id: 4c9832b3823ee6784064877d3eb10c189fc26e98a1286c0d8a5bc82169ed702c
    name: gardener-controller-manager-7fhn9ikan73n-7jhka
    version: 1.0.0
  lastOperation:
    description: Shoot cluster state has been successfully reconciled.
    lastUpdateTime: '2020-04-13T14:34:27Z'
    progress: 100
    state: Succeeded
    type: Reconcile
  observedGeneration: 1
  seed: seed1
  hibernated: false
  technicalID: shoot--core--crazy-botany
  uid: d8608cfa-2856-11e8-8fdc-0a580af181af
```

### `Plant` resource

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: crazy-plant-secret
  namespace: garden-core
type: Opaque
data:
  kubeconfig: base64(kubeconfig-for-plant-cluster)

---
apiVersion: core.gardener.cloud/v1beta1
kind: Plant
metadata:
  name: crazy-plant
  namespace: garden-core
spec:
  secretRef:
    name: crazy-plant-secret
  endpoints:
  - name: Cluster GitHub repository
    purpose: management
    url: https://github.com/my-org/my-cluster-repo
  - name: GKE cluster page
    purpose: management
    url: https://console.cloud.google.com/kubernetes/clusters/details/europe-west1-b/plant?project=my-project&authuser=1&tab=details
status:
  clusterInfo:
    provider:
      type: gce
      region: europe-west4-c
    kubernetes:
      version: v1.11.10-gke.5
  conditions:
  - lastTransitionTime: "2020-03-01T11:31:37Z"
    lastUpdateTime: "2020-04-14T18:00:29Z"
    message: API server /healthz endpoint responded with success status code. [response_time:8ms]
    reason: HealthzRequestFailed
    status: "True"
    type: APIServerAvailable
  - lastTransitionTime: "2020-04-01T06:26:56Z"
    lastUpdateTime: "2020-04-14T18:00:29Z"
    message: Every node registered to the cluster is ready.
    reason: EveryNodeReady
    status: "True"
    type: EveryNodeReady
```
