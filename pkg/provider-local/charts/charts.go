// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package charts

import (
	"embed"
	"path/filepath"
)

var (
	// ChartShootStorageClasses is the Helm chart for the shoot-storageclasses chart.
	//go:embed shoot-storageclasses
	ChartShootStorageClasses embed.FS
	// ChartPathShootStorageClasses is the path to the shoot-storageclasses chart.
	ChartPathShootStorageClasses = filepath.Join("shoot-storageclasses")

	// ChartShootSystemComponents is the Helm chart for the shoot-system-components chart.
	//go:embed shoot-system-components
	ChartShootSystemComponents embed.FS
	// ChartPathShootSystemComponents is the path to the shoot-system-components chart.
	ChartPathShootSystemComponents = filepath.Join("shoot-system-components")
)
