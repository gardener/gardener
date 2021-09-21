// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"flag"
	"fmt"
	"os"
	goruntime "runtime"

	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	configvalidation "github.com/gardener/gardener/pkg/controllermanager/apis/config/validation"
	"github.com/gardener/gardener/pkg/controllermanager/controller"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server"
	"github.com/gardener/gardener/pkg/server/routes"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
)

// Options has all the context and parameters needed to run a Gardener controller manager.
type Options struct {
	// ConfigFile is the location of the Gardener controller manager's configuration file.
	ConfigFile string
	config     *config.ControllerManagerConfiguration
	scheme     *runtime.Scheme
	codecs     serializer.CodecFactory
}

// AddFlags adds flags for a specific Gardener controller manager to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, "The path to the configuration file.")
}

// NewOptions returns a new Options object.
func NewOptions() (*Options, error) {
	o := &Options{
		config: new(config.ControllerManagerConfiguration),
	}

	o.scheme = runtime.NewScheme()
	o.codecs = serializer.NewCodecFactory(o.scheme)

	if err := config.AddToScheme(o.scheme); err != nil {
		return nil, err
	}
	if err := controllermanagerconfigv1alpha1.AddToScheme(o.scheme); err != nil {
		return nil, err
	}

	return o, nil
}

// loadConfigFromFile loads the contents of file and decodes it as a
// ControllerManagerConfiguration object.
func (o *Options) loadConfigFromFile(file string) (*config.ControllerManagerConfiguration, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return o.decodeConfig(data)
}

// decodeConfig decodes data as a ControllerManagerConfiguration object.
func (o *Options) decodeConfig(data []byte) (*config.ControllerManagerConfiguration, error) {
	configObj, gvk, err := o.codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*config.ControllerManagerConfiguration)
	if !ok {
		return nil, fmt.Errorf("got unexpected config type: %v", gvk)
	}
	return config, nil
}

func (o *Options) configFileSpecified() error {
	if len(o.ConfigFile) == 0 {
		return fmt.Errorf("missing Gardener controller manager config file")
	}
	return nil
}

// Validate validates all the required options.
func (o *Options) validate(args []string) error {
	if len(args) != 0 {
		return errors.New("arguments are not supported")
	}

	return nil
}

func (o *Options) run(ctx context.Context) error {
	if len(o.ConfigFile) > 0 {
		c, err := o.loadConfigFromFile(o.ConfigFile)
		if err != nil {
			return err
		}
		o.config = c
	}

	if errs := configvalidation.ValidateControllerManagerConfiguration(o.config); len(errs) > 0 {
		return errs.ToAggregate()
	}

	// Add feature flags
	if err := controllermanagerfeatures.FeatureGate.SetFromMap(o.config.FeatureGates); err != nil {
		return err
	}
	kubernetes.UseCachedRuntimeClients = controllermanagerfeatures.FeatureGate.Enabled(features.CachedRuntimeClients)

	gardener, err := NewGardener(ctx, o.config)
	if err != nil {
		return err
	}

	return gardener.Run(ctx)
}

// NewCommandStartGardenerControllerManager creates a *cobra.Command object with default parameters
func NewCommandStartGardenerControllerManager() *cobra.Command {
	opts, err := NewOptions()
	if err != nil {
		panic(err)
	}

	cmd := &cobra.Command{
		Use:   "gardener-controller-manager",
		Short: "Launch the Gardener controller manager",
		Long: `In essence, the Gardener is an extension API server along with a bundle
of Kubernetes controllers which introduce new API objects in an existing Kubernetes
cluster (which is called Garden cluster) in order to use them for the management of
further Kubernetes clusters (which are called Shoot clusters).
To do that reliably and to offer a certain quality of service, it requires to control
the main components of a Kubernetes cluster (etcd, API server, controller manager, scheduler).
These so-called control plane components are hosted in Kubernetes clusters themselves
(which are called Seed clusters).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := opts.configFileSpecified(); err != nil {
				return err
			}
			if err := opts.validate(args); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	return cmd
}

// Gardener represents all the parameters required to start the
// Gardener controller manager.
type Gardener struct {
	Config         *config.ControllerManagerConfiguration
	ClientMap      clientmap.ClientMap
	Logger         *logrus.Logger
	Recorder       record.EventRecorder
	LeaderElection *leaderelection.LeaderElectionConfig
	HealthManager  healthz.Manager
}

// NewGardener is the main entry point of instantiating a new Gardener controller manager.
func NewGardener(ctx context.Context, cfg *config.ControllerManagerConfiguration) (*Gardener, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Initialize logger
	logger := logger.NewLogger(cfg.LogLevel, cfg.LogFormat)
	logger.Info("Starting Gardener controller manager...")
	logger.Infof("Version: %+v", version.Get())
	logger.Infof("Feature Gates: %s", controllermanagerfeatures.FeatureGate.String())

	if flag := flag.Lookup("v"); flag != nil {
		if err := flag.Value.Set(fmt.Sprintf("%d", cfg.KubernetesLogLevel)); err != nil {
			return nil, err
		}
	}

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.GardenClientConnection.Kubeconfig = kubeconfig
	}

	restCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.GardenClientConnection, nil)
	if err != nil {
		return nil, err
	}

	clientMap, err := clientmapbuilder.NewDelegatingClientMapBuilder().
		WithGardenClientMapBuilder(clientmapbuilder.NewGardenClientMapBuilder().WithRESTConfig(restCfg)).
		WithPlantClientMapBuilder(clientmapbuilder.NewPlantClientMapBuilder()).
		WithLogger(logger).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build ClientMap: %w", err)
	}

	k8sGardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = utils.CreateRecorder(k8sGardenClient.Kubernetes(), "gardener-controller-manager")
	)
	if cfg.LeaderElection.LeaderElect {
		k8sGardenClientLeaderElection, err := kubernetesclientset.NewForConfig(restCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create garden client for leader election: %w", err)
		}

		leaderElectionConfig, err = utils.MakeLeaderElectionConfig(
			cfg.LeaderElection.LeaderElectionConfiguration,
			cfg.LeaderElection.LockObjectNamespace,
			cfg.LeaderElection.LockObjectName,
			k8sGardenClientLeaderElection,
			recorder,
		)
		if err != nil {
			return nil, err
		}
	}

	return &Gardener{
		Config:         cfg,
		Logger:         logger,
		Recorder:       recorder,
		ClientMap:      clientMap,
		LeaderElection: leaderElectionConfig,
	}, nil
}

// Run runs the Gardener. This should never exit.
func (g *Gardener) Run(ctx context.Context) error {
	controllerCtx, controllerCancel := context.WithCancel(ctx)
	defer controllerCancel()

	// Initialize /healthz manager.
	g.HealthManager = healthz.NewDefaultHealthz()

	// Prepare a reusable run function.
	run := func(ctx context.Context) error {
		g.HealthManager.Start()
		return g.startControllers(ctx)
	}

	g.startServer(ctx)

	leaderElectionCtx, leaderElectionCancel := context.WithCancel(context.Background())

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				g.Logger.Info("Acquired leadership, starting controllers.")
				if err := run(controllerCtx); err != nil {
					g.Logger.Errorf("failed to run controllers: %v", err)
				}
				leaderElectionCancel()
			},
			OnStoppedLeading: func() {
				g.Logger.Info("Lost leadership, terminating.")
				controllerCancel()
			},
		}
		leaderElector, err := leaderelection.NewLeaderElector(*g.LeaderElection)
		if err != nil {
			return fmt.Errorf("couldn't create leader elector: %w", err)
		}
		leaderElector.Run(leaderElectionCtx)
		return nil
	}

	// Leader election is disabled, thus run directly until done.
	leaderElectionCancel()
	return run(controllerCtx)
}

func (g *Gardener) startServer(ctx context.Context) {
	builder := server.
		NewBuilder().
		WithBindAddress(g.Config.Server.HTTP.BindAddress).
		WithPort(g.Config.Server.HTTP.Port).
		WithHandler("/metrics", promhttp.Handler()).
		WithHandlerFunc("/healthz", healthz.HandlerFunc(g.HealthManager))

	if g.Config.Debugging != nil && g.Config.Debugging.EnableProfiling {
		routes.Profiling{}.AddToBuilder(builder)
		if g.Config.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	go builder.Build().Start(ctx)
}

func (g *Gardener) startControllers(ctx context.Context) error {
	return controller.NewGardenControllerFactory(
		g.ClientMap,
		g.Config,
		g.Recorder,
	).Run(ctx)
}
