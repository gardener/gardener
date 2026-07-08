// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/victorialogs"
)

// NewVictoriaLogs returns new VictoriaLogs deployer.
func NewVictoriaLogs(
	c client.Client,
	namespace string,
	clusterType component.ClusterType,
	replicas int32,
	priorityClassName string,
	storage *resource.Quantity,
	isGardenCluster bool,
	pvcAutoscaling victorialogs.PVCAutoscalingConfig,
) (
	component.DeployWaiter,
	error,
) {
	victoriaLogsImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVictoriaLogs)
	if err != nil {
		return nil, err
	}
	if victoriaLogsImage.Repository == nil || victoriaLogsImage.Tag == nil {
		return nil, fmt.Errorf("image %q from imagevector has no repository or tag set", imagevector.ContainerImageNameVictoriaLogs)
	}

	deployer := victorialogs.New(c, namespace, victorialogs.Values{
		ImageRepository:   *victoriaLogsImage.Repository,
		ImageTag:          *victoriaLogsImage.Tag,
		Storage:           storage,
		IsGardenCluster:   isGardenCluster,
		ClusterType:       clusterType,
		Replicas:          replicas,
		PriorityClassName: priorityClassName,
		PVCAutoscaling:    pvcAutoscaling,
	})

	return deployer, nil
}
