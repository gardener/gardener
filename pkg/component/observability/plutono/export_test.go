// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plutono

import "io/fs"

// GardenAndShootDashboards exposes the embedded garden-shoot dashboards for testing.
func GardenAndShootDashboards() fs.FS {
	return gardenAndShootDashboards
}
