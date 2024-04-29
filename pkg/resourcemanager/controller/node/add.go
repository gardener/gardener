// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/node/agentreconciliationdelay"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/node/criticalcomponents"
)

// AddToManager adds all node controllers to the given manager.
func AddToManager(mgr manager.Manager, targetCluster cluster.Cluster, cfg config.ResourceManagerConfiguration) error {
	if cfg.Controllers.NodeCriticalComponents.Enabled {
		if err := (&criticalcomponents.Reconciler{
			Config: cfg.Controllers.NodeCriticalComponents,
		}).AddToManager(mgr, targetCluster); err != nil {
			return fmt.Errorf("failed adding node-critical-components controller: %w", err)
		}
	}

	if cfg.Controllers.NodeAgentReconciliationDelay.Enabled {
		if err := (&agentreconciliationdelay.Reconciler{
			Config: cfg.Controllers.NodeAgentReconciliationDelay,
		}).AddToManager(mgr, targetCluster); err != nil {
			return fmt.Errorf("failed adding node-agent-reconciliation-delay controller: %w", err)
		}
	}

	return nil
}
