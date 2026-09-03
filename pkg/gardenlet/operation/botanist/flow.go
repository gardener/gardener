// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultInterval = 5 * time.Second
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
		SkipIf: b.Shoot.Credentials == nil || b.Shoot.IsWorkerless,
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

	return flow.NewTaskGroup(TaskGroupReconcileCustomResourceDefinitions, tasks...).SkipIf(!b.Shoot.IsSelfHosted())
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
	}).WithDependencies(TaskGroupReconcileCustomResourceDefinitions).SkipIf(!b.Shoot.IsSelfHosted())
}

// TaskGroupReconcileReferencedResources is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileReferencedResources flow.TaskID = "TaskGroupReconcileReferencedResources"

// ReconcileReferencedResourcesTaskGroup returns the flow.TaskGroup for reconciling the resources referenced in the
// Shoot specification.
func (b *Botanist) ReconcileReferencedResourcesTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupReconcileReferencedResources, flow.Task{
		Name: "Reconciling referenced resources",
		Fn:   flow.TaskFn(b.DeployReferencedResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
	}).WithDependencies(TaskGroupDeployNamespaces, TaskGroupReconcileRuntimeGardenerResourceManager)
}

// TaskGroupInitializeSecretsManagement is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupInitializeSecretsManagement flow.TaskID = "TaskGroupInitializeSecretsManagement"

// InitializeSecretsManagementTaskGroup returns the flow.TaskGroup for initializing the secret management.
func (b *Botanist) InitializeSecretsManagementTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(TaskGroupInitializeSecretsManagement, flow.Task{
		Name: "Initializing secrets management",
		Fn:   b.InitializeSecretsManagement,
	}).WithDependencies(TaskGroupReconcileClusterResource)
}

// TaskGroupReconcileRuntimeGardenerResourceManager is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileRuntimeGardenerResourceManager flow.TaskID = "TaskGroupReconcileRuntimeGardenerResourceManager"

// ReconcileRuntimeGardenerResourceManagerTaskGroup returns the flow.TaskGroup for deploying the runtime
// gardener-resource-manager instances.
func (b *Botanist) ReconcileRuntimeGardenerResourceManagerTaskGroup(podNetworkAvailable, shootIsGarden, skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileRuntimeGardenerResourceManager).
			SkipIf(!b.Shoot.IsSelfHosted() || shootIsGarden || b.isGardenadmBootstrap()).
			WithDependencies(
				TaskGroupDeployNamespaces,
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileCustomResourceDefinitions,
			)

		deployGardenerResourceManager = g.Add(flow.Task{
			Name: "Deploying runtime gardener-resource-manager",
			Fn: func(ctx context.Context) error {
				if b.Shoot.IsSelfHosted() {
					b.Shoot.Components.ControlPlane.RuntimeResourceManager.SetBootstrapControlPlaneNode(!podNetworkAvailable)
				}
				return b.DeployRuntimeGardenerResourceManager(ctx)
			},
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until runtime gardener-resource-manager reports readiness",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.ControlPlane.RuntimeResourceManager.Wait(ctx)
			},
			SkipIf:       b.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
	)

	return g
}

// TaskGroupReconcileGardenerResourceManager is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileGardenerResourceManager flow.TaskID = "TaskGroupReconcileGardenerResourceManager"

// ReconcileGardenerResourceManagerTaskGroup returns the flow.TaskGroup for deploying the gardener-resource-manager
// instances.
func (b *Botanist) ReconcileGardenerResourceManagerTaskGroup(podNetworkAvailable, skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileGardenerResourceManager).
			WithDependencies(
				TaskGroupDeployNamespaces,
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileCustomResourceDefinitions,
				TaskGroupReconcileRuntimeGardenerResourceManager,
			)

		deployGardenerResourceManager = g.Add(flow.Task{
			Name: "Deploying gardener-resource-manager",
			Fn: func(ctx context.Context) error {
				if b.Shoot.IsSelfHosted() {
					b.Shoot.Components.ControlPlane.ResourceManager.SetBootstrapControlPlaneNode(!podNetworkAvailable)
				}
				return b.DeployGardenerResourceManager(ctx)
			},
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until gardener-resource-manager reports readiness",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.ControlPlane.ResourceManager.Wait(ctx)
			},
			SkipIf:       b.Shoot.HibernationEnabled || skipReadiness,
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
		g = flow.NewTaskGroup(TaskGroupReconcileSystemResources).
			WithDependencies(
				TaskGroupReconcileGardenerResourceManager,
				TaskGroupReconcileReferencedResources,
			)

		_ = g.Add(flow.Task{
			Name: "Deploying seed system resources",
			Fn: func(ctx context.Context) error {
				return seedsystem.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, seedsystem.Values{ManagePriorityClasses: true}).Deploy(ctx)
			},
			SkipIf: !b.Shoot.IsSelfHosted(),
		})
		_ = g.Add(flow.Task{
			Name:   "Deploying shoot system resources",
			Fn:     b.DeployShootSystem,
			SkipIf: b.isGardenadmBootstrap() || b.Shoot.HibernationEnabled,
		})
	)

	return g
}

// TaskGroupReconcileMachineControllerManager is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileMachineControllerManager flow.TaskID = "TaskGroupReconcileMachineControllerManager"

// ReconcileMachineControllerManagerTaskGroup returns the flow.TaskGroup for deploying the machine-controller-manager.
func (b *Botanist) ReconcileMachineControllerManagerTaskGroup() flow.TaskGroup {
	return flow.
		NewTaskGroup(TaskGroupReconcileMachineControllerManager, flow.Task{
			Name:   "Deploying machine-controller-manager",
			Fn:     flow.TaskFn(b.DeployMachineControllerManager).RetryUntilTimeout(time.Second, time.Minute),
			SkipIf: !b.Shoot.HasManagedInfrastructure() || b.Shoot.IsWorkerless,
		}).
		WithDependencies(
			TaskGroupInitializeSecretsManagement,
			TaskGroupDeployCloudProviderSecret,
			TaskGroupReconcileGardenerResourceManager,
		).
		WithDependenciesIf(b.isGardenlet(),
			TaskGroupReconcileInfrastructure,
			TaskGroupReconcileOperatingSystemConfig,
			TaskGroupReconcileStaticPods,
		)
}

// TaskGroupReconcileBackupResources is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileBackupResources flow.TaskID = "TaskGroupReconcileBackupResources"

// ReconcileBackupResourcesTaskGroup returns the flow.TaskGroup for deploying the BackupBucket or BackupEntry extension
// resources and waiting for their readiness.
func (b *Botanist) ReconcileBackupResourcesTaskGroup(allowBackup, isCopyOfBackupsRequired, skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileBackupResources).
			WithDependencies(
				TaskGroupDeployNamespaces,
				TaskGroupReconcileReferencedResources,
			)

		deploySourceBackupEntry = g.Add(flow.Task{
			Name:   "Deploying source backup entry",
			Fn:     b.DeploySourceBackupEntry,
			SkipIf: !isCopyOfBackupsRequired || b.Shoot.IsSelfHosted(),
		})
		waitUntilSourceBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the source backup entry has been reconciled",
			Fn:           b.Shoot.Components.SourceBackupEntry.Wait,
			SkipIf:       skipReadiness || !isCopyOfBackupsRequired || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deploySourceBackupEntry),
		})

		deployBackupBucketInGarden = g.Add(flow.Task{
			Name: "Deploying BackupBucket for ETCD data",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.BackupBucket.Deploy(ctx)
			},
			SkipIf: !allowBackup || !b.Shoot.IsSelfHosted(),
		})
		waitUntilBackupBucketInGardenReconciled = g.Add(flow.Task{
			Name: "Waiting until the BackupBucket for ETCD data has been reconciled",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.BackupBucket.Wait(ctx)
			},
			SkipIf:       skipReadiness || !allowBackup || !b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployBackupBucketInGarden),
		})

		deployBackupEntryInGarden = g.Add(flow.Task{
			Name:         "Deploying BackupEntry for ETCD data",
			Fn:           b.DeployBackupEntry,
			SkipIf:       !allowBackup,
			Dependencies: flow.NewTaskIDs(waitUntilSourceBackupEntryInGardenReconciled, waitUntilBackupBucketInGardenReconciled),
		})
		waitUntilBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the BackupEntry for ETCD data has been reconciled",
			Fn:           b.Shoot.Components.BackupEntry.Wait,
			SkipIf:       skipReadiness || !allowBackup,
			Dependencies: flow.NewTaskIDs(deployBackupEntryInGarden),
		})

		copyEtcdBackups = g.Add(flow.Task{
			Name:         "Copying etcd backups to new seed's backup bucket",
			Fn:           b.DeployEtcdCopyBackupsTask,
			SkipIf:       !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryInGardenReconciled, waitUntilSourceBackupEntryInGardenReconciled),
		})
		waitUntilEtcdBackupsCopied = g.Add(flow.Task{
			Name:         "Waiting until etcd backups are copied",
			Fn:           b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Wait,
			SkipIf:       skipReadiness || !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(copyEtcdBackups),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying copy etcd backups task resource",
			Fn:           b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Destroy,
			SkipIf:       !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(waitUntilEtcdBackupsCopied),
		})
	)

	return g
}

// TaskGroupReconcileInfrastructure is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileInfrastructure flow.TaskID = "TaskGroupReconcileInfrastructure"

// ReconcileInfrastructureTaskGroup returns the flow.TaskGroup for deploying the Infrastructure extension resource and
// waiting for its readiness.
func (b *Botanist) ReconcileInfrastructureTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileInfrastructure).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupDeployCloudProviderSecret,
				TaskGroupReconcileReferencedResources,
			)

		deployInfrastructure = g.Add(flow.Task{
			Name:   "Deploying Shoot infrastructure",
			Fn:     b.DeployInfrastructure,
			SkipIf: !b.Shoot.HasManagedInfrastructure() || b.Shoot.IsWorkerless,
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until Shoot infrastructure has been reconciled",
			Fn: func(ctx context.Context) error {
				if !skipReadiness {
					if err := b.WaitForInfrastructure(ctx); err != nil {
						return err
					}
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskDeployInfrastructure)
			},
			SkipIf:       !b.Shoot.HasManagedInfrastructure() || b.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})
	)

	return g
}

// TaskGroupReconcileControlPlane is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileControlPlane flow.TaskID = "TaskGroupReconcileControlPlane"

// ReconcileControlPlaneTaskGroup returns the flow.TaskGroup for deploying the ControlPlane extension resource and
// waiting for its readiness.
func (b *Botanist) ReconcileControlPlaneTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileControlPlane).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupDeployCloudProviderSecret,
				TaskGroupReconcileGardenerResourceManager,
				TaskGroupReconcileInfrastructure,
				TaskGroupReconcileReferencedResources,
			).
			WithDependenciesIf(b.isGardenlet(),
				TaskGroupReconcileStaticPods,
			)

		deployControlPlane = g.Add(flow.Task{
			Name:   "Deploying shoot control plane components",
			Fn:     b.DeployControlPlane,
			SkipIf: b.Shoot.IsWorkerless,
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until shoot control plane has been reconciled",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.ControlPlane.Wait(ctx)
			},
			SkipIf:       b.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
	)

	return g
}

// TaskGroupReconcileDNSRecords is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileDNSRecords flow.TaskID = "TaskGroupReconcileDNSRecords"

// ReconcileDNSRecordsTaskGroup returns the flow.TaskGroup for deploying the DNSRecords extension resources
// and waiting for their readiness.
func (b *Botanist) ReconcileDNSRecordsTaskGroup() flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileDNSRecords).
			SkipIf(b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted()).
			WithDependencies(
				TaskGroupReconcileReferencedResources,
			)

		_ = g.Add(flow.Task{
			Name: "Deploying internal domain DNS record",
			Fn: func(ctx context.Context) error {
				if err := b.DeployOrDestroyInternalDNSRecord(ctx); err != nil {
					return err
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskDeployDNSRecordInternal)
			},
		})
		_ = g.Add(flow.Task{
			Name: "Deploying external domain DNS record",
			Fn: func(ctx context.Context) error {
				if err := b.DeployOrDestroyExternalDNSRecord(ctx); err != nil {
					return err
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskDeployDNSRecordExternal)
			},
		})
	)

	return g
}

// TaskGroupReconcileExtensionsBeforeKubeAPIServer is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileExtensionsBeforeKubeAPIServer flow.TaskID = "TaskGroupReconcileExtensionsBeforeKubeAPIServer"

// ReconcileExtensionsBeforeKubeAPIServerTaskGroup returns the flow.TaskGroup for deploying the Extensions resources
// before kube-apiserver and waiting for their readiness.
func (b *Botanist) ReconcileExtensionsBeforeKubeAPIServerTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileExtensionsBeforeKubeAPIServer).
			SkipIf(b.Shoot.HibernationEnabled).
			WithDependencies(
				TaskGroupDeployCloudProviderSecret,
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileReferencedResources,
				TaskGroupReconcileInfrastructure,
			)

		deployExtensionResources = g.Add(flow.Task{
			Name: "Deploying extension resources before kube-apiserver",
			Fn:   flow.TaskFn(b.DeployExtensionsBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources handled before kube-apiserver are ready",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer,
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(deployExtensionResources),
		})
	)

	return g
}

// TaskGroupReconcileExtensionsAfterKubeAPIServer is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileExtensionsAfterKubeAPIServer flow.TaskID = "TaskGroupReconcileExtensionsAfterKubeAPIServer"

// ReconcileExtensionsAfterKubeAPIServerTaskGroup returns the flow.TaskGroup for deploying the Extensions resources
// after kube-apiserver and waiting for their readiness.
func (b *Botanist) ReconcileExtensionsAfterKubeAPIServerTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		deployExtensionAfterKAPIMsg = "Deploying extension resources after kube-apiserver"
		waitExtensionAfterKAPIMsg   = "Waiting until extension resources handled after kube-apiserver are ready"
	)
	if b.Shoot.HibernationEnabled {
		deployExtensionAfterKAPIMsg = "Hibernating extension resources before kube-apiserver hibernation"
		waitExtensionAfterKAPIMsg = "Waiting until extension resources hibernated before kube-apiserver hibernation are ready"
	}

	var (
		g = flow.NewTaskGroup(TaskGroupReconcileExtensionsAfterKubeAPIServer).
			WithDependencies(
				TaskGroupDeployCloudProviderSecret,
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileReferencedResources,
				TaskGroupReconcileInfrastructure,
			).
			WithDependenciesIf(b.Shoot.IsSelfHosted(),
				TaskGroupReconcileStaticPods,
			)

		deployExtensionResources = g.Add(flow.Task{
			Name: deployExtensionAfterKAPIMsg,
			Fn:   flow.TaskFn(b.DeployExtensionsAfterKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name:         waitExtensionAfterKAPIMsg,
			Fn:           b.Shoot.Components.Extensions.Extension.WaitAfterKubeAPIServer,
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(deployExtensionResources),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name: "Deleting stale extension resources",
			Fn:   flow.TaskFn(b.Shoot.Components.Extensions.Extension.DeleteStaleResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until stale extension resources are deleted",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources,
			SkipIf:       b.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
	)

	return g
}

// TaskGroupReconcileExtensionsAfterWorker is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileExtensionsAfterWorker flow.TaskID = "TaskGroupReconcileExtensionsAfterWorker"

// ReconcileExtensionsAfterWorkerTaskGroup returns the flow.TaskGroup for deploying the Extensions resources
// after the Worker resource and waiting for their readiness.
func (b *Botanist) ReconcileExtensionsAfterWorkerTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileExtensionsAfterWorker).
			SkipIf(b.Shoot.IsWorkerless).
			WithDependencies(
				TaskGroupDeployCloudProviderSecret,
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileReferencedResources,
				TaskGroupReconcileWorker,
			)

		deployExtensionResources = g.Add(flow.Task{
			Name: "Deploying extension resources after workers",
			Fn:   b.DeployExtensionsAfterWorker,
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources handled after workers are ready",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitAfterWorker,
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(deployExtensionResources),
		})
	)

	return g
}

// TaskGroupReconcileContainerRuntime is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileContainerRuntime flow.TaskID = "TaskGroupReconcileContainerRuntime"

// ReconcileContainerRuntimeTaskGroup returns the flow.TaskGroup for deploying the ContainerRuntime extension resource
// and waiting for its readiness.
func (b *Botanist) ReconcileContainerRuntimeTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileContainerRuntime).
			SkipIf(b.Shoot.IsWorkerless).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileReferencedResources,
				TaskGroupReconcileGardenerResourceManager,
			)

		deployContainerRuntimeResources = g.Add(flow.Task{
			Name: "Deploying container runtime resources",
			Fn:   flow.TaskFn(b.DeployContainerRuntime).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until container runtime resources are ready",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.ContainerRuntime.Wait(ctx)
			},
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(deployContainerRuntimeResources),
		})
		deleteStaleContainerRuntimeResources = g.Add(flow.Task{
			Name: "Deleting stale container runtime resources",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.ContainerRuntime.DeleteStaleResources(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until stale container runtime resources are deleted",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.ContainerRuntime.WaitCleanupStaleResources(ctx)
			},
			SkipIf:       b.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deleteStaleContainerRuntimeResources),
		})
	)

	return g
}

// TaskGroupReconcileOperatingSystemConfig is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileOperatingSystemConfig flow.TaskID = "TaskGroupReconcileOperatingSystemConfig"

// ReconcileOperatingSystemConfigTaskGroup returns the flow.TaskGroup for deploying the OperatingSystemConfig extension
// resource and waiting for its readiness.
func (b *Botanist) ReconcileOperatingSystemConfigTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.
			NewTaskGroup(TaskGroupReconcileOperatingSystemConfig).
			SkipIf(b.Shoot.IsSelfHosted() && !b.isGardenadmBootstrap()).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupDeployCloudProviderSecret,
				TaskGroupReconcileGardenerResourceManager,
				TaskGroupReconcileReferencedResources,
			).
			WithDependenciesIf(b.isGardenlet(),
				TaskGroupReconcileInfrastructure,
				TaskGroupReconcileControlPlane,
				TaskGroupReconcileExtensionsAfterKubeAPIServer,
			)

		deployOperatingSystemConfig = g.Add(flow.Task{
			Name: "Deploying OperatingSystemConfig resources for worker pools",
			Fn: func(ctx context.Context) error {
				if b.isGardenadmBootstrap() {
					return b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy(ctx)
				}
				return b.DeployOperatingSystemConfig(ctx)
			},
			SkipIf: b.Shoot.IsWorkerless,
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until OperatingSystemConfig for worker pools have been reconciled",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.OperatingSystemConfig.Wait(ctx)
			},
			SkipIf:       b.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployOperatingSystemConfig),
		})
		deleteStaleOperatingSystemConfigResources = g.Add(flow.Task{
			Name: "Delete stale OperatingSystemConfig resources",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.OperatingSystemConfig.DeleteStaleResources(ctx)
			},
			SkipIf:       b.Shoot.IsWorkerless || b.isGardenadmBootstrap(),
			Dependencies: flow.NewTaskIDs(deployOperatingSystemConfig),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until stale OperatingSystemConfig resources are deleted",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.OperatingSystemConfig.WaitCleanupStaleResources(ctx)
			},
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || skipReadiness || b.isGardenadmBootstrap(),
			Dependencies: flow.NewTaskIDs(deleteStaleOperatingSystemConfigResources),
		})
	)

	return g
}

// TaskGroupReconcileWorker is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileWorker flow.TaskID = "TaskGroupReconcileWorker"

// ReconcileWorkerTaskGroup returns the flow.TaskGroup for deploying the Worker extension resource. It waits until its
// status was updated with the latest machine deployments, deploys cluster-autoscaler and finally waits for the pools
// to get reconciled.
func (b *Botanist) ReconcileWorkerTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileWorker).
			SkipIf(b.Shoot.IsWorkerless || !b.Shoot.HasManagedInfrastructure()).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupDeployCloudProviderSecret,
				TaskGroupReconcileGardenerResourceManager,
				TaskGroupReconcileInfrastructure,
				TaskGroupReconcileMachineControllerManager,
				TaskGroupReconcileSystemResources,
			).
			WithDependenciesIf(b.isGardenlet(),
				TaskGroupReconcileOperatingSystemConfig,
			)

		shootHasPendingInPlaceUpdateWorkers = func(shoot *gardencorev1beta1.Shoot) bool {
			return shoot.Status.InPlaceUpdates != nil && shoot.Status.InPlaceUpdates.PendingWorkerUpdates != nil &&
				(len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate) > 0 || len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate) > 0)
		}

		deployWorker = g.Add(flow.Task{
			Name: "Configuring worker pools",
			Fn:   b.DeployWorker,
		})
		waitUntilWorkerStatusUpdate = g.Add(flow.Task{
			Name: "Waiting until worker resource status is updated with latest machine deployments",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.Worker.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)
			},
			SkipIf:       b.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployWorker),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying cluster-autoscaler",
			Fn:           b.DeployClusterAutoscaler,
			SkipIf:       b.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerStatusUpdate),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until worker pools have been reconciled",
			Fn: func(ctx context.Context) error {
				if err := b.Shoot.Components.Extensions.Worker.Wait(ctx); err != nil {
					return err
				}

				// If the worker is ready, all the AutoInPlaceUpdate worker pools should be updated already, so we can remove them from the status.
				if shootHasPendingInPlaceUpdateWorkers(b.Shoot.GetInfo()) {
					// gardenlet's shoot status reconciler might concurrently update the status, so we need to use optimistic locking.
					if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, false, true, func(shoot *gardencorev1beta1.Shoot) error {
						shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate = nil

						if len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate) == 0 {
							shoot.Status.InPlaceUpdates.PendingWorkerUpdates = nil
						}

						if shoot.Status.InPlaceUpdates.PendingWorkerUpdates == nil {
							shoot.Status.InPlaceUpdates = nil
						}

						return nil
					}); err != nil {
						return fmt.Errorf("failed to remove pending AutoInPlaceUpdate worker pools from status: %w", err)
					}
				}

				// If there are no pending workers rollouts for in-place updates, we can remove the force in-place update annotation.
				if (b.Shoot.GetInfo().Status.InPlaceUpdates == nil || b.Shoot.GetInfo().Status.InPlaceUpdates.PendingWorkerUpdates == nil) &&
					kubernetesutils.HasMetaDataAnnotation(b.Shoot.GetInfo(), v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationForceInPlaceUpdate) {
					return b.Shoot.UpdateInfo(ctx, b.GardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
						delete(shoot.Annotations, v1beta1constants.GardenerOperation)
						return nil
					})
				}

				return nil
			},
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerStatusUpdate),
		})
	)

	return g
}

// TaskGroupReconcileShootNamespaces is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileShootNamespaces flow.TaskID = "TaskGroupReconcileShootNamespaces"

// ReconcileShootNamespacesTaskGroup returns the flow.TaskGroup for deploying the shoot namespaces and waiting for their
// readiness.
func (b *Botanist) ReconcileShootNamespacesTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileShootNamespaces).
			WithDependencies(
				TaskGroupReconcileGardenerResourceManager,
			)

		deployShootNamespaces = g.Add(flow.Task{
			Name: "Deploying shoot namespaces system component",
			Fn:   b.Shoot.Components.SystemComponents.Namespaces.Deploy,
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespaces have been reconciled",
			Fn:           b.Shoot.Components.SystemComponents.Namespaces.Wait,
			SkipIf:       b.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployShootNamespaces),
		})
	)

	return g
}

// TaskGroupReconcileSystemComponents is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileSystemComponents flow.TaskID = "TaskGroupReconcileSystemComponents"

// ReconcileSystemComponentsTaskGroup returns the flow.TaskGroup for reconciling shoot system components.
func (b *Botanist) ReconcileSystemComponentsTaskGroup(kubeProxyEnabled, skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileSystemComponents).
			WithDependencies(
				TaskGroupReconcileInfrastructure,
				TaskGroupReconcileGardenerResourceManager,
				TaskGroupReconcileShootNamespaces,
			).
			WithDependenciesIf(b.isGardenlet(),
				TaskGroupReconcileControlPlane,
				TaskGroupReconcileExtensionsAfterKubeAPIServer, // Extensions might deploy webhooks for system components
			)

		deployNetwork = g.Add(flow.Task{
			Name:   "Deploying shoot network plugin",
			Fn:     b.DeployNetwork,
			SkipIf: b.Shoot.IsWorkerless,
		})
		waitUntilNetworkReady = g.Add(flow.Task{
			Name: "Waiting until shoot network plugin has been reconciled",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.Extensions.Network.Wait(ctx)
			},
			SkipIf:       b.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployNetwork),
		})

		deployKubeProxy = g.Add(flow.Task{
			Name:   "Deploying kube-proxy system component",
			Fn:     b.DeployKubeProxy,
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || !kubeProxyEnabled,
		})
		_ = g.Add(flow.Task{
			Name: "Deleting stale kube-proxy DaemonSets",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.KubeProxy.DeleteStaleResources(ctx)
			},
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || !kubeProxyEnabled,
			Dependencies: flow.NewTaskIDs(deployKubeProxy),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting kube-proxy system component",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.KubeProxy.Destroy(ctx)
			},
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || kubeProxyEnabled,
		})

		_ = g.Add(flow.Task{
			Name:         "Check CoreDNS migration status",
			Fn:           b.CheckDNSServiceMigration,
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilNetworkReady),
		})
		deployCoreDNS = g.Add(flow.Task{
			Name: "Deploying CoreDNS system component",
			Fn: func(ctx context.Context) error {
				if err := b.DeployCoreDNS(ctx); err != nil {
					return err
				}
				if controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskRestartCoreAddons) {
					return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskRestartCoreAddons)
				}
				return nil
			},
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilNetworkReady),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until CoreDNS system component is ready",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.CoreDNS.Wait(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployCoreDNS),
			SkipIf:       !b.Shoot.IsSelfHosted(),
		})

		_ = g.Add(flow.Task{
			Name:         "Reconcile node-local-dns system component",
			Fn:           b.ReconcileNodeLocalDNS,
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilNetworkReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying shoot cluster identity",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.DeployClusterIdentity(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.HibernationEnabled,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying metrics-server system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.MetricsServer.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying node-problem-detector system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.NodeProblemDetector.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled,
		})
		_ = g.Add(flow.Task{
			Name:   "Deploying blackbox-exporter",
			Fn:     flow.TaskFn(b.ReconcileBlackboxExporterCluster).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
		})
		_ = g.Add(flow.Task{
			Name:   "Deploying node-exporter",
			Fn:     flow.TaskFn(b.ReconcileNodeExporter).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled,
		})
		_ = g.Add(flow.Task{
			Name:   "Deploying addon Kubernetes Dashboard",
			Fn:     flow.TaskFn(b.DeployKubernetesDashboard).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
		})
		_ = g.Add(flow.Task{
			Name:   "Deploying addon Nginx Ingress Controller",
			Fn:     flow.TaskFn(b.DeployNginxIngressAddon).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
		})
	)

	return g
}

// TaskGroupReconcileVPNComponents is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileVPNComponents flow.TaskID = "TaskGroupReconcileVPNComponents"

// ReconcileVPNComponentsTaskGroup returns the flow.TaskGroup for reconciling VPN components.
func (b *Botanist) ReconcileVPNComponentsTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileVPNComponents).
			SkipIf(b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted()).
			WithDependencies(
				TaskGroupDeployNamespaces,
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileGardenerResourceManager,
			)

		deployVPNSeedServer = g.Add(flow.Task{
			Name: "Deploying vpn-seed-server",
			Fn:   flow.TaskFn(b.DeployVPNServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying apiserver-proxy",
			Fn:   flow.TaskFn(b.DeployAPIServerProxy).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		deployVPNShoot = g.Add(flow.Task{
			Name:         "Deploying vpn-shoot system component",
			Fn:           flow.TaskFn(b.DeployVPNShoot).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployVPNSeedServer),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until vpn-shoot system component is ready",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.VPNShoot.Wait(ctx)
			}).Recover(flow.TaskFn(b.RecoverStaleVPNShootPods).ToRecoverFn()),
			SkipIf:       b.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployVPNShoot),
		})
	)

	return g
}

// TaskGroupReconcileETCDs is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileETCDs flow.TaskID = "TaskGroupReconcileETCDs"

// ReconcileETCDsTaskGroup returns the flow.TaskGroup for deploying etcd-druid, the ETCDs resources, and waiting for
// their readiness.
func (b *Botanist) ReconcileETCDsTaskGroup(shootIsGarden, isRestoringHAControlPlane, skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileETCDs).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupDeployCloudProviderSecret,
			).
			WithDependenciesIf(b.isGardenlet(),
				TaskGroupReconcileBackupResources,
			)

		deployEtcdDruid = g.Add(flow.Task{
			Name: "Deploying ETCD Druid",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.ControlPlane.EtcdDruid.Deploy(ctx)
			},
			SkipIf: shootIsGarden || !b.Shoot.IsSelfHosted(),
		})
		configureEtcd = g.Add(flow.Task{
			Name: "Configuring static pod control plane node IP addresses for ETCDs",
			Fn: func(ctx context.Context) error {
				nodes, err := b.ListControlPlaneNodes(ctx)
				if err != nil {
					return fmt.Errorf("failed listing control plane nodes: %w", err)
				}

				ip, err := kubernetesutils.NodeInternalIP(nodes[0], b.Shoot.PreferIPv6())
				if err != nil {
					return fmt.Errorf("failed determining IP address of first control plane node: %w", err)
				}

				b.Shoot.Components.ControlPlane.EtcdMain.SetStaticPodControlPlaneNodesIPAddresses(ip)
				b.Shoot.Components.ControlPlane.EtcdEvents.SetStaticPodControlPlaneNodesIPAddresses(ip)
				return nil
			},
			SkipIf: !b.Shoot.IsSelfHosted(),
		})
		deployEtcds = g.Add(flow.Task{
			Name:         "Deploying main and events ETCDs",
			Fn:           flow.TaskFn(b.DeployEtcd).RetryUntilTimeout(5*time.Second, helper.GetEtcdDeployTimeout(b.Shoot, 30*time.Second)),
			Dependencies: flow.NewTaskIDs(deployEtcdDruid, configureEtcd),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until main and event ETCDs have been reconciled",
			Fn:           b.WaitUntilEtcdsReady,
			SkipIf:       (!isRestoringHAControlPlane && b.Shoot.HibernationEnabled) || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployEtcds),
		})
	)

	return g
}

// TaskGroupReconcileStaticPods is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupReconcileStaticPods flow.TaskID = "TaskGroupReconcileStaticPods"

// ReconcileStaticControlPlanePodsTaskGroup returns the flow.TaskGroup for deploying the static control plane
// deployments to the cluster (with replicas=0). It then translates them into static pod manifests, adds them to the
// OperatingSystemConfig, updates the ManagedResource containing the gardener-node-agent OSC Secret, and waits for the
// changes to be rolled out.
func (b *Botanist) ReconcileStaticControlPlanePodsTaskGroup(useBootstrapEtcd bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupReconcileStaticPods).
			SkipIf(!b.Shoot.IsSelfHosted()).
			WithDependencies(
				TaskGroupInitializeSecretsManagement,
				TaskGroupReconcileGardenerResourceManager,
				TaskGroupReconcileETCDs,
			).
			WithDependenciesIf(b.isGardenlet(),
				TaskGroupReconcileExtensionsBeforeKubeAPIServer,
			)

		_ = g.Add(flow.Task{
			Name: "Reconciling cluster-admin access resources for kubeconfig on control plane nodes",
			Fn: func(ctx context.Context) error {
				return component.OpWait(b.Shoot.Components.AdminAccess).Deploy(ctx)
			},
		})
		deployControlPlaneDeployments = g.Add(flow.Task{
			Name: "Deploying control plane components as Deployments/StatefulSets and updating gardener-node-agent Secret",
			Fn: func(ctx context.Context) error {
				return b.DeployStaticControlPlaneDeployments(ctx, useBootstrapEtcd)
			},
		})
		// 'gardenadm init' creates the OperatingSystemConfig and gardener-node-agent secrets based on the Shoot
		// manifest on the local filesystem (manually crafted). However, after 'gardenadm connect', the Shoot gets
		// registered with the gardener-apiserver, which runs additional mutations via admission plugins that might
		// change the Shoot spec in a way that affects the computation of the OSC/GNA secret names. Hence, when
		// gardenlet reconciles the first time after 'gardenadm init', we have to update the respective label on the
		// nodes to make sure that gardener-node-agent uses the correct secrets going forward.
		updateNodeAgentSecretNameLabels = g.Add(flow.Task{
			Name: fmt.Sprintf("Updating %s label on nodes after 'gardenadm init' to reflect computations of gardenlet", v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName),
			Fn: func(ctx context.Context) error {
				if err := b.UpdateNodeAgentSecretNameLabelsOnNodes(ctx); err != nil {
					return fmt.Errorf("error updating %s labels on nodes: %w", v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, err)
				}
				if err := b.Shoot.Components.Extensions.OperatingSystemConfig.DeleteStaleResources(ctx); err != nil {
					return fmt.Errorf("error deleting stale OperatingSystemConfig resources: %w", err)
				}
				if err := b.Shoot.Components.Extensions.OperatingSystemConfig.WaitCleanupStaleResources(ctx); err != nil {
					return fmt.Errorf("error waiting until stale OperatingSystemConfig resources have been deleted: %w", err)
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskUpdateGardenerNodeAgentSecretName)
			},
			SkipIf:       !controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskUpdateGardenerNodeAgentSecretName),
			Dependencies: flow.NewTaskIDs(deployControlPlaneDeployments),
		})
		waitUntilControlPlaneComponentsRolledOut = g.Add(flow.Task{
			Name: "Waiting until control plane components (static pods) are rolled out and ready",
			Fn: func(ctx context.Context) error {
				return b.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, b.isGardenlet(), true)
			},
			Dependencies: flow.NewTaskIDs(updateNodeAgentSecretNameLabels),
		})
	)

	b.handleCredentialsRotationForStaticPods(&g, waitUntilControlPlaneComponentsRolledOut)
	return g
}

func (b *Botanist) handleCredentialsRotationForStaticPods(g *flow.TaskGroup, readyForSecondRollout flow.TaskIDer) {
	var (
		rotationPreparingPhases            = sets.New(gardencorev1beta1.RotationPreparing, gardencorev1beta1.RotationPreparingWithoutWorkersRollout)
		caRotationPreparing                = rotationPreparingPhases.Has(v1beta1helper.GetShootCARotationPhase(b.Shoot.GetInfo().Status.Credentials))
		etcdEncryptionKeyRotationPreparing = rotationPreparingPhases.Has(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials))

		patchKubeAPIServerDeployment = g.Add(flow.Task{
			Name: "Marking kube-apiserver deployment as ready for encryption key switch",
			Fn: func(ctx context.Context) error {
				return shared.MarkAPIServerDeploymentAsReadyForEncryptionKeySwitch(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer)
			},
			SkipIf:       !etcdEncryptionKeyRotationPreparing,
			Dependencies: flow.NewTaskIDs(readyForSecondRollout),
		})
		patchETCDs = g.Add(flow.Task{
			Name: "Marking Etcd resources as peer CA rotation completed",
			Fn: flow.Parallel(
				b.Shoot.Components.ControlPlane.EtcdMain.MarkAsPeerCARolloutCompleted,
				b.Shoot.Components.ControlPlane.EtcdEvents.MarkAsPeerCARolloutCompleted,
			),
			SkipIf:       !caRotationPreparing,
			Dependencies: flow.NewTaskIDs(readyForSecondRollout),
		})

		deployEtcds = g.Add(flow.Task{
			Name:         "Deploying main and events ETCDs (second rollout)",
			Fn:           flow.TaskFn(b.DeployEtcd).RetryUntilTimeout(5*time.Second, helper.GetEtcdDeployTimeout(b.Shoot, 30*time.Second)),
			SkipIf:       !caRotationPreparing,
			Dependencies: flow.NewTaskIDs(patchETCDs),
		})
		waitUntilEtcdsReady = g.Add(flow.Task{
			Name:         "Waiting until main and event ETCDs have been reconciled (second rollout)",
			Fn:           b.WaitUntilEtcdsReady,
			SkipIf:       !caRotationPreparing,
			Dependencies: flow.NewTaskIDs(deployEtcds),
		})

		deployControlPlaneDeployments = g.Add(flow.Task{
			Name: "Deploying control plane components as Deployments/StatefulSets and updating gardener-node-agent Secret (second rollout)",
			Fn: func(ctx context.Context) error {
				return b.DeployStaticControlPlaneDeployments(ctx, false)
			},
			SkipIf:       !etcdEncryptionKeyRotationPreparing && !caRotationPreparing,
			Dependencies: flow.NewTaskIDs(patchKubeAPIServerDeployment, waitUntilEtcdsReady),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until control plane components (static pods) are rolled out and ready (second rollout)",
			Fn: func(ctx context.Context) error {
				return b.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, true, true)
			},
			SkipIf:       !etcdEncryptionKeyRotationPreparing && !caRotationPreparing,
			Dependencies: flow.NewTaskIDs(deployControlPlaneDeployments),
		})
	)
}

// TaskGroupHibernateControlPlane is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupHibernateControlPlane flow.TaskID = "TaskGroupHibernateControlPlane"

// HibernateControlPlaneTaskGroup hibernates the control plane of a hosted shoot.
func (b *Botanist) HibernateControlPlaneTaskGroup(skipReadiness bool) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(TaskGroupHibernateControlPlane).
			SkipIf(!b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted()).
			WithDependencies(
				TaskGroupReconcileWorker,
				TaskGroupReconcileExtensionsAfterKubeAPIServer,
			)

		hibernateControlPlane = g.Add(flow.Task{
			Name: "Hibernating control plane",
			Fn:   flow.TaskFn(b.HibernateControlPlane).RetryUntilTimeout(defaultInterval, 2*time.Minute),
		})

		// logic is inverted here
		// extensions that are deployed before the kube-apiserver are hibernated after it
		hibernateExtensionResourcesAfterKAPIHibernation = g.Add(flow.Task{
			Name:         "Hibernating extension resources after kube-apiserver hibernation",
			Fn:           flow.TaskFn(b.DeployExtensionsBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources hibernated after kube-apiserver hibernation are ready",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer,
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(hibernateExtensionResourcesAfterKAPIHibernation),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying ingress domain DNS record if hibernated",
			Fn:           b.DestroyIngressDNSRecord,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record if hibernated",
			Fn:           b.DestroyExternalDNSRecord,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record if hibernated",
			Fn:           b.DestroyInternalDNSRecord,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
	)

	return g
}

func (b *Botanist) isGardenadmBootstrap() bool {
	return b.Shoot.IsSelfHosted() && !b.Shoot.RunsControlPlane()
}

func (b *Botanist) isGardenlet() bool {
	return strings.HasPrefix(b.Shoot.GetInfo().Status.Gardener.Name, v1beta1constants.DeploymentNameGardenlet)
}
