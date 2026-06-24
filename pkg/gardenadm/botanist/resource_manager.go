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
		SystemComponentsConfigWebhookEnabled: true,
		HighAvailabilityConfigWebhookEnabled: true,
		PriorityClassName:                    v1beta1constants.PriorityClassNameShootControlPlane400,
		SecretNameServerCA:                   v1beta1constants.SecretNameCACluster,
		SystemComponentTolerations:           gardenerutils.ExtractSystemComponentsTolerations(b.Shoot.GetInfo().Spec.Provider.Workers),
		PodKubeAPIServerLoadBalancingWebhook: resourcemanager.PodKubeAPIServerLoadBalancingWebhook{
			Enabled: features.DefaultFeatureGate.Enabled(features.IstioTLSTermination),
			Configs: []resourcemanager.PodKubeAPIServerLoadBalancingWebhookConfig{
				{
					NamespaceSelector: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
				},
			},
		},
		// Disable the vpa-in-place-updates webhook as there are no VPA components that manage VPA resources and
		// there is no reason for the GRM webhook to be deployed.
		//
		// Further more, upon invocation, the GRM's /webhooks/vpa-in-place-updates endpoint,
		// introduced by the webhook, fails to verify the request certificate with the following error message:
		//
		// "x509: certificate is valid for machine-0, not gardener-resource-manager.kube-system.svc"
		//
		// indicating that the gardenadm's initialization flow introduces a side effect when redeplying the GRM.
		//
		// GRM's vpa-in-place-updates webhook is planned to be removed soon in favor of setting the update mode to InPlaceOrRecreate explicitly.
		// For more details, see https://github.com/gardener/gardener/issues/12955.
		VPAInPlaceUpdatesEnabled: false,
	})
}
