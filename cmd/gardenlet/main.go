// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	runtimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/gardener/gardener/cmd/gardenlet/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/utils/flow"
)

func main() {
	utils.DeduplicateWarnings()
	features.RegisterFeatureGates()

	flow.RegisterMetrics(runtimemetrics.Registry)

	if err := app.NewCommand().ExecuteContext(signals.SetupSignalHandler()); err != nil {
		panic(err)
	}
}
