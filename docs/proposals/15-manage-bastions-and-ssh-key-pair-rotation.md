---
title: 15 Bastion Management and SSH Key Pair Rotation
gep-number: 15
creation-date: 2021-03-31
status: implementable
authors:
- "@petersutter"
reviewers:
- "@rfranzke"
---

# GEP-15: Bastion Management and SSH Key Pair Rotation

## Table of Contents

- [GEP-15: Bastion Management and SSH Key Pair Rotation](#gep-15-bastion-management-and-ssh-key-pair-rotation)
  - [Table of Contents](#table-of-contents)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
    - [Involved Components](#involved-components)
    - [SSH Flow](#ssh-flow)
    - [Resource Example](#resource-example)
  - [SSH Key Pair Rotation](#ssh-key-pair-rotation)
    - [Rotation Proposal](#rotation-proposal)
    - [Limitations](#limitations)

## Motivation
`gardenctl` (v1) has the functionality to setup `ssh` sessions to the targeted shoot cluster (nodes). To this end, infrastructure resources like VMs, public IPs, firewall rules, etc., have to be created. `gardenctl` will clean up the resources after termination of the `ssh` session (or rather, when the operator is done with her work). However, there were issues in the past where these infrastructure resources were not properly cleaned up afterwards, e.g., due to some error (no retries, either). Hence, the proposal is to have a dedicated controller (for each infrastructure) that manages the infrastructure resources and their cleanup. The current `gardenctl` also reused the `ssh` node credentials for the bastion host. While that's possible, it would be safer to rather use personal or generated `ssh` key pairs to access the bastion host.
The static shoot-specific `ssh` key pair should be rotated regularly, e.g., once in the maintenance time window. This also means that we cannot create the node VMs anymore with infrastructure public keys as these cannot be revoked or rotated (e.g. in AWS) without terminating the VM itself.

Changes to the `Bastion` resource should only be allowed for controllers on seeds that are responsible for it. This cannot be restricted when using custom resources.
The proposal, as outlined below, suggests to implement the necessary changes in the gardener core components and to adapt the [SeedAuthorizer](https://github.com/gardener/gardener/issues/1723) to consider `Bastion` resources that the Gardener API Server serves.


### Goals
- Operators can request and will be granted time-limited `ssh` access to shoot cluster nodes via bastion hosts.
- To that end, requestors must present their public `ssh` key and only this will be installed into `sshd` on the bastion hosts.
- The bastion hosts will be firewalled and ingress traffic will be permitted only from the client IP of the requestor. Except for traffic on port 22 to the cluster worker nodes, no egress from the bastion is allowed.
- The actual node `ssh` private key (resp. key pair) will be rotated by Gardener and access to the nodes is only possible with this constantly rotated key pair and not with the personal one that is used only for the bastion host.
- Bastion host and access is granted only for the extent of this operator request (of course multiple `ssh` sessions are possible, in parallel or repeatedly, but after "the time is up", access is no longer possible).
- By these means (personal public key and allow-listed client IP) nobody else can use (a.k.a. impersonate) the requestor (not even other operators).
- Necessary infrastructure resources for `ssh` access (such as VMs, public IPs, firewall rules, etc.) are automatically created and also terminated after usage, but at the latest after the above mentioned time span is up.

### Non-Goals
- Node-specific access
- Auditability on operating system level (not only auditing the `ssh` login, but everything that is done on a node and other respective resources, e.g., by using dedicated operating system users)
- Reuse of temporarily created necessary infrastructure resources by different users

## Proposal

### Involved Components
The following is a list of involved components that either need to be newly introduced, or extended if already existing:
- Gardener API Server (`GAPI`)
  - New `operations.gardener.cloud` API Group
  - New resource type `Bastion`, see [resource example](#resource-example) below
  - New Admission Webhooks for `Bastion` resource
  - `SeedAuthorizer`: The `SeedAuthorizer` and dependency graph needs to be extended to consider the `Bastion` resource https://github.com/gardener/gardener/tree/master/pkg/admissioncontroller/webhook/auth/seed/graph
  - Is configured with `timeToLive`, the time to add to the current time on each heartbeat
- `gardenlet`
  - Deploys `Bastion` CRD under the `extensions.gardener.cloud` API Group to the Seed, see [resource example](#resource-example) below
  - Similarly to `BackupBucket`s or `BackupEntry`, the `gardenlet` watches the `Bastion` resource in the garden cluster and creates a seed-local `Bastion` resource, on which the provider specific bastion controller acts upon
- `gardenctlv2` (or any other client)
  - Creates `Bastion` resource in the garden cluster
  - Establishes an `ssh` connection to a shoot node, using a bastion host as proxy
  - Heartbeats / keeps alive the `Bastion` resource during `ssh` connection
- Gardener extension provider <infra>
  - Provider specific bastion controller
  - Should be added to gardener-extension-provider-<infra> repos, e.g. https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller
  - Has the permission to update the `Bastion/status` subresource on the seed cluster
  - Runs on seed (of course)
- Gardener Controller Manager (`GCM`)
  - `Bastion` heartbeat controller
    - Cleans up `Bastion` resource on missing heartbeat
    - Is configured with a `maxLifetime` for the `Bastion` resource
- Gardener (RBAC)
  - The project `admin` role should be extended to allow CRUD operations on the `Bastion` resource. The `gardener.cloud:system:project-member-aggregation` `ClusterRole` needs to be updated accordingly (https://github.com/gardener/gardener/blob/master/charts/gardener/controlplane/charts/application/templates/rbac-user.yaml)

### SSH Flow
0. Users should only get the RBAC permission to `create` / `update` `Bastion` resources for a namespace, if they should be allowed to `ssh` onto the shoot nodes in this namespace. A project member with `admin` role will have these permissions.
1. User/`gardenctlv2` creates the `Bastion` resource in the garden cluster (see [resource example](#resource-example) below).
    - First, gardenctl would figure out the own public IP of the user's machine. Either by calling an external service (gardenctl (v1) uses https://github.com/gardener/gardenctl/blob/master/pkg/cmd/miscellaneous.go#L226) or by calling a binary that prints the public IP(s) to `stdout`. The binary should be configurable. The result is set under `spec.ingress[].ipBlock.cidr`.
    - Creates new `ssh` key pair. The newly created key pair is used only once for each bastion host, so it has a 1:1 relationship to it. It is cleaned up after it is not used anymore, e.g., if the `Bastion` resource was deleted.
    - The public `ssh` key is set under `spec.sshPublicKey`.
    - The targeted shoot is set under `spec.shootRef`.
2. GAPI Admission Plugin for the `Bastion` resource in the garden cluster.
    - On creation, sets `metadata.annotations["gardener.cloud/created-by"]` according to the user that created the resource.
    - When `gardener.cloud/operation: keepalive` is set, it will be removed by GAPI from the annotations and `status.lastHeartbeatTimestamp` will be set with the current timestamp. The `status.expirationTimestamp` will be calculated by taking the last heartbeat timestamp and adding `x` minutes (configurable, default `60` Minutes).
    - Validates that only the creator of the bastion (see `gardener.cloud/created-by` annotation) can update `spec.ingress`.
    - Validates that a Bastion can only be created for a Shoot if that Shoot is already assigned to a Seed.
    - Sets `spec.seedName` and `spec.providerType` based on the `spec.shootRef`.
3. `gardenlet`
    - Watches `Bastion` resource for own seed under api group `operations.gardener.cloud` in the garden cluster.
    - Creates `Bastion` custom resource under api group `extensions.gardener.cloud/v1alpha1` in the seed cluster.
      - Populates bastion user data under field under `spec.userData`, similar to https://github.com/gardener/gardenctl/blob/1e3e5fa1d5603e2161f45046ba7c6b5b4107369e/pkg/cmd/ssh.go#L160-L171. This means that the `spec.sshPublicKey` from the `Bastion` resource in the garden cluster will end up in the `authorized_keys` file on the bastion host.
4. Gardener extension provider <infra> / Bastion Controller on Seed:
    - With own `Bastion` Custom Resource Definition in the seed under the api group `extensions.gardener.cloud/v1alpha1`.
    - Watches `Bastion` custom resources that are created by the `gardenlet` in the seed.
    - The controller reads the `cloudprovider` credentials from the seed-shoot namespace.
    - Deploys infrastructure resources:
        - Bastion VM. Uses user data from `spec.userData`.
        - Attaches public IP, creates security group, firewall rules, etc.
    - Updates the status of the `Bastion` resource:
        - With bastion IP under `status.ingress.ip` or hostname under `status.ingress.hostname`.
        - Updates the `status.lastOperation` with the status of the last reconcile operation.
5. `gardenlet`
    - Syncs back the `status.ingress` and `status.conditions` of the `Bastion` resource in the seed to the garden cluster in case it changed.
6. `gardenctl`
    - Initiates `ssh` session once `status.conditions['BastionReady']` is true of the `Bastion` resource in the garden cluster.
        - Locates the private `ssh` key matching `spec["sshPublicKey"]` which was configured beforehand by the user.
        - Reads the bastion IP (`status.ingress.ip`) or hostname (`status.ingress.hostname`).
        - Reads the private key from the `ssh` key pair for the shoot node.
        - Opens an `ssh` connection to the bastion and from there to the respective shoot node.
    - Runs heartbeat in parallel as long as the `ssh` session is open by annotating the `Bastion` resource with `gardener.cloud/operation: keepalive`.
7. `GCM`:
    - Once `status.expirationTimestamp` is reached, the `Bastion` will be marked for deletion.
8. `gardenlet`:
    - Once the `Bastion` resource in the garden cluster is marked for deletion, it marks the `Bastion` resource in the seed for deletion.
9. Gardener extension provider <infra> / Bastion Controller on Seed:
    - All created resources will be cleaned up.
    - On succes, removes finalizer on `Bastion` resource in seed.
10. `gardenlet`:
    - Removes finalizer on `Bastion` resource in garden cluster.

### Resource Example

`Bastion` resource in the garden cluster
```yaml
apiVersion: operations.gardener.cloud/v1alpha1
kind: Bastion
metadata:
  generateName: cli-
  name: cli-abcdef
  namespace: garden-myproject
  annotations:
    gardener.cloud/created-by: foo # immutable, set by the GAPI Admission Plugin
    # gardener.cloud/operation: keepalive # this annotation is removed by the GAPI and the status.lastHeartbeatTimestamp and status.expirationTimestamp will be updated accordingly
spec:
  shootRef: # namespace cannot be set / it's the same as .metadata.namespace
    name: my-cluster # immutable

  # the following fields are set by the GAPI
  seedName: aws-eu2
  providerType: aws

  sshPublicKey: c3NoLXJzYSAuLi4K # immutable, public `ssh` key of the user

  ingress: # can only be updated by the creator of the bastion
  - ipBlock:
      cidr: 1.2.3.4/32 # public IP of the user. CIDR is a string representing the IP Block. Valid examples are "192.168.1.1/24" or "2001:db9::/64"

status:
  observedGeneration: 1

  # the following fields are managed by the controller in the seed and synced by gardenlet
  ingress: # IP or hostname of the bastion
    ip: 1.2.3.5
    # hostname: foo.bar

  conditions:
  - type: BastionReady # when the `status` is true of condition type `BastionReady`, the client can initiate the `ssh` connection
    status: 'True'
    lastTransitionTime: "2021-03-19T11:59:00Z"
    lastUpdateTime: "2021-03-19T11:59:00Z"
    reason: BastionReady
    message: Bastion for the cluster is ready.

  # the following fields are only set by the GAPI
  lastHeartbeatTimestamp: "2021-03-19T11:58:00Z" # will be set when setting the annotation gardener.cloud/operation: keepalive
  expirationTimestamp: "2021-03-19T12:58:00Z" # extended on each keepalive
```

`Bastion` custom resource in the seed cluster
```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Bastion
metadata:
  name: cli-abcdef
  namespace: shoot--myproject--mycluster
spec:
  userData: |- # this is normally base64-encoded, but decoded for the example. Contains spec.sshPublicKey from Bastion resource in garden cluster
    #!/bin/bash
    # create user
    # add ssh public key to authorized_keys
    # ...

  ingress:
  - ipBlock:
      cidr: 1.2.3.4/32

  type: aws # from extensionsv1alpha1.DefaultSpec

status:
  observedGeneration: 1
  ingress:
    ip: 1.2.3.5
    # hostname: foo.bar
  conditions:
  - type: BastionReady
    status: 'True'
    lastTransitionTime: "2021-03-19T11:59:00Z"
    lastUpdateTime: "2021-03-19T11:59:00Z"
    reason: BastionReady
    message: Bastion for the cluster is ready.
```

## SSH Key Pair Rotation
Currently, the `ssh` key pair for the shoot nodes are created once during shoot cluster creation. These key pairs should be rotated on a regular basis.

### Rotation Proposal
- `gardeneruser` original user data [component](../../pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser):
    - The `gardeneruser` create script should be changed into a reconcile script, and renamed accordingly. It needs to be adapted so that the `authorized_keys` file will be updated / overwritten with the current and old `ssh` public key from the cloud-config user data.
- Rotation trigger:
    - Once in the maintenance time window
    - On demand, by annotating the shoot with `gardener.cloud/operation: rotate-ssh-keypair`
- On rotation trigger:
    - `gardenlet`
        - Prerequisite of `ssh` key pair rotation: all nodes of all the worker pools have successfully applied the desired version of their cloud-config user data.
        - Creates or updates the secret `ssh-keypair.old` with the content of `ssh-keypair` in the seed-shoot namespace. The old private key can be used by clients as fallback, in case the new `ssh` public key is not yet applied on the node.
        - Generates new `ssh-keypair` secret.
        - The `OperatingSystemConfig` needs to be re-generated and deployed with the new and old `ssh` public key.
    - As usual (for more details, see [Contract: OperatingSystemConfig Resource](../extensions/operatingsystemconfig.md)):
        - Once the `cloud-config-<X>` secret in the `kube-system` namespace of the shoot cluster is updated, it will be picked up by the [`downloader` script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader/templates/scripts/download-cloud-config.tpl.sh) (checks every 30s for updates).
        - The `downloader` runs the ["execution" script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor/templates/scripts/execute-cloud-config.tpl.sh) from the `cloud-config-<X>` secret.
        - The "execution" script includes also the original user data script, which it writes to `PATH_CLOUDCONFIG`, compares it against the previous cloud config and runs the script in case it has changed.
        - Running the [original user data](../../pkg/operation/botanist/component/extensions/operatingsystemconfig/original) script will also run the `gardeneruser` component, where the `authorized_keys` file will be updated.
        - After the most recent cloud-config user data was applied, the "execution" script annotates the node with `checksum/cloud-config-data: <cloud-config-checksum>` to indicate the success.

### Limitations

Each operating system has its own default user (e.g. `core`, `admin`, `ec2-user`, etc). These users get their SSH keys during VM creation (however, there is a different handling on Google Cloud Platform, as stated below). These keys currently do not get rotated respectively are not removed from the `authorized_keys` file. This means that the initial `ssh` key will still be valid for the default operating system user.

On Google Cloud Platform, the VMs do not have any static users (i.e. no `gardener` user) and there is an agent on the nodes that syncs the users with their SSH keypairs from the GCP IAM service.
