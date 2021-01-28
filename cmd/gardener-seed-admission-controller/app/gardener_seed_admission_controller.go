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
	"net/http"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/seedadmission"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var logger = gardenerlogger.NewLogger("info")

// Options has all the context and parameters needed to run a Gardener seed admission controller.
type Options struct {
	// BindAddress is the address the HTTP server should bind to.
	BindAddress string
	// Port is the port that should be opened by the HTTP server.
	Port int
	// ServerCertPath is the path to a server certificate.
	ServerCertPath string
	// ServerKeyPath is the path to a TLS private key.
	ServerKeyPath string
	// Kubeconfig is path to a kubeconfig file. If not given it uses the in-cluster config.
	Kubeconfig string
}

// AddFlags adds flags for a specific Scheduler to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.BindAddress, "bind-address", "0.0.0.0", "address to bind to")
	fs.IntVar(&o.Port, "port", 9443, "server port")
	fs.StringVar(&o.ServerCertPath, "tls-cert-path", "", "path to server certificate")
	fs.StringVar(&o.ServerKeyPath, "tls-private-key-path", "", "path to client certificate")
	fs.StringVar(&o.Kubeconfig, "kubeconfig", "", "path to a kubeconfig")
}

// Validate validates all the required options.
func (o *Options) validate(args []string) error {
	if len(o.BindAddress) == 0 {
		return fmt.Errorf("missing bind address")
	}

	if o.Port == 0 {
		return fmt.Errorf("missing port")
	}

	if len(o.ServerCertPath) == 0 {
		return fmt.Errorf("missing server tls cert path")
	}

	if len(o.ServerKeyPath) == 0 {
		return fmt.Errorf("missing server tls key path")
	}

	if len(args) != 0 {
		return errors.New("arguments are not supported")
	}

	return nil
}

func (o *Options) run(ctx context.Context) {
	run(ctx, o.BindAddress, o.Port, o.ServerCertPath, o.ServerKeyPath, o.Kubeconfig)
}

// NewCommandStartGardenerSeedAdmissionController creates a *cobra.Command object with default parameters
func NewCommandStartGardenerSeedAdmissionController() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "gardener-seed-admission-controller",
		Short: "Launch the Gardener seed admission controller",
		Long:  `The Gardener seed admission controller serves a validation webhook endpoint for resources in the seed clusters.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()

			utilruntime.Must(opts.validate(args))

			logger.Infof("Starting Gardener seed admission controller...")
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				logger.Infof("FLAG: --%s=%s", flag.Name, flag.Value)
			})

			opts.run(cmd.Context())
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	return cmd
}

// run runs the Gardener seed admission controller. This should never exit.
func run(ctx context.Context, bindAddress string, port int, certPath, keyPath, kubeconfigPath string) {
	var (
		log = logzap.New(logzap.UseDevMode(false))
		mux = http.NewServeMux()
	)

	k8sClient, err := kubernetes.NewClientFromFile("", kubeconfigPath, kubernetes.WithClientOptions(client.Options{
		Scheme: kubernetes.SeedScheme,
	}))
	if err != nil {
		logger.Errorf("unable to create kubernetes client: %+v", err)
		panic(err)
	}

	// prepare an injection func that will inject needed dependencies into webhook and handler.
	var setFields inject.Func
	setFields = func(i interface{}) error {
		if _, err := inject.InjectorInto(setFields, i); err != nil {
			return err
		}
		if _, err := inject.ClientInto(k8sClient.DirectClient(), i); err != nil {
			return err
		}
		if _, err := inject.LoggerInto(log, i); err != nil {
			return err
		}
		// inject scheme into webhook, needed to construct and a decoder for decoding the included objects
		// decoder will be inject by webhook into handler
		if _, err := inject.SchemeInto(kubernetes.SeedScheme, i); err != nil {
			return err
		}
		return nil
	}

	extensionDeletionProtection := &webhook.Admission{Handler: seedadmission.NewExtensionDeletionProtection(logger)}
	defaultSchedulerName := &webhook.Admission{Handler: admission.HandlerFunc(seedadmission.DefaultShootControlPlanePodsSchedulerName)}
	if err := setFields(extensionDeletionProtection); err != nil {
		panic(fmt.Errorf("error injecting dependencies into webhook handler: %w", err))
	}
	if err := setFields(defaultSchedulerName); err != nil {
		panic(fmt.Errorf("error injecting dependencies into webhook handler: %w", err))
	}

	mux.Handle("/webhooks/validate-extension-crd-deletion", extensionDeletionProtection)
	mux.Handle(
		// in the future we might want to have additional scheduler names
		// so lets have the handler be of pattern "/webhooks/default-pod-scheduler-name/{scheduler-name}"
		fmt.Sprintf(seedadmission.GardenerShootControlPlaneSchedulerWebhookPath),
		defaultSchedulerName,
	)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", bindAddress, port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServeTLS(certPath, keyPath); err != http.ErrServerClosed {
			logger.Errorf("Could not start HTTPS server: %v", err)
			panic(err)
		}
	}()

	<-ctx.Done()
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(timeoutCtx); err != nil {
		logger.Errorf("Error when shutting down HTTPS server: %v", err)
	}
	logger.Info("HTTPS servers stopped.")
}
