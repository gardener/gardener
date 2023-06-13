// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package botanist

import (
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/metricsserver"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultMetricsServer returns a deployer for the metrics-server.
func (b *Botanist) DefaultMetricsServer() (component.DeployWaiter, error) {
	image, err := b.ImageVector.FindImage(images.ImageNameMetricsServer, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
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
		KubernetesVersion: b.Shoot.KubernetesVersion,
	}

	return metricsserver.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		values,
	), nil
}
