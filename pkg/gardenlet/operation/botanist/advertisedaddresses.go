// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// UpdateAdvertisedAddresses updates the shoot.status.advertisedAddresses with the list of
// addresses on which the API server of the shoot is accessible.
func (b *Botanist) UpdateAdvertisedAddresses(ctx context.Context) error {
	return b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, false, false, func(shoot *gardencorev1beta1.Shoot) error {
		addresses, err := b.ToAdvertisedAddresses()
		if err != nil {
			return err
		}
		shoot.Status.AdvertisedAddresses = addresses
		return nil
	})
}

// ToAdvertisedAddresses returns list of advertised addresses on a Shoot cluster.
func (b *Botanist) ToAdvertisedAddresses() ([]gardencorev1beta1.ShootAdvertisedAddress, error) {
	var addresses []gardencorev1beta1.ShootAdvertisedAddress

	if b.Shoot == nil {
		return addresses, nil
	}

	if b.Shoot.ExternalClusterDomain != nil && len(*b.Shoot.ExternalClusterDomain) > 0 {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressExternal,
			URL:  "https://" + gardenerutils.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
		})
	}

	if b.ControlPlaneWildcardCert != nil {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressWildcardTLSSeedBound,
			URL:  "https://" + b.ComputeKubeAPIServerHost(),
		})
	}

	if len(b.Shoot.InternalClusterDomain) > 0 {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressInternal,
			URL:  "https://" + gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		})
	}

	if len(b.APIServerAddress) > 0 && len(addresses) == 0 {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressUnmanaged,
			URL:  "https://" + b.APIServerAddress,
		})
	}

	hasCustomIssuer := func(shoot *gardencorev1beta1.Shoot) bool {
		return shoot != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.Issuer != nil
	}

	if len(b.Shoot.InternalClusterDomain) > 0 ||
		hasCustomIssuer(b.Shoot.GetInfo()) ||
		v1beta1helper.HasManagedIssuer(b.Shoot.GetInfo()) {
		externalHostname := b.Shoot.ComputeOutOfClusterAPIServerAddress(true)
		serviceAccountConfig, err := b.computeKubeAPIServerServiceAccountConfig(externalHostname)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressServiceAccountIssuer,
			URL:  serviceAccountConfig.Issuer,
		})
	}

	return addresses, nil
}
