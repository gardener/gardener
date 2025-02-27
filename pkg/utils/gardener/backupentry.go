// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var backupEntryDelimiter = "--"

// GenerateBackupEntryName returns BackupEntry resource name created from provided <seedNamespace> and <shootUID>.
func GenerateBackupEntryName(shootTechnicalID string, shootStatusUID, shootUID types.UID) (string, error) {
	if shootTechnicalID == "" {
		return "", errors.New("can't generate backup entry name with an empty shoot technical ID")
	}

	uid := shootStatusUID
	if uid == "" {
		uid = shootUID
	}
	if uid == "" {
		return "", errors.New("can't generate backup entry name with an empty shoot UID")
	}

	return shootTechnicalID + backupEntryDelimiter + string(uid), nil
}

// ExtractShootDetailsFromBackupEntryName returns Shoot resource technicalID its UID from provided <backupEntryName>.
func ExtractShootDetailsFromBackupEntryName(backupEntryName string) (shootTechnicalID string, shootUID types.UID) {
	tokens := strings.Split(backupEntryName, backupEntryDelimiter)
	uid := tokens[len(tokens)-1]

	shootTechnicalID = strings.TrimPrefix(backupEntryName, v1beta1constants.BackupSourcePrefix+"-")
	shootTechnicalID = strings.TrimSuffix(shootTechnicalID, uid)
	shootTechnicalID = strings.TrimSuffix(shootTechnicalID, backupEntryDelimiter)
	shootUID = types.UID(uid)
	return
}

// GetBackupEntrySeedNames returns the spec.seedName and the status.seedName field in case the provided object is a
// BackupEntry.
func GetBackupEntrySeedNames(obj client.Object) (*string, *string) {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return nil, nil
	}
	return backupEntry.Spec.SeedName, backupEntry.Status.SeedName
}
