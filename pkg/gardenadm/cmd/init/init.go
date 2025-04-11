// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap the first control plane node",
		Long:  "Bootstrap the first control plane node",

		Example: `# Bootstrap the first control plane node
gardenadm init`,

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

func run(ctx context.Context, opts *Options) error {
	b, err := bootstrapControlPlane(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed bootstrapping control plane: %w", err)
	}

	var (
		g = flow.NewGraph("init")

		deployNamespace = g.Add(flow.Task{
			Name: "Deploying control plane namespace",
			Fn:   b.DeployControlPlaneNamespace,
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
		bootstrapKubelet = g.Add(flow.Task{
			Name:         "Creating real bootstrap token for kubelet and restart unit",
			Fn:           b.BootstrapKubelet,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		_ = g.Add(flow.Task{
			Name:         "Approving kubelet server certificate signing request if necessary",
			Fn:           flow.TaskFn(b.ApproveKubeletServerCertificateSigningRequest).RetryUntilTimeout(2*time.Second, time.Minute),
			Dependencies: flow.NewTaskIDs(bootstrapKubelet),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			// TODO: Do this only when Network is not available on control plane nodes, i.e., only when
			//  CNI/kube-proxy/CoreDNS are not ready/available yet.
			Name: "Deploying gardener-resource-manager with bootstrap mode",
			Fn: func(ctx context.Context) error {
				b.Shoot.Components.ControlPlane.ResourceManager.SetBootstrapControlPlaneNode(true)
				return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement),
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
		_ = g.Add(flow.Task{
			Name:         "Deploying shoot system resources",
			Fn:           b.DeployShootSystem,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady),
		})
		deployExtensionControllers = g.Add(flow.Task{
			Name: "Deploying extension controllers",
			Fn: func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, false)
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady),
		})
		waitForExtensionControllersReady = g.Add(flow.Task{
			Name:         "Waiting until extension controllers report readiness",
			Fn:           b.WaitUntilExtensionControllerInstallationsHealthy,
			Dependencies: flow.NewTaskIDs(deployExtensionControllers),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprintf(opts.Out, `
Your Shoot cluster control-plane has initialized successfully!

To start using your cluster, you need to run the following as a regular user:

  mkdir -p $HOME/.kube
  sudo cp -i %s $HOME/.kube/config
  sudo chown $(id -u):$(id -g) $HOME/.kube/config
  kubectl get nodes

You can now join any number of machines by running the following on each node
as root:

  gardenadm join <TODO>

Note that the mentioned kubeconfig file will be disabled once you deploy the
gardenlet and connect this cluster to an existing Gardener installation by
running on any node:

  gardenadm connect <TODO>

Please use the shoots/adminkubeconfig subresource to retrieve a kubeconfig,
see https://gardener.cloud/docs/gardener/shoot/shoot_access/.
`, botanist.PathKubeconfig)

	return nil
}

func bootstrapControlPlane(ctx context.Context, opts *Options) (*botanist.AutonomousBotanist, error) {
	cloudProfile, project, shoot, controllerRegistrations, controllerDeployments, err := gardenadm.ReadManifests(opts.Log, os.DirFS(opts.ConfigDir))
	if err != nil {
		return nil, fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", opts.ConfigDir, err)
	}

	extensions, err := botanist.ComputeExtensions(shoot, controllerRegistrations, controllerDeployments)
	if err != nil {
		return nil, fmt.Errorf("failed computing extensions: %w", err)
	}

	b, err := botanist.NewAutonomousBotanist(ctx, opts.Log, nil, project, cloudProfile, shoot, extensions)
	if err != nil {
		return nil, fmt.Errorf("failed constructing botanist: %w", err)
	}

	kubeconfigFileExists, err := b.FS.Exists(botanist.PathKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed checking whether kubeconfig file %s exists: %w", botanist.PathKubeconfig, err)
	}

	if kubeconfigFileExists {
		b.Logger.Info("Found existing kubeconfig file, skipping initialization of control plane", "path", botanist.PathKubeconfig)
	}

	var (
		clientSet kubernetes.Interface
		g         = flow.NewGraph("bootstrap")

		initializeSecretsManagement = g.Add(flow.Task{
			Name:   "Initializing secrets management",
			Fn:     b.InitializeSecretsManagement,
			SkipIf: kubeconfigFileExists,
		})
		writeKubeletBootstrapKubeconfig = g.Add(flow.Task{
			Name:         "Writing kubelet bootstrap kubeconfig with a fake token to disk to make kubelet start",
			Fn:           b.WriteKubeletBootstrapKubeconfig,
			SkipIf:       kubeconfigFileExists,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		deployOperatingSystemConfigSecretForNodeAgent = g.Add(flow.Task{
			Name:         "Generating OperatingSystemConfig and deploying Secret for gardener-node-agent",
			Fn:           b.DeployOperatingSystemConfigSecretForNodeAgent,
			SkipIf:       kubeconfigFileExists,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		applyOperatingSystemConfig = g.Add(flow.Task{
			Name:         "Applying OperatingSystemConfig using gardener-node-agent's reconciliation logic",
			Fn:           b.ApplyOperatingSystemConfig,
			SkipIf:       kubeconfigFileExists,
			Dependencies: flow.NewTaskIDs(writeKubeletBootstrapKubeconfig, deployOperatingSystemConfigSecretForNodeAgent),
		})
		initializeClientSet = g.Add(flow.Task{
			Name: "Initializing connection to Kubernetes control plane",
			Fn: flow.TaskFn(func(_ context.Context) error {
				clientSet, err = b.CreateClientSet(ctx)
				return err
			}).RetryUntilTimeout(2*time.Second, time.Minute),
			Dependencies: flow.NewTaskIDs(applyOperatingSystemConfig),
		})
		_ = g.Add(flow.Task{
			Name: "Importing secrets into control plane",
			Fn: func(ctx context.Context) error {
				return b.MigrateSecrets(ctx, b.SeedClientSet.Client(), clientSet.Client())
			},
			SkipIf:       kubeconfigFileExists,
			Dependencies: flow.NewTaskIDs(initializeClientSet),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: b.Logger,
	}); err != nil {
		return nil, flow.Errors(err)
	}

	return botanist.NewAutonomousBotanist(ctx, opts.Log, clientSet, project, cloudProfile, shoot, extensions)
}
