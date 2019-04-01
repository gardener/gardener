# Extensibility overview

Currently, everything is developed in-tree, i.e., all cloud providers and the configuration for all the supported operating systems are released together with the Gardener core itself.
The project has grown to a size where it gets more and more difficult to add new providers and to maintain the existing code base.
As a consequence, in order to become more agile and flexible again, we proposed [GEP-1](../proposals/01-extensibility.md) (Gardener Enhancement Proposal).
The document describes a future architecture and envisions to keep the Gardener core logic independent of provider-specific knowledge (similar to what Kubernetes is trying to achieve with their [out-of-tree cloud providers](https://github.com/kubernetes/enhancements/issues/88) or their [CSI volume plugins](https://github.com/kubernetes/community/pull/1258)).

## Basic concepts

Gardener keeps running in the "garden cluster" and implements the core logic of shoot cluster reconciliation/deletion.
Extensions are Kubernetes controllers themselves (like Gardener) and run in the seed clusters.
As usual, we try to use Kubernetes wherever applicable.
We rely on Kubernetes extension concepts in order to enable extensibility for Gardener.
The main ideas of GEP-1 are the following:

1. During the shoot reconciliation process Gardener will write CRDs into the seed cluster that are watched and managed by the extension controllers. They will reconcile (based on the `.spec`) and report whether everything went well or errors occurred in the CRD's `.status` field.

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
    apiVersion: aws.extensions.gardener.cloud/v1alpha1
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
    apiVersion: aws.extensions.gardener.cloud/v1alpha1
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

Gardener waits until the `.status.lastOperation`/`.status.lastError` indicates that the operation reached a final state and either continuous with the next step or stops and reports the potential error.
The extension-specific output in `.status.providerStatus` is - similar to `.spec.providerConfig` - not evaluated and simply forwarded to CRDs in subsequent steps.

**Example 2**:

Gardener deploys the control plane components into the seed cluster, e.g. the `kube-controller-manager` deployment with the following flags:

```yaml
apiVersion: apps/v1
kind: Deployment
...
spec:
  template:
    spec:
      containers:
      - command:
        - /hyperkube
        - controller-manager
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

If you are interested in writing an extension, or generally in digging deeper to find out the nitty-gritty details of the extension concepts please work through [GEP-1](../proposals/01-extensibility.md).
We are looking forward to your feedback if you have any!

## Current status

We have started implementing GEP-1 and are in the process of getting experience with the first extensions.
Based on the resources discussed in GEP-1 the following table provides information about which parts of the domain-specific knowledge that is/was in-tree has been moved out already:

| Status             | Resource                | Providers                                                                            |
|--------------------|-------------------------|--------------------------------------------------------------------------------------|
| :white_check_mark: | `DNS`                   |  alicloud-dns<br>aws-route53<br>azure-dns<br>google-clouddns<br> openstack-designate |
| :x:                | `BackupInfrastructure`  | aws<br>azure<br>gcp<br>openstack                                                     |
| :x:                | `Generic`               | cert-service                                                                         |
| :x:                | `Infrastructure`        | alicloud<br>aws<br>azure<br>gcp<br>openstack<br>local (discontinued/replaced)        |
| :white_check_mark: | `OperatingSystemConfig` | coreos,coreos-alicloud                                                               |
| :x:                | `Worker`                | alicloud<br>aws<br>azure<br>gcp<br>openstack                                         |
