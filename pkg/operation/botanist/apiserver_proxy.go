// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/pkg/operation/botanist/component/apiserverproxy"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultAPIServerProxy returns a deployer for the apiserver-proxy.
func (b *Botanist) DefaultAPIServerProxy() (apiserverproxy.Interface, error) {
	image, err := b.ImageVector.FindImage(images.ImageNameApiserverProxy, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	sidecarImage, err := b.ImageVector.FindImage(images.ImageNameApiserverProxySidecar, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := apiserverproxy.Values{
		APIServerProxyImage:        image.String(),
		APIServerProxySidecarImage: sidecarImage.String(),
		ProxySeedServerHost:        b.outOfClusterAPIServerFQDN(),
		ProxySeedServerPort:        "8443",
		AdminPort:                  16910,
		PodMutatorEnabled:          b.APIServerSNIPodMutatorEnabled(),
		PSPDisabled:                b.Shoot.PSPDisabled,
	}

	return apiserverproxy.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, b.SecretsManager, values), nil
}

// DeployAPIServerProxy deploys the apiserver-proxy.
func (b *Botanist) DeployAPIServerProxy(ctx context.Context) error {
	b.Shoot.Components.SystemComponents.APIServerProxy.SetAdvertiseIPAddress(b.APIServerClusterIP)

	if !b.APIServerSNIEnabled() {
		return b.Shoot.Components.SystemComponents.APIServerProxy.Destroy(ctx)
	}

	return b.Shoot.Components.SystemComponents.APIServerProxy.Deploy(ctx)
}
