// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/apiserverproxy"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultAPIServerProxy returns a deployer for the apiserver-proxy.
func (b *Botanist) DefaultAPIServerProxy() (apiserverproxy.Interface, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameApiserverProxy, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	sidecarImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameApiserverProxySidecar, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var (
		// we don't want to use AUTO for single-stack clusters as it causes an unnecessary failed lookup
		// ref https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/cluster.proto#enum-config-cluster-v3-cluster-dnslookupfamily
		dnsLookupFamily = "V4_ONLY"
	)

	if gardencorev1beta1.IsIPv6SingleStack(b.Shoot.GetInfo().Spec.Networking.IPFamilies) {
		dnsLookupFamily = "V6_ONLY"
	}

	values := apiserverproxy.Values{
		Image:               image.String(),
		SidecarImage:        sidecarImage.String(),
		ProxySeedServerHost: b.outOfClusterAPIServerFQDN(),
		DNSLookupFamily:     dnsLookupFamily,
	}

	return apiserverproxy.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, b.SecretsManager, values), nil
}

// DeployAPIServerProxy deploys the apiserver-proxy.
func (b *Botanist) DeployAPIServerProxy(ctx context.Context) error {
	if !b.ShootUsesDNS() {
		return b.Shoot.Components.SystemComponents.APIServerProxy.Destroy(ctx)
	}

	b.Shoot.Components.SystemComponents.APIServerProxy.SetAdvertiseIPAddress(b.APIServerClusterIP)

	return b.Shoot.Components.SystemComponents.APIServerProxy.Deploy(ctx)
}
