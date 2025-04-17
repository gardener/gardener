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

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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

	podNetworkAvailable, err := b.IsPodNetworkAvailable(ctx)
	if err != nil {
		return fmt.Errorf("failed checking whether pod network is already available: %w", err)
	}

	var (
		g                = flow.NewGraph("init")
		kubeProxyEnabled = v1beta1helper.KubeProxyEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy)

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
			Name: "Deploying gardener-resource-manager",
			Fn: func(ctx context.Context) error {
				b.Shoot.Components.ControlPlane.ResourceManager.SetBootstrapControlPlaneNode(!podNetworkAvailable)
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
				return b.ReconcileExtensionControllerInstallations(ctx, !podNetworkAvailable)
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
		deployShootNamespaces = g.Add(flow.Task{
			Name:         "Deploying shoot namespaces system component",
			Fn:           b.Shoot.Components.SystemComponents.Namespaces.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		waitUntilShootNamespacesReady = g.Add(flow.Task{
			Name:         "Waiting until shoot namespaces have been reconciled",
			Fn:           b.Shoot.Components.SystemComponents.Namespaces.Wait,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, deployShootNamespaces),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying kube-proxy system component",
			Fn:           b.DeployKubeProxy,
			SkipIf:       !kubeProxyEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, waitUntilShootNamespacesReady, waitUntilExtensionControllersReady),
		})
		deployNetwork = g.Add(flow.Task{
			Name:         "Deploying shoot network plugin",
			Fn:           b.DeployNetwork,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, waitUntilShootNamespacesReady, waitUntilExtensionControllersReady),
		})
		waitUntilNetworkReady = g.Add(flow.Task{
			Name:         "Waiting until shoot network plugin has been reconciled",
			Fn:           b.Shoot.Components.Extensions.Network.Wait,
			Dependencies: flow.NewTaskIDs(deployNetwork),
		})
		deployCoreDNS = g.Add(flow.Task{
			Name:         "Deploying CoreDNS system component",
			Fn:           b.DeployCoreDNS,
			Dependencies: flow.NewTaskIDs(waitUntilNetworkReady, deployNetworkPolicies),
		})
		waitUntilCoreDNSReady = g.Add(flow.Task{
			Name:         "Waiting until CoreDNS system component is ready",
			Fn:           b.Shoot.Components.SystemComponents.CoreDNS.Wait,
			Dependencies: flow.NewTaskIDs(deployCoreDNS),
		})

		deployGardenerResourceManagerIntoPodNetwork = g.Add(flow.Task{
			Name: "Redeploying gardener-resource-manager into pod network",
			Fn: func(ctx context.Context) error {
				b.Shoot.Components.ControlPlane.ResourceManager.SetBootstrapControlPlaneNode(false)
				return b.Shoot.Components.ControlPlane.ResourceManager.Deploy(ctx)
			},
			SkipIf:       podNetworkAvailable,
			Dependencies: flow.NewTaskIDs(waitUntilCoreDNSReady),
		})
		waitUntilGardenerResourceManagerInPodNetworkReady = g.Add(flow.Task{
			Name:         "Waiting until gardener-resource-manager (in pod network) reports readiness",
			Fn:           b.Shoot.Components.ControlPlane.ResourceManager.Wait,
			SkipIf:       podNetworkAvailable,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManagerIntoPodNetwork),
		})
		deployExtensionControllersIntoPodNetwork = g.Add(flow.Task{
			Name: "Redeploying extension controllers into pod network",
			Fn: func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, false)
			},
			SkipIf:       podNetworkAvailable,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerInPodNetworkReady),
		})
		waitUntilExtensionControllersInPodNetworkReady = g.Add(flow.Task{
			Name:         "Waiting until extension controllers (in pod network) report readiness",
			Fn:           b.WaitUntilExtensionControllerInstallationsHealthy,
			SkipIf:       podNetworkAvailable,
			Dependencies: flow.NewTaskIDs(deployExtensionControllersIntoPodNetwork),
		})
		syncPointBootstrapped = flow.NewTaskIDs(
			deployNetworkPolicies,
			waitUntilGardenerResourceManagerReady,
			waitUntilGardenerResourceManagerInPodNetworkReady,
			waitUntilExtensionControllersReady,
			waitUntilExtensionControllersInPodNetworkReady,
		)

		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying shoot control plane components",
			Fn:           b.DeployControlPlane,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been reconciled",
			Fn:           b.Shoot.Components.Extensions.ControlPlane.Wait,
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
		deployControlPlaneDeployments = g.Add(flow.Task{
			Name:         "Deploying control plane components as Deployments for static pod translation",
			Fn:           b.DeployControlPlaneDeployments,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying OperatingSystemConfig and activating gardener-node-agent",
			Fn:           b.ActivateGardenerNodeAgent,
			Dependencies: flow.NewTaskIDs(deployControlPlaneDeployments),
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
