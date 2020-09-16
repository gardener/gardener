// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"

	"github.com/gardener/gardener/cmd/gardener-apiserver/app"
	"github.com/gardener/gardener/pkg/apiserver/features"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
)

func main() {
	features.RegisterFeatureGates()

	logs.InitLogs()
	defer logs.FlushLogs()

	stopCh := genericapiserver.SetupSignalHandler()
	command := app.NewCommandStartGardenerAPIServer(os.Stdout, os.Stderr, stopCh)
	command.Flags().AddGoFlagSet(flag.CommandLine)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
