## Motivation
`gardenctl` (v1) has the functionality to setup ssh sessions to the targeted shoot cluster. For this, infrastructure resources like vms, firewall rules etc. have to be created. `gardenctl` will clean up the resources after the SSH session. However there were issues in the past where that infrastructure resources did not get cleaned up properly, for example due to some error and was not retried. Hence the proposal, to have a dedicated controller (for each infrastructure) that manages the infrastructure resources. `gardenctl` also re-used the ssh node credentials for the bastion host. Instead, a new temporary SSH key pair should be created for the bastion host.
The static shoot-specific SSH key pair should be rotated regularily, for example once in the maintenance time window.

In a [previous proposal](https://github.com/gardener/gardenctl/issues/508) it was suggested that an extension, running on the seed cluster, watches `Bastion` custom resources on the garden cluster, acting upon it and updating it's status accordingly.
However changes to the `Bastion` resource should only be allowed for controllers on seeds that are responsible for it. This cannot be restricted when using custom resources.
The proposal, as outlined below, suggests to implement the necessary changes in the gardener core components and to adapt the [SeedAuthorizer](https://github.com/gardener/gardener/issues/1723) to consider `Bastion` resources that the Gardener API Server serves.


### Goals
- Operators have own SSH keys to access the bastion host
- The SSH private key for the bastion host is encrypted with the user's public PGP key, so that only the user who requested the SSH session can access the bastion
- A controller on the seed ensures that the temporary infrastructure resources for the bastion are cleaned up after afterwards

### Non-Goals
- Restricting access to specific node groups and to have temporary OS users and SSH keys for a user/operator on the shoot worker nodes (for auditing purposes). We had the impression that the effort doesn't justify the security improvement (at least for now as the first step; we might reconsider this at a later point in time).
- Shared bastion host, in case multiple operators want to access the same shoot
- Use machine controller manager to create the bastion

## Proposal

### Involved Components
The following is a list of involved components, that either need to be newly introduced or extended if already existing
- `gardenctlv2` (or any other client)
    - Creates `Bastion` resource
    - SSH to shoot node through a bastion
    - Heartbeats / keeps alive the `Bastion` resource during SSH connection
- Gardener extension provider <infra>
    - Provider specific bastion controller
    - Should be added to gardener-extension-provider-<infra> repos, e.g. https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller
    - Has the permission to update the `Bastion/status` subresource on the seed cluster
    - Runs on seed (of course)
- Gardener API Server (`GAPI`)
    - New `operations.gardener.cloud` API Group
    - New resource type `Bastion`
    - New Admission Webhooks for `Bastion` resource
    - `SeedAuthorizer`: The `SeedAuthorizer` and dependency graph needs to be extended to consider the `Bastion` resource https://github.com/gardener/gardener/tree/master/pkg/admissioncontroller/webhooks/auth/seed/graph
- Gardener Controller Manager (`GCM`)
    - `Bastion` heartbeat controller
        - Cleans up `Bastion` resource on missing heartbeat.
        - Is configured with a `maxLifetime` and `timeToLife` for the `Bastion` resource
- `gardenlet`
    - Similar to `BackupBucket`s or `BackupEntry`, the `gardenlet` watches the `Bastion` resource in the garden cluster and creates a seed-local `Bastion` resource, on which the provider specific bastion controller acts upon

### SSH Flow
0. Users should only get the RBAC permission to `create` / `update` `Bastion` resources for a namespace, if they should be allowed to SSH onto the shoot nodes in this namespace.
1. User/`gardenctlv2` creates `Bastion` resource in garden cluster (see resource example below)
    - First, gardenctl would figure out the external IP. Either by calling an external service (gardenctl (v1) uses https://github.com/gardener/gardenctl/blob/master/pkg/cmd/miscellaneous.go#L226) or by calling a binary that prints the external IP(s) to stdt out. The binary should be configurable. The result is set under `spec.clientIP`
    - the public PGP key of the user is set under `spec.publicKey`. The key that should be used needs to be configured beforehand by the user
    - The targeted shoot is set under `spec.shootRef`
2. GAPI Admission Control for the `Bastion` resource in the garden cluster
    - Mutating Webhook
        - according to shootRef, sets the `spec.seedName`
        - according to shootRef, sets the `spec.providerType`
        - on creation, sets `metadata.annotations["operations.gardener.cloud/created-by"]` according to the user that created the resource
    - Validating Webhook for the `Bastion` resource
        - For security reasons, it validates that only the user who created the resource can update the spec so that for example another user cannot sneak in his own ip for the bastion firewall rule and own public PGP key, to be able to decrypt the SSH private key
3. `gardenlet`
    - Watches `Bastion` resource for own seed under api group `operations.gardener.cloud` in the garden cluster
    - Creates `Bastion` custom resource under api group `extensions.gardener.cloud/v1alpha1` in the seed cluster
4. Gardener extension provider <infra> / Bastion Controller on Seed:
    - With own `Bastion` Custom Resource Definition in the seed under the api group `extensions.gardener.cloud/v1alpha1`
    - Watches `Bastion` custom resources that are created by the `gardenlet` in the seed
    - Creates SSH key pair in memory. Stores the secret key encrypted under `status.id_rsa.enc` using `spec.publicKey`. Stores the public key under `status.id_rsa.pub`
    - Controller reads `cloudprovider` credentials from seed-shoot namespace
    - Deploy infrastructure resources
        - Bastion VM. User data similar to https://github.com/gardener/gardenctl/blob/1e3e5fa1d5603e2161f45046ba7c6b5b4107369e/pkg/cmd/ssh.go#L160-L171. Writes `status.id_rsa.pub` into `authorized_keys` file.
        - create security groups / firewall rules etc.
    - Updates status of `Bastion` resource:
        - With bastion IP under `status.bastionIP`
        - Sets `status.state` to `Ready` on resource so that the client knows when to initiate the SSH connection
5. `gardenlet`
    - Once the `Bastion` resource is in ready state, it syncs back the state to the garden cluster
6. gardenctl
    - initiates SSH session
        - reads `status["id_rsa.enc"]`, decrypts it with users private PGP key
        - reads bastion IP from `status.bastionIP`
        - reads the private key from the SSH key pair for the shoot node
        - opens SSH to the bastion and from there to the respective shoot node
    - runs heartbeat in parallel as long as the SSH session is open by annotating the `Bastion` resource with `operations.gardener.cloud/operation: keepalive`
7. `GCM`:
    - Once `status.expirationDate` is reached, the `Bastion` will be marked for deletion
8. `gardenlet`:
    - Once the `Bastion` resource in the garden cluster is marked for deletion, it marks the `Bastion` resource in the seed for deletion
9. Gardener extension provider <infra> / Bastion Controller on Seed:
    - all created resources will be cleaned up
    - On succes, removes finalizer on `Bastion` resource in seed
10. `gardenlet`:
    - removes finalizer on `Bastion` resource in garden cluster

**Example**
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
    # operations.gardener.cloud/operation: keepalive # this annotation is removed by the mutating webhook and the last-heartbeat timestamp and/or the status.expirationDate will be updated accordingly
spec:
  shootRef: # namespace cannot be set / it's the same as .metadata.namespace
    name: my-cluster

  # seedName: aws-eu2 # is set by the mutating webhook
  # providerType: aws # is set by the mutating webhook

  publicKey: LS0tLS1CRUdJTiBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCi4uLgotLS0tLUVORCBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCg== # user's PGP public key.

  clientIP: # external ip of the user
    ipv4: 1.2.3.4
    # ipv6: ::1

status:
  # the following fields are managed by the controller in the seed and synced by gardenlet
  bastionIP: 1.2.3.5
  state: Ready
  id_rsa.enc: LS0tLS1CRUdJTiBQR1AgTUVTU0FHRS0tLS0tCi4uLgotLS0tLUVORCBQR1AgTUVTU0FHRS0tLS0tCg== # SSH private key, enrypted with spec.publicKey
  id_rsa.pub: c3NoLXJzYSAuLi4K

  # the following fields are only set by the mutating webhook
  expirationDate: "2021-03-19T12:58:00Z" # extended on each keepalive
```

`Bastion` custom resource in the seed cluster
```yaml
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Bastion
metadata:
  name: cli-abcdef
  namespace: shoot--myproject--mycluster
  annotations:
    operations.gardener.cloud/created-by: foo
    operations.gardener.cloud/last-heartbeat-at: "2021-03-19T11:58:00Z"
spec:
  publicKey: LS0tLS1CRUdJTiBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCi4uLgotLS0tLUVORCBQR1AgUFVCTElDIEtFWSBCTE9DSy0tLS0tCg== # user's PGP public key.

  clientIP: # external ip of the user
    ipv4: 1.2.3.4
    # ipv6: ::1

status:
  bastionIP: 1.2.3.5
  state: Ready
  id_rsa.enc: LS0tLS1CRUdJTiBQR1AgTUVTU0FHRS0tLS0tCi4uLgotLS0tLUVORCBQR1AgTUVTU0FHRS0tLS0tCg== # SSH private key, enrypted with spec.publicKey
  id_rsa.pub: c3NoLXJzYSAuLi4K

  expirationDate: "2021-03-19T12:58:00Z"
```

## SSH Key Pair Rotation
Currently, the SSH key pair for the shoot nodes are created once during shoot cluster creation. These key pairs should be rotated on a regular basis.

### Proposal
- `gardeneruser` original user data [component](https://github.com/gardener/gardener/tree/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser):
    - The `gardeneruser` [create script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser/templates/scripts/create.tpl.sh) should be changed into a reconcile script script, and renamed accordingly. It needs to be adapted so that the `authorized_keys` file will be updated / overwritten with the current SSH public key from the cloud-config user data.
- Rotation trigger:
    - once in the maintenance time window
    - On demand, by annotating the shoot with `gardener.cloud/operation: rotate-ssh-keypair`
- On rotation trigger:
    - `gardenlet`
        - Prerequisite of SSH key pair rotation: all nodes of all the worker pools have successfully applied the desired version of their cloud-config user data
        - Creates secret `ssh-keypair.old` with the content of `ssh-keypair` in the seed-shoot namespace. The old private key can be used by clients as fallback, in case the new SSH public key is not yet applied on the node
        - Generates new `ssh-keypair`
        - The `OperatingSystemConfig` needs to be re-generated and deployed
        - Once the cloud-config user data is applied to all nodes (as described below) the `ssh-keypair.old` can be deleted
    - As usual (more details on https://github.com/gardener/gardener/blob/master/docs/extensions/operatingsystemconfig.md):
        - Once the `cloud-config-<X>` secret in the `kube-system` namespace of the shoot cluster is updated, it will be picked up by the [`downloader` script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader/templates/scripts/download-cloud-config.tpl.sh) (checks every 30s for updates)
        - The `downloader` runs the ["execution" script](https://github.com/gardener/gardener/blob/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor/templates/scripts/execute-cloud-config.tpl.sh) from the `cloud-config-<X>` secret
        - The "execution" script includes also the original user data script, which it writes to `PATH_CLOUDCONFIG`, compares it against the previous cloud config and runs the script in case it has changed
        - Running the [original user data](https://github.com/gardener/gardener/tree/master/pkg/operation/botanist/component/extensions/operatingsystemconfig/original) script will also run the `gardeneruser` component, where the `authorized_keys` file will be updated
        - After the most recent cloud-config user data was applied, the "execution" script annotates the node with `checksum/cloud-config-data: <cloud-config-checksum>` to indicate the success
