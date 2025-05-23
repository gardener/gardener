// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/metricsserver"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultMetricsServer returns a deployer for the metrics-server.
func (b *Botanist) DefaultMetricsServer() (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameMetricsServer, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var kubeAPIServerHost *string
	if b.ShootUsesDNS() {
		kubeAPIServerHost = ptr.To(b.outOfClusterAPIServerFQDN())
	}

	values := metricsserver.Values{
		Image:             image.String(),
		VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
		KubeAPIServerHost: kubeAPIServerHost,
	}

	return metricsserver.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		values,
	), nil
}
