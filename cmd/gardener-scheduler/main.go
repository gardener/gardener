// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/gardener/gardener/cmd/gardener-scheduler/app"
	"github.com/gardener/gardener/cmd/utils"
	"github.com/gardener/gardener/pkg/scheduler/features"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	features.RegisterFeatureGates()

	ctx := utils.ContextFromStopChannel(signals.SetupSignalHandler())
	command := app.NewCommandStartGardenerScheduler(ctx)

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
