# Extensibility Overview

Initially, everything was developed in-tree in the Gardener project. All cloud providers and the configuration for all the supported operating systems were released together with the Gardener core itself.
But as the project grew, it got more and more difficult to add new providers and maintain the existing code base.
As a consequence and in order to become agile and flexible again, we proposed [GEP-1](../proposals/01-extensibility.md) (Gardener Enhancement Proposal) and later [gardener/gardener#9635](https://github.com/gardener/gardener/issues/9635) as an enhancement.
The document describes an out-of-tree extension architecture that keeps the Gardener core logic independent of provider-specific knowledge (similar to what Kubernetes has achieved with [out-of-tree cloud providers](https://github.com/kubernetes/enhancements/issues/88) or with [CSI volume plugins](https://github.com/kubernetes/community/pull/1258)).

## Basic Concepts

Gardener components run in the garden and seed clusters, implementing the core logic for garden, seed, and shoot cluster reconciliation and deletion.
Extensions are Kubernetes controllers themselves (like Gardener) and run in the garden runtime and seed clusters.
As usual, we try to use Kubernetes wherever applicable.
We rely on Kubernetes extension concepts in order to enable extensibility for Gardener.

### Building Blocks

Extensions consist of the following building blocks:

1. A Helm chart as the vehicle to generally deploy extension controllers to a Kubernetes clusters
1. Extension controllers that reconcile objects of the API group `extensions.gardener.cloud`. These controllers take over outsourced tasks, like creating the shoot infrastructure or deploying components to the control-plane. Optionally, extensions can bring their own webhooks to mutate resources deployed by Gardener.
1. Optionally, a Helm chart with an admission component inside. The admission controller runs in the garden runtime cluster and validates extension specific settings of the `Shoot` (given in `providerConfig` fields). See [admission](./admission.md) for more details.

### Registration

Before an extension can be used, it needs to be made known to the system. The [gardener-operator](../concepts/operator.md) automates much of the registration process, making `Extension` resources (group `operator.gardener.cloud`) the preferred method for registering extensions. For more information, see the [Registration documentation](./registration.md).
Practically, many extensions provide basic example manifests to start with the registration in their `example` directory ([example1](https://github.com/gardener/gardener-extension-provider-aws/tree/master/example), [example2](https://github.com/gardener/gardener-extension-shoot-cert-service/tree/master/example)).

### Kinds and Types

Extensions are defined by their Kinds (defined by Gardener - see [resources](../resources)) and Types.

For example, the following is an extension resource of Kind `Infrastructure` and Type `local`, which means we need a Gardener extension `local` that reconciles `Infrastructure` resources.

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Infrastructure
metadata:
  name: infrastructure
  namespace: shoot--core--aws-01
spec:
  type: local
```

### Classes

A Gardener landscape consists of various cluster types which extension controllers may consider during reconciliation.
The `.spec.class` field identifies the different deployment cases.

#### Garden

Extension controllers serve the garden (run in garden runtime), e.g., installing certificates for API and ingress endpoints.
In the course of the `Garden` reconciliation, the `gardener-operator` creates `BackupBucket`, `DNSRecord` and `Extension` resources (group `extensions.gardener.cloud`) which triggers the responsible extension controllers to reconcile them.

#### Seed

Extension controllers serve the seed (run in seed), e.g., requesting a wildcard certificate for the seed's ingress domain.
In the course of the `Seed` reconciliation, the `gardenlet` creates `DNSRecord` and `Extension` resources (group `extensions.gardener.cloud`) which triggers the responsible extension controllers to reconcile them.

#### Shoot

Extension controllers serve the shoot (run in seed), e.g., deploying a certificate controller into the control-plane namespace.
In the course of the `Shoot` reconciliation, the `gardenlet` creates various extension resources (group `extensions.gardener.cloud`) which triggers the responsible extension controllers to reconcile them.

### `gardenlet` Reconciliation Walkthrough

Resources of group `extensions.gardener.cloud` are always created by Gardener itself, either in the garden runtime or in the seed cluster.
To get a better understanding of how the concept works, we will walk through the reconciliation process of a `Shoot` resource in the seed cluster.

1. During the shoot reconciliation process, Gardener will write CRDs into the seed cluster that are watched and managed by the extension controllers. They will reconcile (based on the `.spec`) and report whether everything went well or errors occurred in the CRD's `.status` field.

1. Gardener keeps deploying the provider-independent control plane components (etcd, kube-apiserver, etc.). However, some of these components might still need little customization by providers, e.g., additional configuration, flags, etc. In this case, the extension controllers register webhooks in order to manipulate the manifests.

**Example 1**:

Gardener creates a new AWS shoot cluster and requires the preparation of infrastructure in order to proceed (networks, security groups, etc.).
It writes the following CRD into the seed cluster:

```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Infrastructure
metadata:
  name: infrastructure
  namespace: shoot--core--aws-01
spec:
  type: aws
  providerConfig:
    apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
    kind: InfrastructureConfig
    networks:
      vpc:
        cidr: 10.250.0.0/16
      internal:
      - 10.250.112.0/22
      public:
      - 10.250.96.0/22
      workers:
      - 10.250.0.0/19
    zones:
    - eu-west-1a
  dns:
    apiserver: api.aws-01.core.example.com
  region: eu-west-1
  secretRef:
    name: my-aws-credentials
  sshPublicKey: |
    base64(key)
```

Please note that the `.spec.providerConfig` is a raw blob and not evaluated or known in any way by Gardener.
Instead, it was specified by the user (in the `Shoot` resource) and just "forwarded" to the extension controller.
Only the AWS controller understands this configuration and will now start provisioning/reconciling the infrastructure.
It reports in the `.status` field the result:

```yaml
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
  providerStatus:
    apiVersion: aws.provider.extensions.gardener.cloud/v1alpha1
    kind: InfrastructureStatus
    vpc:
      id: vpc-1234
      subnets:
      - id: subnet-acbd1234
        name: workers
        zone: eu-west-1
      securityGroups:
      - id: sg-xyz12345
        name: workers
    iam:
      nodesRoleARN: <some-arn>
      instanceProfileName: foo
    ec2:
      keyName: bar
```

Gardener waits until the `.status.lastOperation` / `.status.lastError` indicates that the operation reached a final state and either continuous with the next step, or stops and reports the potential error.
The extension-specific output in `.status.providerStatus` is - similar to `.spec.providerConfig` - not evaluated, and simply forwarded to CRDs in subsequent steps.

**Example 2**:

Gardener deploys the control plane components into the seed cluster, e.g., the `kube-controller-manager` deployment with the following flags:

```yaml
apiVersion: apps/v1
kind: Deployment
...
spec:
  template:
    spec:
      containers:
      - command:
        - /usr/local/bin/kube-controller-manager
        - --allocate-node-cidrs=true
        - --attach-detach-reconcile-sync-period=1m0s
        - --controllers=*,bootstrapsigner,tokencleaner
        - --cluster-cidr=100.96.0.0/11
        - --cluster-name=shoot--core--aws-01
        - --cluster-signing-cert-file=/srv/kubernetes/ca/ca.crt
        - --cluster-signing-key-file=/srv/kubernetes/ca/ca.key
        - --concurrent-deployment-syncs=10
        - --concurrent-replicaset-syncs=10
...
```

The AWS controller requires some additional flags in order to make the cluster functional.
It needs to provide a Kubernetes cloud-config and also some cloud-specific flags.
Consequently, it registers a `MutatingWebhookConfiguration` on `Deployment`s and adds these flags to the container:

```yaml
        - --cloud-provider=external
        - --external-cloud-volume-plugin=aws
        - --cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf
```

Of course, it would have needed to create a `ConfigMap` containing the cloud config and to add the proper `volume` and `volumeMounts` to the manifest as well.

(Please note for this special example: The Kubernetes community is also working on making the `kube-controller-manager` provider-independent.
However, there will most probably be still components other than the `kube-controller-manager` which need to be adapted by extensions.)

If you are interested in writing an extension, or generally in digging deeper to find out the nitty-gritty details of the extension concepts, please read [GEP-1](../proposals/01-extensibility.md).
We are truly looking forward to your feedback!

## Known Extensions

We track all extensions of Gardener in the [known Gardener Extensions List](../README.md#known-extension-implementations) repo.
