// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultResourceManager returns an instance of Gardener Resource Manager with defaults configured for being deployed
// in a Shoot namespace.
func (b *Botanist) DefaultResourceManager() (resourcemanager.Interface, error) {
	var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
	if b.Config != nil && b.Config.NodeToleration != nil {
		defaultNotReadyTolerationSeconds = b.Config.NodeToleration.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = b.Config.NodeToleration.DefaultUnreachableTolerationSeconds
	}

	var (
		newFunc = shared.NewTargetGardenerResourceManager

		values = resourcemanager.Values{
			ClusterIdentity:                           b.Seed.GetInfo().Status.ClusterIdentity,
			HighAvailabilityConfigWebhookEnabled:      true,
			DefaultNotReadyToleration:                 defaultNotReadyTolerationSeconds,
			DefaultUnreachableToleration:              defaultUnreachableTolerationSeconds,
			IsWorkerless:                              b.Shoot.IsWorkerless,
			KubernetesServiceHost:                     new(b.Shoot.ComputeOutOfClusterAPIServerAddress(true)),
			LogLevel:                                  logger.InfoLevel,
			LogFormat:                                 logger.FormatJSON,
			NodeAgentReconciliationMaxDelay:           b.Shoot.OSCSyncJitterPeriod,
			NodeAgentAuthorizerEnabled:                true,
			NodeAgentAuthorizerAuthorizeWithSelectors: new(gardenerutils.IsAuthorizeWithSelectorsEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer)),
			// TODO(shafeeqes): Remove PodTopologySpreadConstraints webhook once the
			// MatchLabelKeysInPodTopologySpread feature gate is locked to true.
			PodTopologySpreadConstraintsEnabled: gardenerutils.IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(b.Shoot.GetInfo()),
			PriorityClassName:                   v1beta1constants.PriorityClassNameShootControlPlane400,
			RuntimeKubernetesVersion:            b.Seed.KubernetesVersion,
			SchedulingProfile:                   v1beta1helper.ShootSchedulingProfile(b.Shoot.GetInfo()),
			SecretNameServerCA:                  v1beta1constants.SecretNameCACluster,
			SystemComponentTolerations:          gardenerutils.ExtractSystemComponentsTolerations(b.Shoot.GetInfo().Spec.Provider.Workers),
			TargetNamespaces:                    []string{metav1.NamespaceSystem, v1beta1constants.KubernetesDashboardNamespace, corev1.NamespaceNodeLease},
			TopologyAwareRoutingEnabled:         b.Shoot.TopologyAwareRoutingEnabled,
			VPAInPlaceUpdatesEnabled:            true,
		}
	)

	if b.Shoot.HasManagedInfrastructure() {
		values.MachineNamespace = new(b.Shoot.ControlPlaneNamespace)
	}

	if b.Shoot.IsSelfHosted() {
		values.KubernetesServiceHost = nil
		// Disable the vpa-in-place-updates webhook as there are no VPA components that manage VPA resources and
		// there is no reason for the GRM webhook to be deployed.
		//
		// GRM's vpa-in-place-updates webhook is planned to be removed soon in favor of setting the update mode to InPlaceOrRecreate explicitly.
		// For more details, see https://github.com/gardener/gardener/issues/12955.
		values.VPAInPlaceUpdatesEnabled = false

		if !b.Shoot.RunsControlPlane() {
			newFunc = shared.NewRuntimeGardenerResourceManager
			values.HighAvailabilityConfigWebhookEnabled = false
			values.PriorityClassName = v1beta1constants.PriorityClassNameSeedSystemCritical
			// When GRM does not run inside the self-hosted shoot cluster (gardenadm bootstrap case), we remove the `node-role.kubernetes.io/control-plane` toleration
			// as it is not required.
			values.SystemComponentTolerations = slices.DeleteFunc(values.SystemComponentTolerations, func(t corev1.Toleration) bool {
				return t.Key == "node-role.kubernetes.io/control-plane"
			})
		}
	}

	return newFunc(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.SecretsManager, values)
}

// DefaultRuntimeGardenerResourceManager returns the gardener-resource-manager component for deploying it to the garden
// namespace (self-hosted shoot scenario).
func (b *Botanist) DefaultRuntimeGardenerResourceManager() (resourcemanager.Interface, error) {
	return shared.NewRuntimeGardenerResourceManager(b.SeedClientSet.Client(), v1beta1constants.GardenNamespace, b.SecretsManager, resourcemanager.Values{
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
		// Furthermore, upon invocation, the GRM's /webhooks/vpa-in-place-updates endpoint,
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

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	return shared.DeployGardenerResourceManager(
		ctx,
		b.SeedClientSet.Client(),
		b.Clock,
		b.SecretsManager,
		b.Shoot.Components.ControlPlane.ResourceManager,
		b.Shoot.ControlPlaneNamespace,
		func(ctx context.Context) (int32, error) {
			return b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameGardenerResourceManager, 2)
		},
		func() string { return b.Shoot.ComputeInClusterAPIServerAddress(true) })
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameGardenerResourceManager}, 1)
}
