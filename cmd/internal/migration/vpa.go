// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	utilsvpa "github.com/gardener/gardener/pkg/utils/vpa"
)

// RemoveVPAInPlaceUpdatesMutatedLabel removes the `vpa-in-place-updates.resources.gardener.cloud/mutated` label from
// all VerticalPodAutoscalers in the cluster.
//
// TODO(DobromirNPeev): Remove this function after Gardener v1.166 has been released.
func RemoveVPAInPlaceUpdatesMutatedLabel(ctx context.Context, c client.Client, log logr.Logger) error {
	return utilsvpa.RemoveInPlaceUpdatesMutatedLabel(ctx, c, log)
}
