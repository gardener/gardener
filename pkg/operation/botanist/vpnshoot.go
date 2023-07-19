// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/component/vpnshoot"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVPNShoot returns a deployer for the VPNShoot
func (b *Botanist) DefaultVPNShoot() (vpnshoot.Interface, error) {
	image, err := b.ImageVector.FindImage(imagevector.ImageNameVpnShootClient, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := vpnshoot.Values{
		Image:      image.String(),
		VPAEnabled: b.Shoot.WantsVerticalPodAutoscaler,
		ReversedVPN: vpnshoot.ReversedVPNValues{
			Header:      "outbound|1194||" + vpnseedserver.ServiceName + "." + b.Shoot.SeedNamespace + ".svc.cluster.local",
			Endpoint:    b.outOfClusterAPIServerFQDN(),
			OpenVPNPort: 8132,
		},
		HighAvailabilityEnabled:              b.Shoot.VPNHighAvailabilityEnabled,
		HighAvailabilityNumberOfSeedServers:  b.Shoot.VPNHighAvailabilityNumberOfSeedServers,
		HighAvailabilityNumberOfShootClients: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
		PSPDisabled:                          b.Shoot.PSPDisabled,
	}

	return vpnshoot.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		values,
	), nil
}
