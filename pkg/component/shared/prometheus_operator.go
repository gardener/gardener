// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
)

// NewPrometheusOperator instantiates a new prometheus-operator component.
func NewPrometheusOperator(
	c client.Client,
	gardenNamespaceName string,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	operatorImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePrometheusOperator)
	if err != nil {
		return nil, err
	}

	reloaderImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameConfigmapReloader)
	if err != nil {
		return nil, err
	}

	return prometheusoperator.New(
		c,
		gardenNamespaceName,
		prometheusoperator.Values{
			Image:               operatorImage.String(),
			ImageConfigReloader: reloaderImage.String(),
			PriorityClassName:   priorityClassName,
		},
	), nil
}
