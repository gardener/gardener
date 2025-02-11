// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/networking/apiserverproxy"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultAPIServerProxy returns a deployer for the apiserver-proxy.
func (b *Botanist) DefaultAPIServerProxy() (apiserverproxy.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameApiserverProxy, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	sidecarImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameApiserverProxySidecar, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
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
		SeedNamespace:       b.Shoot.SeedNamespace,
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
