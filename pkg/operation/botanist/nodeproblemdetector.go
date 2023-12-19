// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/nodeproblemdetector"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNodeProblemDetector returns a deployer for the NodeProblemDetector.
func (b *Botanist) DefaultNodeProblemDetector() (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameNodeProblemDetector, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nodeproblemdetector.Values{
		Image:             image.String(),
		VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
		PSPDisabled:       b.Shoot.PSPDisabled,
		KubernetesVersion: b.Shoot.KubernetesVersion,
	}

	if b.ShootUsesDNS() {
		values.APIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	return nodeproblemdetector.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		values,
	), nil
}
