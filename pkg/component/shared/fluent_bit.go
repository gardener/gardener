// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
)

// NewFluentBit instantiates a new `Fluent-bit` component.
func NewFluentBit(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	fluentBitImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameFluentBit)
	if err != nil {
		return nil, err
	}

	fluentBitInitImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameFluentBitPluginInstaller)
	if err != nil {
		return nil, err
	}

	deployer = fluentoperator.NewFluentBit(
		c,
		gardenNamespaceName,
		fluentoperator.FluentBitValues{
			Image:              fluentBitImage.String(),
			InitContainerImage: fluentBitInitImage.String(),
			PriorityClass:      priorityClassName,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
