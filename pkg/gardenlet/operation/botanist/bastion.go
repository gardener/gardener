// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
)

// DeleteBastions deletes all bastions from the Shoot namespace in the Seed.
func (b *Botanist) DeleteBastions(ctx context.Context) error {
	return extensions.DeleteExtensionObjects(
		ctx,
		b.SeedClientSet.Client(),
		&extensionsv1alpha1.BastionList{},
		b.Shoot.ControlPlaneNamespace,
		func(_ extensionsv1alpha1.Object) bool {
			return true
		},
	)
}
