// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultResourceManager returns an instance of Gardener Resource Manager with defaults configured for being deployed in a Shoot namespace
func (b *Botanist) DefaultResourceManager() (resourcemanager.Interface, error) {
	version, err := semver.NewVersion(b.SeedClientSet.Version())
	if err != nil {
		return nil, err
	}

	var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
	if b.Config != nil && b.Config.NodeToleration != nil {
		nodeToleration := b.Config.NodeToleration
		defaultNotReadyTolerationSeconds = nodeToleration.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = nodeToleration.DefaultUnreachableTolerationSeconds
	}

	return shared.NewTargetGardenerResourceManager(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.ImageVector,
		b.SecretsManager,
		b.Seed.GetInfo().Status.ClusterIdentity,
		defaultNotReadyTolerationSeconds,
		defaultUnreachableTolerationSeconds,
		version,
		logger.InfoLevel, logger.FormatJSON,
		"",
		true,
		v1beta1constants.PriorityClassNameShootControlPlane400,
		v1beta1helper.ShootSchedulingProfile(b.Shoot.GetInfo()),
		v1beta1constants.SecretNameCACluster,
		gardenerutils.ExtractSystemComponentsTolerations(b.Shoot.GetInfo().Spec.Provider.Workers),
		b.Shoot.TopologyAwareRoutingEnabled,
		b.Shoot.IsWorkerless,
	)
}

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	return shared.DeployGardenerResourceManager(
		ctx,
		b.SeedClientSet.Client(),
		b.SecretsManager,
		b.Shoot.Components.ControlPlane.ResourceManager,
		b.Shoot.SeedNamespace,
		func(ctx context.Context) (int32, error) {
			return b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameGardenerResourceManager, 2, false)
		},
		func() string { return b.Shoot.ComputeInClusterAPIServerAddress(true) })
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), kubernetesutils.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager), 1)
}
