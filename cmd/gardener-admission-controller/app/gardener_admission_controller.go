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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	configvalidation "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/validation"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/auditpolicy"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/internaldomainsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/kubeconfigsecret"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/namespacedeletion"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/resourcesize"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission/seedrestriction"
	seedauthorizer "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed"
	seedauthorizergraph "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/auth/seed/graph"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/server/routes"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// Name is a const for the name of this component.
	Name = "gardener-admission-controller"
)

var (
	configDecoder runtime.Decoder

	gracefulShutdownTimeout = 5 * time.Second
)

func init() {
	configScheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(
		config.AddToScheme,
		configv1alpha1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(configScheme))
	configDecoder = serializer.NewCodecFactory(configScheme).UniversalDecoder()
}

// options has all the context and parameters needed to run a Gardener admission controller.
type options struct {
	// configFile is the location of the Gardener controller manager's configuration file.
	configFile string

	// config is the decoded admission controller config.
	config *config.AdmissionControllerConfiguration
}

func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.configFile, "config", o.configFile, "Path to configuration file.")
}

func (o *options) complete() error {
	if len(o.configFile) == 0 {
		return fmt.Errorf("missing config file")
	}

	data, err := os.ReadFile(o.configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	configObj, err := runtime.Decode(configDecoder, data)
	if err != nil {
		return fmt.Errorf("error decoding config: %w", err)
	}

	config, ok := configObj.(*config.AdmissionControllerConfiguration)
	if !ok {
		return fmt.Errorf("got unexpected config type: %T", configObj)
	}
	o.config = config

	return nil
}

func (o *options) validate() error {
	if errs := configvalidation.ValidateAdmissionControllerConfiguration(o.config); len(errs) > 0 {
		return errs.ToAggregate()
	}

	return nil
}

// run runs gardener-admission-controller using the specified options.
func (o *options) run(ctx context.Context) error {
	log := logf.Log

	log.Info("Starting Gardener admission controller...", "version", version.Get())

	log.Info("getting rest config")
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		o.config.GardenClientConnection.Kubeconfig = kubeconfig
	}

	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&o.config.GardenClientConnection, nil)
	if err != nil {
		return err
	}

	log.Info("setting up manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme:                  kubernetes.GardenScheme,
		LeaderElection:          false,
		HealthProbeBindAddress:  fmt.Sprintf("%s:%d", o.config.Server.HealthProbes.BindAddress, o.config.Server.HealthProbes.Port),
		MetricsBindAddress:      fmt.Sprintf("%s:%d", o.config.Server.Metrics.BindAddress, o.config.Server.Metrics.Port),
		Host:                    o.config.Server.HTTPS.BindAddress,
		Port:                    o.config.Server.HTTPS.Port,
		CertDir:                 o.config.Server.HTTPS.TLS.ServerCertDir,
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

	log.Info("setting up healthcheck endpoints")
	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return err
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return err
	}

	log.Info("setting up graph for seed authorization handler")
	graph := seedauthorizergraph.New(log, mgr.GetClient())
	if err := graph.Setup(ctx, mgr.GetCache()); err != nil {
		return err
	}

	log.Info("setting up webhook server")
	server := mgr.GetWebhookServer()

	namespaceValidationHandler, err := namespacedeletion.New(ctx, logf.Log.WithName(namespacedeletion.HandlerName), mgr.GetCache())
	if err != nil {
		return err
	}
	seedRestrictionHandler, err := seedrestriction.New(ctx, logf.Log.WithName(seedrestriction.HandlerName), mgr.GetCache())
	if err != nil {
		return err
	}
	logSeedAuth := logf.Log.WithName(seedauthorizer.AuthorizerName)

	server.Register(seedauthorizer.WebhookPath, seedauthorizer.NewHandler(logSeedAuth, seedauthorizer.NewAuthorizer(logSeedAuth, graph)))
	server.Register(seedrestriction.WebhookPath, &webhook.Admission{Handler: seedRestrictionHandler})
	server.Register(namespacedeletion.WebhookPath, &webhook.Admission{Handler: namespaceValidationHandler})
	server.Register(kubeconfigsecret.WebhookPath, &webhook.Admission{Handler: kubeconfigsecret.New(logf.Log.WithName(kubeconfigsecret.HandlerName))})
	server.Register(resourcesize.WebhookPath, &webhook.Admission{Handler: resourcesize.New(logf.Log.WithName(resourcesize.HandlerName), o.config.Server.ResourceAdmissionConfiguration)})
	server.Register(auditpolicy.WebhookPath, &webhook.Admission{Handler: auditpolicy.New(logf.Log.WithName(auditpolicy.HandlerName))})
	server.Register(internaldomainsecret.WebhookPath, &webhook.Admission{Handler: internaldomainsecret.New(logf.Log.WithName(internaldomainsecret.HandlerName))})

	if utils.IsTrue(o.config.Server.EnableDebugHandlers) {
		log.Info("registering debug handlers")
		server.Register(seedauthorizergraph.DebugHandlerPath, seedauthorizergraph.NewDebugHandler(graph))
	}

	log.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "error running manager")
		return err
	}

	return nil
}

// NewGardenerAdmissionControllerCommand creates a *cobra.Command object with default parameters.
func NewGardenerAdmissionControllerCommand() *cobra.Command {
	var (
		log = logzap.New(logzap.UseDevMode(false), func(opts *logzap.Options) {
			encCfg := zap.NewProductionEncoderConfig()
			// overwrite time encoding to human readable format
			encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
			opts.Encoder = zapcore.NewJSONEncoder(encCfg)
		})
		opts = &options{}
	)
	logf.SetLogger(log)

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

			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			log.Info("Starting " + Name + "...")
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value))
			})

			return opts.run(cmd.Context())
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.addFlags(flags)
	return cmd
}
