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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultKubeStateMetrics returns a deployer for the kube-state-metrics.
func (b *Botanist) DefaultKubeStateMetrics() (kubestatemetrics.Interface, error) {
	image, err := b.ImageVector.FindImage(images.ImageNameKubeStateMetrics, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return kubestatemetrics.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		kubestatemetrics.Values{
			ClusterType:       component.ClusterTypeShoot,
			Image:             image.String(),
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane100,
			Replicas:          b.Shoot.GetReplicas(1),
			IsWorkerless:      b.Shoot.IsWorkerless,
		},
	), nil
}

// DeployKubeStateMetrics deploys or destroys the kube-state-metrics to the shoot namespace in the seed.
func (b *Botanist) DeployKubeStateMetrics(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.ControlPlane.KubeStateMetrics.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.KubeStateMetrics.Deploy(ctx)
}
