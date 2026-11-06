// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Functions exported for testing.

var UseShootAccessTokensForSelfHostedShootControlPlane = func(b *Botanist, ctx context.Context) (bool, error) {
	return b.useShootAccessTokensForSelfHostedShootControlPlane(ctx)
}

var AppendStaticAdminKubeconfigToFiles = func(b *Botanist, files []extensionsv1alpha1.File) ([]extensionsv1alpha1.File, error) {
	return b.appendStaticAdminKubeconfigToFiles(files)
}

var AppendDynamicAdminKubeconfigToFiles = func(b *Botanist, files []extensionsv1alpha1.File) ([]extensionsv1alpha1.File, error) {
	return b.appendDynamicAdminKubeconfigToFiles(files)
}

var WorkerPoolsForWorkerResource = func(b *Botanist) []gardencorev1beta1.Worker {
	return b.workerPoolsForWorkerResource()
}
