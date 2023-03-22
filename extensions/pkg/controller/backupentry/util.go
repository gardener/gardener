// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry

import (
	"fmt"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ExtractShootDetailsFromBackupEntryName returns Shoot resource technicalID its UID from provided <backupEntryName>.
func ExtractShootDetailsFromBackupEntryName(backupEntryName string) (shootTechnicalID, shootUID string) {
	backupEntryName = strings.TrimPrefix(backupEntryName, fmt.Sprintf("%s-", v1beta1constants.BackupSourcePrefix))
	tokens := strings.Split(backupEntryName, "--")
	shootUID = tokens[len(tokens)-1]
	shootTechnicalID = strings.TrimSuffix(backupEntryName, "--"+shootUID)
	return shootTechnicalID, shootUID
}
