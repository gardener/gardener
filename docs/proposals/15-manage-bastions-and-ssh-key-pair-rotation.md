---
title: Bastion Management and SSH Key Pair Rotation
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

- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
    - [Involved Components](#involved-components)
    - [SSH Flow](#ssh-flow)
    - [Resource Example](#resource-example)
- [SSH Key Pair Rotation](#ssh-key-pair-rotation)
    - [Rotation Proposal](#rotation-proposal)

## Motivation
`gardenctl` (v1) has the functionality to setup SSH sessions to the targeted shoot cluster. For this, infrastructure resources like vms, firewall rules etc. have to be created. `gardenctl` will clean up the resources after the SSH session. However, there were issues in the past where that infrastructure resources did not get cleaned up properly, for example due to some error and was not retried. Hence, the proposal is to have a dedicated controller (for each infrastructure) that manages the infrastructure resources. `gardenctl` also re-used the SSH node credentials for the bastion host. Instead, a new temporary SSH key pair should be created for the bastion host.
The static shoot-specific SSH key pair should be rotated regularly, for example once in the maintenance time window.

Changes to the `Bastion` resource should only be allowed for controllers on seeds that are responsible for it. This cannot be restricted when using custom resources.
The proposal, as outlined below, suggests to implement the necessary changes in the gardener core components and to adapt the [SeedAuthorizer](https://github.com/gardener/gardener/issues/1723) to consider `Bastion` resources that the Gardener API Server serves.


### Goals
- Operators can request and will be granted **one-time**, **personal** , and **time-limited** `ssh` access to cluster nodes:
  - **one-time**: Credentials are under Gardener control and can be used only for the extent of this request (of course multiple `ssh` sessions are possible, in parallel or repeatedly, but after "the time is up", see time-limited property, they will be revoked and can never be used ever again; this, btw., prohibits use of private credentials that Gardener cannot permanently revoke/rotate)
  - **personal**: Credentials are bound to the requestor and logins and logs can be properly audited and traced back to the requestor
  - **time-limited** Credentials are valid only for a specific time span and cannot be used beyond that time span
- The respective generated **personal `ssh` credentials will be made available to the requestor** and nobody else can use (a.k.a. impersonate) them (not even other operators).
- Necessary infrastructure resources for `ssh` access (such as VMs, firewall rules, etc.) are automatically created and also terminated after usage, but at the latest after the above mentioned max time span.

### Non-Goals
- Node-specific access
- Auditability on operating system level (not only auditing the SSH login, but everything that is done on a node and other respective resources, e.g. by using dedicated operating system users)
- Reuse of temporarily created necessary infrastructure resources by different users

## Proposal

### Involved Components
The following is a list of involved components, that either need to be newly introduced or extended if already existing
- Gardener API Server (`GAPI`)
  - New `operations.gardener.cloud` API Group
  - New resource type `Bastion`, see [resource example](#resource-example) below
  - New Admission Webhooks for `Bastion` resource
  - `SeedAuthorizer`: The `SeedAuthorizer` and dependency graph needs to be extended to consider the `Bastion` resource https://github.com/gardener/gardener/tree/master/pkg/admissioncontroller/webhooks/auth/seed/graph
- `gardenlet`
  - Deploys `Bastion` CRD under the `extensions.gardener.cloud` API Group to the Seed, see [resource example](#resource-example) below
  - Similar to `BackupBucket`s or `BackupEntry`, the `gardenlet` watches the `Bastion` resource in the garden cluster and creates a seed-local `Bastion` resource, on which the provider specific bastion controller acts upon
- `gardenctlv2` (or any other client)
  - Creates `Bastion` resource in the garden cluster
  - Establishes an SSH connection to a shoot node, using a bastion host as proxy
  - Heartbeats / keeps alive the `Bastion` resource during SSH connection
- Gardener extension provider <infra>
  - Provider specific bastion controller
  - Should be added to gardener-extension-provider-<infra> repos, e.g. https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller
  - Has the permission to update the `Bastion/status` subresource on the seed cluster
  - Runs on seed (of course)
- Gardener Controller Manager (`GCM`)
  - `Bastion` heartbeat controller
    - Cleans up `Bastion` resource on missing heartbeat.
    - Is configured with a `maxLifetime` and `timeToLife` for the `Bastion` resource
- Gardener (RBAC)
  - The project `admin` role should be extended to allow CRUD operations on the `Bastion` resource. The `gardener.cloud:system:project-member-aggregation` `ClusterRole` needs to be updated accordingly (https://github.com/gardener/gardener/blob/master/charts/gardener/controlplane/charts/application/templates/rbac-user.yaml)

### SSH Flow
0. Users should only get the RBAC permission to `create` / `update` `Bastion` resources for a namespace, if they should be allowed to SSH onto the shoot nodes in this namespace. A project member with `admin` role will have these permissions.
1. User/`gardenctlv2` creates `Bastion` resource in garden cluster (see [resource example](#resource-example) below)
    - First, gardenctl would figure out the own public IP of the user's machine. Either by calling an external service (gardenctl (v1) uses https://github.com/gardener/gardenctl/blob/master/pkg/cmd/miscellaneous.go#L226) or by calling a binary that prints the public IP(s) to stdout. The binary should be configurable. The result is set under `spec.ingress[].from[].ipBlock.cidr`
    - the public PGP key of the user is set under `spec.pgpPublicKey`. The key that should be used needs to be configured beforehand by the user
    - The targeted shoot is set under `spec.shootRef`
2. GAPI Admission Control for the `Bastion` resource in the garden cluster
    - Mutating Webhook
        - on creation, sets `metadata.annotations["operations.gardener.cloud/created-by"]` according to the user that created the resource
        - when `operations.gardener.cloud/operation: keepalive` is set, calculates new `status.expirationTimestamp`
3. `gardenlet`
    - Watches `Bastion` resource for own seed under api group `operations.gardener.cloud` in the garden cluster
    - Creates `Bastion` custom resource under api group `extensions.gardener.cloud/v1alpha1` in the seed cluster
4. `GCM`:
    - During reconcile of the `Bastion` resource:
      - Creates SSH key pair in memory. Stores the secret key encrypted under `status.sshPrivateKey` using `spec.pgpPublicKey`. Stores the public key under `status.sshPublicKey`
        - *Sidenote: We do not want to accept the users own SSH public key because then we cannot control how well they were picked, whether they are rotated and stored safely (the user hasn't leaked her private key), etc.*
      - according to `spec.shootRef`, sets the `status.seedName`
      - according to `spec.shootRef`, sets the `status.providerType`
5. Gardener extension provider <infra> / Bastion Controller on Seed:
    - With own `Bastion` Custom Resource Definition in the seed under the api group `extensions.gardener.cloud/v1alpha1`
    - Watches `Bastion` custom resources that are created by the `gardenlet` in the seed
    - Controller reads `cloudprovider` credentials from seed-shoot namespace
    - Deploy infrastructure resources
        - Bastion VM. User data similar to https://github.com/gardener/gardenctl/blob/1e3e5fa1d5603e2161f45046ba7c6b5b4107369e/pkg/cmd/ssh.go#L160-L171. Writes `status.sshPublicKey` into `authorized_keys` file.
        - create security groups / firewall rules etc.
    - Updates status of `Bastion` resource:
        - With bastion IP under `status.ingress.ip` or hostname under `status.ingress.hostname`
        - Updates the `status.lastOperation` with the status of the last reconcile operation
6. `gardenlet`
    - Syncs back the `status.ingress` and `status.conditions` of the `Bastion` resource in the seed to the garden cluster in case it changed
7. `gardenctl`
    - initiates SSH session once `status.operation['BastionReady']` is true of the `Bastion` resource in the garden cluster
        - reads `status["sshPrivateKey"]`, decrypts it with users private PGP key
        - reads bastion IP (`status.ingress.ip`) or hostname (`status.ingress.hostname`)
        - reads the private key from the SSH key pair for the shoot node
        - opens SSH connection to the bastion and from there to the respective shoot node
    - runs heartbeat in parallel as long as the SSH session is open by annotating the `Bastion` resource with `operations.gardener.cloud/operation: keepalive`
8. `GCM`:
    - Once `status.expirationTimestamp` is reached, the `Bastion` will be marked for deletion
9. `gardenlet`:
    - Once the `Bastion` resource in the garden cluster is marked for deletion, it marks the `Bastion` resource in the seed for deletion
10. Gardener extension provider <infra> / Bastion Controller on Seed:
    - all created resources will be cleaned up
    - On succes, removes finalizer on `Bastion` resource in seed
11. `gardenlet`:
    - removes finalizer on `Bastion` resource in garden cluster

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
    operations.gardener.cloud/created-by: foo # set by the mutating webhook
    operations.gardener.cloud/last-heartbeat-at: "2021-03-19T11:58:00Z"
    # operations.gardener.cloud/operation: keepalive # this annotation is removed by the mutating webhook and the last-heartbeat timestamp and/or the status.expirationTimestamp will be updated accordingly
spec:
  shootRef: # namespace cannot be set / it's the same as .metadata.namespace
    name: my-cluster

  pgpPublicKey: LS0tLS1CRUdJTiBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCi4uLgotLS0tLUVORCBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCg== # user's PGP public key, immutable

  ingress:
  - from:
    - ipBlock:
        cidr: 1.2.3.4/32 # public IP of the user. CIDR is a string representing the IP Block. Valid examples are "192.168.1.1/24" or "2001:db9::/64"

status:
  # the following fields are set by the GCM
  seedName: aws-eu2
  providerType: aws
  sshPrivateKey: LS0tLS1CRUdJTiBQR1AgTUVTU0FHRS0tLS0tCi4uLgotLS0tLUVORCBQR1AgTUVTU0FHRS0tLS0tCg== # SSH private key, enrypted with spec.pgpPublicKey
  sshPublicKey: c3NoLXJzYSAuLi4K

  # the following fields are managed by the controller in the seed and synced by gardenlet
  ingress: # IP or hostname of the bastion
    ip: 1.2.3.5
    # hostname: foo.bar
  conditions:
  - type: BastionReady # when the `status` is true of condition type `BastionReady`, the client can initiate the SSH connection
    status: 'True'
    lastTransitionTime: "2021-03-19T11:59:00Z"
    lastUpdateTime: "2021-03-19T11:59:00Z"
    reason: BastionReady
    message: Bastion for the cluster is ready.

  # the following fields are only set by the mutating webhook
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
  sshPublicKey: c3NoLXJzYSAuLi4K # from status["sshPublicKey"] of Bastion resource in garden cluster

  ingress:
  - from:
    - ipBlock:
        cidr: 1.2.3.4/32

status:
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
Currently, the SSH key pair for the shoot nodes are created once during shoot cluster creation. These key pairs should be rotated on a regular basis.

### Rotation Proposal
- `gardeneruser` original user data [component](https://github.com/gardener/gardener/tree/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser):
    - The `gardeneruser` [create script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser/templates/scripts/create.tpl.sh) should be changed into a reconcile script script, and renamed accordingly. It needs to be adapted so that the `authorized_keys` file will be updated / overwritten with the current and old SSH public key from the cloud-config user data.
- Rotation trigger:
    - Once in the maintenance time window
    - On demand, by annotating the shoot with `gardener.cloud/operation: rotate-ssh-keypair`
- On rotation trigger:
    - `gardenlet`
        - Prerequisite of SSH key pair rotation: all nodes of all the worker pools have successfully applied the desired version of their cloud-config user data
        - Creates or updates the secret `ssh-keypair.old` with the content of `ssh-keypair` in the seed-shoot namespace. The old private key can be used by clients as fallback, in case the new SSH public key is not yet applied on the node
        - Generates new `ssh-keypair` secret
        - The `OperatingSystemConfig` needs to be re-generated and deployed with the new and old SSH public key
    - As usual (more details on https://github.com/gardener/gardener/blob/master/docs/extensions/operatingsystemconfig.md):
        - Once the `cloud-config-<X>` secret in the `kube-system` namespace of the shoot cluster is updated, it will be picked up by the [`downloader` script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader/templates/scripts/download-cloud-config.tpl.sh) (checks every 30s for updates)
        - The `downloader` runs the ["execution" script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor/templates/scripts/execute-cloud-config.tpl.sh) from the `cloud-config-<X>` secret
        - The "execution" script includes also the original user data script, which it writes to `PATH_CLOUDCONFIG`, compares it against the previous cloud config and runs the script in case it has changed
        - Running the [original user data](https://github.com/gardener/gardener/tree/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/original) script will also run the `gardeneruser` component, where the `authorized_keys` file will be updated
        - After the most recent cloud-config user data was applied, the "execution" script annotates the node with `checksum/cloud-config-data: <cloud-config-checksum>` to indicate the success
