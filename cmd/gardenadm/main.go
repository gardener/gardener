// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate go run ../../hack/tools/cli-reference-generator -O ../../docs/cli-reference/gardenadm gardenadm

package main

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardenadm/app"
	"github.com/gardener/gardener/cmd/utils"
)

func main() {
	utils.DeduplicateWarnings()

	if err := app.NewCommand().ExecuteContext(signals.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
