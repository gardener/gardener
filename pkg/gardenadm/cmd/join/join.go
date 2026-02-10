// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
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
	b.Shoot = &shootpkg.Shoot{KubernetesVersion: version, ControlPlaneNamespace: metav1.NamespaceSystem}
	b.Shoot.SetInfo(nil)

	b.Logger.Info("Retrieving short-lived shoot cluster kubeconfig via bootstrap token")
	shootClientSet, err := cmd.InitializeTemporaryClientSet(ctx, b, bootstrapClientSet)
	if err != nil {
		return fmt.Errorf("failed retrieving short-lived kubeconfig: %w", err)
	}
	b.Logger.Info("Successfully retrieved short-lived bootstrap kubeconfig")
	b.ShootClientSet = shootClientSet

	cluster, err := gardenerextensions.GetCluster(ctx, b.ShootClientSet.Client(), b.Shoot.ControlPlaneNamespace)
	if err != nil {
		return fmt.Errorf("failed to read Cluster resource %s: %w", b.Shoot.ControlPlaneNamespace, err)
	}
	b.Shoot.SetInfo(cluster.Shoot)

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: b.HostName}}

	nodeJoinedAlready := true
	if err := b.ShootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed retrieving node %s: %w", node.Name, err)
		}
		nodeJoinedAlready = false
	}

	var (
		g                       = flow.NewGraph("join")
		gardenerNodeAgentSecret *corev1.Secret

		ensureNoActiveShootReconciliation = g.Add(flow.Task{
			Name: "Ensuring shoot is not concurrently reconciled by gardenlet when joining control plane node",
			Fn: func(_ context.Context) error {
				if lastOperation := b.Shoot.GetInfo().Status.LastOperation; lastOperation != nil && slices.Contains([]gardencorev1beta1.LastOperationState{
					gardencorev1beta1.LastOperationStateProcessing,
					gardencorev1beta1.LastOperationStateError,
					gardencorev1beta1.LastOperationStateAborted,
				}, lastOperation.State) {
					return fmt.Errorf("the Shoot's status.lastOperation.state must indicate successful reconciliation before joining a control plane node: %s", lastOperation.State)
				}
				return nil
			},
			SkipIf: !opts.ControlPlane,
		})
		determineGardenerNodeAgentSecretName = g.Add(flow.Task{
			Name: "Determining gardener-node-agent Secret containing the configuration for this node",
			Fn: func(ctx context.Context) error {
				var err error
				gardenerNodeAgentSecret, err = GetGardenerNodeAgentSecret(ctx, opts, b)
				return err
			},
		})
		syncPointReadyForGardenerNodeInit = flow.NewTaskIDs(
			determineGardenerNodeAgentSecretName,
			ensureNoActiveShootReconciliation,
		)

		generateGardenerNodeInitConfig = g.Add(flow.Task{
			Name: "Preparing gardener-node-init configuration",
			Fn: func(ctx context.Context) error {
				return b.PrepareGardenerNodeInitConfiguration(ctx, gardenerNodeAgentSecret.Name, opts.ControlPlaneAddress, opts.CertificateAuthority, opts.BootstrapToken)
			},
			SkipIf:       nodeJoinedAlready,
			Dependencies: flow.NewTaskIDs(syncPointReadyForGardenerNodeInit),
		})
		applyOperatingSystemConfig = g.Add(flow.Task{
			Name:         "Applying OperatingSystemConfig using gardener-node-agent's reconciliation logic",
			Fn:           b.ApplyOperatingSystemConfig,
			SkipIf:       nodeJoinedAlready,
			Dependencies: flow.NewTaskIDs(generateGardenerNodeInitConfig),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting for node to join the cluster and become ready",
			Fn: func(ctx context.Context) error {
				log := b.Logger.WithValues("nodeName", node.Name)

				return flow.Sequential(
					func(ctx context.Context) error {
						return waitForNodeToJoinCluster(ctx, log, b.ShootClientSet.Client(), node)
					},
					func(ctx context.Context) error {
						return waitForOperatingSystemConfigToBeApplied(ctx, log, b.ShootClientSet.Client(), node, gardenerNodeAgentSecret)
					},
					func(ctx context.Context) error {
						return waitForNodeReadiness(ctx, log, b.ShootClientSet.Client(), node)
					},
				)(ctx)
			},
			Dependencies: flow.NewTaskIDs(applyOperatingSystemConfig),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
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

// GetGardenerNodeAgentSecret retrieves the Secret for gardener-node-agent which contains the operating system
// configuration for this node.
func GetGardenerNodeAgentSecret(ctx context.Context, opts *Options, b *botanist.GardenadmBotanist) (*corev1.Secret, error) {
	workerPoolName, err := getWorkerPoolName(ctx, opts, b)
	if err != nil {
		return nil, fmt.Errorf("failed to determine worker pool name in Shoot manifest: %w", err)
	}

	secretList := &corev1.SecretList{}
	if err := b.ShootClientSet.Client().List(ctx, secretList, client.InNamespace(b.Shoot.ControlPlaneNamespace), client.MatchingLabels{
		v1beta1constants.GardenRole:      v1beta1constants.GardenRoleOperatingSystemConfig,
		v1beta1constants.LabelWorkerPool: workerPoolName,
	}); err != nil {
		return nil, fmt.Errorf("failed listing gardener-node-agent secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return nil, fmt.Errorf("no gardener-node-agent secrets found for worker pool %q", workerPoolName)
	}

	gardenerNodeAgentSecret := secretList.Items[0]
	if len(secretList.Items) > 1 {
		opts.Log.V(1).Info("Multiple gardener-node-agent secrets found, using the first one", "secretName", gardenerNodeAgentSecret.Name)
	}

	return &gardenerNodeAgentSecret, nil
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

func waitForNodeToJoinCluster(ctx context.Context, log logr.Logger, c client.Client, node *corev1.Node) error {
	log.Info("Waiting for node to join the cluster")
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.MinorError(fmt.Errorf("node %s did not yet join the cluster: %w", node.Name, err))
			}
			return retry.SevereError(fmt.Errorf("failed to get node %s: %w", node.Name, err))
		}
		return retry.Ok()
	})
}

func waitForOperatingSystemConfigToBeApplied(ctx context.Context, log logr.Logger, c client.Client, node *corev1.Node, gardenerNodeAgentSecret *corev1.Secret) error {
	log.Info("Waiting for operating system config to be fully applied")
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
			return retry.SevereError(fmt.Errorf("failed to get node %s: %w", node.Name, err))
		}

		secretChecksum := gardenerNodeAgentSecret.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig]
		if nodeChecksum, ok := node.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig]; nodeChecksum != secretChecksum {
			if !ok {
				return retry.MinorError(fmt.Errorf("the last successfully applied operating system config on node %q hasn't been reported yet", node.Name))
			} else {
				return retry.MinorError(fmt.Errorf("the last successfully applied operating system config on node %q is outdated (current: %s, desired: %s)", node.Name, nodeChecksum, secretChecksum))
			}
		}

		return retry.Ok()
	})
}

func waitForNodeReadiness(ctx context.Context, log logr.Logger, c client.Client, node *corev1.Node) error {
	log.Info("Waiting for node to get ready")
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	return retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
			return retry.SevereError(fmt.Errorf("failed to get node %s: %w", node.Name, err))
		}

		if err := health.CheckNode(node); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
