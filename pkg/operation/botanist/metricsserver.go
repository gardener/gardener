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

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"k8s.io/utils/pointer"
)

// DefaultMetricsServer returns a deployer for the metrics-server.
func (b *Botanist) DefaultMetricsServer() (metricsserver.Interface, error) {
	image, err := b.ImageVector.FindImage(charts.ImageNameMetricsServer, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var kubeAPIServerHost *string
	if b.APIServerSNIEnabled() {
		kubeAPIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	return metricsserver.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		image.String(),
		b.Shoot.WantsVerticalPodAutoscaler,
		kubeAPIServerHost,
	), nil
}

// DeployMetricsServer deploys the metrics-server.
func (b *Botanist) DeployMetricsServer(ctx context.Context) error {
	b.Shoot.Components.SystemComponents.MetricsServer.SetSecrets(metricsserver.Secrets{
		CA:     component.Secret{Name: metricsserver.SecretNameCA, Checksum: b.LoadCheckSum(metricsserver.SecretNameCA), Data: b.LoadSecret(metricsserver.SecretNameCA).Data},
		Server: component.Secret{Name: metricsserver.SecretNameServer, Checksum: b.LoadCheckSum(metricsserver.SecretNameServer), Data: b.LoadSecret(metricsserver.SecretNameServer).Data},
	})

	return b.Shoot.Components.SystemComponents.MetricsServer.Deploy(ctx)
}
