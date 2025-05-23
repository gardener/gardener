// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package charts

import (
	"embed"
	"path/filepath"
)

var (
	// ChartGardenlet is the Helm chart for the gardener/gardenlet chart.
	//go:embed gardener/gardenlet
	ChartGardenlet embed.FS
	// ChartPathGardenlet is the path to the gardener/gardenlet chart.
	ChartPathGardenlet = filepath.Join("gardener", "gardenlet")
)
