// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon OperatingSystemConfig resources.
type Actuator interface {
	// Reconcile the operating system config.
	Reconcile(context.Context, *extensionsv1alpha1.OperatingSystemConfig) ([]byte, *string, []string, error)
	// Delete the operating system config.
	Delete(context.Context, *extensionsv1alpha1.OperatingSystemConfig) error
	// Restore the operating system config.
	Restore(context.Context, *extensionsv1alpha1.OperatingSystemConfig) ([]byte, *string, []string, error)
	// Migrate the operating system config.
	Migrate(context.Context, *extensionsv1alpha1.OperatingSystemConfig) error
}
