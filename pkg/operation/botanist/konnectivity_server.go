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

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/konnectivity"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultKonnectivityServer returns a deployer for the konnectivity-server.
func (b *Botanist) DefaultKonnectivityServer() (konnectivity.KonnectivityServer, error) {
	if !b.Shoot.KonnectivityTunnelEnabled || !b.APIServerSNIEnabled() {
		ks, err := konnectivity.NewServer(&konnectivity.ServerOptions{
			Client:    b.K8sSeedClient.Client(),
			Namespace: b.Shoot.SeedNamespace,
		})
		if err != nil {
			return nil, err
		}

		return konnectivity.OpDestroy(ks), nil
	}

	image, err := b.ImageVector.FindImage(charts.ImageNameKonnectivityServer)
	if err != nil {
		return nil, err
	}

	return konnectivity.NewServer(&konnectivity.ServerOptions{
		Client:    b.K8sSeedClient.Client(),
		Namespace: b.Shoot.SeedNamespace,
		Image:     image.String(),
		Replicas:  b.Shoot.GetReplicas(2),
		Hosts: []string{
			gutil.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
			gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		},
		IstioIngressLabels: b.Config.SNI.Ingress.Labels,
	})
}

// DeployKonnectivityServer deploys the KonnectivityServer.
func (b *Botanist) DeployKonnectivityServer(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KonnectivityServer.SetSecrets(konnectivity.ServerSecrets{
		Kubeconfig: component.Secret{Name: konnectivity.SecretNameServerKubeconfig, Checksum: b.CheckSums[konnectivity.SecretNameServerKubeconfig]},
		Server:     component.Secret{Name: konnectivity.SecretNameServerTLS, Checksum: b.CheckSums[konnectivity.SecretNameServerTLS]},
		ClientCA:   component.Secret{Name: konnectivity.SecretNameServerCA, Checksum: b.CheckSums[konnectivity.SecretNameServerCA]},
	})

	return b.Shoot.Components.ControlPlane.KonnectivityServer.Deploy(ctx)
}
