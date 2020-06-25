// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/network"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultNetwork creates the default deployer for the Network custom resource.
func (b *Botanist) DefaultNetwork(seedClient client.Client) component.DeployWaiter {
	return network.New(
		b.Logger,
		seedClient,
		&network.Values{
			Namespace:                               b.Shoot.SeedNamespace,
			Name:                                    b.Shoot.Info.Name,
			IsInRestorePhaseOfControlPlaneMigration: b.isRestorePhase(),
			Type:                                    b.Shoot.Info.Spec.Networking.Type,
			ProviderConfig:                          b.Shoot.Info.Spec.Networking.ProviderConfig,
			PodCIDR:                                 b.Shoot.Networks.Pods,
			ServiceCIDR:                             b.Shoot.Networks.Services,
		},
		network.DefaultInterval,
		network.DefaultSevereThreshold,
		network.DefaultTimeout,
	)
}

// DeployNetwork deploys the Network custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployNetwork(ctx context.Context) error {
	if err := b.Shoot.Components.Network.Deploy(ctx); err != nil {
		return err
	}

	if b.isRestorePhase() {
		return b.restoreExtensionObject(ctx, b.K8sSeedClient.Client(), &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}, extensionsv1alpha1.NetworkResource)
	}
	return nil
}
