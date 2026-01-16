// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/victorialogs"
)

// NewVictoriaLogs returns new VictoriaLogs deployer
func NewVictoriaLogs(
	c client.Client,
	namespace string,
	clusterType component.ClusterType,
	replicas int32,
	priorityClassName string,
	storage *resource.Quantity,
	isGardenCluster bool,
) (
	component.DeployWaiter,
	error,
) {
	victoriaLogsImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVictoriaLogs)
	if err != nil {
		return nil, err
	}

	deployer := victorialogs.New(c, namespace, victorialogs.Values{
		Image:             victoriaLogsImage.String(),
		Storage:           storage,
		IsGardenCluster:   isGardenCluster,
		ClusterType:       clusterType,
		Replicas:          replicas,
		PriorityClassName: priorityClassName,
	})

	return deployer, nil
}
