// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	"fmt"
	"os"
)

var (
	gardenerPreviousVersion    = os.Getenv("GARDENER_PREVIOUS_VERSION")
	gardenerPreviousGitVersion = os.Getenv("GARDENER_PREVIOUS_RELEASE")
	gardenerCurrentVersion     = os.Getenv("GARDENER_NEXT_VERSION")
	gardenerCurrentGitVersion  = os.Getenv("GARDENER_NEXT_RELEASE")

	gardenerInfoPreUpgrade  = fmt.Sprintf(" (Gardener version: %s, Git version: %s)", gardenerPreviousVersion, gardenerPreviousGitVersion)
	gardenerInfoPostUpgrade = fmt.Sprintf(" (Gardener version: %s, Git version: %s)", gardenerCurrentVersion, gardenerCurrentGitVersion)
)
