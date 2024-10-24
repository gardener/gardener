// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/gardener/gardener/cmd/gardenadm/app"
	"github.com/gardener/gardener/cmd/utils"
)

func main() {
	utils.DeduplicateWarnings()

	_ = app.NewCommand().ExecuteContext(signals.SetupSignalHandler())
}
