// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"os"
	goruntime "runtime"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Name is a const for the name of this component.
const Name = "gardener-admission-controller"

var (
	log                     = logf.Log
	gracefulShutdownTimeout = 5 * time.Second
)

func (o *options) run(ctx context.Context) error {
	log, err := logger.NewZapLogger(o.config.LogLevel, o.config.LogFormat)
	if err != nil {
		return fmt.Errorf("error instantiating zap logger: %w", err)
	}

	log.Info("Starting Gardener admission controller", "version", version.Get())

	log.Info("Getting rest config")
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		o.config.GardenClientConnection.Kubeconfig = kubeconfig
	}

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&o.config.GardenClientConnection, nil, kubernetes.AuthTokenFile)
	if err != nil {
		return err
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:                  kubernetes.GardenScheme,
		LeaderElection:          false,
		HealthProbeBindAddress:  fmt.Sprintf("%s:%d", o.config.Server.HealthProbes.BindAddress, o.config.Server.HealthProbes.Port),
		MetricsBindAddress:      fmt.Sprintf("%s:%d", o.config.Server.Metrics.BindAddress, o.config.Server.Metrics.Port),
		Host:                    o.config.Server.Webhooks.BindAddress,
		Port:                    o.config.Server.Webhooks.Port,
		CertDir:                 o.config.Server.Webhooks.TLS.ServerCertDir,
		GracefulShutdownTimeout: &gracefulShutdownTimeout,
		Logger:                  log,
	})
	if err != nil {
		return err
	}

	if o.config.Debugging != nil && o.config.Debugging.EnableProfiling {
		if err := (routes.Profiling{}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding profiling handlers to manager: %w", err)
		}
		if o.config.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	log.Info("Setting up healthcheck endpoints")
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}

	log.Info("Setting up webhook server")
	server := mgr.GetWebhookServer()

	log.Info("Setting up readycheck for webhook server")
	if err := mgr.AddReadyzCheck("webhook-server", server.StartedChecker()); err != nil {
		return err
	}

	log.Info("Starting manager")
	return mgr.Start(ctx)
}

// NewGardenerAdmissionControllerCommand creates a *cobra.Command object with default parameters.
func NewGardenerAdmissionControllerCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Long:  Name + " serves webhook endpoints for resources in the garden cluster.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.complete(); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}

			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value)) //nolint:logcheck
			})

			return opts.run(cmd.Context())
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)
	return cmd
}
