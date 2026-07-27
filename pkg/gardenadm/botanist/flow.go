// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// TaskGroupReconcileExtensionControllers is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileExtensionControllers flow.TaskID = "TaskGroupReconcileExtensionControllers"

// ReconcileExtensionControllersTaskGroup returns the flow.TaskGroup for deploying the extension controllers and waiting
// for their readiness. If podNetworkAvailable is true, the deployment reconciles the controllers into the pod network.
func (b *GardenadmBotanist) ReconcileExtensionControllersTaskGroup(podNetworkAvailable bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileExtensionControllers).WithDependencies(botanist.TaskGroupReconcileGardenerResourceManager)

		deployExtensionControllers = g.Add(flow.Task{
			Name: "Deploying extension controllers",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, !podNetworkAvailable)
			}).RetryUntilTimeout(5*time.Second, 30*time.Second),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension controllers report readiness",
			Fn:           b.WaitUntilExtensionControllerInstallationsHealthy,
			Dependencies: flow.NewTaskIDs(deployExtensionControllers),
		})
	)

	return g
}

// TaskGroupReconcileNetworkPolicies is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileNetworkPolicies flow.TaskID = "TaskGroupReconcileNetworkPolicies"

// ReconcileNetworkPoliciesTaskGroup returns the flow.TaskGroup for reconciling the network policies.
func (b *GardenadmBotanist) ReconcileNetworkPoliciesTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupReconcileNetworkPolicies, flow.Task{
		Name: "Deploying network policies",
		Fn:   b.ApplyNetworkPolicies,
	}).WithDependencies(botanist.TaskGroupReconcileGardenerResourceManager, TaskGroupReconcileExtensionControllers)
}
