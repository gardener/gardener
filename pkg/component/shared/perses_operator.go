// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/persesoperator"
)

// NewPersesOperator instantiates a new perses-operator component.
func NewPersesOperator(
	c client.Client,
	gardenNamespaceName string,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePersesOperator)
	if err != nil {
		return nil, err
	}

	return persesoperator.New(
		c,
		gardenNamespaceName,
		persesoperator.Values{
			Image:             image.String(),
			PriorityClassName: priorityClassName,
		},
	), nil
}
