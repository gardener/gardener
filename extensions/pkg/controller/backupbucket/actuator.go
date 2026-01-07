// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon [extensionsv1alpha1.BackupBucket] resources.
type Actuator interface {
	// Reconcile reconciles the [extensionsv1alpha1.BackupBucket] resource.
	//
	// Implementations should ensure that the backup bucket is created or
	// updated to reach the desired state.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.BackupBucket) error

	// Delete is invoked when the [extensionsv1alpha1.BackupBucket]
	// resource is deleted.
	//
	// Implementations must wait until the backup bucket is gracefully
	// deleted.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.BackupBucket) error
}
