// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesinformers "k8s.io/client-go/informers"
	criclient "k8s.io/cri-client/pkg"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
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
		stoppedContainers = g.Add(flow.Task{
			Name: "Stopping containers",
			Fn: func(ctx context.Context) error {
				return stopContainers(ctx, b, node)
			},
			Dependencies: flow.NewTaskIDs(deletedNode),
		})
		cleanedUpOSC = g.Add(flow.Task{
			Name: "Cleaning up operating system configuration",
			Fn: func(ctx context.Context) error {
				return cleanUpOperatingSystemConfig(ctx, b, node)
			},
			Dependencies: flow.NewTaskIDs(stoppedContainers),
		})
		_ = g.Add(flow.Task{
			Name: "Cleaning up gardener node agent folder",
			Fn: func(ctx context.Context) error {
				return cleanUpNodeAgentFolder(b)
			},
			Dependencies: flow.NewTaskIDs(cleanedUpOSC),
		})
		_ = g.Add(flow.Task{
			Name: "Cleaning up kubelet folder",
			Fn: func(ctx context.Context) error {
				return cleanUpKubeletFolder(b)
			},
			Dependencies: flow.NewTaskIDs(cleanedUpOSC),
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

func cordonAndDrainNode(ctx context.Context, b *botanist.GardenadmBotanist, nodeName string, opts *Options) error {
	var (
		informerFactory = kubernetesinformers.NewSharedInformerFactory(b.ShootClientSet.Kubernetes(), time.Minute)
		pdbLister       = informerFactory.Policy().V1().PodDisruptionBudgets().Lister()
		podLister       = informerFactory.Core().V1().Pods().Lister()
		podsHaveSynced  = informerFactory.Core().V1().Pods().Informer().HasSynced
		synced          = informerFactory.WaitForCacheSyncWithContext(ctx)
		buf             = bytes.NewBuffer([]byte{})
		errBuf          = bytes.NewBuffer([]byte{})
	)

	if err := synced.AsError(); err != nil {
		return fmt.Errorf("failed waiting for informer cache to sync: %w", err)
	}
	for k, v := range synced.Synced {
		// Only if desired log some information similar to this.
		b.Logger.Info(fmt.Sprintf("Cache synced: %s=>%t", k, v))
	}
	informerFactory.StartWithContext(ctx)

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
		return fmt.Errorf("failed stopping some pods: %w", errors.Join(errs...))
	}

	return nil
}

func cleanUpOperatingSystemConfig(ctx context.Context, b *botanist.GardenadmBotanist, node *corev1.Node) error {
	b.Logger.Info("Checking last-applied OperatingSystemConfig for cleanup")
	oscFileContent, err := b.FS.ReadFile(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			b.Logger.Info("No last-applied OperatingSystemConfig found, skipping cleanup")
			return nil
		}
		return fmt.Errorf("cannot read last-applied OperatingSystemConfig: %w", err)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeletconfigv1beta1.AddToScheme(scheme))
	oscDecoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(oscDecoder, oscFileContent, osc); err != nil {
		return fmt.Errorf("unable to decode OperatingSystemConfig read from file path %s: %w", nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath, err)
	}

	var errs []error
	b.Logger.Info("Stopping systemd units")
	for _, unit := range append(osc.Spec.Units, osc.Status.ExtensionUnits...) {
		b.Logger.Info("Stopping systemd unit", "unit", unit.Name)
		if err := b.DBus.Stop(ctx, nil, node, unit.Name); err != nil {
			errs = append(errs, err)
		}
		b.Logger.Info("Disabling systemd unit", "unit", unit.Name)
		if err := b.DBus.Disable(ctx, unit.Name); err != nil {
			errs = append(errs, err)
		}
	}

	b.Logger.Info("Removing installed files")
	for _, file := range append(osc.Spec.Files, osc.Status.ExtensionFiles...) {
		b.Logger.Info("Removing file", "path", file.Path)
		if err := b.FS.Remove(file.Path); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed cleaning up OperatingSystemConfig: %w", errors.Join(errs...))
	}

	return nil
}

// Failing to clean up the gardener-node-agent folder will block the node from joining the cluster again.
func cleanUpNodeAgentFolder(b *botanist.GardenadmBotanist) error {
	b.Logger.Info("Cleaning up gardener-node-agent folder")
	entries, err := os.ReadDir(nodeagentconfigv1alpha1.BaseDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed listing garden-node-agent folder: %w", err)
	}

	for _, entry := range entries {
		b.Logger.Info("Removing gardener-node-agent file", "file", entry.Name())
		if err := b.FS.RemoveAll(nodeagentconfigv1alpha1.BaseDir + "/" + entry.Name()); err != nil {
			return fmt.Errorf("failed removing %q in gardener-node-agent dir %q: %w", entry.Name(), nodeagentconfigv1alpha1.BaseDir, err)
		}
	}

	return nil
}

func cleanUpKubeletFolder(b *botanist.GardenadmBotanist) error {
	b.Logger.Info("Cleaning up kubelet folder")
	entries, err := os.ReadDir(kubelet.PathKubeletDirectory)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed listing kubelet folder: %w", err)
	}

	if err := unmountKubeletSubFolders(b); err != nil {
		return fmt.Errorf("failed unmounting kubelet sub folders: %w", err)
	}

	for _, entry := range entries {
		b.Logger.Info("Removing kubelet file", "file", entry.Name())
		if err := b.FS.RemoveAll(kubelet.PathKubeletDirectory + "/" + entry.Name()); err != nil {
			return fmt.Errorf("failed removing %q in kubelet dir %q: %w", entry.Name(), kubelet.PathKubeletDirectory, err)
		}
	}

	return nil
}

func unmountKubeletSubFolders(b *botanist.GardenadmBotanist) error {
	// Add trailing '/' to prevent unmounting the actual directory.
	kubletFolderPrefix := kubelet.PathKubeletDirectory + "/"

	mounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return fmt.Errorf("failed reading /proc/mounts: %w", err)
	}

	var errs []error
	for _, mount := range strings.Split(string(mounts), "\n") {
		// Looking for entries like "tmpfs /var/lib/kubelet/pods/..."
		words := strings.Split(mount, " ")
		if len(words) < 2 || !strings.HasPrefix(words[1], kubletFolderPrefix) {
			continue
		}

		b.Logger.Info("Unmounting kubelet sub folder", "path", words[1])
		if err := syscall.Unmount(words[1], 0); err != nil {
			if err == syscall.EINVAL {
				// Entry may have already been unmounted due to duplicate entries => ignore it
				continue
			}

			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed unmounting folders: %w", errors.Join(errs...))
	}

	return nil
}
