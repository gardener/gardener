// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"k8s.io/utils/pointer"
)

// DefaultNodeProblemDetector returns a deployer for the NodeProblemDetector.
func (b *Botanist) DefaultNodeProblemDetector() (component.DeployWaiter, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameNodeProblemDetector, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nodeproblemdetector.Values{
		Image:      image.String(),
		VPAEnabled: b.Shoot.WantsVerticalPodAutoscaler,
	}

	if b.APIServerSNIEnabled() {
		values.APIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	return nodeproblemdetector.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		values,
	), nil
}
