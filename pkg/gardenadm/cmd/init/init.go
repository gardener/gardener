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
	cloudProfile, project, shoot, err := gardenadm.ReadManifests(opts.Log, os.DirFS(opts.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", opts.ConfigDir, err)
	}

	b, err := botanist.NewAutonomousBotanist(ctx, opts.Log, project, cloudProfile, shoot)
	if err != nil {
		return fmt.Errorf("failed constructing botanist: %w", err)
	}

	var (
		g = flow.NewGraph("init")

		initializeSecretsManagement = g.Add(flow.Task{
			Name: "Initializing secrets management",
			Fn:   b.InitializeSecretsManagement,
		})
		writeKubeletBootstrapKubeconfig = g.Add(flow.Task{
			Name:         "Writing kubelet bootstrap kubeconfig with a fake token to disk to make kubelet start",
			Fn:           b.WriteKubeletBootstrapKubeconfig,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		deployOperatingSystemConfigSecretForNodeAgent = g.Add(flow.Task{
			Name:         "Generating OperatingSystemConfig and deploying Secret for gardener-node-agent",
			Fn:           b.DeployOperatingSystemConfigSecretForNodeAgent,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		applyOperatingSystemConfig = g.Add(flow.Task{
			Name:         "Applying OperatingSystemConfig using gardener-node-agent's reconciliation logic",
			Fn:           b.ApplyOperatingSystemConfig,
			Dependencies: flow.NewTaskIDs(writeKubeletBootstrapKubeconfig, deployOperatingSystemConfigSecretForNodeAgent),
		})
		initializeClientSet = g.Add(flow.Task{
			Name: "Initializing connection to Kubernetes control plane",
			Fn: flow.TaskFn(func(_ context.Context) error {
				b.SeedClientSet, err = b.CreateClientSet(ctx)
				return err
			}).RetryUntilTimeout(2*time.Second, time.Minute),
			Dependencies: flow.NewTaskIDs(applyOperatingSystemConfig),
		})
		_ = g.Add(flow.Task{
			Name:         "Creating real bootstrap token for kubelet and restart unit",
			Fn:           b.BootstrapKubelet,
			Dependencies: flow.NewTaskIDs(initializeClientSet),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprintf(opts.IOStreams.Out, `
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
