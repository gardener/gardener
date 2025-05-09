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

// GenerateBackupEntryName returns BackupEntry resource name created from provided <controlPlaneNamespace> and
// <shootUID>.
func GenerateBackupEntryName(controlPlaneNamespace string, shootStatusUID, shootUID types.UID) (string, error) {
	if controlPlaneNamespace == "" {
		return "", errors.New("can't generate backup entry name with an empty shoot technical ID")
	}

	uid := shootStatusUID
	if uid == "" {
		uid = shootUID
	}
	if uid == "" {
		return "", errors.New("can't generate backup entry name with an empty shoot UID")
	}

	return controlPlaneNamespace + backupEntryDelimiter + string(uid), nil
}

// ExtractShootDetailsFromBackupEntryName returns the control plane namespace of the Shoot and its UID from the provided
// <backupEntryName>.
func ExtractShootDetailsFromBackupEntryName(backupEntryName string) (controlPlaneNamespace string, shootUID types.UID) {
	tokens := strings.Split(backupEntryName, backupEntryDelimiter)
	uid := tokens[len(tokens)-1]

	controlPlaneNamespace = strings.TrimPrefix(backupEntryName, v1beta1constants.BackupSourcePrefix+"-")
	controlPlaneNamespace = strings.TrimSuffix(controlPlaneNamespace, uid)
	controlPlaneNamespace = strings.TrimSuffix(controlPlaneNamespace, backupEntryDelimiter)
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
