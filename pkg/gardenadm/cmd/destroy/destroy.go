// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm"
	gardenadmbotanist "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the infrastructure of a Self-Hosted Shoot Cluster",
		Long: `Destroy the infrastructure of a Self-Hosted Shoot Cluster (machines, networks, etc.) that was created with
"gardenadm bootstrap".

Like "gardenadm bootstrap", this command deploys the provider extensions and machine-controller-manager to a
temporary bootstrap cluster. It restores the Infrastructure and Worker resources from the ShootState in the config
directory and deletes them, which deletes all machines and the infrastructure in the cloud provider account.

The config directory must contain the same manifests that were used for "gardenadm bootstrap" and the current
ShootState of the self-hosted shoot cluster.

Warning: the self-hosted shoot cluster must not be running anymore, or at least its machine-controller-manager must
be stopped. Otherwise, it recreates the machines that are deleted by this command.`,

		Example: `# Destroy the infrastructure
gardenadm destroy --config-dir /path/to/manifests`,

		Args: cobra.NoArgs,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.ParseArgs(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}

			return run(cmd.Context(), opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

// NewClientSetFromFile is an alias for botanist.NewClientSetFromFile.
// Exposed for unit testing.
var NewClientSetFromFile = gardenadmbotanist.NewClientSetFromFile

func run(ctx context.Context, opts *Options) error {
	resources, err := gardenadm.ReadManifests(opts.Log, gardenadmbotanist.DirFS(opts.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", opts.ConfigDir, err)
	}

	if !v1beta1helper.HasManagedInfrastructure(resources.Shoot) {
		return fmt.Errorf("shoot has unmanaged infrastructure, there is nothing to destroy: " +
			"use `gardenadm reset` on the nodes instead")
	}
	if resources.ShootState == nil {
		return fmt.Errorf("ShootState is missing in the config directory: " +
			"the state of the infrastructure and machines is required to destroy them")
	}

	clientSet, err := NewClientSetFromFile(opts.Kubeconfig, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	if err := cmd.EnsureNoGardenletOrOperator(ctx, clientSet.Client(), "destroy"); err != nil {
		return err
	}

	// Instruct the botanist to restore the extension resources from the ShootState, like after a control plane
	// migration. The machine objects are restored as well, so that machine-controller-manager can delete the machines.
	resources.Shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type: gardencorev1beta1.LastOperationTypeRestore,
	}

	b, err := gardenadmbotanist.NewGardenadmBotanistFromResources(ctx, opts.Log, clientSet, resources, false)
	if err != nil {
		return err
	}

	var (
		g        = flow.NewGraph("destroy")
		reporter = flow.NewCommandLineProgressReporter(opts.ErrOut)

		deployNamespaces            = g.AddGroup(b.DeployNamespacesTaskGroup())
		_                           = g.AddGroup(b.DeployCloudProviderSecretTaskGroup())
		_                           = g.AddGroup(b.ReconcileCustomResourceDefinitionsTaskGroup())
		_                           = g.AddGroup(b.ReconcileClusterResourceTaskGroup())
		initializeSecretsManagement = g.AddGroup(b.InitializeSecretsManagementTaskGroup())
		deployPriorityClassCritical = g.Add(flow.Task{
			Name:         "Deploying PriorityClass for gardener-resource-manager",
			Fn:           b.DeployPriorityClassCritical,
			Dependencies: flow.NewTaskIDs(deployNamespaces, initializeSecretsManagement),
		})
		_ = g.AddGroup(
			b.ReconcileRuntimeGardenerResourceManagerTaskGroup(true, false, false).
				WithDependencies(deployPriorityClassCritical),
		)
		_ = g.AddGroup(
			b.ReconcileGardenerResourceManagerTaskGroup(true, false).
				WithDependencies(deployPriorityClassCritical),
		)
		_                             = g.AddGroup(b.ReconcileReferencedResourcesTaskGroup())
		_                             = g.AddGroup(b.ReconcileSystemResourcesTaskGroup())
		reconcileExtensionControllers = g.AddGroup(b.ReconcileExtensionControllersTaskGroup(true))
		reconcileNetworkPolicies      = g.AddGroup(b.ReconcileNetworkPoliciesTaskGroup())
		syncPointBootstrapped         = flow.NewTaskIDs(
			reconcileNetworkPolicies,
			reconcileExtensionControllers,
		)

		restoreInfrastructure = g.AddGroup(
			b.ReconcileInfrastructureTaskGroup(false).
				WithDependencies(syncPointBootstrapped),
		)
		_ = g.AddGroup(
			b.ReconcileOperatingSystemConfigTaskGroup(false).
				WithDependencies(syncPointBootstrapped),
		)
		_ = g.AddGroup(
			b.ReconcileMachineControllerManagerTaskGroup().
				WithDependencies(syncPointBootstrapped),
		)
		restoreWorker = g.AddGroup(
			b.ReconcileWorkerTaskGroup(false).
				WithDependencies(syncPointBootstrapped, botanist.TaskGroupReconcileOperatingSystemConfig),
		)

		// machine-controller-manager runs without a target cluster in the bootstrap cluster (see
		// `gardenadm bootstrap`), so the machines are deleted without draining the nodes.
		destroyWorker = g.Add(flow.Task{
			Name: "Destroying shoot workers",
			Fn: func(ctx context.Context) error {
				if err := b.Shoot.Components.Extensions.Worker.Destroy(ctx); err != nil {
					return err
				}
				return b.Shoot.Components.Extensions.Worker.WaitCleanup(ctx)
			},
			Dependencies: flow.NewTaskIDs(restoreWorker),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting machine-controller-manager",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.ControlPlane.MachineControllerManager.Destroy(ctx)
			},
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		_ = g.Add(flow.Task{
			Name: "Destroying shoot infrastructure",
			Fn: func(ctx context.Context) error {
				if err := b.Shoot.Components.Extensions.Infrastructure.Destroy(ctx); err != nil {
					return err
				}
				return b.Shoot.Components.Extensions.Infrastructure.WaitCleanup(ctx)
			},
			Dependencies: flow.NewTaskIDs(restoreInfrastructure, destroyWorker),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              opts.Log,
		ProgressReporter: reporter,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprint(opts.Out, `
The machines and the infrastructure of the self-hosted shoot cluster have been destroyed.

Other resources are not destroyed by this command yet, e.g., the DNS record of the API server,
the load balancer exposing the API server, and resources created by workloads in the cluster
(load balancers of Services, volumes of PersistentVolumeClaims, etc.).

You can delete the bootstrap cluster now.
`)

	return nil
}
