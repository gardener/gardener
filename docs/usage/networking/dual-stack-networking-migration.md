---
title: Dual-stack network migration
description: Migrate IPv4 shoots to dual-stack IPv4,IPv6 network
---

# Dual-Stack Network Migration

This document provides a guide for migrating IPv4-only or IPv6-only Gardener shoot clusters to dual-stack networking (IPv4 and IPv6).

## Overview

Dual-stack networking allows clusters to operate with both IPv4 and IPv6 protocols. This configuration is controlled via the `spec.networking.ipFamilies` field, which accepts the following values:
- `[IPv4]`
- `[IPv6]`
- `[IPv4, IPv6]`
- `[IPv6, IPv4]`

### Key Considerations

- Adding a new protocol is only allowed as the second element in the array, ensuring the primary protocol remains unchanged.
- Migration involves multiple reconciliation runs to ensure a smooth transition without disruptions.

## Preconditions

Gardener supports multiple different network configurations, including running with pod overlay network or native routing. Currently, there is only native routing as supported operating mode for dual-stack networking in Gardener. This means that the pod overlay network needs to be disabled before starting the dual-stack migration. Otherwise, pod-to-pod cross-node communication may not work as expected after the migration.

At the moment, this only affects IPv4-only clusters, which should be migrated to dual-stack networking. IPv6-only clusters always use native routing.

You can check whether you cluster uses overlay network or native routing by looking for `spec.networking.providerConfig.overlay.enabled` in your cluster's manifest. If it is set to `true` or not present, the cluster is using the pod overlay network. If it is set to `false`, the cluster is using native routing.

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

### Step 1: Update Networking Configuration

Modify the `spec.networking.ipFamilies` field to include the desired dual-stack configuration. For example, change `[IPv4]` to `[IPv4, IPv6]`.

### Step 2: Infrastructure Reconciliation

Changing the `ipFamilies` field triggers an infrastructure reconciliation. This step applies necessary changes to the underlying infrastructure to support dual-stack networking.

### Step 3: Control Plane Updates

Depending on the infrastructure, control plane components will be updated or reconfigured to support dual-stack networking.

### Step 4: Node Rollout

Nodes must support the new network protocol. However, node rollout is a manual step and is not triggered automatically. It should be performed during a maintenance window to minimize disruptions. Over time, this step may occur automatically, for example, during Kubernetes minor version updates that involve node replacements.

Cluster owners can monitor the progress of this step by checking the `DualStackNodesMigrationReady` constraint in the shoot status. During shoot reconciliation, the system verifies if all nodes support dual-stack networking and updates the migration state accordingly.

### Step 5: Final Reconciliation

Once all nodes are migrated, the remaining control plane components and the Container Network Interface (CNI) are configured for dual-stack networking. The migration constraint is removed at the end of this step.

## Post-Migration Behavior

After completing the migration:
- The shoot cluster supports dual-stack networking.
- New pods will receive IP addresses from both address families.
- Existing pods will only receive a second IP address upon recreation.
- If full dual-stack networking is required all pods need to be rolled.
