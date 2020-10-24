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
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	configvalidation "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/validation"
	"github.com/gardener/gardener/pkg/admissioncontroller/server/handlers/webhooks"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server"
	"github.com/gardener/gardener/pkg/version/verflag"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// options has all the context and parameters needed to run a Gardener admission controller.
type options struct {
	// configFile is the location of the Gardener controller manager's configuration file.
	configFile string
	scheme     *runtime.Scheme
	codecs     serializer.CodecFactory
}

// addFlags adds flags for a specific Gardener controller manager to the specified FlagSet.
func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.configFile, "config", o.configFile, "Path to configuration file.")
}

// newOptions returns a new Options object.
func newOptions() (*options, error) {
	o := &options{
		scheme: runtime.NewScheme(),
	}

	o.codecs = serializer.NewCodecFactory(o.scheme)

	if err := config.AddToScheme(o.scheme); err != nil {
		return nil, err
	}
	if err := admissioncontrollerconfigv1alpha1.AddToScheme(o.scheme); err != nil {
		return nil, err
	}

	return o, nil
}

// loadConfigFromFile loads the contents of file and decodes it as a
// ControllerManagerConfiguration object.
func (o *options) loadConfigFromFile(file string) (*config.AdmissionControllerConfiguration, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return o.decodeConfig(data)
}

func (o *options) configFileSpecified() error {
	if len(o.configFile) == 0 {
		return fmt.Errorf("missing Gardener admission controller config file")
	}
	return nil
}

// Validate validates all the required options.
func (o *options) validate(args []string) error {
	if len(args) != 0 {
		return errors.New("arguments are not supported")
	}

	return nil
}

// decodeConfig decodes data as a ControllerManagerConfiguration object.
func (o *options) decodeConfig(data []byte) (*config.AdmissionControllerConfiguration, error) {
	configObj, gvk, err := o.codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*config.AdmissionControllerConfiguration)
	if !ok {
		return nil, fmt.Errorf("got unexpected config type: %v", gvk)
	}
	return config, nil
}

// AdmissionController contains all necessary information to run the admission controller.
type AdmissionController struct {
	Config    *config.AdmissionControllerConfiguration
	ClientMap clientmap.ClientMap
}

func (o *options) run(ctx context.Context) error {
	c, err := o.loadConfigFromFile(o.configFile)
	if err != nil {
		return err
	}

	if errs := configvalidation.ValidateAdmissionControllerConfiguration(c); len(errs) > 0 {
		return errs.ToAggregate()
	}

	admissionController, err := NewAdmissionController(c)
	if err != nil {
		return err
	}

	return admissionController.Run(ctx)
}

// NewAdmissionController creates a new AdmissionController instance.
func NewAdmissionController(cfg *config.AdmissionControllerConfiguration) (*AdmissionController, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Initialize logger
	logger := logger.NewLogger(cfg.LogLevel)
	logger.Info("Starting Gardener admission controller...")

	// Prepare a Kubernetes client object for the Garden cluster which contains all the Clientsets
	// that can be used to access the Kubernetes API.
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.GardenClientConnection.Kubeconfig = kubeconfig
	}

	restCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&cfg.GardenClientConnection, nil)
	if err != nil {
		return nil, err
	}

	// Enable cache globally for garden client.
	kubernetes.UseCachedRuntimeClients = true

	clientMap, err := clientmapbuilder.NewGardenClientMapBuilder().WithLogger(logger).WithRESTConfig(restCfg).Build()

	if err != nil {
		return nil, fmt.Errorf("failed to build ClientMap: %w", err)
	}

	return &AdmissionController{
		ClientMap: clientMap,
		Config:    cfg,
	}, nil
}

// Run starts the Gardener admission controller.
func (a *AdmissionController) Run(ctx context.Context) error {
	k8sGardenClient, err := a.ClientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return err
	}

	if err := a.ClientMap.Start(ctx.Done()); err != nil {
		return err
	}

	namespaceValidationHandler, err := webhooks.NewValidateNamespaceDeletionHandler(ctx, k8sGardenClient)
	if err != nil {
		return err
	}

	healthManager := healthz.NewDefaultHealthz()
	defer healthManager.Stop()
	healthManager.Start()
	// Set health manager always to `true` since we don't run any real health checks yet.
	// It is only required to identify a healthy and ready webhook after it successfully synced required caches.
	healthManager.Set(true)

	// Start webhook server
	server.
		NewBuilder().
		WithBindAddress(a.Config.Server.HTTPS.BindAddress).
		WithPort(a.Config.Server.HTTPS.Port).
		WithTLS(a.Config.Server.HTTPS.TLS.ServerCertPath, a.Config.Server.HTTPS.TLS.ServerKeyPath).
		WithHandler("/webhooks/validate-namespace-deletion", namespaceValidationHandler).
		WithHandler("/webhooks/validate-kubeconfig-secrets", webhooks.NewValidateKubeconfigSecretsHandler()).
		WithHandler("/webhooks/validate-resource-size", webhooks.NewValidateResourceSizeHandler(a.Config.Server.ResourceAdmissionConfiguration)).
		WithHandlerFunc("/healthz", healthz.HandlerFunc(healthManager)).
		WithHandlerFunc("/readyz", healthz.HandlerFunc(healthManager)).
		Build().
		Start(ctx)

	return nil
}

// NewCommandStartGardenerControllerManager creates a *cobra.Command object with default parameters.
func NewCommandStartGardenerAdmissionController() *cobra.Command {
	opts, err := newOptions()
	if err != nil {
		panic(err)
	}

	cmd := &cobra.Command{
		Use:   "gardener-admission-controller",
		Short: "Launch the Gardener admission controller",
		Long:  `The Gardener admission controller serves a validation webhook endpoint for resources in the garden cluster.`,
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
	opts.addFlags(flags)
	return cmd
}
