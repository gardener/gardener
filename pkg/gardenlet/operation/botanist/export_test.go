// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/component/etcd/etcd"
)

// Functions exported for testing.

var UseShootAccessTokensForSelfHostedShootControlPlane = func(b *Botanist, ctx context.Context) (bool, error) {
	return b.useShootAccessTokensForSelfHostedShootControlPlane(ctx)
}

var CrossSeedPeerHostnames = crossSeedPeerHostnames

var SetLiveMigrationEtcdValues = func(b *Botanist, ctx context.Context, values *etcd.Values, role string) error {
	return b.setLiveMigrationEtcdValues(ctx, values, role)
}
