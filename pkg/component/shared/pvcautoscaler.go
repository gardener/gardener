// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/pvcautoscaler"
)

// NewPVCAutoscaler instantiates a new `PVCAutoscaler` component.
func NewPVCAutoscaler(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	priorityClassName string,
	managedResourceName string,
	prometheusServiceName string,
	serviceMonitorLabel string,
	injectScrapeTargetAnnotations func(*corev1.Service, ...networkingv1.NetworkPolicyPort) error,
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
			ManagedResourceName:           managedResourceName,
			PrometheusServiceName:         prometheusServiceName,
			ServiceMonitorLabel:           serviceMonitorLabel,
			InjectScrapeTargetAnnotations: injectScrapeTargetAnnotations,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
