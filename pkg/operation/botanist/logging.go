// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DeploySeedLogging will install the logging stack for the Shoot in the Seed clusters.
func (b *Botanist) DeploySeedLogging(ctx context.Context) error {
	if !b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) {
		if err := b.Shoot.Components.Logging.ShootRBACProxy.Destroy(ctx); err != nil {
			return err
		}

		if err := b.Shoot.Components.Logging.ShootEventLogger.Destroy(ctx); err != nil {
			return err
		}

		return b.Shoot.Components.Logging.Vali.Destroy(ctx)
	}

	// TODO(rickardsjp, istvanballok): Remove in release v1.77 once the Loki to Vali migration is complete.
	if exists, err := b.lokiPvcExists(ctx); err != nil {
		return err
	} else if exists {
		if err := b.destroyLokiBasedShootLoggingStackRetainingPvc(ctx); err != nil {
			return err
		}
		// If a Loki PVC exists, rename it to Vali.
		if err := b.renameLokiPvcToVali(ctx); err != nil {
			return err
		}
	}

	if b.isShootEventLoggerEnabled() {
		if err := b.Shoot.Components.Logging.ShootEventLogger.Deploy(ctx); err != nil {
			return err
		}
	} else {
		if err := b.Shoot.Components.Logging.ShootEventLogger.Destroy(ctx); err != nil {
			return err
		}
	}

	// check if vali is enabled in gardenlet config, default is true
	if !gardenlethelper.IsValiEnabled(b.Config) {
		// Because ShootNodeLogging is installed as part of the Vali pod
		// we have to delete it too in case it was previously deployed
		if err := b.Shoot.Components.Logging.ShootRBACProxy.Destroy(ctx); err != nil {
			return err
		}

		return b.Shoot.Components.Logging.Vali.Destroy(ctx)
	}

	if b.isShootNodeLoggingEnabled() {
		if err := b.Shoot.Components.Logging.ShootRBACProxy.Deploy(ctx); err != nil {
			return err
		}
	} else {
		if err := b.Shoot.Components.Logging.ShootRBACProxy.Destroy(ctx); err != nil {
			return err
		}
	}

	return b.Shoot.Components.Logging.Vali.Deploy(ctx)
}

func (b *Botanist) lokiPvcExists(ctx context.Context) (bool, error) {
	return common.LokiPvcExists(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, b.Logger)
}

func (b *Botanist) renameLokiPvcToVali(ctx context.Context) error {
	return common.RenameLokiPvcToValiPvc(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, b.Logger)
}

func (b *Botanist) destroyLokiBasedShootLoggingStackRetainingPvc(ctx context.Context) error {
	if err := b.destroyLokiBasedShootNodeLogging(ctx); err != nil {
		return err
	}

	// The EventLogger is not dependent on Loki/Vali and therefore doesn't need to be deleted.
	// if err := b.Shoot.Components.Logging.ShootEventLogger.Destroy(ctx); err != nil {
	// 	return err
	// }

	return common.DeleteLokiRetainPvc(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, b.Logger)
}

func (b *Botanist) destroyLokiBasedShootNodeLogging(ctx context.Context) error {
	if err := b.Shoot.Components.Logging.ShootRBACProxy.Destroy(ctx); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, b.SeedClientSet.Client(),
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: b.Shoot.SeedNamespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: b.Shoot.SeedNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: b.Shoot.SeedNamespace}},
	)
}

func (b *Botanist) isShootNodeLoggingEnabled() bool {
	if b.Shoot != nil && !b.Shoot.IsWorkerless && b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) &&
		gardenlethelper.IsValiEnabled(b.Config) && b.Config != nil &&
		b.Config.Logging != nil && b.Config.Logging.ShootNodeLogging != nil {

		for _, purpose := range b.Config.Logging.ShootNodeLogging.ShootPurposes {
			if gardencore.ShootPurpose(b.Shoot.Purpose) == purpose {
				return true
			}
		}
	}
	return false
}

func (b *Botanist) isShootEventLoggerEnabled() bool {
	return b.Shoot != nil && b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) && gardenlethelper.IsEventLoggingEnabled(b.Config)
}

// DefaultEventLogger returns a deployer for the shoot-event-logger.
func (b *Botanist) DefaultEventLogger() (component.Deployer, error) {
	imageEventLogger, err := b.ImageVector.FindImage(images.ImageNameEventLogger, imagevector.RuntimeVersion(b.SeedVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return eventlogger.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		eventlogger.Values{
			Image:    imageEventLogger.String(),
			Replicas: b.Shoot.GetReplicas(1),
		})
}

func (b *Botanist) defaultVali() (component.Deployer, error) {
	var ingressClass string

	k8Version, err := semver.NewVersion(*b.Seed.GetInfo().Status.KubernetesVersion)
	if err != nil {
		return nil, fmt.Errorf("can't get seed k8s version: %w", err)
	}

	hvpaEnabled := features.DefaultFeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = features.DefaultFeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	if b.isShootNodeLoggingEnabled() {
		ingressClass, err = gardenerutils.ComputeNginxIngressClassForSeed(b.Seed.GetInfo(), b.Seed.GetInfo().Status.KubernetesVersion)
		if err != nil {
			return nil, err
		}
	}

	return shared.NewVali(
		b.SeedClientSet.Client(),
		b.ImageVector,
		b.Shoot.GetReplicas(1),
		b.Shoot.SeedNamespace,
		ingressClass,
		v1beta1constants.PriorityClassNameShootControlPlane100,
		component.ClusterTypeShoot,
		b.ComputeValiHost(),
		b.SecretsManager,
		nil,
		k8Version,
		b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config),
		b.isShootNodeLoggingEnabled(),
		true,
		hvpaEnabled,
		nil)
}
