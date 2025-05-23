// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/component/shared"
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
			DefaultNotReadyToleration:                 defaultNotReadyTolerationSeconds,
			DefaultUnreachableToleration:              defaultUnreachableTolerationSeconds,
			IsWorkerless:                              b.Shoot.IsWorkerless,
			KubernetesServiceHost:                     ptr.To(b.Shoot.ComputeOutOfClusterAPIServerAddress(true)),
			LogLevel:                                  logger.InfoLevel,
			LogFormat:                                 logger.FormatJSON,
			NodeAgentReconciliationMaxDelay:           b.Shoot.OSCSyncJitterPeriod,
			NodeAgentAuthorizerEnabled:                true,
			NodeAgentAuthorizerAuthorizeWithSelectors: ptr.To(gardenerutils.IsAuthorizeWithSelectorsEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer, b.Shoot.KubernetesVersion)),
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
		}
	)

	if b.Shoot.IsAutonomous() {
		values.NodeAgentAuthorizerEnabled = false // TODO(rfranzke): Revisit this once autonomous shoot clusters progress.
		values.KubernetesServiceHost = nil

		if b.Shoot.RunsControlPlane() {
			newFunc = shared.NewCombinedGardenerResourceManager
			values.TargetNamespaces = nil
		} else {
			newFunc = shared.NewRuntimeGardenerResourceManager
			// TODO(timebertt): consider disabling the highavailabilityconfig webhook
			values.PriorityClassName = v1beta1constants.PriorityClassNameSeedSystemCritical
		}
	}

	return newFunc(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.SecretsManager, values)
}

// DeployGardenerResourceManager deploys the gardener-resource-manager
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	return shared.DeployGardenerResourceManager(
		ctx,
		b.SeedClientSet.Client(),
		b.SecretsManager,
		b.Shoot.Components.ControlPlane.ResourceManager,
		b.Shoot.ControlPlaneNamespace,
		func(ctx context.Context) (int32, error) {
			return b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameGardenerResourceManager, 2, false)
		},
		func() string { return b.Shoot.ComputeInClusterAPIServerAddress(true) })
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameGardenerResourceManager}, 1)
}
