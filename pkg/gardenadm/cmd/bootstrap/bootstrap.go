// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/publicip"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{
		Options:          globalOpts,
		PublicIPDetector: publicip.IpifyDetector{},
	}

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the infrastructure for a Self-Hosted Shoot Cluster",
		Long:  "Bootstrap the infrastructure for a Self-Hosted Shoot Cluster (networks, machines, etc.)",

		Example: `# Bootstrap the infrastructure
gardenadm bootstrap --config-dir /path/to/manifests`,

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
var NewClientSetFromFile = botanist.NewClientSetFromFile

func run(ctx context.Context, opts *Options) error {
	clientSet, err := NewClientSetFromFile(opts.Kubeconfig, kubernetes.SeedScheme)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	if err := ensureNoGardenletOrOperator(ctx, clientSet.Client()); err != nil {
		return err
	}

	b, err := botanist.NewGardenadmBotanistFromManifests(ctx, opts.Log, clientSet, opts.ConfigDir, false)
	if err != nil {
		return err
	}

	hasMigratedExtensionKind, err := getMigratedExtensionKinds(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace)
	if err != nil {
		return fmt.Errorf("failed determining migrated extension kinds: %w", err)
	}

	var (
		g = flow.NewGraph("bootstrap")

		deployNamespace = g.Add(flow.Task{
			Name: "Deploying control plane namespace",
			Fn:   b.DeployControlPlaneNamespace,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           b.DeployCloudProviderSecret,
			SkipIf:       b.Shoot.Credentials == nil,
			Dependencies: flow.NewTaskIDs(deployNamespace),
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
		deployPriorityClassCritical = g.Add(flow.Task{
			Name:         "Deploying PriorityClass for gardener-resource-manager",
			Fn:           b.DeployPriorityClassCritical,
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager",
			Fn:           b.Shoot.Components.ControlPlane.ResourceManager.Deploy,
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement, deployPriorityClassCritical),
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
		deployExtensionControllers = g.Add(flow.Task{
			Name: "Deploying extension controllers",
			Fn: func(ctx context.Context) error {
				return b.ReconcileExtensionControllerInstallations(ctx, false)
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
		syncPointBootstrapped = flow.NewTaskIDs(
			deployNetworkPolicies,
			waitUntilGardenerResourceManagerReady,
			waitUntilExtensionControllersReady,
		)

		deployInfrastructure = g.Add(flow.Task{
			Name:         "Deploying Shoot infrastructure",
			Fn:           b.DeployInfrastructure,
			SkipIf:       hasMigratedExtensionKind[extensionsv1alpha1.InfrastructureResource],
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})
		waitUntilInfrastructureReady = g.Add(flow.Task{
			Name:         "Waiting until Shoot infrastructure has been reconciled",
			Fn:           b.WaitForInfrastructure,
			SkipIf:       hasMigratedExtensionKind[extensionsv1alpha1.InfrastructureResource],
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})

		deployOperatingSystemConfig = g.Add(flow.Task{
			Name:         "Deploying OperatingSystemConfig for control plane machines",
			Fn:           b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})
		waitUntilOperatingSystemConfigReady = g.Add(flow.Task{
			Name:         "Waiting until OperatingSystemConfig for control plane machines has been reconciled",
			Fn:           b.Shoot.Components.Extensions.OperatingSystemConfig.Wait,
			Dependencies: flow.NewTaskIDs(deployOperatingSystemConfig),
		})

		deployMachineControllerManager = g.Add(flow.Task{
			Name:         "Deploying machine-controller-manager",
			Fn:           b.DeployMachineControllerManager,
			Dependencies: flow.NewTaskIDs(syncPointBootstrapped),
		})

		deployWorker = g.Add(flow.Task{
			Name:         "Deploying control plane machines",
			Fn:           b.DeployWorker,
			SkipIf:       hasMigratedExtensionKind[extensionsv1alpha1.WorkerResource],
			Dependencies: flow.NewTaskIDs(waitUntilInfrastructureReady, waitUntilOperatingSystemConfigReady, deployMachineControllerManager),
		})
		waitUntilWorkerReady = g.Add(flow.Task{
			Name:         "Waiting until control plane machines have been deployed",
			Fn:           b.Shoot.Components.Extensions.Worker.Wait,
			SkipIf:       hasMigratedExtensionKind[extensionsv1alpha1.WorkerResource],
			Dependencies: flow.NewTaskIDs(deployWorker),
		})
		listControlPlaneMachines = g.Add(flow.Task{
			Name:         "Listing control plane machines",
			Fn:           b.ListControlPlaneMachines,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady),
		})

		// Scale down machine-controller-manager to prevent it from interfering with Machine objects that will be migrated
		// to the self-hosted shoot. Scaling down instead of deleting it, allows operators/developers to simply scale it up
		// again in case they need to redeploy a control plane machine manually because of errors.
		scaleDownMachineControllerManager = g.Add(flow.Task{
			Name: "Scaling down machine-controller-manager",
			Fn: func(ctx context.Context) error {
				b.Shoot.Components.ControlPlane.MachineControllerManager.SetReplicas(0)
				return component.OpWait(b.Shoot.Components.ControlPlane.MachineControllerManager).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady),
		})

		deployDNSRecord = g.Add(flow.Task{
			Name:         "Deploying DNSRecord pointing to the first control plane machine",
			Fn:           b.DeployBootstrapDNSRecord,
			SkipIf:       hasMigratedExtensionKind[extensionsv1alpha1.DNSRecordResource],
			Dependencies: flow.NewTaskIDs(listControlPlaneMachines),
		})

		// In contrast to the usual Shoot migrate flow, we don't delete the extension objects after executing the migrate
		// operation. The extension controllers are supposed to skip any reconcile operation if the last operation is of
		// type "Migrate". Also, this makes it easier to allow re-running `gardenadm bootstrap` in case of failures
		// down the line. If we deleted the extension objects, we would need to restore them when re-running the flow.
		migrateExtensionResources = g.Add(flow.Task{
			Name: "Preparing extension resources for migration to self-hosted shoot",
			Fn: flow.Parallel(
				component.MigrateAndWait(b.Shoot.Components.Extensions.Infrastructure),
				component.MigrateAndWait(b.Shoot.Components.Extensions.Worker),
				component.MigrateAndWait(b.Shoot.Components.Extensions.ExternalDNSRecord),
			),
			Dependencies: flow.NewTaskIDs(scaleDownMachineControllerManager, deployDNSRecord),
		})

		// In contrast to a usual Shoot control plane migration, there is no garden cluster where the ShootState is stored.
		// In this flow, the ShootState is only stored in memory (in the fake garden client). This is sufficient for this
		// use case as we can copy it to the control plane machines. If we lose the ShootState (e.g., re-run of the flow)
		// we can re-construct the ShootState from the objects in the bootstrap cluster.
		compileShootState = g.Add(flow.Task{
			Name: "Compiling ShootState",
			Fn: func(ctx context.Context) error {
				return shootstate.Deploy(ctx, b.Clock, b.GardenClient, b.SeedClientSet.Client(), b.Shoot.GetInfo(), false)
			},
			Dependencies: flow.NewTaskIDs(migrateExtensionResources),
		})

		deployBastion = g.Add(flow.Task{
			Name: "Deploying and connecting to bastion host",
			Fn: func(ctx context.Context) error {
				b.Bastion.Values.IngressCIDRs = opts.BastionIngressCIDRs
				return component.OpWait(b.Bastion).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(waitUntilInfrastructureReady),
		})
		// TODO(timebertt): destroy Bastion after successfully bootstrapping the control plane

		connectToMachine = g.Add(flow.Task{
			Name:         "Connecting to the first control plane machine",
			Fn:           flow.TaskFn(b.ConnectToControlPlaneMachine).RetryUntilTimeout(5*time.Second, 5*time.Minute),
			Dependencies: flow.NewTaskIDs(listControlPlaneMachines, deployBastion),
		})
		copyManifests = g.Add(flow.Task{
			Name: "Copying manifests to the first control plane machine",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.CopyManifests(ctx, os.DirFS(opts.ConfigDir))
			}).Timeout(time.Minute),
			Dependencies: flow.NewTaskIDs(connectToMachine, compileShootState),
		})

		bootstrapControlPlane = g.Add(flow.Task{
			Name: "Bootstrapping control plane on the first control plane machine",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.SSHConnection().RunWithStreams(ctx, nil, opts.Out, opts.ErrOut,
					fmt.Sprintf("%s=%q /opt/bin/gardenadm init -d %q --log-level=%s",
						imagevector.OverrideEnv, botanist.ImageVectorOverrideFile, botanist.ManifestsDir, opts.LogLevel,
					),
				)
			}).Timeout(30 * time.Minute),
			Dependencies: flow.NewTaskIDs(deployDNSRecord, copyManifests),
		})

		fetchKubeconfig = g.Add(flow.Task{
			Name: "Fetching kubeconfig from the first control plane machine",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.FetchKubeconfig(ctx, opts.KubeconfigWriter)
			}).Timeout(time.Minute),
			Dependencies: flow.NewTaskIDs(bootstrapControlPlane),
		})

		_ = fetchKubeconfig

		// In contrast to the usual Shoot migrate flow, we don't delete the shoot control plane namespace at the end.
		// The bootstrap cluster is designed to be temporary and thrown away after successfully executing
		// `gardenadm bootstrap`. Correctly deleting the control plane namespace would need the correct order and would
		// still orphan some global resources. We spare the effort of implementing this cleanup and instruct users to
		// throw away the bootstrap cluster afterward.
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprintf(opts.Out, `
Warning: this command is work in progress.

For now, you can connect to the self-hosted Shoot cluster control-plane by
fetching the kubeconfig from the secret "%[1]s/kubeconfig"
on the bootstrap cluster:

  kubectl get secret -n %[1]s kubeconfig -o jsonpath='{.data.kubeconfig}' | base64 --decode > %[1]s-kubeconfig.yaml
  export KUBECONFIG=$PWD/%[1]s-kubeconfig.yaml
  kubectl get nodes

Note that the API server of the self-hosted Shoot cluster control-plane might
not be accessible from your current machine.
`, b.Shoot.GetInfo().Status.TechnicalID)
	return nil
}

// ensureNoGardenletOrOperator is a safety check that prevents operators from accidentally executing
// `gardenadm bootstrap` on a cluster that is already used as a runtime cluster with gardener-operator or as a seed
// cluster. Doing so would lead to conflicts when `gardenadm bootstrap` starts deploying components like provider
// extensions.
func ensureNoGardenletOrOperator(ctx context.Context, c client.Reader) error {
	for _, key := range []client.ObjectKey{
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardener-operator"},
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet"},
	} {
		if err := c.Get(ctx, key, &appsv1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed checking if %q deployment exists: %w", key, err)
		}

		return fmt.Errorf("deployment %q exists on the targeted cluster. "+
			"`gardenadm bootstrap` does not support targeting a cluster that is already used as a runtime cluster with gardener-operator or as a seed cluster. "+
			"Please consult the gardenadm documentation", key)
	}

	return nil
}

// getMigratedExtensionKinds returns a map of all extension kinds that will eventually be migrated in the `gardenadm
// bootstrap` flow. If at least one of the extension objects in the given namespace has the last operation type Migrate,
// the map value will be true for this kind.
// This is used to skip the extension reconciliation when re-running the flow after starting the extension migration.
func getMigratedExtensionKinds(ctx context.Context, c client.Reader, namespace string) (map[string]bool, error) {
	relevantExtensionKinds := map[string]client.ObjectList{
		extensionsv1alpha1.InfrastructureResource: &extensionsv1alpha1.InfrastructureList{},
		extensionsv1alpha1.WorkerResource:         &extensionsv1alpha1.WorkerList{},
		extensionsv1alpha1.DNSRecordResource:      &extensionsv1alpha1.DNSRecordList{},
	}

	out := make(map[string]bool, len(relevantExtensionKinds))
	for kind, list := range relevantExtensionKinds {
		if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
			if meta.IsNoMatchError(err) {
				out[kind] = false
				continue
			}
			return nil, fmt.Errorf("error listing %s objects: %w", kind, err)
		}

		hasMigrated := false
		if err := meta.EachListItem(list, func(obj runtime.Object) error {
			extensionObject, err := extensions.Accessor(obj)
			if err != nil {
				return err
			}

			lastOperation := extensionObject.GetExtensionStatus().GetLastOperation()
			if lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate {
				hasMigrated = true
			}
			return nil
		}); err != nil {
			return nil, err
		}

		out[kind] = hasMigrated
	}

	return out, nil
}
