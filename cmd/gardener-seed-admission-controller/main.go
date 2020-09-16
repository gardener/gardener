// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/gardener/gardener/cmd/gardener-seed-admission-controller/app"
	"github.com/gardener/gardener/cmd/utils"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	ctx := utils.ContextFromStopChannel(signals.SetupSignalHandler())
	command := app.NewCommandStartGardenerSeedAdmissionController(ctx)

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
