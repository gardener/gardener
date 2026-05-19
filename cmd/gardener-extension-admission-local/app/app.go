// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/component-base/version/verflag"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscmdcontroller "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionscmdwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityinstall "github.com/gardener/gardener/pkg/apis/security/install"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	admissioncmd "github.com/gardener/gardener/pkg/provider-local/admission/cmd"
	localinstall "github.com/gardener/gardener/pkg/provider-local/apis/local/install"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// AdmissionName is the name of the admission component.
const AdmissionName = "admission-local"

var log = logf.Log.WithName("gardener-extension-admission-local")

// NewAdmissionCommand creates a new command for running the local admission webhook.
func NewAdmissionCommand(ctx context.Context) *cobra.Command {
	var (
		restOpts = &extensionscmdcontroller.RESTOptions{}
		mgrOpts  = &extensionscmdcontroller.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        extensionscmdcontroller.LeaderElectionNameID(AdmissionName),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:       443,
			MetricsBindAddress:      ":8080",
			HealthBindAddress:       ":8081",
			WebhookCertDir:          "/tmp/admission-local-cert",
		}
		// options for the webhook server
		webhookServerOptions = &extensionscmdwebhook.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}
		webhookSwitches = admissioncmd.GardenWebhookSwitchOptions()
		webhookOptions  = extensionscmdwebhook.NewAddToManagerOptions(
			AdmissionName,
			"",
			nil,
			nil,
			webhookServerOptions,
			webhookSwitches,
		)

		aggOption = extensionscmdcontroller.NewOptionAggregator(
			restOpts,
			mgrOpts,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("admission-%s", local.Type),

		RunE: func(_ *cobra.Command, _ []string) error {
			verflag.PrintAndExitIfRequested()

			if gardenKubeconfig := os.Getenv("GARDEN_KUBECONFIG"); gardenKubeconfig != "" {
				log.Info("Getting rest config for garden from GARDEN_KUBECONFIG", "path", gardenKubeconfig)
				restOpts.Kubeconfig = gardenKubeconfig
			}

			if err := aggOption.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(&componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   100.0,
				Burst: 130,
			}, restOpts.Completed().Config)

			managerOptions := mgrOpts.Completed().Options()

			log.Info("Configuring source cluster option")
			inClusterConfig, err := rest.InClusterConfig()
			if err != nil {
				return fmt.Errorf("could not get in-cluster config: %w", err)
			}
			managerOptions.LeaderElectionConfig = inClusterConfig

			mgr, err := manager.New(restOpts.Completed().Config, managerOptions)
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}

			gardencoreinstall.Install(mgr.GetScheme())
			securityinstall.Install(mgr.GetScheme())

			if err := localinstall.AddToScheme(mgr.GetScheme()); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}

			sourceCluster, err := cluster.New(inClusterConfig, func(opts *cluster.Options) {
				opts.Logger = log
				opts.Cache.DefaultNamespaces = map[string]cache.Config{v1beta1constants.GardenNamespace: {}}
			})
			if err != nil {
				return err
			}

			if err := mgr.AddHealthzCheck("source-informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, sourceCluster.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
				return err
			}
			if err := mgr.AddReadyzCheck("source-informer-sync", gardenerhealthz.NewCacheSyncHealthz(sourceCluster.GetCache())); err != nil {
				return err
			}

			if err = mgr.Add(sourceCluster); err != nil {
				return err
			}

			log.Info("Setting up webhook server")
			if _, err := webhookOptions.Completed().AddToManager(ctx, mgr, sourceCluster); err != nil {
				return err
			}

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

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("error running manager: %w", err)
			}

			return nil
		},
	}

	verflag.AddFlags(cmd.Flags())
	aggOption.AddFlags(cmd.Flags())

	return cmd
}
