// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Functions exported for testing.

var UseShootAccessTokensForSelfHostedShootControlPlane = func(b *Botanist, ctx context.Context) (bool, error) {
	return b.useShootAccessTokensForSelfHostedShootControlPlane(ctx)
}

var WorkerPoolsForWorkerResource = func(b *Botanist) []gardencorev1beta1.Worker {
	return b.workerPoolsForWorkerResource()
}

// SetSourceMemberRemovalIntervalForTest overrides the poll interval used by WaitUntilSourceEtcdMembersRemoved.
// Returns a function that restores the original value.
func SetSourceMemberRemovalIntervalForTest(d time.Duration) func() {
	old := sourceMemberRemovalInterval
	sourceMemberRemovalInterval = d
	return func() { sourceMemberRemovalInterval = old }
}
