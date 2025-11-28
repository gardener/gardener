// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	_ "github.com/gardener/machine-controller-manager/pkg/util/client/metrics/prometheus" // for client metric registration
	"github.com/gardener/machine-controller-manager/pkg/util/provider/app"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/app/options"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	_ "github.com/gardener/machine-controller-manager/pkg/util/reflector/prometheus" // for reflector metric registration
	_ "github.com/gardener/machine-controller-manager/pkg/util/workqueue/prometheus" // for workqueue metric registration
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/provider-local/machine-provider/local"
)

func main() {
	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON))

	s := options.NewMCServer()
	s.AddFlags(pflag.CommandLine)

	flag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	driver, err := newDriver(s.ControlKubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if err := app.Run(s, driver); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newDriver(kubeconfig string) (driver.Driver, error) {
	if kubeconfig == "inClusterConfig" {
		kubeconfig = ""
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	s := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(s))
	utilruntime.Must(machinev1alpha1.AddToScheme(s))

	c, err := client.New(restConfig, client.Options{Scheme: s})
	if err != nil {
		return nil, err
	}

	return local.NewDriver(c), nil
}
