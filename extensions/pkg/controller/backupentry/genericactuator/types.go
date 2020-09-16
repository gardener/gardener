// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// EtcdBackupSecretName is the name of secret having credentials for etcd backups.
	EtcdBackupSecretName string = "etcd-backup"

	backupBucketName string = "bucketName"
)

// BackupEntryDelegate preforms provider specific operation with BackupBucket resources.
type BackupEntryDelegate interface {
	// Delete deletes the BackupBucket.
	Delete(context.Context, *extensionsv1alpha1.BackupEntry) error
	// GetETCDSecretData returns the updated secret data as per provider requirement.
	GetETCDSecretData(context.Context, *extensionsv1alpha1.BackupEntry, map[string][]byte) (map[string][]byte, error)
}
