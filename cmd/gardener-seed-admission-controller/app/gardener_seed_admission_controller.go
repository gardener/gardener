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
	"flag"
	"fmt"
	goruntime "runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/routes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
)

// Name is a const for the name of this component.
const Name = "gardener-seed-admission-controller"

var (
	log                     = logf.Log
	gracefulShutdownTimeout = 5 * time.Second
)

// NewSeedAdmissionControllerCommand creates a new *cobra.Command able to run gardener-seed-admission-controller.
func NewSeedAdmissionControllerCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   Name,
		Short: "Launch the " + Name,
		Long:  Name + " serves validating and mutating webhook endpoints for resources in seed clusters.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.validate(); err != nil {
				return err
			}

			log.Info("Starting "+Name, "version", version.Get())
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value)) //nolint:logcheck
			})

			return opts.Run(cmd.Context())
		},
		SilenceUsage: true,
	}

	flags := cmd.Flags()
	flags.AddGoFlagSet(flag.CommandLine)
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	return cmd
}

// Run runs gardener-seed-admission-controller using the specified options.
func (o *Options) Run(ctx context.Context) error {
	log.Info("Getting rest config")
	restConfig, err := config.GetConfig()
	if err != nil {
		return err
	}

	log.Info("Setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:                  kubernetes.SeedScheme,
		LeaderElection:          false,
		Host:                    o.BindAddress,
		MetricsBindAddress:      o.MetricsBindAddress,
		HealthProbeBindAddress:  o.HealthBindAddress,
		Port:                    o.Port,
		CertDir:                 o.ServerCertDir,
		GracefulShutdownTimeout: &gracefulShutdownTimeout,
	})
	if err != nil {
		return err
	}

	if o.EnableProfiling {
		if err := (routes.Profiling{}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding profiling handlers to manager: %w", err)
		}
		if o.EnableContentionProfiling {
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
