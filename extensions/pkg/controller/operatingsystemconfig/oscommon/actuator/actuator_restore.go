// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Restore reconciles the update of a OperatingSystemConfig regenerating the os-specific format
func (a *Actuator) Restore(ctx context.Context, log logr.Logger, config *extensionsv1alpha1.OperatingSystemConfig) ([]byte, *string, []string, []string, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	return a.Reconcile(ctx, log, config)
}
