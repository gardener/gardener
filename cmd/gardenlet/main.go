// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardenlet/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func main() {
	utils.DeduplicateWarnings()
	features.RegisterFeatureGates()

	if err := app.NewCommand().ExecuteContext(signals.SetupSignalHandler()); err != nil {
		panic(err)
	}
}
