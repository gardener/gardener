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
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	cmdutils "github.com/gardener/gardener/cmd/utils"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configvalidation "github.com/gardener/gardener/pkg/gardenlet/apis/config/validation"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	"github.com/gardener/gardener/pkg/gardenlet/controller"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server"
	"github.com/gardener/gardener/pkg/server/routes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func run(ctx context.Context, o *Options) error {
	c, err := o.loadConfigFromFile(o.ConfigFile)
	if err != nil {
		return fmt.Errorf("unable to read the configuration file: %w", err)
	}

	if errs := configvalidation.ValidateGardenletConfiguration(c, nil, false); len(errs) > 0 {
		return fmt.Errorf("errors validating the configuration: %+v", errs)
	}

	o.config = c

	// Add feature flags
	if err := gardenletfeatures.FeatureGate.SetFromMap(o.config.FeatureGates); err != nil {
		return err
	}

	if gardenletfeatures.FeatureGate.Enabled(features.ReversedVPN) && !gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		return fmt.Errorf("inconsistent feature gate: APIServerSNI is required for ReversedVPN (APIServerSNI: %t, ReversedVPN: %t)",
			gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI), gardenletfeatures.FeatureGate.Enabled(features.ReversedVPN))
	}

	gardenlet, err := NewGardenlet(ctx, o.config)
	if err != nil {
		return err
	}

	return gardenlet.Run(ctx)
}

// NewCommandStartGardenlet creates a *cobra.Command object with default parameters
func NewCommandStartGardenlet() *cobra.Command {
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
			verflag.PrintAndExitIfRequested()

			if err := opts.configFileSpecified(); err != nil {
				return err
			}
			if err := opts.validate(args); err != nil {
				return err
			}
			return run(cmd.Context(), opts)
		},
		SilenceUsage: true,
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	return cmd
}

// Gardenlet represents all the parameters required to start the
// Gardenlet.
type Gardenlet struct {
	Config                               *config.GardenletConfiguration
	Identity                             *gardencorev1beta1.Gardener
	GardenClusterIdentity                string
	ClientMap                            clientmap.ClientMap
	Log                                  logr.Logger
	Recorder                             record.EventRecorder
	LeaderElection                       *leaderelection.LeaderElectionConfig
	HealthManager                        healthz.Manager
	CertificateManager                   *certificate.Manager
	ClientCertificateExpirationTimestamp *metav1.Time
}

// NewGardenlet is the main entry point of instantiating a new Gardenlet.
func NewGardenlet(ctx context.Context, cfg *config.GardenletConfiguration) (*Gardenlet, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	log, err := logger.NewZapLogger(*cfg.LogLevel, *cfg.LogFormat)
	if err != nil {
		return nil, fmt.Errorf("error instantiating zap logger: %w", err)
	}

	logf.SetLogger(log)
	klog.SetLogger(log)

	log.Info("Starting gardenlet", "version", version.Get())
	log.Info("Feature Gates", "featureGates", gardenletfeatures.FeatureGate.String())

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("GARDEN_KUBECONFIG"); kubeconfig != "" {
		cfg.GardenClientConnection.Kubeconfig = kubeconfig
	}
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.SeedClientConnection.Kubeconfig = kubeconfig
	}

	var (
		clientCertificateExpirationTimestamp *metav1.Time
		kubeconfigFromBootstrap              []byte
		csrName                              string
		seedName                             string
	)

	// constructs a seed client for `SeedClientConnection.kubeconfig` or if not set,
	// creates a seed client based on the service account token mounted into the gardenlet container running in Kubernetes
	// when running outside of Kubernetes, `SeedClientConnection.kubeconfig` has to be set either directly or via the environment variable "KUBECONFIG"
	seedClientForBootstrap, err := kubernetes.NewClientFromFile(
		"",
		cfg.SeedClientConnection.ClientConnectionConfiguration.Kubeconfig,
		kubernetes.WithClientConnectionOptions(cfg.SeedClientConnection.ClientConnectionConfiguration),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, err
	}

	restCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.GardenClientConnection.ClientConnectionConfiguration, kubeconfigFromBootstrap)
	if err != nil {
		return nil, err
	}

	gardenClientMapBuilder := clientmapbuilder.NewGardenClientMapBuilder().
		WithRESTConfig(restCfg).
		// gardenlet does not have the required RBAC permissions for listing/watching the following resources, so let's prevent any
		// attempts to cache them
		WithUncached(
			&gardencorev1alpha1.ExposureClass{},
			&gardencorev1alpha1.ShootState{},
			&gardencorev1beta1.CloudProfile{},
			&gardencorev1beta1.ControllerDeployment{},
			&gardencorev1beta1.Project{},
			&gardencorev1beta1.SecretBinding{},
			&certificatesv1.CertificateSigningRequest{},
			&coordinationv1.Lease{},
			&corev1.Namespace{},
			&corev1.ConfigMap{},
			&corev1.Event{},
			&eventsv1.Event{},
		).
		ForSeed(cfg.SeedConfig.Name)

	seedClientMapBuilder := clientmapbuilder.NewSeedClientMapBuilder().
		WithClientConnectionConfig(&cfg.SeedClientConnection.ClientConnectionConfiguration)
	shootClientMapBuilder := clientmapbuilder.NewShootClientMapBuilder().
		WithClientConnectionConfig(&cfg.ShootClientConnection.ClientConnectionConfiguration)

	clientMap, err := clientmapbuilder.NewDelegatingClientMapBuilder(log).
		WithGardenClientMapBuilder(gardenClientMapBuilder).
		WithSeedClientMapBuilder(seedClientMapBuilder).
		WithShootClientMapBuilder(shootClientMapBuilder).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build ClientMap: %w", err)
	}

	k8sGardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	// Delete bootstrap auth data if certificate was newly acquired
	if len(csrName) > 0 && len(seedName) > 0 {
		log.Info("Deleting bootstrap authentication data used to request a certificate")
		if err := bootstrap.DeleteBootstrapAuth(ctx, k8sGardenClient.Client(), k8sGardenClient.Client(), csrName, seedName); err != nil {
			return nil, err
		}
	}

	// Set up leader election if enabled and prepare event recorder.
	var (
		leaderElectionConfig *leaderelection.LeaderElectionConfig
		recorder             = cmdutils.CreateRecorder(k8sGardenClient.Kubernetes(), "gardenlet")
	)
	if cfg.LeaderElection.LeaderElect {
		seedRestCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.SeedClientConnection.ClientConnectionConfiguration, nil)
		if err != nil {
			return nil, err
		}

		k8sSeedClientLeaderElection, err := kubernetesclientset.NewForConfig(seedRestCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for leader election: %w", err)
		}

		leaderElectionConfig, err = cmdutils.MakeLeaderElectionConfig(
			*cfg.LeaderElection,
			k8sSeedClientLeaderElection,
			cmdutils.CreateRecorder(k8sSeedClientLeaderElection, "gardenlet"),
		)
		if err != nil {
			return nil, err
		}
	}

	gardenClusterIdentity := &corev1.ConfigMap{}
	if err := k8sGardenClient.Client().Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ClusterIdentity), gardenClusterIdentity); err != nil {
		return nil, fmt.Errorf("unable to get Gardener`s cluster-identity ConfigMap: %w", err)
	}

	clusterIdentity, ok := gardenClusterIdentity.Data[v1beta1constants.ClusterIdentity]
	if !ok {
		return nil, errors.New("unable to extract Gardener`s cluster identity from cluster-identity ConfigMap")
	}

	// create the certificate manager to schedule certificate rotations
	var certificateManager *certificate.Manager
	if cfg.GardenClientConnection.KubeconfigSecret != nil {
		certificateManager = certificate.NewCertificateManager(log, clientMap, seedClientForBootstrap.Client(), cfg)
	}

	return &Gardenlet{
		Identity:                             identity,
		GardenClusterIdentity:                clusterIdentity,
		Config:                               cfg,
		Log:                                  log,
		Recorder:                             recorder,
		ClientMap:                            clientMap,
		LeaderElection:                       leaderElectionConfig,
		CertificateManager:                   certificateManager,
		ClientCertificateExpirationTimestamp: clientCertificateExpirationTimestamp,
	}, nil
}

// Run runs the Gardenlet. This should never exit.
func (g *Gardenlet) Run(ctx context.Context) error {
	controllerCtx, controllerCancel := context.WithCancel(ctx)
	defer controllerCancel()

	// Initialize /healthz manager.
	healthGracePeriod := time.Duration((*g.Config.Controllers.Seed.LeaseResyncSeconds)*(*g.Config.Controllers.Seed.LeaseResyncMissThreshold)) * time.Second
	g.HealthManager = healthz.NewPeriodicHealthz(clock.RealClock{}, healthGracePeriod)

	if g.CertificateManager != nil {
		g.CertificateManager.ScheduleCertificateRotation(controllerCtx, controllerCancel, g.Recorder)
	}

	// Start HTTPS server.
	if g.Config.Server.HTTPS.TLS == nil {
		g.Log.Info("No TLS server certificates provided, self-generating them now")

		_, _, tempDir, err := secrets.SelfGenerateTLSServerCertificate("gardenlet", []string{
			"gardenlet",
			fmt.Sprintf("gardenlet.%s", v1beta1constants.GardenNamespace),
			fmt.Sprintf("gardenlet.%s.svc", v1beta1constants.GardenNamespace),
		}, nil)
		if err != nil {
			return err
		}

		g.Config.Server.HTTPS.TLS = &config.TLSServer{
			ServerCertPath: filepath.Join(tempDir, secrets.DataKeyCertificate),
			ServerKeyPath:  filepath.Join(tempDir, secrets.DataKeyPrivateKey),
		}

		g.Log.Info("TLS server certificates successfully self-generated")
	}

	g.startServer(ctx)

	// Prepare a reusable run function.
	run := func(ctx context.Context) error {
		g.HealthManager.Start()
		return g.startControllers(ctx)
	}

	leaderElectionCtx, leaderElectionCancel := context.WithCancel(context.Background())

	// If leader election is enabled, run via LeaderElector until done and exit.
	if g.LeaderElection != nil {
		g.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				g.Log.Info("Acquired leadership, starting controllers")
				if err := run(controllerCtx); err != nil {
					g.Log.Error(err, "Failed to run controllers")
				}
				leaderElectionCancel()
			},
			OnStoppedLeading: func() {
				g.Log.Info("Lost leadership, terminating")
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

func (g *Gardenlet) startServer(ctx context.Context) {
	builder := server.
		NewBuilder().
		WithBindAddress(g.Config.Server.HTTPS.BindAddress).
		WithPort(g.Config.Server.HTTPS.Port).
		WithTLS(g.Config.Server.HTTPS.TLS.ServerCertPath, g.Config.Server.HTTPS.TLS.ServerKeyPath).
		WithHandler("/metrics", promhttp.Handler()).
		WithHandlerFunc("/healthz", healthz.HandlerFunc(g.HealthManager))

	if g.Config.Debugging != nil && g.Config.Debugging.EnableProfiling {
		routes.Profiling{}.AddToBuilder(builder)
		if g.Config.Debugging.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
	}

	go builder.Build(g.Log).Start(ctx)
}

func (g *Gardenlet) startControllers(ctx context.Context) error {
	return controller.NewGardenletControllerFactory(
		g.Log,
		g.ClientMap,
		g.Config,
		g.Identity,
		g.GardenClusterIdentity,
		g.Recorder,
		g.HealthManager,
		g.ClientCertificateExpirationTimestamp,
	).Run(ctx)
}
