// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubernetesinformers "k8s.io/client-go/informers"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset control plane or worker nodes and remove them from the cluster",
		Long: `Reset control plane or worker nodes and remove them from the cluster.

This command helps to remove a node from an existing self-hosted shoot cluster.
It ensures that the components deployed to this node are removed and the node is properly deregistered as a control plane or worker node.`,
		Example: `# Reset a node and remove it from the cluster
gardenadm reset --token <token> --ca-certificate <ca-cert> <control-plane-address>`,

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
	b, err := cmd.InitializeGardenadmWithTemporaryClientSet(ctx, opts.Log, opts.ControlPlaneAddress, opts.CertificateAuthority, opts.Token)
	if err != nil {
		return fmt.Errorf("failed initializing gardenadm botanist with temporary client set: %w", err)
	}

	node, err := nodeagent.FetchNodeByHostName(ctx, b.ShootClientSet.Client(), b.HostName)
	if err != nil {
		return fmt.Errorf("failed retrieving node for hostname %s: %w", b.HostName, err)
	} else if node == nil {
		return fmt.Errorf("no node found for hostname %s", b.HostName)
	}

	if lastControlPlaneNode, err := isLastControlPlaneNodeWithWorkers(ctx, b, node); err != nil {
		return fmt.Errorf("failed checking if node is last control plane node: %w", err)
	} else if lastControlPlaneNode {
		return fmt.Errorf("cannot reset last control plane node as long as worker nodes still exist; remove the worker nodes first")
	}

	var (
		g        = flow.NewGraph("reset")
		reporter = flow.NewCommandLineProgressReporter(opts.ErrOut)

		cordonAndDrainNodeComplete = g.Add(flow.Task{
			Name: "Cordoning and draining node",
			Fn: func(ctx context.Context) error {
				return cordonAndDrainNode(ctx, b, node.Name, opts)
			},
		})

		// TODO(scheererj): If node is a control plane node, remove its machine IP from the Etcd's
		// .spec.externallyManagedMemberAddresses[], decrease .spec.replicas, and wait for reconciliation

		deletedNode = g.Add(flow.Task{
			Name: "Deleting node from cluster",
			Fn: func(ctx context.Context) error {
				if err := b.ShootClientSet.Client().Delete(ctx, node); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed deleting node %s: %w", node.Name, err)
				}
				return nil
			},
			Dependencies: flow.NewTaskIDs(cordonAndDrainNodeComplete),
		})
		_ = g.Add(flow.Task{
			Name: "Executing 'gardener-node-agent reset'",
			Fn: func(ctx context.Context) error {
				out, err := exec.CommandContext(ctx, nodeagentconfigv1alpha1.BinaryDir+"/gardener-node-agent", "reset", "--config-dir", nodeagentconfigv1alpha1.BaseDir).CombinedOutput()
				b.Logger.Info("gardener-node-agent reset")
				fmt.Fprintln(opts.ErrOut, string(out))
				return err
			},
			Dependencies: flow.NewTaskIDs(deletedNode),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              opts.Log,
		ProgressReporter: reporter,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprintf(opts.Out, `
The node has been successfully removed from the cluster!

The token will be deleted automatically by kube-controller-manager
after it has expired. If you want to delete it right away, run the following
command on any control plane node:

  gardenadm token delete %s
`, opts.Token)

	return nil
}

func isLastControlPlaneNodeWithWorkers(ctx context.Context, b *botanist.GardenadmBotanist, node *corev1.Node) (bool, error) {
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		// Only allow removing the last control plane node if there are no more worker nodes
		nodeList := &corev1.NodeList{}
		if err := b.SeedClientSet.Client().List(ctx, nodeList); err != nil {
			return false, fmt.Errorf("failed retrieving node list: %w", err)
		}

		if controlPlaneNodes, err := b.ListControlPlaneNodes(ctx); err != nil {
			return false, fmt.Errorf("failed listing control plane nodes: %w", err)
		} else if len(controlPlaneNodes) == 1 && len(nodeList.Items) > len(controlPlaneNodes) {
			return true, nil
		}
	}

	return false, nil
}

func cordonAndDrainNode(ctx context.Context, b *botanist.GardenadmBotanist, nodeName string, opts *Options) error {
	var (
		informerFactory = kubernetesinformers.NewSharedInformerFactory(b.ShootClientSet.Kubernetes(), time.Minute)
		pdbLister       = informerFactory.Policy().V1().PodDisruptionBudgets().Lister()
		podLister       = informerFactory.Core().V1().Pods().Lister()
		podsHaveSynced  = informerFactory.Core().V1().Pods().Informer().HasSynced
		buf             = bytes.NewBuffer([]byte{})
		errBuf          = bytes.NewBuffer([]byte{})
	)

	informerFactory.StartWithContext(ctx)
	synced := informerFactory.WaitForCacheSyncWithContext(ctx)
	if err := synced.AsError(); err != nil {
		return fmt.Errorf("failed waiting for informer cache to sync: %w", err)
	}
	for k, v := range synced.Synced {
		b.Logger.Info(fmt.Sprintf("Cache synced: %s=>%t", k, v))
	}

	d := drain.NewDrainOptions(
		b.ShootClientSet.Kubernetes(),
		b.Shoot.KubernetesVersion,
		opts.DrainTimeout,
		math.MaxInt32,
		0,
		0,
		nodeName,
		0,
		true,
		true,
		true,
		true,
		buf,
		errBuf,
		nil,
		nil,
		nil,
		pdbLister,
		nil,
		podLister,
		nil,
		podsHaveSynced,
	)

	if err := d.RunDrain(ctx); err != nil {
		return fmt.Errorf("failed to drain node %s: %w", nodeName, err)
	}

	return nil
}
