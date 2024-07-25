// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/hvpa"
)

// NewHVPA instantiates a new `hvpa-controller` component.
func NewHVPA(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	kubernetesVersion *semver.Version,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ImageNameHvpaController)
	if err != nil {
		return nil, err
	}

	deployer = hvpa.New(c, gardenNamespaceName, hvpa.Values{
		Image:             image.String(),
		PriorityClassName: priorityClassName,
		KubernetesVersion: kubernetesVersion,
	})

	if !enabled {
		deployer = component.OpDestroyWithoutWait(deployer)
	}

	return deployer, nil
}
