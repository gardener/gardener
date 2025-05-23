// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

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
		Short: "Bootstrap worker nodes and join them to the cluster",
		Long: `Bootstrap worker nodes and join them to the cluster.

This command helps to initialize and configure a node to join an existing autonomous shoot cluster.
It ensures that the necessary configurations are applied and the node is properly registered as a worker or control plane node.

Note that further control plane nodes cannot be joined currently.`,
		Example: `# Bootstrap a worker node and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --gardener-node-agent-secret-name <secret-name> <control-plane-address>`,

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
	b, err := botanist.NewAutonomousBotanistWithoutResources(opts.Log)
	if err != nil {
		return fmt.Errorf("failed creating autonomous botanist: %w", err)
	}

	version, err := b.DiscoverKubernetesVersion(opts.ControlPlaneAddress, opts.CertificateAuthority, opts.BootstrapToken)
	if err != nil {
		return fmt.Errorf("failed discovering Kubernetes version of cluster: %w", err)
	}
	b.Shoot = &shootpkg.Shoot{KubernetesVersion: version}

	alreadyJoined, err := b.IsGardenerNodeAgentInitialized(ctx)
	if err != nil {
		return fmt.Errorf("failed checking if gardener-node-agent was already initialized: %w", err)
	}

	if !alreadyJoined {
		var (
			g = flow.NewGraph("join")

			generateGardenerNodeInitConfig = g.Add(flow.Task{
				Name: "Preparing gardener-node-init configuration",
				Fn: func(ctx context.Context) error {
					return b.PrepareGardenerNodeInitConfiguration(ctx, opts.GardenerNodeAgentSecretName, opts.ControlPlaneAddress, opts.CertificateAuthority, opts.BootstrapToken)
				},
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

	fmt.Fprintf(opts.Out, `
Your node has successfully been instructed to join the cluster as a worker!

The bootstrap token will be deleted automatically by kube-controller-manager
after it has expired. If you want to delete it right awy, run the following
on any control plane node:

  gardenadm token delete %s

Run 'kubectl get nodes' on the control-plane to see this node join the cluster.
In case it isn't appearing within two minutes, you can check the logs of
gardener-node-agent by running 'journalctl -u gardener-node-agent', or the
logs of kubelet by running 'journalctl -u kubelet'.
`, opts.BootstrapToken)

	return nil
}
