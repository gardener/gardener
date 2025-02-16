// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon OperatingSystemConfig resources.
type Actuator interface {
	// Reconcile the operating system config.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) ([]byte, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, *extensionsv1alpha1.InPlaceUpdatesStatus, error)
	// Delete the operating system config.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) error
	// ForceDelete forcefully deletes the operating system config.
	ForceDelete(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) error
	// Restore the operating system config.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) ([]byte, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, *extensionsv1alpha1.InPlaceUpdatesStatus, error)
	// Migrate the operating system config.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.OperatingSystemConfig) error
}
