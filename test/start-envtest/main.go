// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/pkg/logger"
	gardenerenvtest "github.com/gardener/gardener/test/envtest"
)

const name = "start-envtest"

func main() {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch an envtest environment",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.validate(); err != nil {
				return err
			}

			log, err := logger.NewZapLogger(logger.DebugLevel, logger.FormatText)
			if err != nil {
				return fmt.Errorf("error instantiating zap logger: %w", err)
			}

			logf.SetLogger(log)
			klog.SetLogger(log)

			log = logf.Log.WithName(name)

			// don't output usage on further errors raised during execution
			cmd.SilenceUsage = true
			// further errors will be logged properly, don't duplicate
			cmd.SilenceErrors = true

			return run(cmd.Context(), log, opts)
		},
	}

	opts.addFlags(cmd.Flags())

	if err := cmd.ExecuteContext(signals.SetupSignalHandler()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

const (
	typeKubernetes = "kubernetes"
	typeGardener   = "gardener"
)

var supportedTypes = sets.New(typeKubernetes, typeGardener)

type options struct {
	environmentType string
	kubeconfig      string
}

func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.environmentType, "environment-type", typeKubernetes, fmt.Sprintf("Type of environment to start. Supported values: %s", strings.Join(sets.List(supportedTypes), ", ")))
	fs.StringVar(&o.kubeconfig, "kubeconfig", path.Join("..", "..", "dev", "envtest-kubeconfig.yaml"), "File to place the environment's admin kubeconfig in.")
}

func (o *options) validate() error {
	if !supportedTypes.Has(o.environmentType) {
		return fmt.Errorf("unsupported environment type %q, supported types are: %s", o.environmentType, strings.Join(sets.List(supportedTypes), ", "))
	}
	return nil
}

type testEnvironment interface {
	Start() (*rest.Config, error)
	Stop() error
}

func run(ctx context.Context, log logr.Logger, opts *options) error {
	log.Info("Starting test environment", "type", opts.environmentType)

	kubeEnvironment := &envtest.Environment{}
	var testEnv testEnvironment = kubeEnvironment

	if opts.environmentType == typeGardener {
		testEnv = &gardenerenvtest.GardenerTestEnvironment{
			Environment: kubeEnvironment,
			GardenerAPIServer: &gardenerenvtest.GardenerAPIServer{
				Args: []string{"--disable-admission-plugins="},
			},
		}
	}

	_, err := testEnv.Start()
	if err != nil {
		return fmt.Errorf("error starting test environment: %w", err)
	}

	defer func() {
		log.Info("Stopping test environment")
		if err := testEnv.Stop(); err != nil {
			log.Error(err, "Error stopping test environment")
		}
	}()

	adminUser, err := kubeEnvironment.ControlPlane.AddUser(envtest.User{
		Name:   "envtest-admin",
		Groups: []string{"system:masters"},
	}, nil)
	if err != nil {
		return fmt.Errorf("error adding admin user: %w", err)
	}

	kubeConfigBytes, err := adminUser.KubeConfig()
	if err != nil {
		return fmt.Errorf("error getting admin user kubeconfig: %w", err)
	}

	if err := os.WriteFile(opts.kubeconfig, kubeConfigBytes, 0600); err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}
	log.Info("Successfully written kubeconfig", "file", opts.kubeconfig)

	defer func() {
		log.Info("Cleaning up kubeconfig file", "file", opts.kubeconfig)
		if err := os.Remove(opts.kubeconfig); err != nil {
			log.Error(err, "Error cleaning up kubeconfig file", "file", opts.kubeconfig)
		}
	}()

	log.Info("Test environment ready!")

	// block until cancelled
	<-ctx.Done()
	log.Info("Stop procedure initiated")

	return nil
}
