// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	victoriaoperator "github.com/gardener/gardener/pkg/component/observability/logging/victoria/operator"
)

// NewVictoriaOperator instantiates a new victoria-operator component.
func NewVictoriaOperator(
	c client.Client,
	gardenNamespaceName string,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVictoriaOperator)
	if err != nil {
		return nil, err
	}

	return victoriaoperator.New(
		c,
		gardenNamespaceName,
		victoriaoperator.Values{
			Image:             image.String(),
			PriorityClassName: priorityClassName,
		},
	), nil
}
