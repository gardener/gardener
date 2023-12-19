// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Delete ignores the deletion of OperatingSystemConfig.
func (a *Actuator) Delete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.OperatingSystemConfig) error {
	return nil
}

// ForceDelete forcefully deletes the OperatingSystemConfig.
func (a *Actuator) ForceDelete(ctx context.Context, log logr.Logger, config *extensionsv1alpha1.OperatingSystemConfig) error {
	return a.Delete(ctx, log, config)
}
