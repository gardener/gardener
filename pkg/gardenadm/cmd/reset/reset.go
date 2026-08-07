// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesinformers "k8s.io/client-go/informers"
	criclient "k8s.io/cri-client/pkg"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/nodeagent"
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
	b, err := botanist.NewGardenadmBotanistWithoutResources(opts.Log)
	if err != nil {
		return fmt.Errorf("failed creating gardenadm botanist: %w", err)
	}

	bootstrapClientSet, err := cmd.NewClientSetFromBootstrapToken(opts.ControlPlaneAddress, opts.CertificateAuthority, opts.Token, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed creating a new bootstrap client set: %w", err)
	}
	version, err := b.DiscoverKubernetesVersion(bootstrapClientSet)
	if err != nil {
		return fmt.Errorf("failed discovering Kubernetes version of cluster: %w", err)
	}
	b.Shoot = &shootpkg.Shoot{KubernetesVersion: version, ControlPlaneNamespace: metav1.NamespaceSystem}
	b.Shoot.SetInfo(nil)

	b.Logger.Info("Retrieving short-lived shoot cluster kubeconfig via token")
	b.ShootClientSet, err = cmd.InitializeTemporaryClientSet(ctx, b, bootstrapClientSet)
	if err != nil {
		return fmt.Errorf("failed retrieving short-lived kubeconfig: %w", err)
	}
	b.Logger.Info("Successfully retrieved short-lived kubeconfig")

	node, err := nodeagent.FetchNodeByHostName(ctx, b.ShootClientSet.Client(), b.HostName)
	if err != nil {
		return fmt.Errorf("failed retrieving node for hostname %s: %w", b.HostName, err)
	} else if node == nil {
		return fmt.Errorf("no node found for hostname %s", b.HostName)
	}

	b.Logger.Info("Cordoning and draining node", "node", node.Name)
	if err := cordonAndDrainNode(ctx, b, node.Name, opts); err != nil {
		return fmt.Errorf("failed cordoning and draining node %s: %w", node.Name, err)
	}

	// TODO(scheererj): If node is a control plane node, remove its machine IP from the Etcd's
	// .spec.externallyManagedMemberAddresses[], decrease .spec.replicas, and wait for reconciliation

	b.Logger.Info("Deleting node from cluster", "node", node.Name)
	if err := b.ShootClientSet.Client().Delete(ctx, node); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed deleting node %s: %w", node.Name, err)
	}

	if err := stopContainers(ctx, b, node); err != nil {
		return fmt.Errorf("failed stopping containers: %w", err)
	}

	return nil
}

func cordonAndDrainNode(ctx context.Context, b *botanist.GardenadmBotanist, nodeName string, opts *Options) error {
	informerFactory := kubernetesinformers.NewSharedInformerFactory(b.ShootClientSet.Kubernetes(), time.Minute)
	pdbLister := informerFactory.Policy().V1().PodDisruptionBudgets().Lister()
	podLister := informerFactory.Core().V1().Pods().Lister()
	podsHaveSynced := informerFactory.Core().V1().Pods().Informer().HasSynced
	synced := informerFactory.WaitForCacheSyncWithContext(ctx)
	if err := synced.AsError(); err != nil {
		return fmt.Errorf("failed waiting for informer cache to sync: %w", err)
	}
	for k, v := range synced.Synced {
		// Only if desired log some information similar to this.
		b.Logger.Info(fmt.Sprintf("Cache synced: %s=>%t", k, v))
	}
	informerFactory.StartWithContext(ctx)

	buf := bytes.NewBuffer([]byte{})
	errBuf := bytes.NewBuffer([]byte{})
	d := drain.NewDrainOptions(
		b.ShootClientSet.Kubernetes(),
		b.Shoot.KubernetesVersion,
		opts.Timeout,
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

func stopContainers(ctx context.Context, b *botanist.GardenadmBotanist, node *corev1.Node) error {
	b.Logger.Info("Stopping gardener-node-agent systemd unit")
	if err := b.DBus.Stop(ctx, nil, node, nodeagentconfigv1alpha1.UnitName); err != nil {
		return fmt.Errorf("failed stopping gardener-node-agent systemd unit: %w", err)
	}

	b.Logger.Info("Stopping kubelet systemd unit")
	if err := b.DBus.Stop(ctx, nil, node, v1beta1constants.OperatingSystemConfigUnitNameKubeletService); err != nil {
		return fmt.Errorf("failed stopping kubelet systemd unit: %w", err)
	}

	b.Logger.Info("Stopping containers")
	runtimeService, err := criclient.NewRemoteRuntimeService(ctx, "unix:///var/run/containerd/containerd.sock", 2*time.Second, nil, false)
	if err != nil {
		return fmt.Errorf("failed creating CRI client: %w", err)
	}
	defer runtimeService.Close(ctx)

	pods, err := runtimeService.ListPodSandbox(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed listing pod sandboxes: %w", err)
	}

	var errs []error
	for _, pod := range pods {
		b.Logger.Info("Stopping pod sandbox", "podID", pod.GetId())
		var lastErr error
		for range 5 { // Retry up to 5 times
			if err := runtimeService.StopPodSandbox(ctx, pod.GetId()); err != nil {
				lastErr = err
				b.Logger.Error(err, "Failed stopping pod sandbox", "podID", pod.Id)
				continue
			}

			if err := runtimeService.RemovePodSandbox(ctx, pod.GetId()); err != nil {
				lastErr = err
				b.Logger.Error(err, "Failed removing pod sandbox", "podID", pod.Id)
				continue
			}

			lastErr = nil
			break
		}

		if lastErr != nil {
			errs = append(errs, lastErr)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Failed stopping some pods: %w", errors.Join(errs...))
	}

	return nil
}
