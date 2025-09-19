---
title: Dual-stack network migration
description: Migrate IPv4 shoots to dual-stack IPv4,IPv6 network
---

# Dual-Stack Network Migration

This document provides a guide for migrating IPv4-only Gardener shoot clusters to dual-stack networking (IPv4 and IPv6).

## Overview

Dual-stack networking allows clusters to operate with both IPv4 and IPv6 protocols. This configuration is controlled via the `spec.networking.ipFamilies` field, which accepts the following values:
- `[IPv4]`
- `[IPv6]`
- `[IPv4, IPv6]`
- `[IPv6, IPv4]`

### Key Considerations

- Single stack IPv4 clusters can be migrated to  dual-stack by adding IPv6 as second element.
- The migration of single stack IPv6 clusters to dual-stack is not supported.
- A dual-stack cluster cannot be migrated to single-stack. Migration from single-stack to dual-stack is a one-way process and cannot be undone.
- Migration involves multiple reconciliation runs to ensure a smooth transition without disruptions.

## Preconditions

Gardener supports multiple different network configurations, including running with pod overlay network or native routing. Currently, there is only native routing as supported operating mode for dual-stack networking in Gardener. This means that the pod overlay network needs to be disabled before starting the dual-stack migration. Otherwise, pod-to-pod cross-node communication may not work as expected after the migration.

At the moment, this only affects IPv4-only clusters, which should be migrated to dual-stack networking. IPv6-only clusters always use native routing.

You can check whether your cluster uses overlay network or native routing by looking for `spec.networking.providerConfig.overlay.enabled` in your cluster's manifest. If it is set to `true` or not present, the cluster is using the pod overlay network. If it is set to `false`, the cluster is using native routing.

Please note that there are infrastructure-specific limitations with regards to cluster size due to one route being added per node. Therefore, please consult the documentation of your infrastructure if your cluster should grow beyond 50 nodes and adapt the route limit quotas accordingly before switching to native routing.

To disable the pod overlay network and thereby switch to native routing, adjust your cluster specification as follows:

```yaml
spec:
  ...
  networking:
    providerConfig:
      overlay:
        enabled: false
  ...
```

## Migration Process

The migration process should usually take place during the corresponding shoot maintenance time window. If you wish to run the migration process earlier, then you need to roll the nodes yourself and then trigger a reconcile so that the status of the `DualStackNodesMigrationReady` constraint is set to `true`. Once this is the case a new reconcile needs to be triggered to update the final components as described in step 5.  

### Step 1: Update Networking Configuration

Modify the `spec.networking.ipFamilies` field to include the desired dual-stack configuration. For example, change `[IPv4]` to `[IPv4, IPv6]`.

### Step 2: Infrastructure Reconciliation

Changing the `ipFamilies` field triggers an infrastructure reconciliation. This step applies necessary changes to the underlying infrastructure to support dual-stack networking.

### Step 3: Control Plane Updates

Depending on the infrastructure, control plane components will be updated or reconfigured to support dual-stack networking.

### Step 4: Node Rollout

Nodes must support the new network protocol. However, node rollout is a manual step and is not triggered automatically. It should be performed during a maintenance window to minimize disruptions. Over time, this step may occur automatically, for example, during Kubernetes minor version updates that involve node replacements.

Cluster owners can monitor the progress of this step by checking the `DualStackNodesMigrationReady` constraint in the shoot status. During shoot reconciliation, the system verifies if all nodes support dual-stack networking and updates the migration state accordingly.

### Step 5: Control Plane and CNI Configuration

Once all nodes are migrated, the remaining control plane components and the Container Network Interface (CNI) are configured for dual-stack networking. The nodes migration constraint is removed at the end of this step and the constraint `DNSServiceMigrationReady` is added with status `progressing`.

### Step 6: Restart of CoreDNS Pods

With the next reconciliation, CoreDNS pods are restarted to obtain IPv6 addresses. The constraint `DNSServiceMigrationReady` is set to status `true` once all pods have both IPv4 and IPv6 addresses.

### Step 7: Switch Service `kube-dns` to Dual-Stack

When all CoreDNS pods have IPv6 addresses, the `kube-dns` service will be configured as a dual-stack service with both IPv4 and IPv6 cluster IPs and the constraint `DNSServiceMigrationReady` will be removed.

## Post-Migration Behavior

After completing the migration:
- The shoot cluster supports dual-stack networking.
- The `kube-dns` service operates with both IPv4 and IPv6 cluster IPs.
- CoreDNS pods handle DNS queries for both address families.
- New pods will receive IP addresses from both address families.
- Existing pods will only receive a second IP address upon recreation.
- If full dual-stack networking is required, all pods need to be rolled.
- Existing services remain IPv4-only until recreated with dual-stack configuration.


# Migrate from Dual-Stack to Single-Stack IPv4

In general, it should not be necessary to migrate from dual-stack to single-stack IPv4. However, if unforeseen problems arise, clusters can be migrated back to single-stack networking. This migration is possible even if the original dual-stack migration is not fully completed.

## Important Considerations

**⚠️ Partial Reversibility**: The migration back to single-stack is not a complete reversal of the dual-stack migration:

- **Infrastructure remnants**: Not all infrastructure changes can be reverted. IPv6 addresses will still be present in the shoot status, representing IPv6 addresses of infrastructure resources that were created during the dual-stack migration.
- **Pod range assignment**: Depending on the infrastructure provider, the method for pod range assignment may differ compared to IPv4 shoots that were never migrated to dual-stack.

## Migration Process

The migration back to single-stack follows a similar pattern to the dual-stack migration but in reverse:

### Step 1: Update Networking Configuration

Remove IPv6 from the `spec.networking.ipFamilies` field. Change the configuration from `[IPv4, IPv6]` back to `[IPv4]`.

```yaml
spec:
  ...
  networking:
    ipFamilies:
    - IPv4  # Remove IPv6 entry
  ...
```

The shoot will reconcile automatically after the configuration change. Control plane components will be reconfigured to IPv4 single-stack mode. During this step, the `DualStackNodesMigrationReady` constraint is set to `true`, indicating that nodes and pods still have IPv6 addresses but the control plane is ready for the migration.

### Step 2: Node Rollout

After the control plane reconfiguration, all nodes must be rolled to remove IPv6 configuration and addresses.

**Note**: Nodes and pods will continue to have IPv6 addresses until they are rolled/recreated.

After rolling all nodes, the `DualStackNodesMigrationReady` constraint will be removed automatically during the next reconciliation. At this point:
- All nodes will have only IPv4 addresses
- All pods will have only IPv4 addresses
- The cluster operates in single-stack IPv4 mode

## Post-Migration State

After completing the migration back to single-stack:
- The cluster operates with IPv4-only networking
- New pods receive only IPv4 addresses
- Services operate with IPv4 cluster IPs only
- Some infrastructure resources may retain IPv6 addresses in their metadata (as noted in the considerations above)