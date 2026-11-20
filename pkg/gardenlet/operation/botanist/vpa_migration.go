// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	utilsvpa "github.com/gardener/gardener/pkg/utils/vpa"
)

// RemoveVPAInPlaceUpdatesMutatedLabel removes the obsolete `vpa-in-place-updates.resources.gardener.cloud/mutated`
// label from all VerticalPodAutoscalers in the kube-system namespace of the shoot cluster.
//
// TODO(DobromirNPeev): Remove this method after Gardener v1.166 has been released.
// Note: This migration targets resources inside a Shoot cluster. A Shoot can be hibernated for extended periods, during
// which gardener-resource-manager is scaled to 0 and cannot perform this migration. Remove only after verifying that
// all Shoots have been reconciled post-migration.
func (b *Botanist) RemoveVPAInPlaceUpdatesMutatedLabel(ctx context.Context) error {
	return utilsvpa.RemoveInPlaceUpdatesMutatedLabel(ctx, b.ShootClientSet.Client(), b.Logger, client.InNamespace(metav1.NamespaceSystem))
}
