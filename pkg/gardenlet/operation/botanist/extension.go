// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultExtension creates the default deployer for the Extension custom resources.
func (b *Botanist) DefaultExtension(ctx context.Context) (extension.Interface, error) {
	return shared.NewExtension(ctx, b.Logger, b.GardenClient, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, extensionsv1alpha1.ExtensionClassShoot, b.Shoot.GetInfo().Spec.Extensions, b.Shoot.IsWorkerless)
}

// DeployExtensionsAfterKubeAPIServer deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensionsAfterKubeAPIServer(ctx context.Context) error {
	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.RestoreAfterKubeAPIServer(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.Extensions.Extension.DeployAfterKubeAPIServer(ctx)
}

// DeployExtensionsAfterWorker deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensionsAfterWorker(ctx context.Context) error {
	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.RestoreAfterWorker(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.Extensions.Extension.DeployAfterWorker(ctx)
}

// DeployExtensionsBeforeKubeAPIServer deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensionsBeforeKubeAPIServer(ctx context.Context) error {
	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.RestoreBeforeKubeAPIServer(ctx, b.Shoot.GetShootState())
	}
	return b.Shoot.Components.Extensions.Extension.DeployBeforeKubeAPIServer(ctx)
}
