// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// TaskGroupReconcileExtensionControllers is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileExtensionControllers flow.TaskID = "TaskGroupReconcileExtensionControllers"

// ReconcileExtensionControllersTaskGroup returns the flow.TaskGroup for deploying the extension controllers and waiting
// for their readiness.
func (b *GardenadmBotanist) ReconcileExtensionControllersTaskGroup(podNetworkAvailable bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileExtensionControllers).WithDependencies(
			botanist.TaskGroupReconcileGardenerResourceManager,
		)

		deployExtensionControllers = g.Add(flow.Task{
			Name: "Deploying extension controllers",
			Fn: func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, !podNetworkAvailable)
			},
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension controllers report readiness",
			Fn:           b.WaitUntilExtensionControllerInstallationsHealthy,
			Dependencies: flow.NewTaskIDs(deployExtensionControllers),
		})
	)

	return g
}
