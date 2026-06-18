// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/pvcautoscaler"
	seedprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/seed"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewPVCAutoscaler instantiates a new `PVCAutoscaler` component.
func NewPVCAutoscaler(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePvcAutoscaler)
	if err != nil {
		return nil, err
	}

	deployer = pvcautoscaler.NewPVCAutoscaler(
		c,
		gardenNamespaceName,
		pvcautoscaler.Values{
			Image:                         image.String(),
			PriorityClassName:             priorityClassName,
			ManagedResourceName:           pvcautoscaler.PVCAutoscalerManagedResourceName,
			PrometheusServiceName:         "prometheus-cache",
			ServiceMonitorLabel:           seedprometheus.Label,
			InjectScrapeTargetAnnotations: gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
