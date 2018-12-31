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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	"k8s.io/client-go/informers"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	componentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/componentconfig/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller"
	gardenerfeatures "github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/server"
	"github.com/gardener/gardener/pkg/server/handlers"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/version"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubeinformers "k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

// Options has all the context and parameters needed to run a Gardener controller manager.
type Options struct {
	// ConfigFile is the location of the Gardener controller manager's configuration file.
	ConfigFile string
	config     *componentconfig.ControllerManagerConfiguration
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
		config: new(componentconfig.ControllerManagerConfiguration),
	}

	o.scheme = runtime.NewScheme()
	o.codecs = serializer.NewCodecFactory(o.scheme)

	if err := componentconfig.AddToScheme(o.scheme); err != nil {
		return nil, err
	}
	if err := componentconfigv1alpha1.AddToScheme(o.scheme); err != nil {
		return nil, err
	}
	if err := gardenv1beta1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	return o, nil
}

// loadConfigFromFile loads the contents of file and decodes it as a
// ControllerManagerConfiguration object.
func (o *Options) loadConfigFromFile(file string) (*componentconfig.ControllerManagerConfiguration, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return o.decodeConfig(data)
}

// decodeConfig decodes data as a ControllerManagerConfiguration object.
func (o *Options) decodeConfig(data []byte) (*componentconfig.ControllerManagerConfiguration, error) {
	configObj, gvk, err := o.codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*componentconfig.ControllerManagerConfiguration)
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

func (o *Options) applyDefaults(in *componentconfig.ControllerManagerConfiguration) (*componentconfig.ControllerManagerConfiguration, error) {
	external, err := o.scheme.ConvertToVersion(in, componentconfigv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	o.scheme.Default(external)

	internal, err := o.scheme.ConvertToVersion(external, componentconfig.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	out := internal.(*componentconfig.ControllerManagerConfiguration)

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
	if err := gardenerfeatures.ControllerFeatureGate.SetFromMap(o.config.FeatureGates); err != nil {
		return err
	}

	gardener, err := NewGardener(o.config)
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
	Config              *componentconfig.ControllerManagerConfiguration
	Identity            *gardenv1beta1.Gardener
	GardenerNamespace   string
	K8sGardenClient     kubernetes.Interface
	K8sGardenInformers  gardeninformers.SharedInformerFactory
	KubeInformerFactory informers.SharedInformerFactory
	Logger              *logrus.Logger
	Recorder            record.EventRecorder
	LeaderElection      *leaderelection.LeaderElectionConfig
}

// NewGardener is the main entry point of instantiating a new Gardener controller manager.
func NewGardener(config *componentconfig.ControllerManagerConfiguration) (*Gardener, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	componentconfig.ApplyEnvironmentToConfig(config)

	// Initialize logger
	logger := logger.NewLogger(config.LogLevel)
	logger.Info("Starting Gardener controller manager...")
	logger.Infof("Feature Gates: %s", gardenerfeatures.ControllerFeatureGate.String())

	if flag := flag.Lookup("v"); flag != nil {
		if err := flag.Value.Set(fmt.Sprintf("%d", config.KubernetesLogLevel)); err != nil {
			return nil, err
		}
	}

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	var (
		kubeconfig         = config.ClientConnection.KubeConfigFile
		gardenerKubeConfig = config.GardenerClientConnection.KubeConfigFile

		gardenerClient = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: gardenerKubeConfig},
			&clientcmd.ConfigOverrides{},
		)
	)

	k8sGardenClient, err := kubernetes.NewClientFromFile(kubeconfig, &config.ClientConnection, client.Options{
		Scheme: kubernetes.GardenScheme,
	})
	if err != nil {
		return nil, err
	}
	k8sGardenClientLeaderElection, err := kubernetes.NewClientFromFile(kubeconfig, nil, client.Options{})
	if err != nil {
		return nil, err
	}

	// Create a GardenV1beta1Client and the respective API group scheme for the Garden API group.
	gardenerClientConfig, err := kubernetes.CreateRESTConfig(gardenerClient, &config.ClientConnection)
	if err != nil {
		return nil, err
	}
	gardenClientset, err := gardenclientset.NewForConfig(gardenerClientConfig)
	if err != nil {
		return nil, err
	}
	k8sGardenClient.SetGarden(gardenClientset)

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = createRecorder(k8sGardenClient.Kubernetes())
	)
	if config.LeaderElection.LeaderElect {
		leaderElectionConfig, err = makeLeaderElectionConfig(config.LeaderElection, k8sGardenClientLeaderElection.Kubernetes(), recorder)
		if err != nil {
			return nil, err
		}
	}

	identity, gardenerNamespace, err := determineGardenerIdentity()
	if err != nil {
		return nil, err
	}

	return &Gardener{
		Identity:            identity,
		GardenerNamespace:   gardenerNamespace,
		Config:              config,
		Logger:              logger,
		Recorder:            recorder,
		K8sGardenClient:     k8sGardenClient,
		K8sGardenInformers:  gardeninformers.NewSharedInformerFactory(k8sGardenClient.Garden(), 0),
		KubeInformerFactory: kubeinformers.NewSharedInformerFactory(k8sGardenClient.Kubernetes(), 0),
		LeaderElection:      leaderElectionConfig,
	}, nil
}

// Run runs the Gardener. This should never exit.
func (g *Gardener) Run(ctx context.Context, cancel context.CancelFunc) error {
	leaderElectionCtx, leaderElectionCancel := context.WithCancel(context.Background())

	// Prepare a reusable run function.
	run := func(ctx context.Context) {
		g.startControllers(ctx)
	}

	// Start HTTP server
	go server.Serve(ctx, g.K8sGardenClient, g.K8sGardenInformers, g.Config.Server)
	handlers.UpdateHealth(true)

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
		g.K8sGardenClient,
		g.K8sGardenInformers,
		g.KubeInformerFactory,
		g.Config,
		g.Identity,
		g.GardenerNamespace,
		g.Recorder,
	).Run(ctx)
}

func createRecorder(kubeClient k8s.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Logger.Debugf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "gardener-controller-manager"})
}

func makeLeaderElectionConfig(config componentconfig.LeaderElectionConfiguration, client k8s.Interface, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	lock, err := resourcelock.New(config.ResourceLock,
		config.LockObjectNamespace,
		config.LockObjectName,
		client.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      hostname,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create resource lock: %v", err)
	}

	return &leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: config.LeaseDuration.Duration,
		RenewDeadline: config.RenewDeadline.Duration,
		RetryPeriod:   config.RetryPeriod.Duration,
	}, nil
}

// We want to determine the Docker container id of the currently running Gardener controller manager because
// we need to identify for still ongoing operations whether another Gardener controller manager instance is
// still operating the respective Shoots. When running locally, we generate a random string because
// there is no container id.
func determineGardenerIdentity() (*gardenv1beta1.Gardener, string, error) {
	var (
		gardenerID        string
		gardenerName      string
		gardenerNamespace = common.GardenNamespace
		err               error
	)

	gardenerName, err = os.Hostname()
	if err != nil {
		return nil, "", fmt.Errorf("unable to get hostname: %v", err)
	}

	// If running inside a Kubernetes cluster (as container) we can read the container id from the proc file system.
	// Otherwise generate a random string for the gardenerID
	if cgroup, err := ioutil.ReadFile("/proc/self/cgroup"); err == nil {
		splitByNewline := strings.Split(string(cgroup), "\n")
		splitBySlash := strings.Split(splitByNewline[0], "/")
		gardenerID = splitBySlash[len(splitBySlash)-1]
	} else {
		gardenerID, err = utils.GenerateRandomString(64)
		if err != nil {
			return nil, "", fmt.Errorf("unable to generate gardenerID: %v", err)
		}
	}

	// If running inside a Kubernetes cluster we will have a service account mount.
	if ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		gardenerNamespace = string(ns)
	}

	return &gardenv1beta1.Gardener{
		ID:      gardenerID,
		Name:    gardenerName,
		Version: version.Get().GitVersion,
	}, gardenerNamespace, nil
}
