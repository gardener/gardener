// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/metricsserver"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultMetricsServer returns a deployer for the metrics-server.
func (b *Botanist) DefaultMetricsServer() (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameMetricsServer, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var kubeAPIServerHost *string
	if b.ShootUsesDNS() {
		kubeAPIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	values := metricsserver.Values{
		Image:             image.String(),
		VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
		KubeAPIServerHost: kubeAPIServerHost,
	}

	return metricsserver.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		values,
	), nil
}
