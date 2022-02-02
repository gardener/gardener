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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensioncrds"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/podschedulername"
)

const (
	// Name is a const for the name of this component.
	Name = "gardener-seed-admission-controller"
)

var (
	gracefulShutdownTimeout = 5 * time.Second

	log = runtimelog.Log
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

// Options has all the context and parameters needed to run gardener-seed-admission-controller.
type Options struct {
	// BindAddress is the address the HTTP server should bind to.
	BindAddress string
	// Port is the port that should be opened by the HTTP server.
	Port int
	// ServerCertDir is the path to server TLS cert and key.
	ServerCertDir string
	// AllowInvalidExtensionResources causes the seed-admission-controller to allow invalid extension resources.
	AllowInvalidExtensionResources bool
}

// AddFlags adds gardener-seed-admission-controller's flags to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.BindAddress, "bind-address", "0.0.0.0", "address to bind to")
	fs.IntVar(&o.Port, "port", 9443, "webhook server port")
	fs.StringVar(&o.ServerCertDir, "tls-cert-dir", "", "directory with server TLS certificate and key (must contain a tls.crt and tls.key file)")
	fs.BoolVar(&o.AllowInvalidExtensionResources, "allow-invalid-extension-resources", false, "Allow invalid extension resources")
}

// validate validates all the required options.
func (o *Options) validate() error {
	if len(o.BindAddress) == 0 {
		return fmt.Errorf("missing bind address")
	}

	if o.Port == 0 {
		return fmt.Errorf("missing port")
	}

	if len(o.ServerCertDir) == 0 {
		return fmt.Errorf("missing server tls cert path")
	}

	return nil
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
		MetricsBindAddress:      "0", // disable for now, as we don't scrape the component
		Host:                    o.BindAddress,
		Port:                    o.Port,
		CertDir:                 o.ServerCertDir,
		GracefulShutdownTimeout: &gracefulShutdownTimeout,
	})
	if err != nil {
		return err
	}

	log.Info("Setting up webhook server")
	server := mgr.GetWebhookServer()

	log.Info("setting up readycheck for webhook server")
	if err := mgr.AddHealthzCheck("readyz", server.StartedChecker()); err != nil {
		return err
	}

	server.Register(extensioncrds.WebhookPath, &webhook.Admission{Handler: extensioncrds.New(runtimelog.Log.WithName(extensioncrds.HandlerName))})
	server.Register(podschedulername.WebhookPath, &webhook.Admission{Handler: admission.HandlerFunc(podschedulername.DefaultShootControlPlanePodsSchedulerName)})
	server.Register(extensionresources.WebhookPath, &webhook.Admission{Handler: extensionresources.New(runtimelog.Log.WithName(extensionresources.HandlerName), o.AllowInvalidExtensionResources)})

	log.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "Error running manager")
		return err
	}

	return nil
}
