// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewRuntimeGardenerResourceManager returns the gardener-resource-manager component for deploying it to the garden
// namespace.
func (b *GardenadmBotanist) NewRuntimeGardenerResourceManager() (resourcemanager.Interface, error) {
	return sharedcomponent.NewRuntimeGardenerResourceManager(b.SeedClientSet.Client(), v1beta1constants.GardenNamespace, b.SecretsManager, resourcemanager.Values{
		DefaultSeccompProfileEnabled:         features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
		HighAvailabilityConfigWebhookEnabled: true,
		PriorityClassName:                    v1beta1constants.PriorityClassNameShootControlPlane400,
		SecretNameServerCA:                   v1beta1constants.SecretNameCACluster,
		SystemComponentTolerations:           gardenerutils.ExtractSystemComponentsTolerations(b.Shoot.GetInfo().Spec.Provider.Workers),
	})
}
