// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	goruntime "runtime"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version/verflag"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	controllerwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/gardener/gardener/cmd/utils/initrun"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/certificates"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/bootstrappers"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller"
	"github.com/gardener/gardener/pkg/operator/webhook"
)

// Name is a const for the name of this component.
const Name = "gardener-operator"

// NewCommand creates a new cobra.Command for running gardener-operator.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			log, err := initrun.InitRun(cmd, opts, Name)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			return run(ctx, cancel, log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return cmd
}

func run(ctx context.Context, cancel context.CancelFunc, log logr.Logger, cfg *operatorconfigv1alpha1.OperatorConfiguration) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

	log.Info("Getting rest config")
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.RuntimeClientConnection.Kubeconfig = kubeconfig
	}

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.RuntimeClientConnection, nil, kubernetes.AuthTokenFile)
	if err != nil {
		return err
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && ptr.Deref(cfg.Debugging.EnableProfiling, false) {
		extraHandlers = routes.ProfilingHandlers
		if ptr.Deref(cfg.Debugging.EnableContentionProfiling, false) {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Logger:                  log,
		Scheme:                  operatorclient.RuntimeScheme,
		GracefulShutdownTimeout: ptr.To(5 * time.Second),

		HealthProbeBindAddress: net.JoinHostPort(cfg.Server.HealthProbes.BindAddress, strconv.Itoa(cfg.Server.HealthProbes.Port)),
		Metrics: metricsserver.Options{
			BindAddress:   net.JoinHostPort(cfg.Server.Metrics.BindAddress, strconv.Itoa(cfg.Server.Metrics.Port)),
			ExtraHandlers: extraHandlers,
		},

		LeaderElection:                *cfg.LeaderElection.LeaderElect,
		LeaderElectionResourceLock:    cfg.LeaderElection.ResourceLock,
		LeaderElectionID:              cfg.LeaderElection.ResourceName,
		LeaderElectionNamespace:       cfg.LeaderElection.ResourceNamespace,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &cfg.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:                 &cfg.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                   &cfg.LeaderElection.RetryPeriod.Duration,

		WebhookServer: controllerwebhook.NewServer(controllerwebhook.Options{
			Host:    cfg.Server.Webhooks.BindAddress,
			Port:    cfg.Server.Webhooks.Port,
			CertDir: "/tmp/gardener-operator-cert",
		}),
	})
	if err != nil {
		return err
	}

	log.Info("Setting up health check endpoints")
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthzWithDeadline(mgr.GetLogger(), clock.RealClock{}, mgr.GetCache(), gardenerhealthz.DefaultCacheSyncDeadline)); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
		return err
	}

	log.Info("Perform Gardener version verification")
	if err := bootstrappers.VerifyGardenerVersion(ctx, mgr.GetLogger(), mgr.GetAPIReader()); err != nil {
		return fmt.Errorf("failed verifying Gardener version: %w", err)
	}

	log.Info("Adding certificate management to manager")
	mode, url := extensionswebhook.ModeService, os.Getenv("WEBHOOK_URL")
	if v := os.Getenv("WEBHOOK_MODE"); v != "" {
		mode = v
	}

	var (
		validatingWebhookConfiguration = webhook.GetValidatingWebhookConfiguration(mode, url)
		mutatingWebhookConfiguration   = webhook.GetMutatingWebhookConfiguration(mode, url)
	)

	if err := certificates.AddCertificateManagementToManager(
		ctx,
		mgr,
		nil,
		clock.RealClock{},
		extensionswebhook.Configs{
			MutatingWebhookConfig:   mutatingWebhookConfiguration,
			ValidatingWebhookConfig: validatingWebhookConfiguration,
		},
		nil,
		nil,
		nil,
		"",
		Name,
		v1beta1constants.GardenNamespace,
		mode,
		url,
	); err != nil {
		return fmt.Errorf("failed adding webhook certificate management to manager: %w", err)
	}

	log.Info("Adding webhook config reconciliation func to manager")
	if err := mgr.Add(reconcileWebhookConfigurations(ctx, mgr, validatingWebhookConfiguration, mutatingWebhookConfiguration)); err != nil {
		return fmt.Errorf("failed adding webhook config reconciliation func: %w", err)
	}

	log.Info("Adding webhook handlers to manager")
	if err := webhook.AddToManager(mgr); err != nil {
		return err
	}

	gardenClientMap, err := clientmapbuilder.
		NewGardenClientMapBuilder().
		WithRuntimeClient(mgr.GetClient()).
		WithClientConnectionConfig(&cfg.VirtualClientConnection).
		WithGardenNamespace(v1beta1constants.GardenNamespace).
		Build(mgr.GetLogger())
	if err != nil {
		return fmt.Errorf("failed to build garden ClientMap: %w", err)
	}
	if err := mgr.Add(gardenClientMap); err != nil {
		return err
	}

	log.Info("Adding controllers to manager")
	if err := controller.AddToManager(cancel, mgr, cfg, gardenClientMap); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func reconcileWebhookConfigurations(
	ctx context.Context,
	mgr manager.Manager,
	validatingWebhookConfiguration *admissionregistrationv1.ValidatingWebhookConfiguration,
	mutatingWebhookConfiguration *admissionregistrationv1.MutatingWebhookConfiguration,
) manager.RunnableFunc {
	return func(context.Context) error {
		mgr.GetLogger().Info("Reconciling webhook configurations",
			"validatingWebhookConfiguration", client.ObjectKeyFromObject(validatingWebhookConfiguration),
			"mutatingWebhookConfiguration", client.ObjectKeyFromObject(mutatingWebhookConfiguration),
		)

		valWebhook := &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: validatingWebhookConfiguration.Name}}
		if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, mgr.GetClient(), valWebhook, func() error {
			// The CA bundle is updated asynchronously by a separate certificates reconciler. Hence, when we update the
			// webhook configuration here, let's make sure to not overwrite existing CA bundles in the webhooks.
			if err := extensionswebhook.InjectCABundleIntoWebhookConfig(validatingWebhookConfiguration, getCurrentCABundle(valWebhook)); err != nil {
				return err
			}

			valWebhook.Webhooks = validatingWebhookConfiguration.Webhooks
			return nil
		}); err != nil {
			return err
		}

		mutWebhook := &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: mutatingWebhookConfiguration.Name}}
		if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, mgr.GetClient(), mutWebhook, func() error {
			// The CA bundle is updated asynchronously by a separate certificates reconciler. Hence, when we update the
			// webhook configuration here, let's make sure to not overwrite existing CA bundles in the webhooks.
			if err := extensionswebhook.InjectCABundleIntoWebhookConfig(mutatingWebhookConfiguration, getCurrentCABundle(mutWebhook)); err != nil {
				return err
			}

			mutWebhook.Webhooks = mutatingWebhookConfiguration.Webhooks
			return nil
		}); err != nil {
			return err
		}

		validatingWebhookConfiguration = valWebhook
		mutatingWebhookConfiguration = mutWebhook
		return nil
	}
}

func getCurrentCABundle(webhookConfig client.Object) []byte {
	// All webhooks in this configuration are served by the same endpoint, hence they all have to use the same CA
	// bundle. We simply take the first bundle we find and consider it the current bundle for all webhooks.

	switch config := webhookConfig.(type) {
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		for _, w := range config.Webhooks {
			if len(w.ClientConfig.CABundle) > 0 {
				return w.ClientConfig.CABundle
			}
		}
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		for _, w := range config.Webhooks {
			if len(w.ClientConfig.CABundle) > 0 {
				return w.ClientConfig.CABundle
			}
		}
	case *admissionregistrationv1beta1.MutatingWebhookConfiguration:
		for _, w := range config.Webhooks {
			if len(w.ClientConfig.CABundle) > 0 {
				return w.ClientConfig.CABundle
			}
		}
	case *admissionregistrationv1beta1.ValidatingWebhookConfiguration:
		for _, w := range config.Webhooks {
			if len(w.ClientConfig.CABundle) > 0 {
				return w.ClientConfig.CABundle
			}
		}
	}

	return nil
}
