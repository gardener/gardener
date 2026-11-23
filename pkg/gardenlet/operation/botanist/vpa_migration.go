// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vpautils "github.com/gardener/gardener/pkg/utils/vpa"
)

// RemoveVPAInPlaceUpdatesMutatedLabel removes the obsolete `vpa-in-place-updates.resources.gardener.cloud/mutated`
// label from all VerticalPodAutoscalers in the kube-system namespace of the shoot cluster.
//
// TODO(DobromirNPeev): Remove this method after Gardener v1.166 has been released.
// Note: This migration targets resources inside a Shoot cluster. A Shoot can be hibernated for extended periods, during
// which its kube-apiserver is not running and the migration cannot be performed. The cleanup is best effort.
func (b *Botanist) RemoveVPAInPlaceUpdatesMutatedLabel(ctx context.Context) error {
	return vpautils.RemoveInPlaceUpdatesMutatedLabel(ctx, b.ShootClientSet.Client(), b.Logger, client.InNamespace(metav1.NamespaceSystem))
}
