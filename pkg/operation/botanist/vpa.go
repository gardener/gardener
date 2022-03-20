// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVerticalPodAutoscaler returns a deployer for the Kubernetes Vertical Pod Autoscaler.
func (b *Botanist) DefaultVerticalPodAutoscaler() (component.DeployWaiter, error) {
	imageAdmissionController, err := b.ImageVector.FindImage(images.ImageNameVpaAdmissionController, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageExporter, err := b.ImageVector.FindImage(images.ImageNameVpaExporter, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageRecommender, err := b.ImageVector.FindImage(images.ImageNameVpaRecommender, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageUpdater, err := b.ImageVector.FindImage(images.ImageNameVpaUpdater, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return vpa.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		vpa.Values{
			ClusterType: vpa.ClusterTypeShoot,
			AdmissionController: vpa.ValuesAdmissionController{
				Image: imageAdmissionController.String(),
			},
			Exporter: vpa.ValuesExporter{
				Image: imageExporter.String(),
			},
			Recommender: vpa.ValuesRecommender{
				Image: imageRecommender.String(),
			},
			Updater: vpa.ValuesUpdater{
				Image: imageUpdater.String(),
			},
		},
	), nil
}

// DeployVerticalPodAutoscaler deploys or destroys the VPA to the shoot namespace in the seed.
func (b *Botanist) DeployVerticalPodAutoscaler(ctx context.Context) error {
	if !b.Shoot.WantsVerticalPodAutoscaler {
		return b.Shoot.Components.ControlPlane.VerticalPodAutoscaler.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.VerticalPodAutoscaler.Deploy(ctx)
}
