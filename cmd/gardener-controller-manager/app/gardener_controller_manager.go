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
	"io/ioutil"
	"os"

	"github.com/gardener/gardener/cmd/utils"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/controllermanager/server/handlers/webhooks"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/informers"
	kubeinformers "k8s.io/client-go/informers"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
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
	if err := gardencorev1beta1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	return o, nil
}

// loadConfigFromFile loads the contents of file and decodes it as a
// ControllerManagerConfiguration object.
func (o *Options) loadConfigFromFile(file string) (*config.ControllerManagerConfiguration, error) {
	data, err := ioutil.ReadFile(file)
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

func (o *Options) applyDefaults(in *config.ControllerManagerConfiguration) (*config.ControllerManagerConfiguration, error) {
	external, err := o.scheme.ConvertToVersion(in, controllermanagerconfigv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	o.scheme.Default(external)

	internal, err := o.scheme.ConvertToVersion(external, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	out := internal.(*config.ControllerManagerConfiguration)

	return out, nil
}

func (o *Options) run(ctx context.Context, cancel context.CancelFunc) error {
	if len(o.ConfigFile) > 0 {
		c, err := o.loadConfigFromFile(o.ConfigFile)
		if err != nil {
			return err
		}
		o.config = c
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

	return gardener.Run(ctx, cancel)
}

// NewCommandStartGardenerControllerManager creates a *cobra.Command object with default parameters
func NewCommandStartGardenerControllerManager(ctx context.Context, cancel context.CancelFunc) *cobra.Command {
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
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.configFileSpecified(); err != nil {
				panic(err)
			}
			if err := opts.validate(args); err != nil {
				panic(err)
			}
			if err := opts.run(ctx, cancel); err != nil {
				panic(err)
			}
		},
	}

	opts.config, err = opts.applyDefaults(opts.config)
	if err != nil {
		panic(err)
	}
	opts.AddFlags(cmd.Flags())
	return cmd
}

// Gardener represents all the parameters required to start the
// Gardener controller manager.
type Gardener struct {
	Config                 *config.ControllerManagerConfiguration
	ClientMap              clientmap.ClientMap
	K8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	KubeInformerFactory    informers.SharedInformerFactory
	Logger                 *logrus.Logger
	Recorder               record.EventRecorder
	LeaderElection         *leaderelection.LeaderElectionConfig
}

// NewGardener is the main entry point of instantiating a new Gardener controller manager.
func NewGardener(ctx context.Context, cfg *config.ControllerManagerConfiguration) (*Gardener, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Initialize logger
	logger := logger.NewLogger(cfg.LogLevel)
	logger.Info("Starting Gardener controller manager...")
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
		Config:                 cfg,
		Logger:                 logger,
		Recorder:               recorder,
		ClientMap:              clientMap,
		K8sGardenCoreInformers: gardencoreinformers.NewSharedInformerFactory(k8sGardenClient.GardenCore(), 0),
		KubeInformerFactory:    kubeinformers.NewSharedInformerFactory(k8sGardenClient.Kubernetes(), 0),
		LeaderElection:         leaderElectionConfig,
	}, nil
}

// Run runs the Gardener. This should never exit.
func (g *Gardener) Run(ctx context.Context, cancel context.CancelFunc) error {
	leaderElectionCtx, leaderElectionCancel := context.WithCancel(context.Background())

	// Prepare a reusable run function.
	run := func(ctx context.Context) {
		var (
			projects        = g.K8sGardenCoreInformers.Core().V1beta1().Projects()
			projectInformer = projects.Informer()
			shoots          = g.K8sGardenCoreInformers.Core().V1beta1().Shoots()
			shootInformer   = shoots.Informer()
		)

		k8sGardenClient, err := g.ClientMap.GetClient(ctx, keys.ForGarden())
		if err != nil {
			panic(fmt.Errorf("failed to get garden client: %+v", err))
		}

		g.K8sGardenCoreInformers.Start(ctx.Done())
		if !cache.WaitForCacheSync(ctx.Done(), projectInformer.HasSynced, shootInformer.HasSynced) {
			panic("Timed out waiting for Garden caches to sync")
		}

		// Start webhook server
		go server.
			NewBuilder().
			WithBindAddress(g.Config.Server.HTTPS.BindAddress).
			WithPort(g.Config.Server.HTTPS.Port).
			WithTLS(g.Config.Server.HTTPS.TLS.ServerCertPath, g.Config.Server.HTTPS.TLS.ServerKeyPath).
			WithHandler("/webhooks/validate-namespace-deletion", webhooks.NewValidateNamespaceDeletionHandler(k8sGardenClient, projects.Lister(), shoots.Lister())).
			WithHandler("/webhooks/validate-kubeconfig-secrets", webhooks.NewValidateKubeconfigSecretsHandler()).
			Build().
			Start(ctx)

		// Start controllers
		g.startControllers(ctx)
	}

	// Initialize /healthz manager.
	healthManager := healthz.NewDefaultHealthz()
	healthManager.Start()

	// Start HTTP server.
	go server.
		NewBuilder().
		WithBindAddress(g.Config.Server.HTTP.BindAddress).
		WithPort(g.Config.Server.HTTP.Port).
		WithHandler("/metrics", promhttp.Handler()).
		WithHandlerFunc("/healthz", healthz.HandlerFunc(healthManager)).
		Build().
		Start(ctx)

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				g.Logger.Info("Acquired leadership, starting controllers.")
				run(ctx)
				leaderElectionCancel()
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
		leaderElector.Run(leaderElectionCtx)
		return nil
	}

	// Leader election is disabled, thus run directly until done.
	leaderElectionCancel()
	run(ctx)
	return nil
}

func (g *Gardener) startControllers(ctx context.Context) {
	controller.NewGardenControllerFactory(
		g.ClientMap,
		g.K8sGardenCoreInformers,
		g.KubeInformerFactory,
		g.Config,
		g.Recorder,
	).Run(ctx)
}
