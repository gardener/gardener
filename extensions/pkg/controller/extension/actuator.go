// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Extension resources.
type Actuator interface {
	// Reconcile the Extension resource.
	Reconcile(ctx context.Context, ex *extensionsv1alpha1.Extension) error
	// Delete the Extension resource.
	Delete(ctx context.Context, ex *extensionsv1alpha1.Extension) error
	// Restore the Extension resource.
	Restore(ctx context.Context, ex *extensionsv1alpha1.Extension) error
	// Migrate the Extension resource.
	Migrate(ctx context.Context, ex *extensionsv1alpha1.Extension) error
}
