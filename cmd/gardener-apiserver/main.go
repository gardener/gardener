// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardener-apiserver/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/apiserver/features"
)

func main() {
	utils.DeduplicateWarnings()
	features.RegisterFeatureGates()

	ctx := signals.SetupSignalHandler()
	if err := app.NewCommand().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
