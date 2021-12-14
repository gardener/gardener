// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// DefaultDependencyWatchdogAccess returns an instance of the Deployer which reconciles the resources so that DependencyWatchdogAccess can access a
// shoot cluster.
func (b *Botanist) DefaultDependencyWatchdogAccess() dependencywatchdog.AccessInterface {
	return dependencywatchdog.NewAccess(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		dependencywatchdog.AccessValues{
			ServerInCluster:    b.Shoot.ComputeInClusterAPIServerAddress(false),
			ServerOutOfCluster: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
		},
	)
}

// DeployDependencyWatchdogAccess deploys the DependencyWatchdogAccess resources.
func (b *Botanist) DeployDependencyWatchdogAccess(ctx context.Context) error {
	b.Shoot.Components.DependencyWatchdogAccess.SetCACertificate(b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secretutils.DataKeyCertificateCA])

	return b.Shoot.Components.DependencyWatchdogAccess.Deploy(ctx)
}
