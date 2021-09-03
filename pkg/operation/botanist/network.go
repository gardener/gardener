// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package botanist

import (
	"context"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/network"
)

// DefaultNetwork creates the default deployer for the Network custom resource.
func (b *Botanist) DefaultNetwork() component.DeployMigrateWaiter {
	return network.New(
		b.Logger,
		b.K8sSeedClient.Client(),
		&network.Values{
			Namespace:      b.Shoot.SeedNamespace,
			Name:           b.Shoot.GetInfo().Name,
			Type:           b.Shoot.GetInfo().Spec.Networking.Type,
			ProviderConfig: b.Shoot.GetInfo().Spec.Networking.ProviderConfig,
			PodCIDR:        b.Shoot.Networks.Pods,
			ServiceCIDR:    b.Shoot.Networks.Services,
		},
		network.DefaultInterval,
		network.DefaultSevereThreshold,
		network.DefaultTimeout,
	)
}

// DeployNetwork deploys the Network custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployNetwork(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Network.Restore(ctx, b.GetShootState())
	}

	return b.Shoot.Components.Extensions.Network.Deploy(ctx)
}
