// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/vpa"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVerticalPodAutoscaler returns a deployer for the Kubernetes Vertical Pod Autoscaler.
func (b *Botanist) DefaultVerticalPodAutoscaler() (vpa.Interface, error) {
	imageAdmissionController, err := b.ImageVector.FindImage(imagevector.ImageNameVpaAdmissionController, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageRecommender, err := b.ImageVector.FindImage(imagevector.ImageNameVpaRecommender, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageUpdater, err := b.ImageVector.FindImage(imagevector.ImageNameVpaUpdater, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var (
		valuesAdmissionController = vpa.ValuesAdmissionController{
			Image:                       imageAdmissionController.String(),
			PriorityClassName:           v1beta1constants.PriorityClassNameShootControlPlane200,
			Replicas:                    pointer.Int32(b.Shoot.GetReplicas(1)),
			TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
		}
		valuesRecommender = vpa.ValuesRecommender{
			Image:             imageRecommender.String(),
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane200,
			Replicas:          pointer.Int32(b.Shoot.GetReplicas(1)),
		}
		valuesUpdater = vpa.ValuesUpdater{
			Image:             imageUpdater.String(),
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane200,
			Replicas:          pointer.Int32(b.Shoot.GetReplicas(1)),
		}
	)

	if vpaConfig := b.Shoot.GetInfo().Spec.Kubernetes.VerticalPodAutoscaler; vpaConfig != nil {
		valuesRecommender.Interval = vpaConfig.RecommenderInterval
		valuesRecommender.RecommendationMarginFraction = vpaConfig.RecommendationMarginFraction

		valuesUpdater.EvictAfterOOMThreshold = vpaConfig.EvictAfterOOMThreshold
		valuesUpdater.EvictionRateBurst = vpaConfig.EvictionRateBurst
		valuesUpdater.EvictionRateLimit = vpaConfig.EvictionRateLimit
		valuesUpdater.EvictionTolerance = vpaConfig.EvictionTolerance
		valuesUpdater.Interval = vpaConfig.UpdaterInterval
	}

	return vpa.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		vpa.Values{
			ClusterType:              component.ClusterTypeShoot,
			Enabled:                  true,
			SecretNameServerCA:       v1beta1constants.SecretNameCACluster,
			RuntimeKubernetesVersion: b.Seed.KubernetesVersion,
			AdmissionController:      valuesAdmissionController,
			Recommender:              valuesRecommender,
			Updater:                  valuesUpdater,
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
