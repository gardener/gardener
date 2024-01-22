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

	"github.com/gardener/gardener/imagevector"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DeployLogging will install the logging stack for the Shoot in the Seed clusters.
func (b *Botanist) DeployLogging(ctx context.Context) error {
	if !b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) {
		return b.DestroySeedLogging(ctx)
	}

	if b.isShootEventLoggerEnabled() {
		if err := b.Shoot.Components.Logging.EventLogger.Deploy(ctx); err != nil {
			return err
		}
	} else {
		if err := b.Shoot.Components.Logging.EventLogger.Destroy(ctx); err != nil {
			return err
		}
	}

	// check if vali is enabled in gardenlet config, default is true
	if !gardenlethelper.IsValiEnabled(b.Config) {
		return b.Shoot.Components.Logging.Vali.Destroy(ctx)
	}

	return b.Shoot.Components.Logging.Vali.Deploy(ctx)
}

// DestroySeedLogging will uninstall the logging stack for the Shoot in the Seed clusters.
func (b *Botanist) DestroySeedLogging(ctx context.Context) error {
	if err := b.Shoot.Components.Logging.EventLogger.Destroy(ctx); err != nil {
		return err
	}

	return b.Shoot.Components.Logging.Vali.Destroy(ctx)
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
	imageEventLogger, err := imagevector.ImageVector().FindImage(imagevector.ImageNameEventLogger, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
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
		},
	)
}

// DefaultVali returns a deployer for Vali.
func (b *Botanist) DefaultVali() (vali.Interface, error) {
	hvpaEnabled := features.DefaultFeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = features.DefaultFeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	return shared.NewVali(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		component.ClusterTypeShoot,
		b.Shoot.GetReplicas(1),
		b.isShootNodeLoggingEnabled(),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		nil,
		b.ComputeValiHost(),
		hvpaEnabled,
		nil,
	)
}
