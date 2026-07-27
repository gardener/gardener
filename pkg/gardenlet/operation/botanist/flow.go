// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// TaskGroupDeployNamespaces is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupDeployNamespaces flow.TaskID = "TaskGroupDeployNamespaces"

// DeployNamespacesTaskGroup returns the flow.TaskGroup for deploying the control plane namespace and the garden
// namespace. The garden namespace is only deployed on machines that run the shoot's control plane, i.e., not during
// `gardenadm bootstrap`.
func (b *Botanist) DeployNamespacesTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupDeployNamespaces,
		flow.Task{
			Name: "Deploying control plane namespace",
			Fn:   b.DeployControlPlaneNamespace,
		},
		flow.Task{
			Name: "Deploying garden namespace",
			Fn: func(ctx context.Context) error {
				return gardenerutils.ReconcileGardenNamespace(ctx, b.SeedClientSet.Client(), v1beta1constants.GardenNamespace, v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo().Spec.Provider.Workers).Zones, true, nil)
			},
			SkipIf: !b.Shoot.RunsControlPlane(),
		},
	)
}

// TaskGroupDeployCloudProviderSecret is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupDeployCloudProviderSecret flow.TaskID = "TaskGroupDeployCloudProviderSecret"

// DeployCloudProviderSecretTaskGroup returns the flow.TaskGroup for deploying the cloud provider secret. This task is
// skipped when no shoot credentials are configured.
func (b *Botanist) DeployCloudProviderSecretTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupDeployCloudProviderSecret, flow.Task{
		Name:   "Deploying cloud provider account secret",
		Fn:     b.DeployCloudProviderSecret,
		SkipIf: b.Shoot.Credentials == nil,
	}).WithDependencies(TaskGroupDeployNamespaces)
}

// TaskGroupReconcileCustomResourceDefinitions is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileCustomResourceDefinitions flow.TaskID = "TaskGroupReconcileCustomResourceDefinitions"

// ReconcileCustomResourceDefinitionsTaskGroup returns the flow.TaskGroup for reconciling the CRDs used by Gardener.
// Each CRD is deployed in an individual task, and it is waited until the CRDs are marked as ready/available.
func (b *Botanist) ReconcileCustomResourceDefinitionsTaskGroup() flow.TaskGroup {
	deployers := map[string]func() (component.DeployWaiter, error){
		"VPA": func() (component.DeployWaiter, error) {
			return vpa.NewCRD(b.SeedClientSet.Client(), nil)
		},
		"Prometheus": func() (component.DeployWaiter, error) {
			return prometheusoperator.NewCRDs(b.SeedClientSet.Client())
		},
		"Fluent": func() (component.DeployWaiter, error) {
			return fluentoperator.NewCRDs(b.SeedClientSet.Client())
		},
		"Extension": func() (component.DeployWaiter, error) {
			return extensioncrds.NewCRD(b.SeedClientSet.Client(), true, true)
		},
		"ETCD": func() (component.DeployWaiter, error) {
			return etcd.NewCRD(b.SeedClientSet.Client(), b.Shoot.KubernetesVersion)
		},
		"Istio": func() (component.DeployWaiter, error) {
			return istio.NewCRD(b.SeedClientSet.Client())
		},
	}

	if b.Shoot.HasManagedInfrastructure() {
		deployers["Machine"] = func() (component.DeployWaiter, error) {
			return machinecontrollermanager.NewCRD(b.SeedClientSet.Client())
		}
	}

	tasks := make([]flow.Task, 0, len(deployers))

	for description, newDeployer := range deployers {
		tasks = append(tasks, flow.Task{
			Name: fmt.Sprintf("Deploying %s CRDs", description),
			Fn: func(ctx context.Context) error {
				d, err := newDeployer()
				if err != nil {
					return fmt.Errorf("failed creating %s deployer: %w", description, err)
				}

				return component.OpWait(d).Deploy(ctx)
			},
		})
	}

	return flow.NewTaskGroup(TaskGroupReconcileCustomResourceDefinitions, tasks...)
}

// TaskGroupReconcileClusterResource is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileClusterResource flow.TaskID = "TaskGroupReconcileClusterResource"

// ReconcileClusterResourceTaskGroup returns the flow.TaskGroup for reconciling the Cluster resource.
func (b *Botanist) ReconcileClusterResourceTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupReconcileClusterResource, flow.Task{
		Name: "Reconciling extensions.gardener.cloud/v1alpha1.Cluster resource",
		Fn: func(ctx context.Context) error {
			return gardenerextensions.SyncClusterResourceToSeed(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo(), b.Shoot.CloudProfile, nil)
		},
	}).WithDependencies(TaskGroupReconcileCustomResourceDefinitions)
}

// TaskGroupInitializeSecretsManagement is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupInitializeSecretsManagement flow.TaskID = "TaskGroupInitializeSecretsManagement"

// InitializeSecretsManagementTaskGroup returns the flow.TaskGroup for initializing the secret management.
func (b *Botanist) InitializeSecretsManagementTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupInitializeSecretsManagement, flow.Task{
		Name: "Initializing internal state of Gardener secrets manager",
		Fn:   b.InitializeSecretsManagement,
	}).WithDependencies(TaskGroupReconcileClusterResource)
}

// TaskGroupReconcileGardenerResourceManager is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileGardenerResourceManager flow.TaskID = "TaskGroupReconcileGardenerResourceManager"

// ReconcileGardenerResourceManagerTaskGroup returns the flow.TaskGroup for deploying the gardener-resource-manager
// instances. It waits for their readiness and also deploys the seed and shoot system resources afterwards.
func (b *Botanist) ReconcileGardenerResourceManagerTaskGroup(podNetworkAvailable, shootIsGarden bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileGardenerResourceManager).WithDependencies(
			TaskGroupDeployNamespaces,
			TaskGroupInitializeSecretsManagement,
			TaskGroupReconcileCustomResourceDefinitions,
		)
		gardenadmBootstrap = b.Shoot.IsSelfHosted() && !b.Shoot.RunsControlPlane()

		deployGardenerResourceManager = g.Add(flow.Task{
			Name: "Deploying gardener-resource-manager",
			Fn: func(ctx context.Context) error {
				b.Shoot.Components.ControlPlane.ResourceManager.SetBootstrapControlPlaneNode(!podNetworkAvailable)

				if shootIsGarden || gardenadmBootstrap {
					return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
				}

				b.Shoot.Components.ControlPlane.RuntimeResourceManager.SetBootstrapControlPlaneNode(!podNetworkAvailable)

				// Deploy sequentially: only `RuntimeResourceManager` installs the `ManagedResource` CRD, and
				// `ResourceManager.Deploy` creates a `ManagedResource` object on the same client.
				return flow.Sequential(
					b.Shoot.Components.ControlPlane.RuntimeResourceManager.Deploy,
					b.Shoot.Components.ControlPlane.ResourceManager.Deploy,
				)(ctx)
			},
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until gardener-resource-manager reports readiness",
			Fn: func(ctx context.Context) error {
				if shootIsGarden || gardenadmBootstrap {
					return b.Shoot.Components.ControlPlane.ResourceManager.Wait(ctx)
				}

				return flow.Parallel(
					b.Shoot.Components.ControlPlane.RuntimeResourceManager.Wait,
					b.Shoot.Components.ControlPlane.ResourceManager.Wait,
				)(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
	)

	return g
}

// TaskGroupReconcileSystemResources is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileSystemResources flow.TaskID = "TaskGroupReconcileSystemResources"

// ReconcileSystemResourcesTaskGroup returns the flow.TaskGroup for deploying the system resources.
func (b *Botanist) ReconcileSystemResourcesTaskGroup() flow.TaskGroup {
	var (
		g                  = flow.NewTaskGroup(TaskGroupReconcileSystemResources).WithDependencies(TaskGroupReconcileGardenerResourceManager)
		gardenadmBootstrap = b.Shoot.IsSelfHosted() && !b.Shoot.RunsControlPlane()

		_ = g.Add(flow.Task{
			Name: "Deploying seed system resources",
			Fn: func(ctx context.Context) error {
				return seedsystem.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, seedsystem.Values{ManagePriorityClasses: true}).Deploy(ctx)
			},
		})
		_ = g.Add(flow.Task{
			Name:   "Deploying shoot system resources",
			Fn:     b.DeployShootSystem,
			SkipIf: gardenadmBootstrap,
		})
	)

	return g
}

// TaskGroupReconcileMachineControllerManager is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileMachineControllerManager flow.TaskID = "TaskGroupReconcileMachineControllerManager"

// ReconcileMachineControllerManagerTaskGroup returns the flow.TaskGroup for deploying the machine-controller-manager.
func (b *Botanist) ReconcileMachineControllerManagerTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupReconcileMachineControllerManager, flow.Task{
		Name:   "Deploying machine-controller-manager",
		Fn:     flow.TaskFn(b.DeployMachineControllerManager).RetryUntilTimeout(time.Second, time.Minute),
		SkipIf: !b.Shoot.HasManagedInfrastructure(),
	}).WithDependencies(
		TaskGroupInitializeSecretsManagement,
		TaskGroupDeployCloudProviderSecret,
		TaskGroupReconcileGardenerResourceManager,
	)
}

// TaskGroupReconcileInfrastructure is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileInfrastructure flow.TaskID = "TaskGroupReconcileInfrastructure"

// ReconcileInfrastructureTaskGroup returns the flow.TaskGroup for deploying the Infrastructure extension resource and
// waiting for its readiness.
func (b *Botanist) ReconcileInfrastructureTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileInfrastructure).WithDependencies(
			TaskGroupInitializeSecretsManagement,
			TaskGroupDeployCloudProviderSecret,
			TaskGroupReconcileGardenerResourceManager,
		)

		deployInfrastructure = g.Add(flow.Task{
			Name:   "Deploying Shoot infrastructure",
			Fn:     b.DeployInfrastructure,
			SkipIf: !b.Shoot.HasManagedInfrastructure(),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Shoot infrastructure has been reconciled",
			Fn:           b.WaitForInfrastructure,
			SkipIf:       !b.Shoot.HasManagedInfrastructure(),
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})
	)

	return g
}

// TaskGroupReconcileControlPlane is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileControlPlane flow.TaskID = "TaskGroupReconcileControlPlane"

// ReconcileControlPlaneTaskGroup returns the flow.TaskGroup for deploying the ControlPlane extension resource and
// waiting for its readiness.
func (b *Botanist) ReconcileControlPlaneTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileControlPlane).WithDependencies(
			TaskGroupInitializeSecretsManagement,
			TaskGroupDeployCloudProviderSecret,
			TaskGroupReconcileGardenerResourceManager,
		)

		deployControlPlane = g.Add(flow.Task{
			Name: "Deploying shoot control plane components",
			Fn:   b.DeployControlPlane,
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been reconciled",
			Fn:           b.Shoot.Components.Extensions.ControlPlane.Wait,
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
	)

	return g
}

// TaskGroupReconcileOperatingSystemConfig is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileOperatingSystemConfig flow.TaskID = "TaskGroupReconcileOperatingSystemConfig"

// ReconcileOperatingSystemConfigTaskGroup returns the flow.TaskGroup for deploying the OperatingSystemConfig extension
// resource and waiting for its readiness.
func (b *Botanist) ReconcileOperatingSystemConfigTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileOperatingSystemConfig).WithDependencies(
			TaskGroupInitializeSecretsManagement,
			TaskGroupDeployCloudProviderSecret,
			TaskGroupReconcileGardenerResourceManager,
		)

		deployOperatingSystemConfig = g.Add(flow.Task{
			Name: "Deploying OperatingSystemConfig for control plane machines",
			Fn:   b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy,
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until OperatingSystemConfig for control plane machines has been reconciled",
			Fn:           b.Shoot.Components.Extensions.OperatingSystemConfig.Wait,
			Dependencies: flow.NewTaskIDs(deployOperatingSystemConfig),
		})
	)

	return g
}

// TaskGroupReconcileWorker is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileWorker flow.TaskID = "TaskGroupReconcileWorker"

// ReconcileWorkerTaskGroup returns the flow.TaskGroup for deploying the Worker extension
// resource and waiting for its readiness.
func (b *Botanist) ReconcileWorkerTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileWorker).WithDependencies(
			TaskGroupInitializeSecretsManagement,
			TaskGroupDeployCloudProviderSecret,
			TaskGroupReconcileGardenerResourceManager,
			TaskGroupReconcileInfrastructure,
			TaskGroupReconcileMachineControllerManager,
		)

		deployWorker = g.Add(flow.Task{
			Name:   "Deploying control plane machines",
			Fn:     b.DeployWorker,
			SkipIf: !b.Shoot.HasManagedInfrastructure(),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until control plane machines have been deployed",
			Fn:           b.Shoot.Components.Extensions.Worker.Wait,
			SkipIf:       !b.Shoot.HasManagedInfrastructure(),
			Dependencies: flow.NewTaskIDs(deployWorker),
		})
	)

	return g
}

// TaskGroupReconcileShootNamespaces is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileShootNamespaces flow.TaskID = "TaskGroupReconcileShootNamespaces"

// ReconcileShootNamespacesTaskGroup returns the flow.TaskGroup for deploying the shoot namespaces and waiting for their
// readiness.
func (b *Botanist) ReconcileShootNamespacesTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileShootNamespaces).WithDependencies(TaskGroupReconcileGardenerResourceManager)

		deployShootNamespaces = g.Add(flow.Task{
			Name: "Deploying shoot namespaces system component",
			Fn:   b.Shoot.Components.SystemComponents.Namespaces.Deploy,
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespaces have been reconciled",
			Fn:           b.Shoot.Components.SystemComponents.Namespaces.Wait,
			Dependencies: flow.NewTaskIDs(deployShootNamespaces),
		})
	)

	return g
}

// TaskGroupReconcileSystemComponents is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileSystemComponents flow.TaskID = "TaskGroupReconcileSystemComponents"

// ReconcileSystemComponentsTaskGroup returns the flow.TaskGroup for reconciling shoot system components.
func (b *Botanist) ReconcileSystemComponentsTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileSystemComponents).WithDependencies(
			TaskGroupReconcileInfrastructure,
			TaskGroupReconcileShootNamespaces,
		)

		_ = g.Add(flow.Task{
			Name:   "Deploying kube-proxy system component",
			Fn:     b.DeployKubeProxy,
			SkipIf: !v1beta1helper.KubeProxyEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy),
		})
		deployNetwork = g.Add(flow.Task{
			Name: "Deploying shoot network plugin",
			Fn:   b.DeployNetwork,
		})
		waitUntilNetworkReady = g.Add(flow.Task{
			Name:         "Waiting until shoot network plugin has been reconciled",
			Fn:           b.Shoot.Components.Extensions.Network.Wait,
			Dependencies: flow.NewTaskIDs(deployNetwork),
		})
		deployCoreDNS = g.Add(flow.Task{
			Name:         "Deploying CoreDNS system component",
			Fn:           b.DeployCoreDNS,
			Dependencies: flow.NewTaskIDs(waitUntilNetworkReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until CoreDNS system component is ready",
			Fn:           b.Shoot.Components.SystemComponents.CoreDNS.Wait,
			Dependencies: flow.NewTaskIDs(deployCoreDNS),
		})
	)

	return g
}
