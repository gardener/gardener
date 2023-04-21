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
	"time"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultResourceManager returns an instance of Gardener Resource Manager with defaults configured for being deployed in a Shoot namespace
func (b *Botanist) DefaultResourceManager() (resourcemanager.Interface, error) {
	image, err := b.ImageVector.FindImage(images.ImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

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

	cfg := resourcemanager.Values{
		AlwaysUpdate:                         pointer.Bool(true),
		ClusterIdentity:                      b.Seed.GetInfo().Status.ClusterIdentity,
		ConcurrentSyncs:                      pointer.Int(20),
		DefaultNotReadyToleration:            defaultNotReadyTolerationSeconds,
		DefaultUnreachableToleration:         defaultUnreachableTolerationSeconds,
		HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
		Image:                                image.String(),
		LogLevel:                             logger.InfoLevel,
		LogFormat:                            logger.FormatJSON,
		MaxConcurrentHealthWorkers:           pointer.Int(10),
		MaxConcurrentTokenInvalidatorWorkers: pointer.Int(5),
		MaxConcurrentTokenRequestorWorkers:   pointer.Int(5),
		MaxConcurrentCSRApproverWorkers:      pointer.Int(5),
		PodTopologySpreadConstraintsEnabled:  true,
		PriorityClassName:                    v1beta1constants.PriorityClassNameShootControlPlane400,
		SchedulingProfile:                    v1beta1helper.ShootSchedulingProfile(b.Shoot.GetInfo()),
		SecretNameServerCA:                   v1beta1constants.SecretNameCACluster,
		SyncPeriod:                           &metav1.Duration{Duration: time.Minute},
		SystemComponentTolerations:           gardenerutils.ExtractSystemComponentsTolerations(b.Shoot.GetInfo().Spec.Provider.Workers),
		TargetDiffersFromSourceCluster:       true,
		TargetDisableCache:                   pointer.Bool(true),
		KubernetesVersion:                    version,
		VPA: &resourcemanager.VPAConfig{
			MinAllowed: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("30Mi"),
			},
		},
		WatchedNamespace:            pointer.String(b.Shoot.SeedNamespace),
		TopologyAwareRoutingEnabled: b.Shoot.TopologyAwareRoutingEnabled,
	}

	return resourcemanager.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		cfg,
	), nil
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
