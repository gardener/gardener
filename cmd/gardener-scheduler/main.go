// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardener-scheduler/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/scheduler/features"
)

func main() {
	utils.DeduplicateWarnings()
	features.RegisterFeatureGates()

	if err := app.NewCommand().ExecuteContext(signals.SetupSignalHandler()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
