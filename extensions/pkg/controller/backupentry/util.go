// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"strings"
)

// ExtractShootDetailsFromBackupEntryName returns Shoot resource technicalID its UID from provided <backupEntryName>.
func ExtractShootDetailsFromBackupEntryName(backupEntryName string) (shootTechnicalID, shootUID string) {
	tokens := strings.Split(backupEntryName, "--")
	shootUID = tokens[len(tokens)-1]
	shootTechnicalID = strings.TrimSuffix(backupEntryName, "--"+shootUID)
	return shootTechnicalID, shootUID
}
