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

const (
	// TaskGroupReconcileExtensionControllers is a flow.TaskID for a logical flow.TaskGroup.
	TaskGroupReconcileExtensionControllers flow.TaskID = "TaskGroupReconcileExtensionControllers"
	// TaskGroupReconcileExtensionControllersInPodNetwork is a flow.TaskID for a logical flow.TaskGroup.
	TaskGroupReconcileExtensionControllersInPodNetwork flow.TaskID = "TaskGroupReconcileExtensionControllersInPodNetwork"
)

// ReconcileExtensionControllersTaskGroup returns the flow.TaskGroup for deploying the extension controllers and waiting
// for their readiness. If podNetworkAvailable is true, it returns the variant that redeploys the extension controllers
// into the pod network and depends on the pod-network `gardener-resource-manager`.
func (b *GardenadmBotanist) ReconcileExtensionControllersTaskGroup(podNetworkAvailable bool) flow.TaskGroup {
	var (
		groupID, taskIDGardenerResourceManagerDependency = TaskGroupReconcileExtensionControllers, botanist.TaskGroupReconcileGardenerResourceManager
		taskNameDeployment                               = "Deploying extension controllers"
		taskNameWait                                     = "Waiting until extension controllers report readiness"
	)
	if podNetworkAvailable {
		groupID, taskIDGardenerResourceManagerDependency = TaskGroupReconcileExtensionControllersInPodNetwork, botanist.TaskGroupReconcileGardenerResourceManagerInPodNetwork
		taskNameDeployment = "Redeploying extension controllers into pod network"
		taskNameWait = "Waiting until extension controllers (in pod network) report readiness"
	}

	var (
		g = flow.NewTaskGroup(groupID).WithDependencies(taskIDGardenerResourceManagerDependency)

		deployExtensionControllers = g.Add(flow.Task{
			Name: taskNameDeployment,
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, !podNetworkAvailable)
			}).RetryUntilTimeout(5*time.Second, 30*time.Second),
		})
		_ = g.Add(flow.Task{
			Name:         taskNameWait,
			Fn:           b.WaitUntilExtensionControllerInstallationsHealthy,
			Dependencies: flow.NewTaskIDs(deployExtensionControllers),
		})
	)

	return g
}

// TaskGroupReconcileNetworkPolicies is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileNetworkPolicies flow.TaskID = "TaskGroupReconcileNetworkPolicies"

// ReconcileNetworkPoliciesTaskGroup returns the flow.TaskGroup for initializing the secret management.
func (b *GardenadmBotanist) ReconcileNetworkPoliciesTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupReconcileNetworkPolicies, flow.Task{
		Name: "Deploying network policies",
		Fn:   b.ApplyNetworkPolicies,
	}).WithDependencies(botanist.TaskGroupReconcileGardenerResourceManager, TaskGroupReconcileExtensionControllers)
}
