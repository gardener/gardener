// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	corev1 "k8s.io/api/core/v1"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsselfhostedshootexposure "github.com/gardener/gardener/pkg/component/extensions/selfhostedshootexposure"
)

// DefaultSelfHostedShootExposure creates the default deployer for the SelfHostedShootExposure resource.
// The actual endpoints are populated at deploy time by the caller via SetEndpoints.
func (b *Botanist) DefaultSelfHostedShootExposure() extensionsselfhostedshootexposure.Interface {
	shoot := b.Shoot.GetInfo()
	pool := v1beta1helper.ControlPlaneWorkerPoolForShoot(shoot.Spec.Provider.Workers)
	values := &extensionsselfhostedshootexposure.Values{
		Name:      shoot.Name,
		Namespace: b.Shoot.ControlPlaneNamespace,
		Type:      *pool.ControlPlane.Exposure.Extension.Type,
		Port:      443,
	}

	// For unmanaged infrastructure no CredentialsRef is set; the provider must handle nil.
	if v1beta1helper.HasManagedInfrastructure(shoot) {
		values.CredentialsRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       v1beta1constants.SecretNameCloudProvider,
			Namespace:  b.Shoot.ControlPlaneNamespace,
		}
	}

	return extensionsselfhostedshootexposure.New(
		b.Logger,
		b.SeedClientSet.Client(),
		values,
		extensionsselfhostedshootexposure.DefaultInterval,
		extensionsselfhostedshootexposure.DefaultSevereThreshold,
		extensionsselfhostedshootexposure.DefaultTimeout,
	)
}
