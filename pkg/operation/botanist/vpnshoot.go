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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVPNShoot returns a deployer for the VPNShoot
func (b *Botanist) DefaultVPNShoot() (vpnshoot.Interface, error) {
	var (
		imageName         = charts.ImageNameVpnShoot
		nodeNetworkCIDR   string
		reversedVPNValues = vpnshoot.ReversedVPNValues{
			Enabled: false,
		}
	)

	if nodeNetwork := b.Shoot.GetInfo().Spec.Networking.Nodes; nodeNetwork != nil {
		nodeNetworkCIDR = *nodeNetwork
	}

	if b.Shoot.ReversedVPNEnabled {
		imageName = charts.ImageNameVpnShootClient

		reversedVPNValues = vpnshoot.ReversedVPNValues{
			Enabled:     true,
			Header:      "outbound|1194||" + vpnseedserver.ServiceName + "." + b.Shoot.SeedNamespace + ".svc.cluster.local",
			Endpoint:    b.outOfClusterAPIServerFQDN(),
			OpenVPNPort: 8132,
		}
	}

	image, err := b.ImageVector.FindImage(imageName, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := vpnshoot.Values{
		Image:       image.String(),
		VPAEnabled:  b.Shoot.WantsVerticalPodAutoscaler,
		ReversedVPN: reversedVPNValues,
		Network: vpnshoot.NetworkValues{
			PodCIDR:     b.Shoot.Networks.Pods.String(),
			ServiceCIDR: b.Shoot.Networks.Services.String(),
			NodeCIDR:    nodeNetworkCIDR,
		},
	}

	return vpnshoot.New(
		b.K8sSeedClient.Client(),
		b.Shoot.SeedNamespace,
		values,
	), nil
}

// DeployVPNShoot deploys the VPNShoot system component.
func (b *Botanist) DeployVPNShoot(ctx context.Context) error {
	secrets := vpnshoot.Secrets{}

	if b.Shoot.ReversedVPNEnabled {
		secrets.TLSAuth = component.Secret{Name: vpnseedserver.VpnSeedServerTLSAuth, Checksum: b.LoadCheckSum(vpnseedserver.VpnSeedServerTLSAuth), Data: b.LoadSecret(vpnseedserver.VpnSeedServerTLSAuth).Data}
		secrets.Server = component.Secret{Name: vpnshoot.SecretNameVPNShootClient, Checksum: b.LoadCheckSum(vpnshoot.SecretNameVPNShootClient), Data: b.LoadSecret(vpnshoot.SecretNameVPNShootClient).Data}
	} else {
		checkSumDH := diffieHellmanKeyChecksum
		openvpnDiffieHellmanSecret := map[string][]byte{"dh2048.pem": []byte(DefaultDiffieHellmanKey)}
		if dh := b.LoadSecret(v1beta1constants.GardenRoleOpenVPNDiffieHellman); dh != nil {
			openvpnDiffieHellmanSecret = dh.Data
			checkSumDH = b.LoadCheckSum(v1beta1constants.GardenRoleOpenVPNDiffieHellman)
		}

		secrets.TLSAuth = component.Secret{Name: kubeapiserver.SecretNameVPNSeedTLSAuth, Checksum: b.LoadCheckSum(kubeapiserver.SecretNameVPNSeedTLSAuth), Data: b.LoadSecret(kubeapiserver.SecretNameVPNSeedTLSAuth).Data}
		secrets.DH = &component.Secret{Name: v1beta1constants.GardenRoleOpenVPNDiffieHellman, Checksum: checkSumDH, Data: openvpnDiffieHellmanSecret}
		secrets.Server = component.Secret{Name: vpnshoot.SecretNameVPNShoot, Checksum: b.LoadCheckSum(vpnshoot.SecretNameVPNShoot), Data: b.LoadSecret(vpnshoot.SecretNameVPNShoot).Data}
	}

	b.Shoot.Components.SystemComponents.VPNShoot.SetSecrets(secrets)

	return b.Shoot.Components.SystemComponents.VPNShoot.Deploy(ctx)
}
