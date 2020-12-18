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

	"github.com/gardener/gardener/pkg/operation/botanist/extensions/extension"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultExtension creates the default deployer for the Extension custom resources.
func (b *Botanist) DefaultExtension(seedClient client.Client) extension.Interface {
	return extension.New(
		b.Logger,
		seedClient,
		&extension.Values{
			Namespace:  b.Shoot.SeedNamespace,
			Extensions: b.Shoot.Extensions,
		},
		extension.DefaultInterval,
		extension.DefaultSevereThreshold,
		extension.DefaultTimeout,
	)
}

// DeployExtensions deploys the Extension custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployExtensions(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Extension.Restore(ctx, b.ShootState)
	}
	return b.Shoot.Components.Extensions.Extension.Deploy(ctx)
}
