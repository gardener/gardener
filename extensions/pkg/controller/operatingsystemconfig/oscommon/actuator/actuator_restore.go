// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Restore reconciles the update of a OperatingSystemConfig regenerating the os-specific format
func (a *Actuator) Restore(ctx context.Context, config *extensionsv1alpha1.OperatingSystemConfig) ([]byte, *string, []string, error) {
	return a.Reconcile(ctx, config)
}
