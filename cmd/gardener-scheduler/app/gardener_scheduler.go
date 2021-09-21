// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"fmt"
	"net"
	"os"
	goruntime "runtime"
	"strconv"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	configloader "github.com/gardener/gardener/pkg/scheduler/apis/config/loader"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/validation"
	shootcontroller "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	schedulerfeatures "github.com/gardener/gardener/pkg/scheduler/features"
	"github.com/gardener/gardener/pkg/server/routes"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Options has all the context and parameters needed to run a GardenerScheduler.
type Options struct {
	// ConfigFile is the location of the GardenerScheduler's configuration file.
	ConfigFile string
}

// AddFlags adds flags for a specific Scheduler to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, "The path to the configuration file.")
}

// Validate validates all the required options.
func (o *Options) validate(args []string) error {
	if len(o.ConfigFile) == 0 {
		return fmt.Errorf("missing GardenerScheduler config file")
	}
	if len(args) != 0 {
		return errors.New("arguments are not supported")
	}

	return nil
}

// NewCommandStartGardenerScheduler creates a *cobra.Command object with default parameters
func NewCommandStartGardenerScheduler() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "gardener-scheduler",
		Short: "Launch the Gardener scheduler",
		Long:  `The Gardener scheduler is a controller that tries to find the best matching seed cluster for a shoot. The scheduler takes the cloud provider and the distance between the seed (hosting the control plane) and the shoot cluster region into account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.validate(args); err != nil {
				return err
			}

			return runCommand(cmd.Context(), opts)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)

	return cmd
}

func runCommand(ctx context.Context, opts *Options) error {
	// Load config file
	cfg, err := configloader.LoadFromFile(opts.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate the configuration
	if errs := validation.ValidateConfiguration(cfg); len(errs) > 0 {
		return fmt.Errorf("configuration is invalid: %v", errs)
	}

	// Add feature flags
	if err := schedulerfeatures.FeatureGate.SetFromMap(cfg.FeatureGates); err != nil {
		return fmt.Errorf("failed to set feature gates: %w", err)
	}

	// Initialize logger
	zapLogger, err := logger.NewZapLogger(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	// set the logger used by sigs.k8s.io/controller-runtime
	zapLogr := logger.NewZapLogr(zapLogger)
	ctrlruntimelog.SetLogger(zapLogr)

	zapLogr.Info("Starting Gardener scheduler...", "version", version.Get())
	zapLogr.Info("Feature Gates", "featureGates", schedulerfeatures.FeatureGate.String())

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}

	// Prepare REST config
	restCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.ClientConnection, nil)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes REST configuration: %w", err)
	}

	// Setup controller-runtime manager
	mgr, err := manager.New(restCfg, manager.Options{
		MetricsBindAddress:         getAddress(cfg.Server.Metrics),
		HealthProbeBindAddress:     getAddress(cfg.Server.HealthProbes),
		LeaderElection:             cfg.LeaderElection.LeaderElect,
		LeaderElectionID:           cfg.LeaderElection.ResourceName,
		LeaderElectionNamespace:    cfg.LeaderElection.ResourceNamespace,
		LeaderElectionResourceLock: cfg.LeaderElection.ResourceLock,
		Logger:                     zapLogr,
	})
	if err != nil {
		return fmt.Errorf("failed to create controller manager: %w", err)
	}

	if cfg.Debugging != nil && cfg.Debugging.EnableProfiling {
		if err := (routes.Profiling{}).AddToManager(mgr); err != nil {
			return fmt.Errorf("failed adding profiling handlers to manager: %w", err)
		}
		if cfg.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}

	// Add APIs
	if err := gardencorev1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("failed to register scheme gardencorev1beta1: %w", err)
	}

	// Add controllers
	if err := shootcontroller.AddToManager(mgr, cfg.Schedulers.Shoot); err != nil {
		return fmt.Errorf("failed to create shoot scheduler controller: %w", err)
	}

	// Start manager and all runnables (the command context is tied to OS signals already)
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	return nil
}

func getAddress(server *config.Server) string {
	if server != nil && server.Port != 0 {
		return net.JoinHostPort(server.BindAddress, strconv.Itoa(server.Port))
	}

	return "0" // 0 means "disabled" in ctrl-runtime speak
}
