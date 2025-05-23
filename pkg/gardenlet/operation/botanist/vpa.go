// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVerticalPodAutoscaler returns a deployer for the Kubernetes Vertical Pod Autoscaler.
func (b *Botanist) DefaultVerticalPodAutoscaler() (vpa.Interface, error) {
	imageAdmissionController, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpaAdmissionController, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageRecommender, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpaRecommender, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageUpdater, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpaUpdater, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var (
		valuesAdmissionController = vpa.ValuesAdmissionController{
			Image:                       imageAdmissionController.String(),
			PriorityClassName:           v1beta1constants.PriorityClassNameShootControlPlane200,
			Replicas:                    ptr.To(b.Shoot.GetReplicas(1)),
			TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
		}
		valuesRecommender = vpa.ValuesRecommender{
			Image:             imageRecommender.String(),
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane200,
			Replicas:          ptr.To(b.Shoot.GetReplicas(1)),
		}
		valuesUpdater = vpa.ValuesUpdater{
			Image:             imageUpdater.String(),
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane200,
			Replicas:          ptr.To(b.Shoot.GetReplicas(1)),
		}
	)

	if vpaConfig := b.Shoot.GetInfo().Spec.Kubernetes.VerticalPodAutoscaler; vpaConfig != nil {
		valuesRecommender.Interval = vpaConfig.RecommenderInterval
		valuesRecommender.RecommendationMarginFraction = vpaConfig.RecommendationMarginFraction
		valuesRecommender.TargetCPUPercentile = vpaConfig.TargetCPUPercentile
		valuesRecommender.RecommendationLowerBoundCPUPercentile = vpaConfig.RecommendationLowerBoundCPUPercentile
		valuesRecommender.RecommendationUpperBoundCPUPercentile = vpaConfig.RecommendationUpperBoundCPUPercentile
		valuesRecommender.CPUHistogramDecayHalfLife = vpaConfig.CPUHistogramDecayHalfLife
		valuesRecommender.TargetMemoryPercentile = vpaConfig.TargetMemoryPercentile
		valuesRecommender.RecommendationLowerBoundMemoryPercentile = vpaConfig.RecommendationLowerBoundMemoryPercentile
		valuesRecommender.RecommendationUpperBoundMemoryPercentile = vpaConfig.RecommendationUpperBoundMemoryPercentile
		valuesRecommender.MemoryHistogramDecayHalfLife = vpaConfig.MemoryHistogramDecayHalfLife
		valuesRecommender.MemoryAggregationInterval = vpaConfig.MemoryAggregationInterval
		valuesRecommender.MemoryAggregationIntervalCount = vpaConfig.MemoryAggregationIntervalCount

		valuesUpdater.EvictAfterOOMThreshold = vpaConfig.EvictAfterOOMThreshold
		valuesUpdater.EvictionRateBurst = vpaConfig.EvictionRateBurst
		valuesUpdater.EvictionRateLimit = vpaConfig.EvictionRateLimit
		valuesUpdater.EvictionTolerance = vpaConfig.EvictionTolerance
		valuesUpdater.Interval = vpaConfig.UpdaterInterval
	}

	return vpa.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
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
