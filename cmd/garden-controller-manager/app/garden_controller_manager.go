// Copyright 2018 The Gardener Authors.
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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	componentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/componentconfig/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller"
	"github.com/gardener/gardener/pkg/logger"
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
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

// Options has all the context and parameters needed to run a Garden controller manager.
type Options struct {
	// ConfigFile is the location of the Garden controller manager's configuration file.
	ConfigFile string
	config     *componentconfig.ControllerManagerConfiguration
	scheme     *runtime.Scheme
	codecs     serializer.CodecFactory
}

// AddFlags adds flags for a specific Garden controller manager to the specified FlagSet.
func AddFlags(options *Options, fs *pflag.FlagSet) {
	fs.StringVar(&options.ConfigFile, "config", options.ConfigFile, "The path to the configuration file.")
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
		return fmt.Errorf("missing Garden controller manager config file")
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

func (o *Options) run() error {
	config := o.config

	if len(o.ConfigFile) > 0 {
		c, err := o.loadConfigFromFile(o.ConfigFile)
		if err != nil {
			return err
		}
		config = c
	}

	gardener, err := NewGardener(config)
	if err != nil {
		return err
	}

	stop := make(chan struct{})
	return gardener.Run(stop)
}

// NewCommandStartGardenControllerManager creates a *cobra.Command object with default parameters
func NewCommandStartGardenControllerManager() *cobra.Command {
	opts, err := NewOptions()
	if err != nil {
		panic(err)
	}

	cmd := &cobra.Command{
		Use:   "garden-controller-manager",
		Short: "Launch the Garden controller manager",
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
			if err := opts.run(); err != nil {
				panic(err)
			}
		},
	}

	opts.config, err = opts.applyDefaults(opts.config)
	if err != nil {
		panic(err)
	}
	AddFlags(opts, cmd.Flags())
	return cmd
}

// Gardener represents all the parameters required to start the
// Garden controller manager.
type Gardener struct {
	Config            *componentconfig.ControllerManagerConfiguration
	Identity          *gardenv1beta1.Gardener
	GardenerNamespace string
	K8sGardenClient   kubernetes.Client
	Logger            *logrus.Logger
	RunningInCluster  bool
	Recorder          record.EventRecorder
	LeaderElection    *leaderelection.LeaderElectionConfig
}

// NewGardener is the main entry point of instantiating a new Garden controller manager.
func NewGardener(config *componentconfig.ControllerManagerConfiguration) (*Gardener, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	componentconfig.ApplyEnvironmentToConfig(config)

	// Initialize logger
	logger := logger.NewLogger(config.LogLevel)
	logger.Info("Starting Garden controller manager...")

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	var (
		kubeconfig       = config.ClientConnection.KubeConfigFile
		runningInCluster = kubeconfig == ""
	)
	k8sGardenClient, err := kubernetes.NewClientFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	k8sGardenClientLeaderElection, err := kubernetes.NewClientFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}

	// Create a GardenV1beta1Client and the respective API group scheme for the Garden API group.
	gardenClientset, err := gardenclientset.NewForConfig(k8sGardenClient.GetConfig())
	if err != nil {
		return nil, err
	}
	k8sGardenClient.SetGardenClientset(gardenClientset)

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = createRecorder(k8sGardenClient.GetClientset())
	)
	if config.LeaderElection.LeaderElect {
		leaderElectionConfig, err = makeLeaderElectionConfig(config.LeaderElection, k8sGardenClientLeaderElection.GetClientset(), recorder)
		if err != nil {
			return nil, err
		}
	}

	identity, gardenerNamespace, err := getGardenerIdentity(k8sGardenClient, runningInCluster, config)
	if err != nil {
		return nil, err
	}

	return &Gardener{
		Identity:          identity,
		GardenerNamespace: gardenerNamespace,
		Config:            config,
		Logger:            logger,
		Recorder:          recorder,
		K8sGardenClient:   k8sGardenClient,
		LeaderElection:    leaderElectionConfig,
		RunningInCluster:  runningInCluster,
	}, nil
}

// Run runs the Gardener. This should never exit.
func (g *Gardener) Run(stopCh chan struct{}) error {
	// React on SIGTERM to allow a graceful shutdown (only when using in-cluster config)
	if g.RunningInCluster {
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			close(stopCh)
		}()
	}

	// Prepare a reusable run function.
	run := func(stop <-chan struct{}) {
		startControllers(g, stopCh)
		<-stop
		close(stopCh)
	}

	// Start HTTP server
	go server.Serve(g.K8sGardenClient, g.Config.Server.BindAddress, g.Config.Server.Port, g.Config.Metrics.Interval.Duration)
	handlers.UpdateHealth(true)

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				g.Logger.Info("Lost leadership, cleaning up.")
				close(stopCh)
			},
		}
		leaderElector, err := leaderelection.NewLeaderElector(*g.LeaderElection)
		if err != nil {
			return fmt.Errorf("couldn't create leader elector: %v", err)
		}
		leaderElector.Run()
		return fmt.Errorf("lost lease")
	}

	// Leader election is disabled, so run inline until done.
	run(stopCh)
	return fmt.Errorf("finished without leader elect")
}

func startControllers(g *Gardener, stopCh <-chan struct{}) error {
	gardenInformerFactory := gardeninformers.NewSharedInformerFactory(g.K8sGardenClient.GetGardenClientset(), g.Config.Controller.Reconciliation.ResyncPeriod.Duration)

	controller.NewGardenControllerFactory(
		g.K8sGardenClient,
		gardenInformerFactory,
		g.Config,
		g.Identity,
		g.GardenerNamespace,
		g.Recorder,
	).Run(stopCh)

	select {}
}

func createRecorder(kubeClient *k8s.Clientset) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Logger.Debugf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "garden-controller-manager"})
}

func makeLeaderElectionConfig(config componentconfig.LeaderElectionConfiguration, client *k8s.Clientset, recorder record.EventRecorder) (*leaderelection.LeaderElectionConfig, error) {
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

func getGardenerNamespace(runningInCluster bool, gardenNamespace string, watchNamespace *string) string {
	if runningInCluster {
		if ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			return string(ns)
		}
	}
	if watchNamespace == nil {
		return gardenNamespace
	}
	return *watchNamespace
}

// We want to determine the Docker container id of the currently running Garden controller manager because
// we need to identify for still ongoing operations whether another Garden controller manager instance is
// still operating the respective Shoots. When running locally, we generate a random string because
// there is no container id.
func getGardenerIdentity(k8sGardenClient kubernetes.Client, runningInCluster bool, config *componentconfig.ControllerManagerConfiguration) (*gardenv1beta1.Gardener, string, error) {
	var (
		gardenerID        = utils.GenerateRandomString(64)
		gardenerName, _   = os.Hostname()
		gardenerNamespace = getGardenerNamespace(runningInCluster, config.GardenNamespace, config.Controller.WatchNamespace)
	)
	if runningInCluster {
		gardenerID = ""
		for {
			if gardenerID != "" {
				break
			}
			pod, err := k8sGardenClient.GetPod(gardenerNamespace, gardenerName)
			if err != nil {
				return nil, "", err
			}
			if len(pod.Status.ContainerStatuses) == 0 || !strings.HasPrefix(pod.Status.ContainerStatuses[0].ContainerID, "docker://") {
				logger.Logger.Info("Waiting until the kubelet has propagated my Docker container id...")
				time.Sleep(1 * time.Second)
				continue
			}
			containerID := pod.Status.ContainerStatuses[0].ContainerID
			gardenerID = strings.Split(containerID, "docker://")[1]
		}
	}
	return &gardenv1beta1.Gardener{
		ID:      gardenerID,
		Name:    gardenerName,
		Version: version.Version,
	}, gardenerNamespace, nil
}
