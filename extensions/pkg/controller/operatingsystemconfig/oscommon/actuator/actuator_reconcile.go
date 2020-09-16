// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Reconcile reconciles the update of a OperatingSystemConfig regenerating the os-specific format
func (a *Actuator) Reconcile(ctx context.Context, config *extensionsv1alpha1.OperatingSystemConfig) ([]byte, *string, []string, error) {
	cloudConfig, cmd, err := CloudConfigFromOperatingSystemConfig(ctx, a.client, config, a.generator)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not generate cloud config: %v", err)
	}

	return cloudConfig, cmd, OperatingSystemConfigUnitNames(config), nil
}
