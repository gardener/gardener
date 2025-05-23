// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
)

// NewFluentOperator instantiates a new `Fluent Operator` component.
func NewFluentOperator(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	operatorImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameFluentOperator)
	if err != nil {
		return nil, err
	}

	deployer = fluentoperator.NewFluentOperator(
		c,
		gardenNamespaceName,
		fluentoperator.Values{
			Image:             operatorImage.String(),
			PriorityClassName: priorityClassName,
		},
	)

	if !enabled {
		deployer = component.OpDestroyWithoutWait(deployer)
	}

	return deployer, nil
}
