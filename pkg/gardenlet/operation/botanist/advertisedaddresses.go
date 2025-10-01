// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// UpdateAdvertisedAddresses updates the shoot.status.advertisedAddresses with the list of
// addresses on which the API server of the shoot is accessible.
func (b *Botanist) UpdateAdvertisedAddresses(ctx context.Context) error {
	return b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, false, false, func(shoot *gardencorev1beta1.Shoot) error {
		addresses, err := b.ToAdvertisedAddresses(ctx)
		if err != nil {
			return err
		}
		shoot.Status.AdvertisedAddresses = addresses
		return nil
	})
}

// ToAdvertisedAddresses returns a list of advertised addresses for a Shoot cluster.
func (b *Botanist) ToAdvertisedAddresses(ctx context.Context) ([]gardencorev1beta1.ShootAdvertisedAddress, error) {
	var addresses []gardencorev1beta1.ShootAdvertisedAddress

	if b.Shoot == nil {
		return addresses, nil
	}

	if b.Shoot.ExternalClusterDomain != nil {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressExternal,
			URL:  "https://" + v1beta1helper.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
		})
	}

	if b.ControlPlaneWildcardCert != nil {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressWildcardTLSSeedBound,
			URL:  "https://" + b.ComputeKubeAPIServerHost(),
		})
	}

	if b.Shoot.InternalClusterDomain != nil {
		addresses = append(addresses, gardencorev1beta1.ShootAdvertisedAddress{
			Name: v1beta1constants.AdvertisedAddressInternal,
			URL:  "https://" + v1beta1helper.GetAPIServerDomain(*b.Shoot.InternalClusterDomain),
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

	if b.Shoot.InternalClusterDomain != nil ||
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

	ingressItems, err := b.GetIngressAdvertisedEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	addresses = append(addresses, ingressItems...)

	return addresses, nil
}

// GetIngressAdvertisedEndpoints returns a list of
// [gardencorev1beta1.ShootAdvertisedAddress] items, which have been derived
// from any existing [networkingv1.Ingress] resources labeled with
// [v1beta1constants.LabelShootEndpointAdvertise].
func (b *Botanist) GetIngressAdvertisedEndpoints(ctx context.Context) ([]gardencorev1beta1.ShootAdvertisedAddress, error) {
	result := make([]gardencorev1beta1.ShootAdvertisedAddress, 0)
	var ingressList networkingv1.IngressList

	if err := b.SeedClientSet.Client().List(
		ctx,
		&ingressList,
		client.InNamespace(b.Shoot.ControlPlaneNamespace),
		client.MatchingLabels(map[string]string{
			v1beta1constants.LabelShootEndpointAdvertise: "true",
		}),
	); err != nil {
		return nil, err
	}

	// Only the TLS items are processed, since
	// [gardencorev1beta1.ShootAdvertisedAddress] is constrained to https://
	// endpoints only.
	for _, ingress := range ingressList.Items {
		for tlsIdx, tlsItem := range ingress.Spec.TLS {
			for hostIdx, hostItem := range tlsItem.Hosts {
				if strings.Contains(hostItem, "*") {
					continue
				}
				addr := gardencorev1beta1.ShootAdvertisedAddress{
					Name: fmt.Sprintf("ingress/%s/%d/%d", ingress.Name, tlsIdx, hostIdx),
					URL:  fmt.Sprintf("https://%s", hostItem),
				}
				result = append(result, addr)
			}
		}
	}

	slices.SortStableFunc(result, func(a, b gardencorev1beta1.ShootAdvertisedAddress) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result, nil
}
