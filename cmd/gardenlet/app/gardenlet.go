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
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gardener/gardener/cmd/utils"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	configvalidation "github.com/gardener/gardener/pkg/gardenlet/apis/config/validation"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	"github.com/gardener/gardener/pkg/gardenlet/controller"
	seedcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/version"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	diskcache "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/informers"
	kubeinformers "k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Options has all the context and parameters needed to run a Gardenlet.
type Options struct {
	// ConfigFile is the location of the Gardenlet's configuration file.
	ConfigFile string
	config     *config.GardenletConfiguration
	scheme     *runtime.Scheme
	codecs     serializer.CodecFactory
}

// AddFlags adds flags for a specific Gardenlet to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ConfigFile, "config", o.ConfigFile, "The path to the configuration file.")
}

// NewOptions returns a new Options object.
func NewOptions() (*Options, error) {
	o := &Options{
		config: new(config.GardenletConfiguration),
	}

	o.scheme = runtime.NewScheme()
	o.codecs = serializer.NewCodecFactory(o.scheme)

	if err := config.AddToScheme(o.scheme); err != nil {
		return nil, err
	}
	if err := configv1alpha1.AddToScheme(o.scheme); err != nil {
		return nil, err
	}
	if err := gardencorev1beta1.AddToScheme(o.scheme); err != nil {
		return nil, err
	}

	return o, nil
}

// loadConfigFromFile loads the content of file and decodes it as a
// GardenletConfiguration object.
func (o *Options) loadConfigFromFile(file string) (*config.GardenletConfiguration, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return o.decodeConfig(data)
}

// decodeConfig decodes data as a GardenletConfiguration object.
func (o *Options) decodeConfig(data []byte) (*config.GardenletConfiguration, error) {
	gardenletConfig := &config.GardenletConfiguration{}
	if _, _, err := o.codecs.UniversalDecoder().Decode(data, nil, gardenletConfig); err != nil {
		return nil, err
	}
	return gardenletConfig, nil
}

func (o *Options) configFileSpecified() error {
	if len(o.ConfigFile) == 0 {
		return fmt.Errorf("missing Gardenlet config file")
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

func (o *Options) applyDefaults(in *config.GardenletConfiguration) (*config.GardenletConfiguration, error) {
	external, err := o.scheme.ConvertToVersion(in, configv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	o.scheme.Default(external)

	internal, err := o.scheme.ConvertToVersion(external, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	out := internal.(*config.GardenletConfiguration)

	return out, nil
}

func run(ctx context.Context, o *Options) error {
	if len(o.ConfigFile) > 0 {
		c, err := o.loadConfigFromFile(o.ConfigFile)
		if err != nil {
			return err
		}

		c, err = o.applyDefaults(c)
		if err != nil {
			return err
		}

		if errs := configvalidation.ValidateGardenletConfiguration(c); len(errs) > 0 {
			return fmt.Errorf("errors validating the configuration: %+v", errs)
		}

		o.config = c
	}

	// Add feature flags
	if err := features.FeatureGate.SetFromMap(o.config.FeatureGates); err != nil {
		return err
	}

	gardenlet, err := NewGardenlet(ctx, o.config)
	if err != nil {
		return err
	}

	return gardenlet.Run(ctx)
}

// NewCommandStartGardenlet creates a *cobra.Command object with default parameters
func NewCommandStartGardenlet(ctx context.Context) *cobra.Command {
	opts, err := NewOptions()
	if err != nil {
		panic(err)
	}

	cmd := &cobra.Command{
		Use:   "gardenlet",
		Short: "Launch the Gardenlet",
		Long: `In essence, the Gardener is an extension API server along with a bundle
of Kubernetes controllers which introduce new API objects in an existing Kubernetes
cluster (which is called Garden cluster) in order to use them for the management of
further Kubernetes clusters (which are called Shoot clusters).
To do that reliably and to offer a certain quality of service, it requires to control
the main components of a Kubernetes cluster (etcd, API server, controller manager, scheduler).
These so-called control plane components are hosted in Kubernetes clusters themselves
(which are called Seed clusters).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.configFileSpecified(); err != nil {
				return err
			}
			if err := opts.validate(args); err != nil {
				return err
			}
			return run(ctx, opts)
		},
	}

	opts.AddFlags(cmd.Flags())

	return cmd
}

// Gardenlet represents all the parameters required to start the
// Gardenlet.
type Gardenlet struct {
	Config                 *config.GardenletConfiguration
	Identity               *gardencorev1beta1.Gardener
	GardenNamespace        string
	K8sGardenClient        kubernetes.Interface
	K8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	KubeInformerFactory    informers.SharedInformerFactory
	Logger                 *logrus.Logger
	Recorder               record.EventRecorder
	LeaderElection         *leaderelection.LeaderElectionConfig
	HealthManager          healthz.Manager
}

func discoveryFromGardenletConfiguration(cfg *config.GardenletConfiguration, kubeconfig []byte) (discovery.CachedDiscoveryInterface, error) {
	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.GardenClientConnection.ClientConnectionConfiguration, kubeconfig)
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

// NewGardenlet is the main entry point of instantiating a new Gardenlet.
func NewGardenlet(ctx context.Context, cfg *config.GardenletConfiguration) (*Gardenlet, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Initialize logger
	logger := logger.NewLogger(*cfg.LogLevel)
	logger.Info("Starting Gardenlet...")
	logger.Infof("Feature Gates: %s", features.FeatureGate.String())

	if flag := flag.Lookup("v"); flag != nil {
		if err := flag.Value.Set(fmt.Sprintf("%d", cfg.KubernetesLogLevel)); err != nil {
			return nil, err
		}
	}

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("GARDEN_KUBECONFIG"); kubeconfig != "" {
		cfg.GardenClientConnection.Kubeconfig = kubeconfig
	}
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.SeedClientConnection.Kubeconfig = kubeconfig
	}

	var (
		kubeconfigFromBootstrap []byte
		csrName                 string
		seedName                string
		err                     error
	)

	if cfg.GardenClientConnection.KubeconfigSecret != nil {
		kubeconfigFromBootstrap, csrName, seedName, err = bootstrapKubeconfig(ctx, logger, cfg.GardenClientConnection, cfg.SeedClientConnection.ClientConnectionConfiguration, cfg.SeedConfig)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("No kubeconfig secret given in `.gardenClientConnection` configuration - skipping kubeconfig bootstrap process.")
	}
	if kubeconfigFromBootstrap == nil {
		logger.Info("No kubeconfig generated from bootstrap process found. Using kubeconfig specified in `.gardenClientConnection` in configuration")
	}

	restCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.GardenClientConnection.ClientConnectionConfiguration, kubeconfigFromBootstrap)
	if err != nil {
		return nil, err
	}

	disc, err := discoveryFromGardenletConfiguration(cfg, kubeconfigFromBootstrap)
	if err != nil {
		return nil, err
	}

	k8sGardenClient, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(restCfg),
		kubernetes.WithClientOptions(client.Options{
			Mapper: restmapper.NewDeferredDiscoveryRESTMapper(disc),
			Scheme: kubernetes.GardenScheme,
		}),
	)
	if err != nil {
		return nil, err
	}

	// Delete bootstrap auth data if certificate was freshly acquired
	if len(csrName) > 0 {
		logger.Infof("Deleting bootstrap authentication data used to request a certificate")
		if err := bootstrap.DeleteBootstrapAuth(ctx, k8sGardenClient.Client(), csrName, seedName); err != nil {
			return nil, err
		}
	}

	seedRestCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.SeedClientConnection.ClientConnectionConfiguration, nil)
	if err != nil {
		return nil, err
	}
	k8sSeedClientLeaderElection, err := k8s.NewForConfig(seedRestCfg)
	if err != nil {
		return nil, err
	}

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = utils.CreateRecorder(k8sGardenClient.Kubernetes(), "gardenlet")
	)
	if cfg.LeaderElection.LeaderElect {
		leaderElectionConfig, err = utils.MakeLeaderElectionConfig(cfg.LeaderElection.LeaderElectionConfiguration, *cfg.LeaderElection.LockObjectNamespace, *cfg.LeaderElection.LockObjectName, k8sSeedClientLeaderElection, recorder)
		if err != nil {
			return nil, err
		}
	}

	identity, gardenNamespace, err := determineGardenletIdentity()
	if err != nil {
		return nil, err
	}

	return &Gardenlet{
		Identity:               identity,
		GardenNamespace:        gardenNamespace,
		Config:                 cfg,
		Logger:                 logger,
		Recorder:               recorder,
		K8sGardenClient:        k8sGardenClient,
		K8sGardenCoreInformers: gardencoreinformers.NewSharedInformerFactory(k8sGardenClient.GardenCore(), 0),
		KubeInformerFactory:    kubeinformers.NewSharedInformerFactory(k8sGardenClient.Kubernetes(), 0),
		LeaderElection:         leaderElectionConfig,
	}, nil
}

func (g *Gardenlet) cleanup() {
	if err := os.RemoveAll(configv1alpha1.DefaultDiscoveryDir); err != nil {
		g.Logger.Errorf("Could not cleanup base discovery cache directory: %v", err)
	}
}

// Run runs the Gardenlet. This should never exit.
func (g *Gardenlet) Run(ctx context.Context) error {
	controllerCtx, controllerCancel := context.WithCancel(ctx)

	defer controllerCancel()
	defer g.cleanup()

	// Initialize /healthz manager.
	g.HealthManager = healthz.NewPeriodicHealthz(seedcontroller.LeaseResyncGracePeriodSeconds * time.Second)
	g.HealthManager.Set(true)

	// Start HTTPS server.
	if g.Config.Server.HTTPS.TLS == nil {
		g.Logger.Info("No TLS server certificates provided... self-generating them now...")

		_, tempDir, err := secrets.SelfGenerateTLSServerCertificate(
			"gardenlet",
			[]string{
				"gardenlet",
				fmt.Sprintf("gardenlet.%s", v1beta1constants.GardenNamespace),
				fmt.Sprintf("gardenlet.%s.svc", v1beta1constants.GardenNamespace),
			},
		)
		if err != nil {
			return err
		}

		g.Config.Server.HTTPS.TLS = &config.TLSServer{
			ServerCertPath: filepath.Join(tempDir, secrets.DataKeyCertificate),
			ServerKeyPath:  filepath.Join(tempDir, secrets.DataKeyPrivateKey),
		}

		g.Logger.Info("TLS server certificates successfully self-generated.")
	}

	go server.
		NewBuilder().
		WithBindAddress(g.Config.Server.HTTPS.BindAddress).
		WithPort(g.Config.Server.HTTPS.Port).
		WithTLS(g.Config.Server.HTTPS.TLS.ServerCertPath, g.Config.Server.HTTPS.TLS.ServerKeyPath).
		WithHandler("/metrics", promhttp.Handler()).
		WithHandlerFunc("/healthz", healthz.HandlerFunc(g.HealthManager)).
		Build().
		Start(ctx)

	// Prepare a reusable run function.
	run := func(ctx context.Context) {
		g.HealthManager.Start()
		g.startControllers(ctx)
	}

	leaderElectionCtx, leaderElectionCancel := context.WithCancel(context.Background())

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				g.Logger.Info("Acquired leadership, starting controllers.")
				run(controllerCtx)
				leaderElectionCancel()
			},
			OnStoppedLeading: func() {
				g.Logger.Info("Lost leadership, terminating.")
				controllerCancel()
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
	run(controllerCtx)
	return nil
}

func (g *Gardenlet) startControllers(ctx context.Context) {
	controller.NewGardenletControllerFactory(
		g.K8sGardenClient,
		g.K8sGardenCoreInformers,
		g.KubeInformerFactory,
		g.Config,
		g.Identity,
		g.GardenNamespace,
		g.Recorder,
		g.HealthManager,
	).Run(ctx)
}

// We want to determine the Docker container id of the currently running Gardenlet because
// we need to identify for still ongoing operations whether another Gardenlet instance is
// still operating the respective Shoots. When running locally, we generate a random string because
// there is no container id.
func determineGardenletIdentity() (*gardencorev1beta1.Gardener, string, error) {
	var (
		validID     = regexp.MustCompile(`([0-9a-f]{64})`)
		gardenletID string

		gardenletName   string
		gardenNamespace = v1beta1constants.GardenNamespace
		err             error
	)

	gardenletName, err = os.Hostname()
	if err != nil {
		return nil, "", fmt.Errorf("unable to get hostname: %v", err)
	}

	// If running inside a Kubernetes cluster (as container) we can read the container id from the proc file system.
	// Otherwise generate a random string for the gardenletID
	if cGroupFile, err := os.Open("/proc/self/cgroup"); err == nil {
		defer cGroupFile.Close()
		reader := bufio.NewReader(cGroupFile)

		var cgroupV1 string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}

			// Store cgroup-v1 result for fall back
			if strings.HasPrefix(line, "1:name=systemd") {
				cgroupV1 = line
			}

			// Always prefer cgroup-v2
			if strings.HasPrefix(line, "0::") {
				if id := extractID(line); validID.MatchString(id) {
					gardenletID = id
					break
				}
			}
		}

		// Fall-back to cgroup-v1 if possible
		if len(gardenletID) == 0 && len(cgroupV1) > 0 {
			gardenletID = extractID(cgroupV1)
		}
	}

	if gardenletID == "" {
		gardenletID, err = gardenerutils.GenerateRandomString(64)
		if err != nil {
			return nil, "", fmt.Errorf("unable to generate gardenletID: %v", err)
		}
	}

	// If running inside a Kubernetes cluster we will have a service account mount.
	if ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		gardenNamespace = string(ns)
	}

	return &gardencorev1beta1.Gardener{
		ID:      gardenletID,
		Name:    gardenletName,
		Version: version.Get().GitVersion,
	}, gardenNamespace, nil
}

func extractID(line string) string {
	var (
		id           string
		splitBySlash = strings.Split(string(line), "/")
	)

	if len(splitBySlash) == 0 {
		return ""
	}

	id = strings.TrimSpace(splitBySlash[len(splitBySlash)-1])
	id = strings.TrimSuffix(id, ".scope")
	id = strings.TrimPrefix(id, "docker-")

	return id
}
