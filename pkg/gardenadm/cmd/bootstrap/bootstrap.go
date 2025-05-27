// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the infrastructure for an Autonomous Shoot Cluster",
		Long:  "Bootstrap the infrastructure for an Autonomous Shoot Cluster (networks, machines, etc.)",

		Example: `# Bootstrap the infrastructure
gardenadm bootstrap --config-dir /path/to/manifests`,

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

// NewClientSetFromFile in alias for botanist.NewClientSetFromFile.
// Exposed for unit testing.
var NewClientSetFromFile = botanist.NewClientSetFromFile

func run(ctx context.Context, opts *Options) error {
	clientSet, err := NewClientSetFromFile(opts.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	if err := ensureNoGardenletOrOperator(ctx, clientSet.Client()); err != nil {
		return err
	}

	b, err := botanist.NewAutonomousBotanistFromManifests(ctx, opts.Log, clientSet, opts.ConfigDir, false)
	if err != nil {
		return err
	}

	// Overwrite the OSC component to initialize the control plane machines with a static dummy user data secret.
	// TODO(timebertt): replace this with a proper OperatingSystemConfig component implementation
	b.Shoot.Components.Extensions.OperatingSystemConfig = &botanist.FakeOSC{
		Client:                 b.SeedClientSet.Client(),
		ControlPlaneWorkerPool: v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo()).Name,
		ControlPlaneNamespace:  b.Shoot.ControlPlaneNamespace,
	}

	var (
		g = flow.NewGraph("bootstrap")

		deployNamespace = g.Add(flow.Task{
			Name: "Deploying control plane namespace",
			Fn:   b.DeployControlPlaneNamespace,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           b.DeployCloudProviderSecret,
			SkipIf:       b.Shoot.Credentials == nil,
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		reconcileCustomResourceDefinitions = g.Add(flow.Task{
			Name: "Reconciling CustomResourceDefinitions",
			Fn:   b.ReconcileCustomResourceDefinitions,
		})
		ensureCustomResourceDefinitionsReady = g.Add(flow.Task{
			Name:         "Ensuring CustomResourceDefinitions are ready",
			Fn:           flow.TaskFn(b.EnsureCustomResourceDefinitionsReady).RetryUntilTimeout(time.Second, time.Minute),
			Dependencies: flow.NewTaskIDs(reconcileCustomResourceDefinitions),
		})
		reconcileClusterResource = g.Add(flow.Task{
			Name: "Reconciling extensions.gardener.cloud/v1alpha1.Cluster resource",
			Fn: func(ctx context.Context) error {
				return gardenerextensions.SyncClusterResourceToSeed(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo(), b.Shoot.CloudProfile, b.Seed.GetInfo())
			},
			Dependencies: flow.NewTaskIDs(ensureCustomResourceDefinitionsReady),
		})
		initializeSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing internal state of Gardener secrets manager",
			Fn:           b.InitializeSecretsManagement,
			Dependencies: flow.NewTaskIDs(reconcileClusterResource),
		})
		deployPriorityClassCritical = g.Add(flow.Task{
			Name:         "Deploying PriorityClass for gardener-resource-manager",
			Fn:           b.DeployPriorityClassCritical,
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager",
			Fn:           b.Shoot.Components.ControlPlane.ResourceManager.Deploy,
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement, deployPriorityClassCritical),
		})
		waitUntilGardenerResourceManagerReady = g.Add(flow.Task{
			Name:         "Waiting until gardener-resource-manager reports readiness",
			Fn:           b.Shoot.Components.ControlPlane.ResourceManager.Wait,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying seed system resources",
			Fn: func(ctx context.Context) error {
				return seedsystem.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, seedsystem.Values{}).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady),
		})
		deployExtensionControllers = g.Add(flow.Task{
			Name: "Deploying extension controllers",
			Fn: func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, false)
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady),
		})
		waitUntilExtensionControllersReady = g.Add(flow.Task{
			Name:         "Waiting until extension controllers report readiness",
			Fn:           b.WaitUntilExtensionControllerInstallationsHealthy,
			Dependencies: flow.NewTaskIDs(deployExtensionControllers),
		})
		deployNetworkPolicies = g.Add(flow.Task{
			Name:         "Deploying network policies",
			Fn:           b.ApplyNetworkPolicies,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployExtensionControllers),
		})
		syncPointBootstrapped = flow.NewTaskIDs(
			deployNetworkPolicies,
			waitUntilGardenerResourceManagerReady,
			waitUntilExtensionControllersReady,
		)

		deployInfrastructure = g.Add(flow.Task{
			Name:         "Deploying Shoot infrastructure",
			Fn:           b.DeployInfrastructure,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})
		waitUntilInfrastructureReady = g.Add(flow.Task{
			Name:         "Waiting until Shoot infrastructure has been reconciled",
			Fn:           b.WaitForInfrastructure,
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})

		deployOperatingSystemConfig = g.Add(flow.Task{
			Name:         "Deploying OperatingSystemConfig for control plane machines",
			Fn:           b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})

		deployMachineControllerManager = g.Add(flow.Task{
			Name:         "Deploying machine-controller-manager",
			Fn:           b.DeployMachineControllerManager,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})

		_ = waitUntilInfrastructureReady
		_ = deployMachineControllerManager
		_ = deployOperatingSystemConfig
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
	}

	opts.Log.Info("Command is work in progress")
	return nil
}

// ensureNoGardenletOrOperator is a safety check that prevents operators from accidentally executing
// `gardenadm bootstrap` on a cluster that is already used as a runtime cluster with gardener-operator or as a seed
// cluster. Doing so would lead to conflicts when `gardenadm bootstrap` starts deploying components like provider
// extensions.
func ensureNoGardenletOrOperator(ctx context.Context, c client.Reader) error {
	for _, key := range []client.ObjectKey{
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardener-operator"},
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet"},
	} {
		if err := c.Get(ctx, key, &appsv1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed checking if %q deployment exists: %w", key, err)
		}

		return fmt.Errorf("deployment %q exists on the targeted cluster. "+
			"`gardenadm bootstrap` does not support targeting a cluster that is already used as a runtime cluster with gardener-operator or as a seed cluster. "+
			"Please consult the gardenadm documentation", key)
	}

	return nil
}
