// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	"github.com/gardener/gardener/pkg/features"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Name is a const for the name of this component.
const Name = "gardener-node-agent"

// NewCommand creates a new cobra.Command for running gardener-node-agent.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.complete(); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}

			log, err := logger.NewZapLogger(opts.config.LogLevel, opts.config.LogFormat)
			if err != nil {
				return fmt.Errorf("error instantiating zap logger: %w", err)
			}

			logf.SetLogger(log)
			klog.SetLogger(log)

			log.Info("Starting "+Name, "version", version.Get())
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value)) //nolint:logcheck
			})

			// don't output usage on further errors raised during execution
			cmd.SilenceUsage = true
			// further errors will be logged properly, don't duplicate
			cmd.SilenceErrors = true

			return run(cmd.Context(), log, opts.config)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)

	return cmd
}

func run(ctx context.Context, log logr.Logger, cfg *config.NodeAgentConfiguration) error {
	log.Info("Feature Gates", "featureGates", features.DefaultFeatureGate)

	log.Info("Getting rest config")
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}
	if cfg.ClientConnection.Kubeconfig == "" {
		return fmt.Errorf("must specify path to a kubeconfig (either via \"KUBECONFIG\" environment variable of via .clientConnection.kubeconfig in component config)")
	}

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.ClientConnection, nil, kubernetes.AuthTokenFile)
	if err != nil {
		return err
	}

	if restConfig.BearerTokenFile == nodeagentv1alpha1.BootstrapTokenFilePath {
		log.Info("Kubeconfig points to the bootstrap token file")
		if err := fetchAccessTokenViaBootstrapToken(ctx, log, restConfig, cfg); err != nil {
			return fmt.Errorf("failed fetching access token via bootstrap token: %w", err)
		}
	}

	var extraHandlers map[string]http.Handler
	if cfg.Debugging != nil && cfg.Debugging.EnableProfiling {
		extraHandlers = routes.ProfilingHandlers
		if cfg.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Logger:                  log,
		Scheme:                  kubernetes.SeedScheme,
		GracefulShutdownTimeout: pointer.Duration(5 * time.Second),

		HealthProbeBindAddress: net.JoinHostPort(cfg.Server.HealthProbes.BindAddress, strconv.Itoa(cfg.Server.HealthProbes.Port)),
		Metrics: metricsserver.Options{
			BindAddress:   net.JoinHostPort(cfg.Server.Metrics.BindAddress, strconv.Itoa(cfg.Server.Metrics.Port)),
			ExtraHandlers: extraHandlers,
		},

		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
				},
			},
		},
		LeaderElection: false,
		Controller: controllerconfig.Controller{
			RecoverPanic: pointer.Bool(true),
		},
	})
	if err != nil {
		return err
	}

	log.Info("Setting up health check endpoints")
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}

	log.Info("Adding controllers to manager")
	if err := controller.AddToManager(mgr, cfg); err != nil {
		return fmt.Errorf("failed adding controllers to manager: %w", err)
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

func fetchAccessTokenViaBootstrapToken(ctx context.Context, log logr.Logger, restConfig *rest.Config, cfg *config.NodeAgentConfiguration) error {
	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("unable to create client with bootstrap token: %w", err)
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cfg.AccessTokenSecretName, Namespace: metav1.NamespaceSystem}}
	log.Info("Fetching access token secret", "secret", client.ObjectKeyFromObject(secret))
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return fmt.Errorf("failed fetching access token from API server: %w", err)
	}

	token := secret.Data[resourcesv1alpha1.DataKeyToken]
	if len(token) == 0 {
		return fmt.Errorf("secret key %q does not exist or empty", resourcesv1alpha1.DataKeyToken)
	}

	log.Info("Successfully fetched access token from API server, writing it to disk", "path", nodeagentv1alpha1.TokenFilePath)
	if err := os.WriteFile(nodeagentv1alpha1.TokenFilePath, token, 0600); err != nil {
		return fmt.Errorf("unable to write access token to %s: %w", nodeagentv1alpha1.TokenFilePath, err)
	}
	log.Info("Token written to disk")

	log.Info("Overwriting kubeconfig to no longer use bootstrap token file", "path", cfg.ClientConnection.Kubeconfig)
	restConfig.BearerTokenFile = nodeagentv1alpha1.TokenFilePath

	kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		Name,
		clientcmdv1.Cluster{Server: restConfig.Host, CertificateAuthorityData: restConfig.CAData},
		clientcmdv1.AuthInfo{TokenFile: nodeagentv1alpha1.TokenFilePath},
	))
	if err != nil {
		return fmt.Errorf("failed encoding kubeconfig: %w", err)
	}

	if err := os.WriteFile(cfg.ClientConnection.Kubeconfig, kubeconfigRaw, 0600); err != nil {
		return fmt.Errorf("unable to write kubeconfig to %s: %w", cfg.ClientConnection.Kubeconfig, err)
	}

	log.Info("Kubeconfig written to disk")
	return nil
}
