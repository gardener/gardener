// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	oteloperator "github.com/gardener/gardener/pkg/component/observability/opentelemetry/operator"
)

// NewOpenTelemetryOperator instantiates a new `OpenTelemetryOperator` component.
func NewOpenTelemetryOperator(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameOpentelemetryOperator)
	if err != nil {
		return nil, err
	}

	deployer = oteloperator.NewOpenTelemetryOperator(
		c,
		gardenNamespaceName,
		oteloperator.Values{
			Image:             image.String(),
			PriorityClassName: priorityClassName,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
