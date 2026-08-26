// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import "context"

// Functions exported for testing.

var UseShootAccessTokensForSelfHostedShootControlPlane = func(b *Botanist, ctx context.Context) (bool, error) {
	return b.useShootAccessTokensForSelfHostedShootControlPlane(ctx)
}
