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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/worker"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultWorker creates the default deployer for the Worker custom resource.
func (b *Botanist) DefaultWorker(seedClient client.Client) shoot.ExtensionWorker {
	return worker.New(
		b.Logger,
		seedClient,
		&worker.Values{
			Namespace:         b.Shoot.SeedNamespace,
			Name:              b.Shoot.Info.Name,
			Type:              b.Shoot.Info.Spec.Provider.Type,
			Region:            b.Shoot.Info.Spec.Region,
			Workers:           b.Shoot.Info.Spec.Provider.Workers,
			KubernetesVersion: b.Shoot.KubernetesVersion,
		},
		worker.DefaultInterval,
		worker.DefaultSevereThreshold,
		worker.DefaultTimeout,
	)
}

// DeployWorker deploys the Worker custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployWorker(ctx context.Context) error {
	b.Shoot.Components.Extensions.Worker.SetSSHPublicKey(b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys])
	b.Shoot.Components.Extensions.Worker.SetInfrastructureProviderStatus(&runtime.RawExtension{Raw: b.Shoot.InfrastructureStatus})
	b.Shoot.Components.Extensions.Worker.SetOperatingSystemConfigMaps(b.Shoot.OperatingSystemConfigsMap)

	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Worker.Restore(ctx, b.ShootState)
	}

	return b.Shoot.Components.Extensions.Worker.Deploy(ctx)
}
