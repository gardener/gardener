// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

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
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// TaskGroupDeployNamespaces is a flow.TaskID for a logical flow.TaskGroup.
const TaskGroupDeployNamespaces flow.TaskID = "TaskGroupDeployNamespaces"

// DeployNamespacesTaskGroup returns the flow.TaskGroup for deploying the control plane namespace and the garden
// namespace. The garden namespace is only deployed for self-hosted shoots.
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
