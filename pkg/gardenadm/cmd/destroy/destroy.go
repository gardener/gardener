// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/gardenadm"
	gardenadmbotanist "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the infrastructure of a Self-Hosted Shoot Cluster",
		Long: `Destroy the infrastructure of a Self-Hosted Shoot Cluster (machines, networks, etc.) that was created with
"gardenadm bootstrap".

Like "gardenadm bootstrap", this command deploys the provider extensions and machine-controller-manager to a
temporary bootstrap cluster. It restores the Infrastructure and Worker resources from the ShootState in the config
directory and deletes them, which deletes all machines and the infrastructure in the cloud provider account.

The config directory must contain the same manifests that were used for "gardenadm bootstrap".

The current ShootState of the self-hosted shoot cluster is required to restore the machines and the infrastructure.
The recommended way is to pass the kubeconfig of the running self-hosted shoot cluster (--shoot-kubeconfig): the
command then stops cluster-autoscaler and machine-controller-manager in the shoot cluster, so that they don't recreate
the machines, and computes the ShootState from the shoot cluster. A ShootState in the config directory is ignored in
this case.
Before deleting the machines, the command also cleans up the shoot cluster on a best-effort basis, like the deletion
of a Gardener-managed shoot cluster: it deletes webhooks, CRDs, APIServices, and workload resources (e.g., Services,
PersistentVolumeClaims, Deployments, and Pods), so that the cloud provider resources created for them (e.g., load
balancers and volumes) are deleted. The control plane of the self-hosted shoot cluster runs in the kube-system,
garden, and extension namespaces, hence these namespaces are skipped, as well as webhooks and APIServices served from
them and protected CRDs (gardener.cloud/deletion-protected=true). If you deployed workloads to these namespaces,
delete them yourself before running this command.

Without --shoot-kubeconfig, the ShootState is read from the config directory. Note that the ShootState copied to the
control plane machines by "gardenadm bootstrap" is outdated as soon as the shoot cluster has changed its machines
(e.g., created the machines of other worker pools). Also, the self-hosted shoot cluster must not be running anymore,
or at least its cluster-autoscaler and machine-controller-manager must be stopped. Otherwise, they recreate the
machines that are deleted by this command.

Limitations:
- Only the machines and the infrastructure are destroyed. Other resources are left behind, e.g., the DNS record of
  the API server and the load balancer exposing the API server. Without --shoot-kubeconfig, also the resources created
  by workloads in the cluster (load balancers of Services, volumes of PersistentVolumeClaims, etc.) are left behind.
- Machines that are missing in the ShootState (e.g., because it is outdated) are not deleted.
- Re-running the command after machines or parts of the infrastructure have been deleted might create them again,
  because they are restored from the ShootState before they are deleted.`,

		Example: `# Destroy the infrastructure, computing the current ShootState from the self-hosted shoot cluster (recommended)
gardenadm destroy --config-dir /path/to/manifests --shoot-kubeconfig /path/to/shoot-kubeconfig

# Destroy the infrastructure, with the ShootState in the config directory
gardenadm destroy --config-dir /path/to/manifests`,

		Args: cobra.NoArgs,

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

// NewClientSetFromFile is an alias for botanist.NewClientSetFromFile.
// Exposed for unit testing.
var NewClientSetFromFile = gardenadmbotanist.NewClientSetFromFile

func run(ctx context.Context, opts *Options) error {
	resources, err := gardenadm.ReadManifests(opts.Log, gardenadmbotanist.DirFS(opts.ConfigDir))
	if err != nil {
		return fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", opts.ConfigDir, err)
	}

	if !v1beta1helper.HasManagedInfrastructure(resources.Shoot) {
		return fmt.Errorf("shoot has unmanaged infrastructure, there is nothing to destroy: " +
			"use `gardenadm reset` on the nodes instead")
	}
	clientSet, err := NewClientSetFromFile(opts.Kubeconfig, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	if err := cmd.EnsureNoGardenletOrOperator(ctx, clientSet.Client(), "destroy"); err != nil {
		return err
	}
	if err := ensureNotSelfHostedShoot(ctx, clientSet.Client()); err != nil {
		return err
	}

	var shootCleanupBotanist *botanist.Botanist
	if opts.ShootKubeconfig != "" {
		if resources.ShootState != nil {
			opts.Log.Info("Ignoring the ShootState in the config directory because --shoot-kubeconfig is set")
		}

		shootClientSet, err := NewClientSetFromFile(opts.ShootKubeconfig, kubernetes.SeedScheme)
		if err != nil {
			return fmt.Errorf("failed creating client for the shoot cluster: %w", err)
		}

		if err := stopMachineControllers(ctx, opts.Log, shootClientSet.Client()); err != nil {
			return err
		}

		// The cleanup lists and deletes resources of the shoot scheme (e.g., APIServices and VolumeSnapshots).
		shootCleanupClientSet, err := NewClientSetFromFile(opts.ShootKubeconfig, kubernetes.ShootScheme)
		if err != nil {
			return fmt.Errorf("failed creating client for cleaning up the shoot cluster: %w", err)
		}
		shootCleanupBotanist = newShootCleanupBotanist(opts.Log, shootCleanupClientSet, resources.Shoot)

		opts.Log.Info("Computing ShootState from the shoot cluster")
		resources.ShootState, err = shootstate.Compute(ctx, shootClientSet.Client(), resources.Shoot, v1beta1helper.ControlPlaneNamespaceForShoot(resources.Shoot))
		if err != nil {
			return fmt.Errorf("failed computing ShootState from the shoot cluster: %w", err)
		}
	}
	if resources.ShootState == nil {
		return fmt.Errorf("ShootState is missing in the config directory: " +
			"the state of the infrastructure and machines is required to destroy them, " +
			"either put it into the config directory or set --shoot-kubeconfig")
	}
	if resources.ShootState.Name != resources.Shoot.Name || resources.ShootState.Namespace != resources.Shoot.Namespace {
		return fmt.Errorf("ShootState %s does not belong to Shoot %s: their names and namespaces must match",
			client.ObjectKeyFromObject(resources.ShootState), client.ObjectKeyFromObject(resources.Shoot))
	}

	if err := removeNodeLabelsFromMachineState(resources.ShootState); err != nil {
		return err
	}

	// Instruct the botanist to restore the extension resources from the ShootState, like after a control plane
	// migration. The machine objects are restored as well, so that machine-controller-manager can delete the machines.
	resources.Shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type: gardencorev1beta1.LastOperationTypeRestore,
	}

	b, err := gardenadmbotanist.NewGardenadmBotanistFromResources(ctx, opts.Log, clientSet, resources, false)
	if err != nil {
		return err
	}
	// Like in `gardenadm bootstrap`, the botanist doesn't run the control plane, so its Worker only contains the
	// control plane worker pool reduced to a single machine. The Worker actuator only restores the machines of the
	// worker pools in the Worker, so use all worker pools to restore and delete all machines.
	b.Shoot.Components.Extensions.Worker, err = b.WorkerWithAllPools()
	if err != nil {
		return err
	}

	var (
		g        = flow.NewGraph("destroy")
		reporter = flow.NewCommandLineProgressReporter(opts.ErrOut)

		syncPointBootstrapped = b.AddBootstrapClusterTasks(g)

		restoreInfrastructure = g.AddGroup(
			b.ReconcileInfrastructureTaskGroup(false).
				WithDependencies(syncPointBootstrapped),
		)
		reconcileOperatingSystemConfig = g.AddGroup(
			b.ReconcileOperatingSystemConfigTaskGroup(false).
				WithDependencies(syncPointBootstrapped),
		)

		// Like in the deletion of hosted shoots, clean up the resources in the shoot cluster before deleting the
		// machines, so that the cloud provider resources created for them (e.g., load balancers of Services and volumes
		// of PersistentVolumeClaims) are deleted by the control plane components running on the machines.
		cleanupWebhooks = g.Add(flow.Task{
			Name:   "Cleaning up webhooks in the shoot cluster",
			Fn:     flow.TaskFn(func(ctx context.Context) error { return shootCleanupBotanist.CleanWebhooks(ctx) }).Timeout(10 * time.Minute),
			SkipIf: shootCleanupBotanist == nil,
		})
		cleanExtendedAPIs = g.Add(flow.Task{
			Name:         "Cleaning extended API groups in the shoot cluster",
			Fn:           flow.TaskFn(func(ctx context.Context) error { return shootCleanupBotanist.CleanExtendedAPIs(ctx) }).Timeout(10 * time.Minute),
			SkipIf:       shootCleanupBotanist == nil || metav1.HasAnnotation(resources.Shoot.ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup),
			Dependencies: flow.NewTaskIDs(cleanupWebhooks),
		})
		cleanKubernetesResources = g.Add(flow.Task{
			Name:         "Cleaning Kubernetes resources in the shoot cluster",
			Fn:           flow.TaskFn(func(ctx context.Context) error { return shootCleanupBotanist.CleanKubernetesResources(ctx) }).Timeout(10 * time.Minute),
			SkipIf:       shootCleanupBotanist == nil,
			Dependencies: flow.NewTaskIDs(cleanExtendedAPIs),
		})

		// The machine-controller-manager is deployed only after the Worker extension has restored the machines and the
		// shoot cluster has been cleaned up. On start, it deletes the VMs without a Machine object as orphans, e.g., when
		// the bootstrap cluster still contains the MachineClasses of `gardenadm bootstrap`. This would delete the
		// machines before they are restored, and while the control plane on the machines still cleans up the resources of
		// the shoot cluster. Hence, the tasks of the Worker and machine-controller-manager task groups are added
		// explicitly, as the Worker task group depends on the machine-controller-manager task group.
		restoreWorker = g.Add(flow.Task{
			Name:         "Restoring shoot workers",
			Fn:           b.DeployWorker,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped, restoreInfrastructure, reconcileOperatingSystemConfig),
		})
		waitUntilMachinesRestored = g.Add(flow.Task{
			Name:         "Waiting until the machines of the shoot workers have been restored",
			Fn:           b.Shoot.Components.Extensions.Worker.WaitUntilWorkerStatusMachineDeploymentsUpdated,
			Dependencies: flow.NewTaskIDs(restoreWorker),
		})
		deployMachineControllerManager = g.Add(flow.Task{
			Name:         "Deploying machine-controller-manager",
			Fn:           flow.TaskFn(b.DeployMachineControllerManager).RetryUntilTimeout(time.Second, time.Minute),
			Dependencies: flow.NewTaskIDs(waitUntilMachinesRestored, cleanKubernetesResources),
		})
		waitUntilWorkerRestored = g.Add(flow.Task{
			Name:         "Waiting until shoot workers have been restored",
			Fn:           b.Shoot.Components.Extensions.Worker.Wait,
			Dependencies: flow.NewTaskIDs(deployMachineControllerManager),
		})

		// machine-controller-manager runs without a target cluster in the bootstrap cluster (see
		// `gardenadm bootstrap`), so the machines are deleted without draining the nodes.
		destroyWorker = g.Add(flow.Task{
			Name:         "Destroying shoot workers",
			Fn:           component.OpDestroyAndWait(b.Shoot.Components.Extensions.Worker).Destroy,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerRestored),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting machine-controller-manager",
			Fn:           component.OpDestroyAndWait(b.Shoot.Components.ControlPlane.MachineControllerManager).Destroy,
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying shoot infrastructure",
			Fn:           component.OpDestroyAndWait(b.Shoot.Components.Extensions.Infrastructure).Destroy,
			Dependencies: flow.NewTaskIDs(restoreInfrastructure, destroyWorker),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              opts.Log,
		ProgressReporter: reporter,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprint(opts.Out, `
The machines and the infrastructure of the self-hosted shoot cluster have been destroyed.

Other resources are not destroyed by this command yet, e.g., the DNS record of the API server
and the load balancer exposing the API server.
`)
	if shootCleanupBotanist == nil {
		fmt.Fprint(opts.Out, `The resources created by workloads in the cluster (load balancers of Services, volumes of
PersistentVolumeClaims, etc.) are left behind as well, because --shoot-kubeconfig was not set.
`)
	}
	fmt.Fprint(opts.Out, `
You can delete the bootstrap cluster now.
`)

	return nil
}
