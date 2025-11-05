// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Bootstrap control plane or worker nodes and join them to the cluster",
		Long: `Bootstrap control plane or worker nodes and join them to the cluster.

This command helps to initialize and configure a node to join an existing self-hosted shoot cluster.
It ensures that the necessary configurations are applied and the node is properly registered as a control plane or worker node.`,
		Example: `# Bootstrap a control plane node and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --control-plane <control-plane-address>

# Bootstrap a worker node and join it to the cluster (by default, it is assigned to the first worker pool in the Shoot manifest)
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> <control-plane-address>

# Bootstrap a worker node in a specific worker pool and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --worker-pool-name <pool-name> <control-plane-address>`,

		Args: cobra.ExactArgs(1),

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
	b, err := botanist.NewGardenadmBotanistWithoutResources(opts.Log)
	if err != nil {
		return fmt.Errorf("failed creating gardenadm botanist: %w", err)
	}

	bootstrapClientSet, err := cmd.NewClientSetFromBootstrapToken(opts.ControlPlaneAddress, opts.CertificateAuthority, opts.BootstrapToken, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed creating a new bootstrap client set: %w", err)
	}
	version, err := b.DiscoverKubernetesVersion(bootstrapClientSet)
	if err != nil {
		return fmt.Errorf("failed discovering Kubernetes version of cluster: %w", err)
	}
	b.Shoot = &shootpkg.Shoot{KubernetesVersion: version}
	b.Shoot.SetInfo(nil)

	alreadyJoined, err := b.IsGardenerNodeAgentInitialized(ctx)
	if err != nil {
		return fmt.Errorf("failed checking if gardener-node-agent was already initialized: %w", err)
	}

	if !alreadyJoined {
		var (
			g                           = flow.NewGraph("join")
			gardenerNodeAgentSecretName string

			retrieveShortLivedKubeconfig = g.Add(flow.Task{
				Name: "Retrieving short-lived kubeconfig cluster to prepare control plane scale-up",
				Fn: func(ctx context.Context) error {
					shootClientSet, err := cmd.InitializeTemporaryClientSet(ctx, b, bootstrapClientSet)
					if err != nil {
						return fmt.Errorf("failed retrieving short-lived kubeconfig: %w", err)
					}

					b.Logger.Info("Successfully retrieved short-lived bootstrap kubeconfig")
					b.ShootClientSet = shootClientSet
					return nil
				},
			})
			determineGardenerNodeAgentSecretName = g.Add(flow.Task{
				Name: "Determining gardener-node-agent Secret containing the configuration for this node",
				Fn: func(ctx context.Context) error {
					var err error
					gardenerNodeAgentSecretName, err = GetGardenerNodeAgentSecretName(ctx, opts, b)
					return err
				},
				Dependencies: flow.NewTaskIDs(retrieveShortLivedKubeconfig),
			})
			syncPointReadyForGardenerNodeInit = flow.NewTaskIDs(
				determineGardenerNodeAgentSecretName,
			)

			generateGardenerNodeInitConfig = g.Add(flow.Task{
				Name: "Preparing gardener-node-init configuration",
				Fn: func(ctx context.Context) error {
					return b.PrepareGardenerNodeInitConfiguration(ctx, gardenerNodeAgentSecretName, opts.ControlPlaneAddress, opts.CertificateAuthority, opts.BootstrapToken)
				},
				Dependencies: flow.NewTaskIDs(syncPointReadyForGardenerNodeInit),
			})
			_ = g.Add(flow.Task{
				Name:         "Applying OperatingSystemConfig using gardener-node-agent's reconciliation logic",
				Fn:           b.ApplyOperatingSystemConfig,
				Dependencies: flow.NewTaskIDs(generateGardenerNodeInitConfig),
			})
		)

		if err := g.Compile().Run(ctx, flow.Opts{
			Log: opts.Log,
		}); err != nil {
			return flow.Errors(err)
		}
	}

	if opts.ControlPlane {
		fmt.Fprintf(opts.Out, `
Your node has successfully been instructed to join the cluster as a control-plane instance!
`)
	} else {
		fmt.Fprintf(opts.Out, `
Your node has successfully been instructed to join the cluster as a worker!
`)
	}

	fmt.Fprintf(opts.Out, `
The bootstrap token will be deleted automatically by kube-controller-manager
after it has expired. If you want to delete it right away, run the following
command on any control plane node:

  gardenadm token delete %s

Run 'kubectl get nodes' on the control-plane to see this node join the cluster.
In case it isn't appearing within two minutes, you can check the logs of
gardener-node-agent by running 'journalctl -u gardener-node-agent', or the
logs of kubelet by running 'journalctl -u kubelet'.
`, opts.BootstrapToken)

	return nil
}

// GetGardenerNodeAgentSecretName retrieves the Secret for gardener-node-agent which contains the operating system
// configuration for this node.
func GetGardenerNodeAgentSecretName(ctx context.Context, opts *Options, b *botanist.GardenadmBotanist) (string, error) {
	workerPoolName, err := getWorkerPoolName(ctx, opts, b)
	if err != nil {
		return "", fmt.Errorf("failed to determine worker pool name in Shoot manifest: %w", err)
	}

	secretList := &corev1.SecretList{}
	if err := b.ShootClientSet.Client().List(ctx, secretList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{
		v1beta1constants.GardenRole:      v1beta1constants.GardenRoleOperatingSystemConfig,
		v1beta1constants.LabelWorkerPool: workerPoolName,
	}); err != nil {
		return "", fmt.Errorf("failed listing gardener-node-agent secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("no gardener-node-agent secrets found for worker pool %q", workerPoolName)
	}

	gardenerNodeAgentSecret := secretList.Items[0]
	if len(secretList.Items) > 1 {
		opts.Log.V(1).Info("Multiple gardener-node-agent secrets found, using the first one", "secretName", gardenerNodeAgentSecret.Name)
	}

	return gardenerNodeAgentSecret.Name, nil
}

func getWorkerPoolName(ctx context.Context, opts *Options, b *botanist.GardenadmBotanist) (string, error) {
	if opts.WorkerPoolName != "" {
		return opts.WorkerPoolName, nil
	}

	cluster, err := gardenerextensions.GetCluster(ctx, b.ShootClientSet.Client(), metav1.NamespaceSystem)
	if err != nil {
		return "", fmt.Errorf("failed reading extensions.gardener.cloud/v1alpha1.Cluster object: %w", err)
	}

	if opts.ControlPlane {
		return getControlPlaneWorkerPoolName(cluster.Shoot.Spec.Provider.Workers)
	}
	return getFirstWorkerPoolName(cluster.Shoot.Spec.Provider.Workers)
}

func getControlPlaneWorkerPoolName(workers []gardencorev1beta1.Worker) (string, error) {
	if pool := helper.ControlPlaneWorkerPoolForShoot(workers); pool != nil {
		return pool.Name, nil
	}

	return "", fmt.Errorf("no control plane worker pool found in Shoot manifest")
}

func getFirstWorkerPoolName(workers []gardencorev1beta1.Worker) (string, error) {
	for _, worker := range workers {
		if worker.ControlPlane == nil {
			return worker.Name, nil
		}
	}

	return "", fmt.Errorf("no non-control-plane pool found in Shoot manifest")
}
