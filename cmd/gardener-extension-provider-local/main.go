// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardener-extension-provider-local/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/logger"
)

func main() {
	utils.DeduplicateWarnings()

	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON))

	if err := app.NewControllerManagerCommand(signals.SetupSignalHandler()).Execute(); err != nil {
		logf.Log.Error(err, "Error executing the main controller command")
		os.Exit(1)
	}
}
