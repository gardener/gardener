// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gardener

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

var backupEntryDelimiter = "--"

// GenerateBackupEntryName returns BackupEntry resource name created from provided <seedNamespace> and <shootUID>.
func GenerateBackupEntryName(shootTechnicalID string, shootUID types.UID) (string, error) {
	if shootTechnicalID == "" {
		return "", fmt.Errorf("can't generate backup entry name with an empty shoot technical ID")
	}
	if shootUID == "" {
		return "", fmt.Errorf("can't generate backup entry name with an empty shoot UID")
	}
	return shootTechnicalID + backupEntryDelimiter + string(shootUID), nil
}

// ExtractShootDetailsFromBackupEntryName returns Shoot resource technicalID its UID from provided <backupEntryName>.
func ExtractShootDetailsFromBackupEntryName(backupEntryName string) (shootTechnicalID string, shootUID types.UID) {
	tokens := strings.Split(backupEntryName, backupEntryDelimiter)
	uid := tokens[len(tokens)-1]

	shootTechnicalID = strings.TrimSuffix(backupEntryName, uid)
	shootTechnicalID = strings.TrimSuffix(shootTechnicalID, backupEntryDelimiter)
	shootUID = types.UID(uid)
	return
}
