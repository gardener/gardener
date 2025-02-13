// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/observability/logging/vali"
	"github.com/gardener/gardener/pkg/component/shared"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/helper"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DeployLogging will install the logging stack for the Shoot in the Seed clusters.
func (b *Botanist) DeployLogging(ctx context.Context) error {
	if !b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) {
		return b.DestroySeedLogging(ctx)
	}

	grmIsPresent, err := b.IsGardenerResourceManagerReady(ctx)
	if err != nil {
		return err
	}
	b.Shoot.Components.ControlPlane.Vali.WithAuthenticationProxy(grmIsPresent)

	if b.isShootEventLoggerEnabled() && grmIsPresent {
		if err := b.Shoot.Components.ControlPlane.EventLogger.Deploy(ctx); err != nil {
			return err
		}
	} else if !b.isShootEventLoggerEnabled() {
		if err := b.Shoot.Components.ControlPlane.EventLogger.Destroy(ctx); err != nil {
			return err
		}
	}

	// check if vali is enabled in gardenlet config, default is true
	if !gardenlethelper.IsValiEnabled(b.Config) {
		return b.Shoot.Components.ControlPlane.Vali.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.Vali.Deploy(ctx)
}

// DestroySeedLogging will uninstall the logging stack for the Shoot in the Seed clusters.
func (b *Botanist) DestroySeedLogging(ctx context.Context) error {
	if err := b.Shoot.Components.ControlPlane.EventLogger.Destroy(ctx); err != nil {
		return err
	}

	return b.Shoot.Components.ControlPlane.Vali.Destroy(ctx)
}

func (b *Botanist) isShootNodeLoggingEnabled() bool {
	if b.Shoot != nil && !b.Shoot.IsWorkerless && b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) &&
		gardenlethelper.IsValiEnabled(b.Config) && b.Config != nil &&
		b.Config.Logging != nil && b.Config.Logging.ShootNodeLogging != nil {
		for _, purpose := range b.Config.Logging.ShootNodeLogging.ShootPurposes {
			if b.Shoot.Purpose == purpose {
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
	imageEventLogger, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameEventLogger, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return eventlogger.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		eventlogger.Values{
			Image:    imageEventLogger.String(),
			Replicas: b.Shoot.GetReplicas(1),
		},
	)
}

// DefaultVali returns a deployer for Vali.
func (b *Botanist) DefaultVali() (vali.Interface, error) {
	return shared.NewVali(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		component.ClusterTypeShoot,
		b.Shoot.GetReplicas(1),
		b.isShootNodeLoggingEnabled(),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		nil,
		b.ComputeValiHost(),
	)
}
