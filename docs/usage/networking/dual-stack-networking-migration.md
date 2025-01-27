---
title: Dual-stack network migration
description: Migrate IPv4 shoots to dual-stack IPv,IPv6 network
---

# Dual-Stack Network Migration

This document provides a guide for migrating IPv4-based or IPv6-based Gardener shoot clusters to dual-stack networking (IPv4 and IPv6).
## Overview

Dual-stack networking allows clusters to operate with both IPv4 and IPv6 protocols. This configuration is controlled via the `shoot.Spec.Networking.IPFamilies` field, which accepts the following values:
- `[IPv4]`
- `[IPv6]`
- `[IPv4, IPv6]`
- `[IPv6, IPv4]`

### Key Considerations
- Adding a new protocol is only allowed as the second element in the array, ensuring the primary protocol remains unchanged.
- Migration involves multiple reconciliation runs to ensure a smooth transition without disruptions.

## Migration Process

### Step 1: Update Networking Configuration
Modify the `shoot.Spec.Networking.IPFamilies` field to include the desired dual-stack configuration. For example, change `[IPv4]` to `[IPv4, IPv6]`.

### Step 2: Infrastructure Reconciliation
Changing the `IPFamilies` field triggers an infrastructure reconciliation. This step applies necessary changes to the underlying infrastructure to support dual-stack networking.

### Step 3: Control Plane Updates
Depending on the infrastructure, control plane components will be updated or reconfigured to support dual-stack networking.

### Step 4: Node Rollout
Nodes must support the new network protocol. However, node rollout is not triggered automatically. It should be performed during a maintenance window to minimize disruptions. During shoot reconciliation, the system verifies if all nodes support dual-stack networking and updates the migration state accordingly.

### Step 5: Final Reconciliation
Once all nodes are migrated, the remaining control plane components and the Container Network Interface (CNI) are configured for dual-stack networking. The migration constraint is removed at the end of this step.

## Post-Migration Behavior

After completing the migration:
- The shoot cluster supports dual-stack networking.
- New pods will receive IP addresses from both protocols.
- Existing pods will only receive a second IP address upon restart.



