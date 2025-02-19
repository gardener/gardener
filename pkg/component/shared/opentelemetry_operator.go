// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/opentelemetryoperator"
)

// NewFluentOperator instantiates a new `Fluent Operator` component.
func NewOpenTelemetryOperator(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	operatorImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameOpentelemetryOperator)
	if err != nil {
		return nil, err
	}

	deployer = opentelemetryoperator.NewOpenTelemetryOperator(
		c,
		gardenNamespaceName,
		opentelemetryoperator.Values{
			Image:             operatorImage.String(),
			PriorityClassName: priorityClassName,
		},
	)

	if !enabled {
		deployer = component.OpDestroyWithoutWait(deployer)
	}

	return deployer, nil
}
