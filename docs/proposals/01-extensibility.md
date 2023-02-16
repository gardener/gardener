# Gardener Extensibility and Extraction of Cloud-Specific/OS-Specific Knowledge ([#308](https://github.com/gardener/gardener/issues/308), [#262](https://github.com/gardener/gardener/issues/262))

## Table of Contents

- [Gardener Extensibility and Extraction of Cloud-Specific/OS-Specific Knowledge (#308, #262)](#gardener-extensibility-and-extraction-of-cloud-specificos-specific-knowledge-308-262)
  - [Table of Contents](#table-of-contents)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [Modification of Existing `CloudProfile` and `Shoot` Resources](#modification-of-existing-cloudprofile-and-shoot-resources)
      - [CloudProfiles](#cloudprofiles)
      - [Shoots](#shoots)
    - [CRD Definitions and Workflow Adaptation](#crd-definitions-and-workflow-adaptation)
      - [Custom Resource Definitions](#custom-resource-definitions)
        - [DNS Records](#dns-records)
        - [Infrastructure Provisioning](#infrastructure-provisioning)
        - [Backup Infrastructure Provisioning](#backup-infrastructure-provisioning)
        - [Cloud Config (User-Data) for Bootstrapping Machines](#cloud-config-user-data-for-bootstrapping-machines)
        - [Worker Pools Definition](#worker-pools-definition)
        - [Generic Resources](#generic-resources)
      - [Shoot State](#shoot-state)
      - [Shoot Health Checks/Conditions](#shoot-health-checksconditions)
      - [Reconciliation Flow](#reconciliation-flow)
      - [Deletion Flow](#deletion-flow)
    - [gardenlet](#gardenlet)
    - [Shoot Control Plane Movement/Migration](#shoot-control-plane-movementmigration)
    - [BackupInfrastructure Migration](#backupinfrastructure-migration)
  - [Registration of External Controllers at Gardener](#registration-of-external-controllers-at-gardener)
  - [Other Cloud-Specific Parts](#other-cloud-specific-parts)
    - [Defaulting and Validation Admission Plugins](#defaulting-and-validation-admission-plugins)
    - [DNS Hosted Zone Admission Plugin](#dns-hosted-zone-admission-plugin)
    - [Shoot Quota Admission Plugin](#shoot-quota-admission-plugin)
    - [Shoot Maintenance Controller](#shoot-maintenance-controller)
  - [Alternatives](#alternatives)

## Summary

Gardener has evolved to a large compound of packages containing lots of highly specific knowledge, which makes it very hard to extend (supporting a new cloud provider, new OS, ..., or behaving differently depending on the underlying infrastructure).

This proposal aims to move out the cloud-specific implementations (called "(cloud) botanists") and the OS-specifics into dedicated controllers, and simultaneously to allow deviation from the standard Gardener deployment.

## Motivation

Currently, it is too hard to support additional cloud providers or operation systems/distributions as everything must be done in-tree, which might affect the implementation of other cloud providers as well.
The various conditions and branches make the code hard to maintain and hard to test.
Every change must be done centrally, requires to completely rebuild Gardener, and cannot be deployed individually. Similarly to the motivation for Kubernetes to extract their cloud-specifics into dedicated cloud-controller-managers or to extract the container/storage/network/... specifics into CRI/CSI/CNI/..., we aim to do the same right now.

### Goals

* Gardener does not contain any cloud-specific knowledge anymore but defines a clear contract, allowing external controllers (botanists) to support different environments (AWS, Azure, GCP, ...).
* Gardener does not contain any operation system-specific knowledge anymore but defines a clear contract, allowing external controllers to support different operation systems/distributions (CoreOS, SLES, Ubuntu, ...).
* It shall become much easier to move control planes of Shoot clusters between Seed clusters ([#232](https://github.com/gardener/gardener/issues/232)), which is a necessary requirement of an automated setup for the Gardener Ring ([#233](https://github.com/gardener/gardener/issues/233)).

### Non-Goals

* We want to also factor out the specific knowledge of the addon deployments (nginx-ingress, kubernetes-dashboard, ...), but we already have dedicated projects/issues for that: [Bouquet Gardener Addon Manager [Deprecated]](https://github.com/gardener/bouquet) and [#246](https://github.com/gardener/gardener/issues/246). We will keep the addons in-tree as part of this proposal and tackle their extraction separately.
* We do not want to make Gardener a plain workflow engine that just executes a given template (which indeed would allow to be generic, open, and extensible in their highest forms, but which would end-up in building a "programming/scripting language" inside a serialization format (YAML/JSON/...)). Rather, we want to have well-defined contracts and APIs, keeping Gardener responsible for the clusters management.

## Proposal

Gardener heavily relies on and implements Kubernetes principles, and its ultimate strategy is to use Kubernetes wherever applicable.
The extension concept in Kubernetes is based on (next to others) `CustomResourceDefinition`s, `ValidatingWebhookConfiguration`s and `MutatingWebhookConfiguration`s, and `InitializerConfiguration`s.
Consequently, Gardener's extensibility concept relies on these mechanisms.

Instead of implementing all aspects directly in Gardener it will deploy some CRDs to the Seed cluster which will be watched by dedicated controllers (also running in the Seed clusters), each one implementing one aspect of cluster management. This way, one complex strongly coupled Gardener implementation covering all infrastructures is decomposed into a set of loosely coupled controllers, implementing aspects of APIs defined by Gardener.
Gardener will just wait until the controllers report that they are done (or have faced an error) in the CRD's `.status` field instead of doing the respective tasks itself.
We will have one specific CRD for every specific operation (e.g., DNS, infrastructure provisioning, machine cloud config generation, ...).
However, there are also parts inside Gardener which can be handled generically (not by cloud botanists) because they are the same or very similar for all the environments.
One example of those is the deployment of a `Namespace` in the Seed, which will run the Shoot's control plane.
Another one is the deployment of a `Service` for the Shoot's kube-apiserver.
In case a cloud botanist needs to cooperate and react on those operations, it should register a `ValidatingWebhookConfiguration`, a `MutatingWebhookConfiguration`, or a `InitializerConfiguration`.
With this approach it can validate, modify, or react on any resource created by Gardener to make it cloud infrastructure specific.

The web hooks should be registered with `failurePolicy=Fail` to ensure that a request made by Gardener fails if the respective web hook is not available.

### Modification of Existing `CloudProfile` and `Shoot` Resources

We will introduce the new API group `gardener.cloud`:

#### CloudProfiles

```yaml
---
apiVersion: gardener.cloud/v1alpha1
kind: CloudProfile
metadata:
  name: aws
spec:
  type: aws
# caBundle: |
#   -----BEGIN CERTIFICATE-----
#   ...
#   -----END CERTIFICATE-----
  dnsProviders:
  - type: aws-route53
  - type: unmanaged
  kubernetes:
    versions:
    - 1.12.1
    - 1.11.0
    - 1.10.5
  machineTypes:
  - name: m4.large
    cpu: "2"
    gpu: "0"
    memory: 8Gi
  # storage: 20Gi   # optional (not needed in every environment, may only be specified if no volumeTypes have been specified)
  ...
  volumeTypes:      # optional (not needed in every environment, may only be specified if no machineType has a `storage` field)
  - name: gp2
    class: standard
  - name: io1
    class: premium
  providerConfig:
    apiVersion: aws.cloud.gardener.cloud/v1alpha1
    kind: CloudProfileConfig
    constraints:
      minimumVolumeSize: 20Gi
      machineImages:
      - name: coreos
        regions:
        - name: eu-west-1
          ami: ami-32d1474b
        - name: us-east-1
          ami: ami-e582d29f
      zones:
      - region: eu-west-1
        zones:
        - name: eu-west-1a
          unavailableMachineTypes: # list of machine types defined above that are not available in this zone
          - name: m4.large
          unavailableVolumeTypes:  # list of volume types defined above that are not available in this zone
          - name: gp2
        - name: eu-west-1b
        - name: eu-west-1c
```

#### Shoots

```yaml
apiVersion: gardener.cloud/v1alpha1
kind: Shoot
metadata:
  name: johndoe-aws
  namespace: garden-dev
spec:
  cloudProfileName: aws
  secretBindingName: core-aws
  cloud:
    type: aws
    region: eu-west-1
    providerConfig:
      apiVersion: aws.cloud.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        vpc: # specify either 'id' or 'cidr'
        # id: vpc-123456
          cidr: 10.250.0.0/16
        internal:
        - 10.250.112.0/22
        public:
        - 10.250.96.0/22
        workers:
        - 10.250.0.0/19
      zones:
      - eu-west-1a
    workerPools:
    - name: pool-01
    # Taints, labels, and annotations are not yet implemented. This requires interaction with the machine-controller-manager, see
    # https://github.com/gardener/machine-controller-manager/issues/174. It is only mentioned here as future proposal.
    # taints:
    # - key: foo
    #   value: bar
    #   effect: PreferNoSchedule
    # labels:
    # - key: bar
    #   value: baz
    # annotations:
    # - key: foo
    #   value: hugo
      machineType: m4.large
      volume: # optional, not needed in every environment, may only be specified if the referenced CloudProfile contains the volumeTypes field
        type: gp2
        size: 20Gi
      providerConfig:
        apiVersion: aws.cloud.gardener.cloud/v1alpha1
        kind: WorkerPoolConfig
        machineImage:
          name: coreos
          ami: ami-d0dcef3
        zones:
        - eu-west-1a
      minimum: 2
      maximum: 2
      maxSurge: 1
      maxUnavailable: 0
  kubernetes:
    version: 1.11.0
    ...
  dns:
    provider: aws-route53
    domain: johndoe-aws.garden-dev.example.com
  maintenance:
    timeWindow:
      begin: 220000+0100
      end: 230000+0100
    autoUpdate:
      kubernetesVersion: true
  backup:
    schedule: "*/5 * * * *"
    maximum: 7
  addons:
    kube2iam:
      enabled: false
    kubernetes-dashboard:
      enabled: true
    cluster-autoscaler:
      enabled: true
    nginx-ingress:
      enabled: true
      loadBalancerSourceRanges: []
    kube-lego:
      enabled: true
      email: john.doe@example.com
```

:information: The specifications for the other cloud providers Gardener already has an implementation for looks similar.

### CRD Definitions and Workflow Adaptation

In the following section, we are outlining the CRD definitions which define the API between Gardener and the dedicated controllers.
After that we will take a look at the current [reconciliation](../../pkg/gardenlet/controller/shoot/shoot/reconciler_reconcile.go)/[deletion](../../pkg/gardenlet/controller/shoot/shoot/reconciler_delete.go) flow and describe how it would look like in case we would implement this proposal.

#### Custom Resource Definitions

Every CRD has a `.spec.type` field containing the respective instance of the dimension the CRD represents, e.g. the cloud provider, the DNS provider, or the operation system name.
Moreover, the `.status` field must contain:

* `observedGeneration` (`int64`), a field indicating on which generation the controller last worked on.
* `state` (`*runtime.RawExtension`), a field which is not interpreted by Gardener but persisted; it should be treated opaque and only be used by the respective CRD-specific controller (it can store anything it needs to re-construct its own state).
* `lastError` (`object`), a field which is optional and only present if the last operation ended with an error state.
* `lastOperation` (`object`), a field which always exists and which indicates what the last operation of the controller was.
* `conditions` (`list`), a field allowing the controller to report health checks for its area of responsibility.

Some CRDs might have a `.spec.providerConfig` or a `.status.providerStatus` field containing controller-specific information that is treated opaque by Gardener and will only be copied to dependent or depending CRDs.

##### DNS Records

Every Shoot needs two DNS records (or three, depending on whether the nginx-ingress addon is enabled), one so-called "internal" record that Gardener uses in the kubeconfigs of the Shoot cluster's system components, and one so-called "external" record which is used in the kubeconfig provided to the user.

```yaml
---
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSProvider
metadata:
  name: alicloud
  namespace: default
spec:
  type: alicloud-dns
  secretRef:
    name: alicloud-credentials
  domains:
    include:
    - my.own.domain.com
---
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSEntry
metadata:
  name: dns
  namespace: default
spec:
  dnsName: dns.my.own.domain.com
  ttl: 600
  targets:
  - 8.8.8.8
status:
  observedGeneration: 4
  state: some-state
  lastError:
    lastUpdateTime: 2018-04-04T07:08:51Z
    description: some-error message
    codes:
    - ERR_UNAUTHORIZED
  lastOperation:
    lastUpdateTime: 2018-04-04T07:24:51Z
    progress: 70
    type: Reconcile
    state: Processing
    description: Currently provisioning ...
  conditions:
  - lastTransitionTime: 2018-07-11T10:18:25Z
    message: DNS record has been created and is available.
    reason: RecordResolvable
    status: "True"
    type: Available
    propagate: false
  providerStatus:
    apiVersion: aws.extensions.gardener.cloud/v1alpha1
    kind: DNSStatus
    ...
```

##### Infrastructure Provisioning

The `Infrastructure` CRD contains the information about VPC, networks, security groups, availability zones, ..., basically, everything that needs to be prepared before an actual VMs/load balancers/... can be provisioned.

```yaml
---
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

##### Backup Infrastructure Provisioning

The `BackupInfrastructure` CRD in the Seeds tells the cloud-specific controller to prepare a blob store bucket/container which can later be used to store etcd backups.

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: BackupInfrastructure
metadata:
  name: etcd-backup
  namespace: shoot--core--aws-01
spec:
  type: aws
  region: eu-west-1
  storageContainerName: asdasjndasd-1293912378a-2213
  secretRef:
    name: my-aws-credentials
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
```

##### Cloud Config (User-Data) for Bootstrapping Machines

Gardener will continue to keep knowledge about the content of the cloud config scripts, but it will hand over it to the respective OS-specific controller which will generate the specific valid representation.
Gardener creates two `MachineCloudConfig` CRDs, one for the cloud-config-downloader (which will later flow into the `WorkerPool` CRD) and one for the real cloud-config (which will be stored as a `Secret` in the Shoot's `kube-system` namespace, and downloaded and executed from the cloud-config-downloader on the machines).

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: MachineCloudConfig
metadata:
  name: pool-01-downloader
  namespace: shoot--core--aws-01
spec:
  type: CoreOS
  units:
  - name: cloud-config-downloader.service
    command: start
    enable: true
    content: |
      [Unit]
      Description=Downloads the original cloud-config from Shoot API Server and executes it
      After=docker.service docker.socket
      Wants=docker.socket
      [Service]
      Restart=always
      RestartSec=30
      EnvironmentFile=/etc/environment
      ExecStart=/bin/sh /var/lib/cloud-config-downloader/download-cloud-config.sh
  files:
  - path: /var/lib/cloud-config-downloader/credentials/kubeconfig
    permissions: 0644
    content:
      secretRef:
        name: cloud-config-downloader
        dataKey: kubeconfig
  - path: /var/lib/cloud-config-downloader/download-cloud-config.sh
    permissions: 0644
    content:
      inline:
        encoding: b64
        data: IyEvYmluL2Jhc2ggL...
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
  cloudConfig: | # base64-encoded
    #cloud-config

    coreos:
      update:
        reboot-strategy: off
      units:
      - name: cloud-config-downloader.service
        command: start
        enable: true
        content: |
          [Unit]
          Description=Downloads the original cloud-config from Shoot API Server and execute it
          After=docker.service docker.socket
          Wants=docker.socket
          [Service]
          Restart=always
          RestartSec=30
          ...
```

:information: The cloud-config-downloader script does not only download the cloud-config initially, but does so at regular intervals, e.g., every `30s`.
If it sees an updated cloud-config, then it applies it again by reloading and restarting all systemd units in order to reflect the changes.
The way how this reloading of the cloud-config happens is OS-specific as well and not known to Gardener anymore, however, it must be part of the script already.
On CoreOS you have to execute `/usr/bin/coreos-cloudinit --from-file=<path>`, whereas on SLES you have to execute `cloud-init --file <path> single -n write_files --frequency=once`.
As Gardener doesn't know these commands, it will write a placeholder expression instead (e.g., `{RELOAD-CLOUD-CONFIG-WITH-PATH:<path>}`) and the OS-specific controller is asked to replace it with the proper expression.

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: MachineCloudConfig
metadata:
  name: pool-01-original # stored as secret and downloaded later
  namespace: shoot--core--aws-01
spec:
  type: CoreOS
  units:
  - name: docker.service
    drop-ins:
    - name: 10-docker-opts.conf
      content: |
        [Service]
        Environment="DOCKER_OPTS=--log-opt max-size=60m --log-opt max-file=3"
  - name: docker-monitor.service
    command: start
    enable: true
    content: |
      [Unit]
      Description=Docker-monitor daemon
      After=kubelet.service
      [Service]
      Restart=always
      EnvironmentFile=/etc/environment
      ExecStart=/opt/bin/health-monitor docker
  - name: kubelet.service
    command: start
    enable: true
    content: |
      [Unit]
      Description=kubelet daemon
      Documentation=https://kubernetes.io/docs/admin/kubelet
      After=docker.service
      Wants=docker.socket rpc-statd.service
      [Service]
      Restart=always
      RestartSec=10
      EnvironmentFile=/etc/environment
      ExecStartPre=/bin/docker run --rm -v /opt/bin:/opt/bin:rw k8s.gcr.io/hyperkube:v1.11.2 cp /hyperkube /opt/bin/
      ExecStartPre=/bin/sh -c 'hostnamectl set-hostname $(cat /etc/hostname | cut -d '.' -f 1)'
      ExecStart=/opt/bin/hyperkube kubelet \
          --allow-privileged=true \
          --bootstrap-kubeconfig=/var/lib/kubelet/kubeconfig-bootstrap \
          ...
  files:
  - path: /var/lib/kubelet/ca.crt
    permissions: 0644
    content:
      secretRef:
        name: ca-kubelet
        dataKey: ca.crt
  - path: /var/lib/cloud-config-downloader/download-cloud-config.sh
    permissions: 0644
    content:
      inline:
        encoding: b64
        data: IyEvYmluL2Jhc2ggL...
  - path: /etc/sysctl.d/99-k8s-general.conf
    permissions: 0644
    content:
      inline:
        data: |
          vm.max_map_count = 135217728
          kernel.softlockup_panic = 1
          kernel.softlockup_all_cpu_backtrace = 1
          ...
  - path: /opt/bin/health-monitor
    permissions: 0755
    content:
      inline:
        data: |
          #!/bin/bash
          set -o nounset
          set -o pipefail

          function docker_monitoring {
          ...
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
  cloudConfig: ...
```

Cloud-specific controllers which might need to add another kernel option or another flag to the kubelet, maybe even another file to the disk, can register a `MutatingWebhookConfiguration` to that resource and modify it upon creation/update.
The task of the `MachineCloudConfig` controller is to only generate the OS-specific cloud-config based on the `.spec` field, but not to add or change any logic related to Shoots.

##### Worker Pools Definition

For every worker pool defined in the `Shoot`, Gardener will create a `WorkerPool` CRD which shall be picked up by a cloud-specific controller and be translated to `MachineClass`es and `MachineDeployment`s.

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: WorkerPool
metadata:
  name: pool-01
  namespace: shoot--core--aws-01
spec:
  cloudConfig: base64(downloader-cloud-config)
  infrastructureProviderStatus:
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
  providerConfig:
    apiVersion: aws.cloud.gardener.cloud/v1alpha1
    kind: WorkerPoolConfig
    machineImage:
      name: CoreOS
      ami: ami-d0dcef3b
    machineType: m4.large
    volumeType: gp2
    volumeSize: 20Gi
    zones:
    - eu-west-1a
  region: eu-west-1
  secretRef:
    name: my-aws-credentials
  minimum: 2
  maximum: 2
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
```

##### Generic Resources

Some components are cloud-specific and must be deployed by the cloud-specific botanists.
Others might need to deploy another pod next to the shoot's control plane or must do anything else.
Some of these might be important for a functional cluster (e.g., the cloud-controller-manager, or a CSI plugin in the future), and controllers should be able to report errors back to the user.
Consequently, in order to trigger the controllers to deploy these components, Gardener would write a `Generic` CRD to the Seed to trigger the deployment.
No operation is depending on the status of these resources, however, the entire reconciliation flow is.

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Generic
metadata:
  name: cloud-components
  namespace: shoot--core--aws-01
spec:
  type: cloud-components
  secretRef:
    name: my-aws-credentials
  shootSpec:
    ...
status:
  observedGeneration: ...
  state: ...
  lastError: ..
  lastOperation: ...
```

#### Shoot State

In order to enable moving the control plane of a Shoot between Seed clusters (e.g., if a Seed cluster is not available anymore or entirely broken), Gardener must store some non-reconstructable state, potentially also the state written by the controllers.
Gardener watches these extension CRDs and copies the `.status.state` in a `ShootState` resource into the Garden cluster.
Any observed status change of the respective CRD-controllers must be immediately reflected in the `ShootState` resource.
The contract between Gardener and those controllers is:

```yaml
---
apiVersion: gardener.cloud/v1alpha1
kind: ShootState
metadata:
  name: shoot--core--aws-01
shootRef:
  name: aws-01
  project: core
state:
  secrets:
  - name: ca
    data: ...
  - name: kube-apiserver-cert
    data: ...
  resources:
  - kind: DNS
    name: record-1
    state: <copied-state-of-dns-crd>
  - kind: Infrastructure
    name: networks
    state: <copied-state-of-infrastructure-crd>
  ...
  <other fields required to keep track of>
```

:information: **Every controller must be capable of reconstructing its own environment based on both the state it has written before and on the real world's conditions/state.**

We cannot assume that Gardener is always online to observe the most recent states the controllers have written to their resources.
Consequently, the information stored here must not be used as "single point of truth", but the controllers must potentially check the real world's status to reconstruct themselves.
However, this must anyway be part of their normal reconciliation logic and is a general best practice for Kubernetes controllers.

#### Shoot Health Checks/Conditions

Some of the existing conditions already contain specific code, which shall be simplified as well.
All of the CRDs described above have a `.status.conditions` field to which the controllers may write relevant health information of their function area.
Gardener will pick them up and copy them over to the Shoots `.status.conditions` (only those conditions setting `propagate=true`).

#### Reconciliation Flow

We are now examining the current Shoot creation/reconciliation flow and describing how it could look like when applying this proposal:

| Operation | Description |
|-----------|-------------|
| botanist.DeployNamespace | Gardener creates the namespace for the Shoot in the Seed cluster. |
| botanist.DeployKubeAPIServerService | Gardener creates a Service of type `LoadBalancer` in the Seed.<br>AWS Botanist registers a Mutating Webhook and adds its AWS-specific annotation. |
| botanist.WaitUntilKubeAPIServerServiceIsReady | Gardener checks the `.status` object of the just created `Service` in the Seed. The contract is that also clouds not supporting load balancers must react on the `Service` object and modify the `.status` to correctly reflect the kube-apiserver's ingress IP. |
| botanist.DeploySecrets | Gardener creates the secrets/certificates it needs like it does today, but it provides utility functions that can be adopted by Botanists/other controllers if they need additional certificates/secrets created on their own. (We should also add labels to all secrets) |
| botanist.Shoot.Components.DNS.Internal{Provider/Entry}.Deploy | Gardener creates a DNS-specific CRD in the Seed, and the responsible DNS-controller picks it up and creates a corresponding DNS record (see the CRD specification above). |
| botanist.Shoot.Components.DNS.External{Provider/Entry}.Deploy | Gardener creates a DNS-specific CRD in the Seed, and the responsible DNS-controller picks it up and creates a corresponding DNS record (see the CRD specification above). |
| shootCloudBotanist.DeployInfrastructure | Gardener creates a Infrastructure-specific CRD in the Seed, and the responsible Botanist picks it up and does its job (see the CRD above). |
| botanist.DeployBackupInfrastructure | Gardener creates a `BackupInfrastructure` resource in the Garden cluster.<br>(The BackupInfrastructure controller creates a BackupInfrastructure-specific CRD in the Seed, and the responsible Botanist picks it up and does its job (see the CRD above)) |
| botanist.WaitUntilBackupInfrastructureReconciled | Gardener checks the `.status` object of the just created `BackupInfrastructure` resource. |
| hybridBotanist.DeployETCD | Gardener only deploys the etcd `StatefulSet` without adding a backup-restore sidecar.<br>The cloud-specific Botanist registers a Mutating Webhook and adds the backup-restore sidecar, and it also creates the `Secret` needed by the backup-restore sidecar. |
| botanist.WaitUntilEtcdReady | Gardener checks the `.status` object of the etcd `Statefulset` and waits until readiness is indicated. |
| hybridBotanist.DeployCloudProviderConfig | Gardener does not execute this anymore because it doesn't know anything about cloud-specific configuration. |
| hybridBotanist.DeployKubeAPIServer | Gardener only deploys the kube-apiserver `Deployment` without any cloud-specific flags/configuration.<br> The cloud-specific Botanist registers a Mutating Webhook and adds whatever is needed for the kube-apiserver to run in its cloud environment. |
| hybridBotanist.DeployKubeControllerManager | Gardener only deploys the kube-controller-manager `Deployment` without any cloud-specific flags/configuration.<br>The cloud-specific Botanist registers a Mutating Webhook and adds whatever is needed for the kube-controller-manager to run in its cloud environment (e.g., the cloud-config). |
| hybridBotanist.DeployKubeScheduler | Gardener only deploys the kube-scheduler `Deployment` without any cloud-specific flags/configuration.<br>The cloud-specific Botanist registers a Mutating Webhook and adds whatever is needed for the kube-scheduler to run in its cloud environment. |
| hybridBotanist.DeployCloudControllerManager | Gardener does not execute this anymore because it doesn't know anything about cloud-specific configuration. The Botanists would be responsible to deploy their own cloud-controller-manager now.<br>They would watch for the kube-apiserver Deployment to exist, and as soon as it does, they would deploy the CCM.<br> (Side note: The Botanist would also be responsible to deploy further controllers needed for this cloud environment, e.g. F5-controllers or CSI plugins). |
| botanist.WaitUntilKubeAPIServerReady | Gardener checks the `.status` object of the kube-apiserver `Deployment` and waits until readiness is indicated. |
| botanist.InitializeShootClients | Unchanged; Gardener creates a Kubernetes client for the Shoot cluster. |
| botanist.DeployMachineControllerManager | Deleted, Gardener no longer deploys MCM itself. See below. |
| hybridBotanist.ReconcileMachines | Gardener creates a `Worker` CRD in the Seed, and the responsible `Worker` controller picks it up and does its job (see the CRD above). It also deploys the machine-controller-manager.<br>Gardener waits until the status indicates that the controller is done. |
| hybridBotanist.DeployKubeAddonManager | This function also computes the CoreOS cloud-config (because the secret storing it is managed by the kube-addon-manager).<br>Gardener would deploy the CloudConfig-specific CRD in the Seed, and the responsible OS controller picks it up and does its job (see the CRD above).<br>The Botanists, which would have to modify something, would register a Webhook for this CloudConfig-specific resource and apply their changes.<br>The rest is mostly unchanged, Gardener generates the manifests for the addons and deploys the kube-addon-manager into the Seed.<br>AWS Botanist registers a Webhook for nginx-ingress.<br>Azure Botanist registers a Webhook for calico.<br>Gardener will no longer deploy the `StorageClass`es. Instead, the Botanists wait until the kube-apiserver is available and deploy them.<br><br>In the long term we want to get rid of optional addons inside the Gardener core and implement a sophisticated addon concept (see [#246](https://github.com/gardener/gardener/issues/246)). |
| shootCloudBotanist.DeployKube2IAMResources | This function would be removed (currently Gardener would execute a Terraform job creating the IAM roles specified in the Shoot manifest). We cannot keep this behavior, the user would be responsible to create the needed IAM roles on its own. |
| botanist.Shoot.Components.Nginx.DNSEtnry | Gardener creates a DNS-specific CRD in the Seed, and the responsible DNS-controller picks it up and creates a corresponding DNS record (see the CRD specification above). |
| botanist.WaitUntilVPNConnectionExists | Unchanged, Gardener checks that it is possible to port-forward to a Shoot pod. |
| seedCloudBotanist.ApplyCreateHook | This function would be removed (actually, only the AWS Botanist implements it).<br>AWS Botanist deploys the aws-lb-readvertiser once the API Server is deployed and updates the ELB health check protocol one the load balancer pointing to the API server is created. |
| botanist.DeploySeedMonitoring | Unchanged, Gardener deploys the monitoring stack into the Seed. |
| botanist.DeployClusterAutoscaler | Unchanged, Gardener deploys the cluster-autoscaler into the Seed. |

:information: We can easily lift the contract later and allow dynamic network plugins or not using the VPN solution at all.
We could also introduce a dedicated `ControlPlane` CRD and leave the complete responsibility of deploying kube-apiserver, kube-controller-manager, etc., to other controllers (if we need it at some point in time).

#### Deletion Flow

We are now examining the current Shoot deletion flow and describe shortly how it could look like when applying this proposal:

| Operation | Description |
|-----------|-------------|
| botanist.DeploySecrets | This is just refreshing the cloud provider secret in the Shoot namespace in the Seed (in case the user has changed it before triggering the deletion). This function would stay as it is. |
| hybridBotanist.RefreshMachineClassSecrets | This function would disappear.<br>The Worker Pool controller needs to watch the referenced secret and update the generated MachineClassSecrets immediately. |
| hybridBotanist.RefreshCloudProviderConfig | This function would disappear. Botanist needs to watch the referenced secret and update the generated cloud-provider-config immediately. |
| botanist.RefreshCloudControllerManagerChecksums | See "hybridBotanist.RefreshCloudProviderConfig". |
| botanist.RefreshKubeControllerManagerChecksums | See "hybridBotanist.RefreshCloudProviderConfig". |
| botanist.InitializeShootClients | Unchanged; Gardener creates a Kubernetes client for the Shoot cluster. |
| botanist.DeleteSeedMonitoring | Unchanged; Gardener deletes the monitoring stack. |
| botanist.DeleteKubeAddonManager | Unchanged; Gardener deletes the kube-addon-manager. |
| botanist.DeleteClusterAutoscaler | Unchanged; Gardener deletes the cluster-autoscaler. |
| botanist.WaitUntilKubeAddonManagerDeleted | Unchanged; Gardener waits until the kube-addon-manager is deleted. |
| botanist.CleanCustomResourceDefinitions | Unchanged, Gardener cleans the CRDs in the Shoot. |
| botanist.CleanKubernetesResources | Unchanged, Gardener cleans all remaining Kubernetes resources in the Shoot. |
| hybridBotanist.DestroyMachines | Gardener deletes the WorkerPool-specific CRD in the Seed, and the responsible WorkerPool-controller picks it up and does its job.<br>Gardener waits until the CRD is deleted. |
| shootCloudBotanist.DestroyKube2IAMResources | This function would disappear (currently Gardener would execute a Terraform job deleting the IAM roles specified in the `Shoot` manifest). We cannot keep this behavior, the user would be responsible to delete the needed IAM roles on its own. |
| shootCloudBotanist.DestroyInfrastructure | Gardener deletes the Infrastructure-specific CRD in the Seed, and the responsible Botanist picks it up and does its job.<br>Gardener waits until the CRD is deleted. |
| botanist.Shoot.Components.DNS.External{Provider/Entry}.Destroy | Gardener deletes the DNS-specific CRD in the Seed, and the responsible DNS-controller picks it up and does its job.<br>Gardener waits until the CRD is deleted. |
| botanist.DeleteKubeAPIServer | Unchanged; Gardener deletes the kube-apiserver. |
| botanist.DeleteBackupInfrastructure | Unchanged; Gardener deletes the `BackupInfrastructure` object in the Garden cluster.<br>(The BackupInfrastructure controller deletes the BackupInfrastructure-specific CRD in the Seed, and the responsible Botanist picks it up and does its job.<br>The BackupInfrastructure controller waits until the CRD is deleted.) |
| botanist.Shoot.Components.DNS.Internal{Provider/Entry}.Destroy | Gardener deletes the DNS-specific CRD in the Seed, and the responsible DNS-controller picks it up and does its job.<br>Gardener waits until the CRD is deleted. |
| botanist.DeleteNamespace | Unchanged; Gardener deletes the Shoot namespace in the Seed cluster. |
| botanist.WaitUntilSeedNamespaceDeleted | Unchanged; Gardener waits until the Shoot namespace in the Seed has been deleted. |
| botanist.DeleteGardenSecrets | Unchanged; Gardener deletes the kubeconfig/ssh-keypair `Secret` in the project namespace in the Garden. |

### gardenlet

One part of the whole extensibility work will also to further split Gardener itself.
Inspired from Kubernetes itself, we plan to move the `Shoot` reconciliation/deletion controller loops, as well as the `BackupInfrastructure` reconciliation/deletion controller loops, into a dedicated "gardenlet" component that will run in the Seed cluster.
With that, it can talk locally to the responsible kube-apiserver and we no longer need to perform every operation out of the Garden cluster.
This approach will also help us with scalability, performance, maintainability, and testability in general.

This architectural change implies that the Kubernetes API server of the Garden cluster must be exposed publicly (or at least be reachable by the registered Seeds). The Gardener controller-manager will remain and will keep its `CloudProfile`, `SecretBinding`, `Quota`, `Project`, and `Seed` controller loops. One part of the seed controller could be to deploy the "gardenlet" into the Seeds, however, this would require network connectivity to the Seed cluster.

### Shoot Control Plane Movement/Migration

Automatically moving control planes is difficult with the current implementation as some resources created in the old Seed must be moved to the new one. However, some of them are not under Gardener's control (e.g., `Machine` resources). Moreover, the old control plane must be deactivated somehow to ensure that not two controllers work on the same things (e.g., virtual machines) from different environments.

Gardener does not only deploy a DNS controller into the Seeds but also into its own Garden cluster.
For every Shoot cluster, Gardener commissions it to create a DNS `TXT` record containing the name of the Seed responsible for the Shoot (holding the control plane), e.g.

```bash
dig -t txt aws-01.core.garden.example.com

...
;; ANSWER SECTION:
aws-01.core.garden.example.com. 120 IN	TXT "Seed=seed-01"
...
```

Gardener always keeps the DNS record up-to-date based on which Seed is responsible.

In the above CRD examples one object in the `.spec` section was omitted, as it is needed to get Shoot control plane movement/migration working (the field is only explained now in this section and not before; it was omitted on purpose to support focusing on the relevant specifications first).
Every CRD also has the following section in its `.spec`:

```yaml
leadership:
  record: aws-01.core.garden.example.com
  value: seed-01
  leaseSeconds: 60
```

Before every operation, the CRD-controllers check this DNS record (based on the `.spec.leadership.leaseSeconds` configuration) and verify that its result is equal to the `.spec.leadership.value` field.
If both match, they know that they should act on the resource, otherwise they stop doing anything.

:information: We will provide an easy-to-use framework for the controllers containing all of these features out-of-the-box in order to allow the developers to focus on writing the actual controller logic.

When a Seed control plane move is triggered, the `.spec.cloud.seed` field of the respective `Shoot` is changed.
Gardener will change the respective DNS record's value (`aws-01.core.garden.example.com`) to contain the new Seed name.
After that it will wait `2*60s` to be sure that all controllers have observed the change.
Then it starts reconciling and applying the CRDs together with a preset `.status.state` into the new Seed (based on its last observations which were stored in the respective `ShootState` object stored in the Garden cluster).
The controllers are - as per contract - asked to reconstruct their own environment based on the `.status.state` they have written before and the real world's status.
Apart from that, the normal reconciliation flow gets executed.

Gardener stores the list of Seeds that were responsible for hosting a Shoots control plane at some time in the Shoots `.status.seeds` list so that it knows which Seeds must be cleaned up (i.e., where the control plane must be deleted because it has been moved).
Once cleaned up, the Seed's name will be removed from that list.

### BackupInfrastructure Migration

One part of the reconciliation flow above is the provisioning of the infrastructure for the Shoot's etcd backups (usually, this is a blob store bucket/container).
Gardener already uses a separate `BackupInfrastructure` resource that is written into the Garden cluster and picked up by a dedicated `BackupInfrastructure` controller (bundled into the Gardener controller manager).
This dedicated resource exists mainly for the reason to allow keeping backups for a certain "grace period" even after the Shoot deletion itself:

```yaml
apiVersion: gardener.cloud/v1alpha1
kind: BackupInfrastructure
metadata:
  name: aws-01-bucket
  namespace: garden-core
spec:
  seed: seed-01
  shootUID: uuid-of-shoot
```

The actual provisioning is executed in a corresponding Seed cluster, as Gardener can only assume network connectivity to the underlying cloud environment in the Seed.
We would like to keep the created artifacts in the Seed (e.g., Terraform state) near to the control plane.
Consequently, when Gardener moves a control plane, it will update the `.spec.seed` field of the `BackupInfrastructure` resource as well.
With the exact same logic described above, the `BackupInfrastructure` controller inside the Gardener will move to the new Seed.

## Registration of External Controllers at Gardener

We want to have a dynamic registration process, i.e. we don't want to hard-code any information about which controllers shall be deployed.
The ideal solution would be to not even requiring a restart of Gardener when a new controller registers.

Every controller is registered by a `ControllerRegistration` resource that introduces every controller together with its supported resources (dimension (`kind`) and shape (`type`) combination) to Gardener:

```yaml
apiVersion: gardener.cloud/v1alpha1
kind: ControllerRegistration
metadata:
  name: dns-aws-route53
spec:
  resources:
  - kind: DNS
    type: aws-route53
# deployment:
#   type: helm
#   providerConfig:
#     chart.tgz: base64(helm-chart)
#     values.yaml: |
#       foo: bar
```

Every `.kind`/`.type` combination may only exist once in the system.

When a `Shoot` shall be reconciled, Gardener can identify based on the referenced `Seed` and the content of the `Shoot` specification which controllers are needed in the respective Seed cluster.
It will demand the operators in the Garden cluster to deploy the controllers they are responsible for to a specific Seed.
This kind of communication happens via CRDs as well:

```yaml
apiVersion: gardener.cloud/v1alpha1
kind: ControllerInstallation
metadata:
  name: dns-aws-route53
spec:
  registrationRef:
    name: dns-aws-route53
  seedRef:
    name: seed-01
status:
  conditions:
  - lastTransitionTime: 2018-08-07T15:09:23Z
    message: The controller has been successfully deployed to the seed.
    reason: ControllerDeployed
    status: "True"
    type: Available
```

The default scenario is that every controller is gets deployed by a dedicated operator that knows how to handle its lifecycle operations like deployment, update, upgrade, deletion.
This operator watches `ControllerInstallation` resources and reacts on those it is responsible for (that it has created earlier).
Gardener is responsible for writing the `.spec` field, the operator is responsible for providing information in the `.status` indicating whether the controller was successfully deployed and is ready to be used.
Gardener will be also able to ask for deletion of controllers from Seeds when they are not needed there anymore by deleting the corresponding `ControllerInstallation` object.

:information: The provided easy-to-use framework for the controllers will also contain these needed features to implement corresponding operators.

For most cases, the controller deployment is very simple (just deploying it into the seed with some static configuration).
In these cases, it would produce unnecessary effort to ask for providing another component (the operator) that deploys the controller.
To simplify this situation, Gardener will be able to react on `ControllerInstallation`s specifying `.spec.registration.deployment.type=helm`.
The controller would be registered with the `ControllerRegistration` resources that would contain a Helm chart with all resources needed to deploy this controller into a seed (plus some static values).
Gardener would render the Helm chart and deploy the resources into the seed.
It will not react if `.spec.registration.deployment.type!=helm`, which allows it to also use any other deployment mechanism. Controllers that are getting deployed by operators would not specify the `.spec.deployment` section in the `ControllerRegistration` at all.

:information: Any controller requiring dynamic configuration values (e.g., based on the cloud provider or the region of the seed) must be installed with the operator approach.

## Other Cloud-Specific Parts

The Gardener API server has a few admission controllers that contain cloud-specific code. We have to replace these parts as well.

### Defaulting and Validation Admission Plugins

Right now, the admission controllers inside the Gardener API server perform a lot of validation and defaulting of fields in the Shoot specification.
The cloud-specific parts of these admission controllers will be replaced by mutating admission webhooks that will get called instead.
As we will have a dedicated operator running in the Garden cluster anyway, it will also get the responsibility to register this webhook if it needs to validate/default parts of the Shoot specification.

Example: The `.spec.cloud.workerPools[*].providerConfig.machineImage` field in the new Shoot manifest mentioned above could be omitted by the user and would get defaulted by the cloud-specific operator.

### DNS Hosted Zone Admission Plugin

For the same reasons, the existing DNS Hosted Zone admission plugin will be removed from the Gardener core and moved into the responsibility of the respective DNS-specific operators running in the Garden cluster.

### Shoot Quota Admission Plugin

The Shoot quota admission plugin validates create or update requests on Shoots and checks that the specified machine/storage configuration is defined as per referenced `Quota` objects.
The cloud-specifics in this controller are no longer needed, as the `CloudProfile` and the `Shoot` resource have been adapted.
The machine/storage configuration is no longer in cloud-specific sections but in hard-wired fields in the general `Shoot` specification (see the example resources above).
The quota admission plugin will be simplified and remains in the Gardener core.

### Shoot Maintenance Controller

Every Shoot cluster can define a maintenance time window in which Gardener will update the Kubernetes patch version (if enabled) and the used machine image version in the Shoot resource.
While the Kubernetes version is not part of the `providerConfig` section in the `CloudProfile` resource, the `machineImage` field is, and thus Gardener can't understand it any longer.
In the future, Gardener has to rely on the cloud-specific operator (probably the same doing the defaulting/validation mentioned before) to update this field.
In the maintenance time window the maintenance controller will update the Kubernetes patch version (if enabled) and add a `trigger.gardener.cloud=maintenance` annotation in the Shoot resource.
The already registered mutating web hook will call the operator who has to remove this annotation and update the `machineImage` in the `.spec.cloud.workerPools[*].providerConfig` sections.

## Alternatives

* Alternative to DNS approach for Shoot control plane movement/migration: We have thought about rotating the credentials when a move is triggered, which would make all controllers ineffective immediately. However, one problem with this is that we require IAM privileges for the users infrastructure account which might be not desired. Another, more complicated problem is that we cannot assume API access in order to create technical users for all cloud environments that might be supported.
