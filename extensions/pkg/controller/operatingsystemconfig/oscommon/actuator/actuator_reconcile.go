// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Reconcile reconciles the update of a OperatingSystemConfig regenerating the os-specific format
func (a *Actuator) Reconcile(ctx context.Context, log logr.Logger, config *extensionsv1alpha1.OperatingSystemConfig) ([]byte, *string, []string, []string, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	cloudConfig, cmd, err := CloudConfigFromOperatingSystemConfig(ctx, log, a.client, config, a.generator)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("could not generate cloud config: %w", err)
	}

	return cloudConfig, cmd, OperatingSystemConfigUnitNames(config), OperatingSystemConfigFilePaths(config), nil, nil, nil
}
