// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// BackupEntryDelegate preforms provider specific operation with BackupBucket resources.
type BackupEntryDelegate interface {
	// Delete deletes the BackupBucket.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry) error
	// GetETCDSecretData returns the updated secret data as per provider requirement.
	GetETCDSecretData(context.Context, logr.Logger, *extensionsv1alpha1.BackupEntry, map[string][]byte) (map[string][]byte, error)
}
