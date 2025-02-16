// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/nodemanagement/nodeproblemdetector"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNodeProblemDetector returns a deployer for the NodeProblemDetector.
func (b *Botanist) DefaultNodeProblemDetector() (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameNodeProblemDetector, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nodeproblemdetector.Values{
		Image:             image.String(),
		VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
		KubernetesVersion: b.Shoot.KubernetesVersion,
	}

	if b.ShootUsesDNS() {
		values.APIServerHost = ptr.To(b.outOfClusterAPIServerFQDN())
	}

	return nodeproblemdetector.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		values,
	), nil
}
