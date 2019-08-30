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
	"os"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenmetrics "github.com/gardener/gardener/pkg/controllermanager/metrics"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/validation"
	shootcontroller "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	"github.com/gardener/gardener/pkg/server"
	"github.com/gardener/gardener/pkg/server/handlers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/client-go/discovery"
	diskcache "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/informers"
	kubeinformers "k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/cmd/utils"
	configloader "github.com/gardener/gardener/pkg/scheduler/apis/config/loader"
)

// Options has all the context and parameters needed to run a GardenerScheduler.
type Options struct {
	// ConfigFile is the location of the GardenerScheduler's configuration file.
	ConfigFile string
	config     *config.SchedulerConfiguration
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

func (o *Options) applyDefaults(in *config.SchedulerConfiguration) (*config.SchedulerConfiguration, error) {

	scheme := configloader.Scheme
	external, err := scheme.ConvertToVersion(in, schedulerconfigv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	scheme.Default(external)

	internal, err := scheme.ConvertToVersion(external, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	out := internal.(*config.SchedulerConfiguration)

	return out, nil
}

func (o *Options) run(ctx context.Context) error {
	if len(o.ConfigFile) > 0 {
		c, err := configloader.LoadFromFile(o.ConfigFile)
		if err != nil {
			return err
		}
		o.config = c
	}

	gardener, err := NewGardenerScheduler(o.config)
	if err != nil {
		return err
	}

	return gardener.Run(ctx)
}

// NewCommandStartGardenerScheduler creates a *cobra.Command object with default parameters
func NewCommandStartGardenerScheduler(ctx context.Context) *cobra.Command {
	opts := &Options{
		config: new(config.SchedulerConfiguration),
	}
	config, err := opts.applyDefaults(opts.config)
	utilruntime.Must(err)
	opts.config = config

	cmd := &cobra.Command{
		Use:   "gardener-scheduler",
		Short: "Launch the Gardener scheduler",
		Long:  `The Gardener scheduler is a controller that tries to find the best matching seed cluster for a shoot. The scheduler takes the cloud provider and the distance between the seed (hosting the control plane) and the shoot cluster region into account.`,
		Run: func(cmd *cobra.Command, args []string) {
			utilruntime.Must(opts.validate(args))
			utilruntime.Must(opts.run(ctx))
		},
	}

	opts.AddFlags(cmd.Flags())
	return cmd
}

// GardenerScheduler represents all the parameters required to start the
// Gardener scheduler.
type GardenerScheduler struct {
	Config                 *config.SchedulerConfiguration
	Identity               *gardenv1beta1.Gardener
	GardenerNamespace      string
	K8sGardenClient        kubernetes.Interface
	K8sGardenInformers     gardeninformers.SharedInformerFactory
	K8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	KubeInformerFactory    informers.SharedInformerFactory
	Logger                 *logrus.Logger
	Recorder               record.EventRecorder
	LeaderElection         *leaderelection.LeaderElectionConfig
}

// NewGardenerScheduler is the main entry point of instantiating a new Gardener Scheduler.
func NewGardenerScheduler(cfg *config.SchedulerConfiguration) (*GardenerScheduler, error) {
	// validate the configuration
	if err := validation.ValidateConfiguration(cfg); err != nil {
		return nil, err
	}

	// Initialize logger
	logger := logger.NewLogger(cfg.LogLevel)
	logger.Info("Starting Gardener scheduler ...")

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.ClientConnection.Kubeconfig = kubeconfig
	}

	restCfg, err := utils.RESTConfigFromClientConnectionConfiguration(cfg.ClientConnection)
	if err != nil {
		return nil, err
	}

	disc, err := discoveryFromSchedulerConfiguration(cfg)
	if err != nil {
		return nil, err
	}

	k8sGardenClient, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(restCfg),
		kubernetes.WithClientOptions(
			client.Options{
				Mapper: restmapper.NewDeferredDiscoveryRESTMapper(disc),
				Scheme: kubernetes.GardenScheme,
			}),
	)
	if err != nil {
		return nil, err
	}

	k8sGardenClientLeaderElection, err := k8s.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = utils.CreateRecorder(k8sGardenClient.Kubernetes(), "gardener-scheduler")
	)
	if cfg.LeaderElection.LeaderElect {
		leaderElectionConfig, err = utils.MakeLeaderElectionConfig(cfg.LeaderElection.LeaderElectionConfiguration, cfg.LeaderElection.LockObjectNamespace, cfg.LeaderElection.LockObjectName, k8sGardenClientLeaderElection, recorder)
		if err != nil {
			return nil, err
		}
	}

	return &GardenerScheduler{
		Config:                 cfg,
		Logger:                 logger,
		Recorder:               recorder,
		K8sGardenClient:        k8sGardenClient,
		K8sGardenInformers:     gardeninformers.NewSharedInformerFactory(k8sGardenClient.Garden(), 0),
		K8sGardenCoreInformers: gardencoreinformers.NewSharedInformerFactory(k8sGardenClient.GardenCore(), 0),
		KubeInformerFactory:    kubeinformers.NewSharedInformerFactory(k8sGardenClient.Kubernetes(), 0),
		LeaderElection:         leaderElectionConfig,
	}, nil
}

func (g *GardenerScheduler) cleanup() {
	if err := os.RemoveAll(schedulerconfigv1alpha1.DefaultDiscoveryDir); err != nil {
		g.Logger.Errorf("Could not cleanup base discovery cache directory: %v", err)
	}
}

// Run runs the Gardener Scheduler. This should never exit.
func (g *GardenerScheduler) Run(ctx context.Context) error {
	defer g.cleanup()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Prepare a reusable run function.
	run := func(ctx context.Context) {
		g.startScheduler(ctx)
	}

	// Start HTTP server (HTTPS not needed because no webhook server is needed at the moment)
	go server.ServeHTTP(ctx, g.Config.Server.HTTP.Port, g.Config.Server.HTTP.BindAddress)
	handlers.UpdateHealth(true)

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				g.Logger.Info("Acquired leadership, starting scheduler.")
				run(leaderCtx)
			},
			OnStoppedLeading: func() {
				g.Logger.Info("Lost leadership, terminating.")
				cancel()
			},
		}

		leaderElector, err := leaderelection.NewLeaderElector(*g.LeaderElection)
		if err != nil {
			return fmt.Errorf("couldn't create leader elector: %v", err)
		}

		leaderElector.Run(ctx)
		return nil
	}
	run(ctx)
	return nil
}

func (g *GardenerScheduler) startScheduler(ctx context.Context) {
	shootScheduler := shootcontroller.NewGardenerScheduler(g.K8sGardenClient, g.K8sGardenInformers, g.Config, g.Recorder)
	//backupBucketScheduler := backupbucketcontroller.NewGardenerScheduler(ctx, g.K8sGardenClient, g.K8sGardenCoreInformers, g.Config, g.Recorder)

	// Initialize the Controller metrics collection.
	gardenmetrics.RegisterControllerMetrics(shootScheduler) //, backupBucketScheduler)

	go shootScheduler.Run(ctx, g.K8sGardenInformers)
	// TOEnable later
	// go backupBucketScheduler.Run(ctx, g.K8sGardenCoreInformers)

	// Shutdown handling
	<-ctx.Done()

	logger.Logger.Infof("I have received a stop signal and will no longer watch events of the Garden API group.")
	logger.Logger.Infof("Bye Bye!")
}

func discoveryFromSchedulerConfiguration(cfg *config.SchedulerConfiguration) (discovery.CachedDiscoveryInterface, error) {
	restConfig, err := utils.RESTConfigFromClientConnectionConfiguration(cfg.ClientConnection)
	if err != nil {
		return nil, err
	}

	discoveryCfg := cfg.Discovery
	var discoveryCacheDir string
	if discoveryCfg.DiscoveryCacheDir != nil {
		discoveryCacheDir = *discoveryCfg.DiscoveryCacheDir
	}

	var httpCacheDir string
	if discoveryCfg.HTTPCacheDir != nil {
		httpCacheDir = *discoveryCfg.HTTPCacheDir
	}

	var ttl time.Duration
	if discoveryCfg.TTL != nil {
		ttl = discoveryCfg.TTL.Duration
	}

	return diskcache.NewCachedDiscoveryClientForConfig(restConfig, discoveryCacheDir, httpCacheDir, ttl)
}
