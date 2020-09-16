// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Migrate ignores the deletion of OperatingSystemConfig
func (a *Actuator) Migrate(ctx context.Context, config *extensionsv1alpha1.OperatingSystemConfig) error {
	return nil
}
