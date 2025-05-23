// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewKubeStateMetrics instantiates a new `kube-state-metrics` component.
func NewKubeStateMetrics(
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	priorityClassName string,
	nameSuffix string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubeStateMetrics, imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	return kubestatemetrics.New(c, gardenNamespaceName, nil, kubestatemetrics.Values{
		ClusterType:       component.ClusterTypeSeed,
		Image:             image.String(),
		PriorityClassName: priorityClassName,
		Replicas:          2,
		NameSuffix:        nameSuffix,
	}), nil
}
