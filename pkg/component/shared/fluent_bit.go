// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentbit"
)

// NewFluentBit instantiates a new `Fluent-bit` component.
func NewFluentBit(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	valiEnabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	fluentBitImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameFluentBit)
	if err != nil {
		return nil, err
	}

	fluentBitInitImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameFluentBitPluginInstaller)
	if err != nil {
		return nil, err
	}

	deployer = fluentbit.New(
		c,
		gardenNamespaceName,
		fluentbit.Values{
			Image:              fluentBitImage.String(),
			InitContainerImage: fluentBitInitImage.String(),
			ValiEnabled:        valiEnabled,
			PriorityClassName:  priorityClassName,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
