// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscmdcontroller "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	extensionsheartbeatcmd "github.com/gardener/gardener/extensions/pkg/controller/heartbeat/cmd"
	extensionscmdwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	localinstall "github.com/gardener/gardener/pkg/provider-local/apis/local/install"
	localbackupbucket "github.com/gardener/gardener/pkg/provider-local/controller/backupbucket"
	localbackupentry "github.com/gardener/gardener/pkg/provider-local/controller/backupentry"
	"github.com/gardener/gardener/pkg/provider-local/controller/backupoptions"
	localbastion "github.com/gardener/gardener/pkg/provider-local/controller/bastion"
	localcontrolplane "github.com/gardener/gardener/pkg/provider-local/controller/controlplane"
	localdnsrecord "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
	localextensionseedcontroller "github.com/gardener/gardener/pkg/provider-local/controller/extension/seed"
	localextensionshootcontroller "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	localhealthcheck "github.com/gardener/gardener/pkg/provider-local/controller/healthcheck"
	localinfrastructure "github.com/gardener/gardener/pkg/provider-local/controller/infrastructure"
	localoperatingsystemconfig "github.com/gardener/gardener/pkg/provider-local/controller/operatingsystemconfig"
	localselfhostedshootexposure "github.com/gardener/gardener/pkg/provider-local/controller/selfhostedshootexposure"
	localworker "github.com/gardener/gardener/pkg/provider-local/controller/worker"
	"github.com/gardener/gardener/pkg/provider-local/local"
	calicoselfhostedshootwebhook "github.com/gardener/gardener/pkg/provider-local/webhook/calicoselfhostedshoot"
	prometheuswebhook "github.com/gardener/gardener/pkg/provider-local/webhook/prometheus"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// NewControllerManagerCommand creates a new command for running a local provider controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		restOpts = &extensionscmdcontroller.RESTOptions{}
		mgrOpts  = &extensionscmdcontroller.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        extensionscmdcontroller.LeaderElectionNameID(local.Name),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:       443,
			WebhookCertDir:          "/tmp/gardener-extensions-cert",
			MetricsBindAddress:      ":8080",
			HealthBindAddress:       ":8081",
		}
		generalOpts = &extensionscmdcontroller.GeneralOptions{}

		// options for the health care controller
		healthCheckCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the bastion controller
		bastionCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the controlplane controller
		controlPlaneCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &localdnsrecord.ControllerOptions{
			MaxConcurrentReconciles: 1,
		}

		// options for the extension controllers
		extensionCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the local backupbucket controller
		localBackupBucketOptions = &backupoptions.ControllerOptions{
			BackupBucketPath:   backupoptions.DefaultBackupPath,
			ContainerMountPath: backupoptions.DefaultContainerMountPath,
		}

		// options for the operatingsystemconfig controller
		operatingSystemConfigCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the infrastructure controller
		infraCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &extensionscmdcontroller.ReconcilerOptions{}

		// options for the worker controller
		workerCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		heartbeatCtrlOptions = &extensionsheartbeatcmd.Options{
			ExtensionName:        local.Name,
			RenewIntervalSeconds: 30,
			Namespace:            os.Getenv("LEADER_ELECTION_NAMESPACE"),
		}

		// options for the webhook server
		webhookServerOptions = &extensionscmdwebhook.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		// options for the prometheus webhook
		prometheusWebhookOptions = &prometheuswebhook.WebhookOptions{}

		controllerSwitches = ControllerSwitchOptions()
		webhookSwitches    = WebhookSwitchOptions()
		webhookOptions     = extensionscmdwebhook.NewAddToManagerOptions(
			local.Name,
			genericactuator.ShootWebhooksResourceName,
			genericactuator.ShootWebhookNamespaceSelector(local.Type),
			generalOpts,
			webhookServerOptions,
			webhookSwitches,
		)

		aggOption = extensionscmdcontroller.NewOptionAggregator(
			restOpts,
			mgrOpts,
			generalOpts,
			extensionscmdcontroller.PrefixOption("bastion-", bastionCtrlOpts),
			extensionscmdcontroller.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			extensionscmdcontroller.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			extensionscmdcontroller.PrefixOption("extension-", extensionCtrlOpts),
			extensionscmdcontroller.PrefixOption("infrastructure-", infraCtrlOpts),
			extensionscmdcontroller.PrefixOption("worker-", workerCtrlOpts),
			extensionscmdcontroller.PrefixOption("backupbucket-", localBackupBucketOptions),
			extensionscmdcontroller.PrefixOption("operatingsystemconfig-", operatingSystemConfigCtrlOpts),
			extensionscmdcontroller.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			extensionscmdcontroller.PrefixOption("heartbeat-", heartbeatCtrlOptions),
			extensionscmdcontroller.PrefixOption("prometheus-", prometheusWebhookOptions),
			controllerSwitches,
			reconcileOpts,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", local.Name),

		RunE: func(_ *cobra.Command, _ []string) error {
			// The calico-self-hosted-shoot webhook is only relevant for self-hosted shoots with unmanaged infrastructure.
			if !generalOpts.SelfHostedShootCluster {
				webhookSwitches.Disabled = append(webhookSwitches.Disabled, calicoselfhostedshootwebhook.WebhookName)
			}

			if err := aggOption.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			if err := heartbeatCtrlOptions.Validate(); err != nil {
				return err
			}

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}
			log := mgr.GetLogger()

			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := localinstall.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := vpaautoscalingv1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := machinev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := druidcorev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := monitoringv1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			// add common meta types to schema for controller-runtime to use v1.ListOptions
			metav1.AddToGroupVersion(scheme, machinev1alpha1.SchemeGroupVersion)

			log.Info("Getting rest config for garden")
			var gardenCluster cluster.Cluster
			if gardenKubeconfigPath := os.Getenv("GARDEN_KUBECONFIG"); gardenKubeconfigPath != "" {
				gardenRESTConfig, err := kubernetes.RESTConfigFromKubeconfigFile(os.Getenv("GARDEN_KUBECONFIG"), kubernetes.AuthTokenFile)
				if err != nil {
					return err
				}

				log.Info("Setting up cluster object for garden")
				gardenCluster, err = cluster.New(gardenRESTConfig, func(opts *cluster.Options) {
					opts.Scheme = kubernetes.GardenScheme
					opts.Logger = log
				})
				if err != nil {
					return fmt.Errorf("failed creating garden cluster object: %w", err)
				}

				log.Info("Setting up checks for garden cluster cache")
				if err := mgr.AddHealthzCheck("garden-informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, gardenCluster.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
					return err
				}
				if err := mgr.AddReadyzCheck("garden-informer-sync", gardenerhealthz.NewCacheSyncHealthz(gardenCluster.GetCache())); err != nil {
					return err
				}

				seedName, shootName, shootNamespace := os.Getenv("SEED_NAME"), os.Getenv("SHOOT_NAME"), os.Getenv("SHOOT_NAMESPACE")

				log.Info("Adding garden cluster to manager")
				if err := mgr.Add(gardenCluster); err != nil {
					return fmt.Errorf("failed adding garden cluster to manager: %w", err)
				}

				if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
					// Use the API reader to avoid triggering a cluster-scoped list/watch for Shoots in the cache.
					// The shoot authorizer only allows name-scoped reads for the gardenlet's own Shoot.
					return verifyGardenAccess(ctx, log, gardenCluster.GetAPIReader(), gardenCluster.GetClient(), seedName, shootName, shootNamespace)
				})); err != nil {
					return fmt.Errorf("could not add garden runnable to manager: %w", err)
				}
			}

			log.Info("Adding controllers to manager")
			// Use the Apply functionality to convey parameters to the controller configs.
			bastionCtrlOpts.Completed().Apply(&localbastion.DefaultAddOptions.Controller)
			controlPlaneCtrlOpts.Completed().Apply(&localcontrolplane.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&localdnsrecord.DefaultAddOptions)
			extensionCtrlOpts.Completed().Apply(&localextensionseedcontroller.DefaultAddOptions.Controller)
			extensionCtrlOpts.Completed().Apply(&localextensionshootcontroller.DefaultAddOptions.Controller)
			healthCheckCtrlOpts.Completed().Apply(&localhealthcheck.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&localinfrastructure.DefaultAddOptions.Controller)
			operatingSystemConfigCtrlOpts.Completed().Apply(&localoperatingsystemconfig.DefaultAddOptions.Controller)
			workerCtrlOpts.Completed().Apply(&localworker.DefaultAddOptions.Controller)
			localBackupBucketOptions.Completed().Apply(&localbackupbucket.DefaultAddOptions)
			localBackupBucketOptions.Completed().Apply(&localbackupentry.DefaultAddOptions)
			heartbeatCtrlOptions.Completed().Apply(&heartbeat.DefaultAddOptions)
			prometheusWebhookOptions.Completed().Apply(&prometheuswebhook.DefaultAddOptions)

			// Apply remaining configs manually.
			localworker.DefaultAddOptions.GardenCluster = gardenCluster
			applyGeneralOptions(generalOpts)
			applyReconcileOptions(reconcileOpts)

			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				return fmt.Errorf("could not add healthcheck: %w", err)
			}
			if err := mgr.AddHealthzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, mgr.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
				return err
			}
			if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
				return fmt.Errorf("could not add readycheck for informers: %w", err)
			}
			if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
				return fmt.Errorf("could not add readycheck of webhook to manager: %w", err)
			}

			atomicShootWebhookConfig, err := webhookOptions.Completed().AddToManager(ctx, mgr, nil)
			if err != nil {
				return fmt.Errorf("could not add webhooks to manager: %w", err)
			}
			localcontrolplane.DefaultAddOptions.ShootWebhookConfig = atomicShootWebhookConfig
			localcontrolplane.DefaultAddOptions.WebhookServerNamespace = webhookOptions.Server.Namespace

			if err := controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
				return fmt.Errorf("could not add controllers to manager: %w", err)
			}

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("error running manager: %w", err)
			}

			return nil
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}

func applyReconcileOptions(reconcileOpts *extensionscmdcontroller.ReconcilerOptions) {
	config := reconcileOpts.Completed()

	localbackupbucket.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localbackupentry.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localbastion.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localcontrolplane.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localextensionseedcontroller.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localextensionshootcontroller.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localdnsrecord.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localoperatingsystemconfig.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localselfhostedshootexposure.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
	localworker.DefaultAddOptions.IgnoreOperationAnnotation = config.IgnoreOperationAnnotation
}

func applyGeneralOptions(generalOpts *extensionscmdcontroller.GeneralOptions) {
	config := generalOpts.Completed()

	localworker.DefaultAddOptions.SelfHostedShootCluster = config.SelfHostedShootCluster

	localbackupbucket.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localbackupentry.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localbastion.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localcontrolplane.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localextensionseedcontroller.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localextensionshootcontroller.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localdnsrecord.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localinfrastructure.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localoperatingsystemconfig.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localselfhostedshootexposure.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localworker.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
	localhealthcheck.DefaultAddOptions.ExtensionClasses = slices.Clone(config.ExtensionClasses)
}

// verifyGardenAccess uses the extension's access to the garden cluster to request objects related to the seed it is
// running on, but doesn't do anything useful with the objects. We do this for verifying the extension's garden access
// in e2e tests. If something fails in this runnable, the extension will crash loop.
func verifyGardenAccess(ctx context.Context, log logr.Logger, reader client.Reader, writer client.Writer, seedName, shootName, shootNamespace string) error {
	log = log.WithName("garden-access")

	var objects []client.Object
	if seedName != "" {
		objects = append(objects, &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seedName}})
	}
	if shootName != "" {
		objects = append(objects, &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: shootNamespace}})
	}

	for _, obj := range objects {
		objectKey := client.ObjectKeyFromObject(obj)
		log.Info("Reading and annotating object", "objectKey", objectKey, "kind", fmt.Sprintf("%T", obj))
		if err := reader.Get(ctx, objectKey, obj); err != nil {
			return fmt.Errorf("failed reading %T %s: %w", obj, objectKey, err)
		}

		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		kubernetesutils.SetMetaDataAnnotation(obj, "provider-local-e2e-test-garden-access", time.Now().UTC().Format(time.RFC3339))
		if err := writer.Patch(ctx, obj, patch); err != nil {
			return fmt.Errorf("failed annotating %T %s: %w", obj, objectKey, err)
		}
	}

	log.Info("Garden access successfully verified")
	return nil
}
