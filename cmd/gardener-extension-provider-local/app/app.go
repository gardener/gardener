// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
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
	localcontrolplane "github.com/gardener/gardener/pkg/provider-local/controller/controlplane"
	localdnsrecord "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
	localextensionshootcontroller "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	localhealthcheck "github.com/gardener/gardener/pkg/provider-local/controller/healthcheck"
	localinfrastructure "github.com/gardener/gardener/pkg/provider-local/controller/infrastructure"
	localingress "github.com/gardener/gardener/pkg/provider-local/controller/ingress"
	localoperatingsystemconfig "github.com/gardener/gardener/pkg/provider-local/controller/operatingsystemconfig"
	localservice "github.com/gardener/gardener/pkg/provider-local/controller/service"
	localworker "github.com/gardener/gardener/pkg/provider-local/controller/worker"
	"github.com/gardener/gardener/pkg/provider-local/local"
	prometheuswebhook "github.com/gardener/gardener/pkg/provider-local/webhook/prometheus"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var hostIP string

func init() {
	addrs, err := net.InterfaceAddrs()
	utilruntime.Must(err)

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				hostIP = ipnet.IP.String()
				break
			}
		}
	}
}

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

		// options for the controlplane controller
		controlPlaneCtrlOpts = &extensionscmdcontroller.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &localdnsrecord.ControllerOptions{
			MaxConcurrentReconciles: 1,
		}

		// options for the ingress controller
		ingressCtrlOpts = &localingress.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the service controller
		serviceCtrlOpts = &localservice.ControllerOptions{
			MaxConcurrentReconciles: 5,
			HostIP:                  hostIP,
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
			webhookServerOptions,
			webhookSwitches,
		)

		aggOption = extensionscmdcontroller.NewOptionAggregator(
			restOpts,
			mgrOpts,
			generalOpts,
			extensionscmdcontroller.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			extensionscmdcontroller.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			extensionscmdcontroller.PrefixOption("infrastructure-", infraCtrlOpts),
			extensionscmdcontroller.PrefixOption("worker-", workerCtrlOpts),
			extensionscmdcontroller.PrefixOption("ingress-", ingressCtrlOpts),
			extensionscmdcontroller.PrefixOption("service-", serviceCtrlOpts),
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
			seedName := os.Getenv("SEED_NAME")

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

				log.Info("Adding garden cluster to manager")
				if err := mgr.Add(gardenCluster); err != nil {
					return fmt.Errorf("failed adding garden cluster to manager: %w", err)
				}

				if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
					return verifyGardenAccess(ctx, log, gardenCluster.GetClient(), seedName)
				})); err != nil {
					return fmt.Errorf("could not add garden runnable to manager: %w", err)
				}
			}

			log.Info("Adding controllers to manager")
			controlPlaneCtrlOpts.Completed().Apply(&localcontrolplane.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&localdnsrecord.DefaultAddOptions)
			healthCheckCtrlOpts.Completed().Apply(&localhealthcheck.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&localinfrastructure.DefaultAddOptions.Controller)
			operatingSystemConfigCtrlOpts.Completed().Apply(&localoperatingsystemconfig.DefaultAddOptions.Controller)
			ingressCtrlOpts.Completed().Apply(&localingress.DefaultAddOptions)
			serviceCtrlOpts.Completed().Apply(&localservice.DefaultAddOptions)
			workerCtrlOpts.Completed().Apply(&localworker.DefaultAddOptions.Controller)
			localworker.DefaultAddOptions.GardenCluster = gardenCluster
			localworker.DefaultAddOptions.AutonomousShootCluster = generalOpts.Completed().AutonomousShootCluster
			localBackupBucketOptions.Completed().Apply(&localbackupbucket.DefaultAddOptions)
			localBackupBucketOptions.Completed().Apply(&localbackupentry.DefaultAddOptions)
			heartbeatCtrlOptions.Completed().Apply(&heartbeat.DefaultAddOptions)
			prometheusWebhookOptions.Completed().Apply(&prometheuswebhook.DefaultAddOptions)

			reconcileOpts.Completed().Apply(&localbackupbucket.DefaultAddOptions.IgnoreOperationAnnotation, &localbackupbucket.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&localcontrolplane.DefaultAddOptions.IgnoreOperationAnnotation, &localcontrolplane.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&localdnsrecord.DefaultAddOptions.IgnoreOperationAnnotation, &localdnsrecord.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&localinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation, &localinfrastructure.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&localoperatingsystemconfig.DefaultAddOptions.IgnoreOperationAnnotation, &localoperatingsystemconfig.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&localworker.DefaultAddOptions.IgnoreOperationAnnotation, &localworker.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(nil, &localhealthcheck.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&localextensionshootcontroller.DefaultAddOptions.IgnoreOperationAnnotation, &localextensionshootcontroller.DefaultAddOptions.ExtensionClass)

			if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
				return fmt.Errorf("could not add readycheck for informers: %w", err)
			}
			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				return fmt.Errorf("could not add healthcheck: %w", err)
			}
			if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
				return fmt.Errorf("could not add readycheck of webhook to manager: %w", err)
			}

			atomicShootWebhookConfig, err := webhookOptions.Completed().AddToManager(ctx, mgr, nil, generalOpts.Completed().AutonomousShootCluster)
			if err != nil {
				return fmt.Errorf("could not add webhooks to manager: %w", err)
			}
			localcontrolplane.DefaultAddOptions.ShootWebhookConfig = atomicShootWebhookConfig
			localcontrolplane.DefaultAddOptions.WebhookServerNamespace = webhookOptions.Server.Namespace

			// Send empty patches on start-up to trigger webhooks
			if !webhookSwitches.Completed().Disabled {
				if err := mgr.Add(&webhookTriggerer{client: mgr.GetClient()}); err != nil {
					return fmt.Errorf("error adding runnable for triggering DNS config webhook: %w", err)
				}
			}

			if err := controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
				return fmt.Errorf("could not add controllers to manager: %w", err)
			}

			log.Info("Started with", "hostIP", serviceCtrlOpts.HostIP)

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("error running manager: %w", err)
			}

			return nil
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}

// verifyGardenAccess uses the extension's access to the garden cluster to request objects related to the seed it is
// running on, but doesn't do anything useful with the objects. We do this for verifying the extension's garden access
// in e2e tests. If something fails in this runnable, the extension will crash loop.
func verifyGardenAccess(ctx context.Context, log logr.Logger, c client.Client, seedName string) error {
	log = log.WithName("garden-access").WithValues("seedName", seedName)

	log.Info("Reading Seed")
	// NB: reading seeds is allowed by gardener.cloud:system:read-global-resources (bound to all authenticated users)
	seed := &gardencorev1beta1.Seed{}
	if err := c.Get(ctx, client.ObjectKey{Name: seedName}, seed); err != nil {
		return fmt.Errorf("failed reading seed %s: %w", seedName, err)
	}

	log.Info("Annotating Seed")
	patch := client.MergeFrom(seed.DeepCopy())
	metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "provider-local-e2e-test-garden-access", time.Now().UTC().Format(time.RFC3339))
	if err := c.Patch(ctx, seed, patch); err != nil {
		return fmt.Errorf("failed annotating seed %s: %w", seedName, err)
	}

	log.Info("Garden access successfully verified")
	return nil
}

type webhookTriggerer struct {
	client client.Client
}

func (w *webhookTriggerer) NeedLeaderElection() bool {
	return true
}

func (w *webhookTriggerer) Start(ctx context.Context) error {
	// Wait for the reconciler to populate the webhook CA into the configurations before triggering the webhooks.
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		webhookConfig := &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "gardener-extension-" + local.Name}}
		if err := w.client.Get(ctx, client.ObjectKeyFromObject(webhookConfig), webhookConfig); err != nil {
			if !apierrors.IsNotFound(err) {
				return retry.SevereError(err)
			}
			return retry.MinorError(fmt.Errorf("webhook was not yet created"))
		}

		for _, webhook := range webhookConfig.Webhooks {
			// We can return when we find the first webhook w/o CA bundle since the reconciler would populate it into
			// all webhooks at the same time.
			if len(webhook.ClientConfig.CABundle) == 0 {
				return retry.MinorError(fmt.Errorf("CA bundle was not yet populated to all webhooks"))
			}
		}

		return retry.Ok()
	}); err != nil {
		return err
	}

	if err := w.trigger(ctx, w.client, nil, w.client.Status(), &corev1.NodeList{}); err != nil {
		return err
	}

	return w.trigger(ctx, w.client, w.client, nil, &appsv1.DeploymentList{}, client.MatchingLabels{"app": "dependency-watchdog-prober"})
}

func (w *webhookTriggerer) trigger(ctx context.Context, reader client.Reader, writer client.Writer, statusWriter client.StatusWriter, objectList client.ObjectList, opts ...client.ListOption) error {
	if err := reader.List(ctx, objectList, opts...); err != nil {
		return err
	}

	return meta.EachListItem(objectList, func(obj runtime.Object) error {
		switch object := obj.(type) {
		case *appsv1.Deployment:
			return writer.Patch(ctx, object, client.RawPatch(types.StrategicMergePatchType, []byte("{}")))
		case *corev1.Node:
			return statusWriter.Patch(ctx, object, client.RawPatch(types.StrategicMergePatchType, []byte("{}")))
		}
		return nil
	})
}
