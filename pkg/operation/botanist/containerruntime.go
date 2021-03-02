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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/containerruntime"
)

// DefaultContainerRuntime creates the default deployer for the ContainerRuntime custom resource.
func (b *Botanist) DefaultContainerRuntime(seedClient client.Client) containerruntime.Interface {
	return containerruntime.New(
		b.Logger,
		seedClient,
		&containerruntime.Values{
			Namespace: b.Shoot.SeedNamespace,
			Workers:   b.Shoot.Info.Spec.Provider.Workers,
		},
		containerruntime.DefaultInterval,
		containerruntime.DefaultSevereThreshold,
		containerruntime.DefaultTimeout,
	)
}

// DeployContainerRuntime deploys the ContainerRuntime custom resources and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployContainerRuntime(ctx context.Context) error {
	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.ContainerRuntime.Restore(ctx, b.ShootState)
	}
	return b.Shoot.Components.Extensions.ContainerRuntime.Deploy(ctx)
}
