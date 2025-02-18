// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	clusterblackboxexporter "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot/cluster"
	controlplaneblackboxexporter "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot/controlplane"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultBlackboxExporterControlPlane returns a deployer for the blackbox-exporter.
func (b *Botanist) DefaultBlackboxExporterControlPlane() (component.DeployWaiter, error) {
	return sharedcomponent.NewBlackboxExporter(
		b.SeedClientSet.Client(),
		b.SecretsManager,
		b.Shoot.ControlPlaneNamespace,
		blackboxexporter.Values{
			ClusterType:       component.ClusterTypeSeed,
			VPAEnabled:        true,
			KubernetesVersion: b.Seed.KubernetesVersion,
			PodLabels: map[string]string{
				v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				// The control plane blackbox-exporter is using the internal cluster domain to probe the shoot API server.
				// Traffic to the istio-ingressgateway needs to be allowed because on some infrastructures kube-proxy shortcuts the network path.
				// It directly forwards the traffic to the target within the cluster (i.e., istio-ingressgateway) instead of first going out and then coming in again.
				gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyIstioIngressNamespaceAlias+"-istio-ingressgateway", 9443): v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port):                   v1beta1constants.LabelNetworkPolicyAllowed,
			},
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane100,
			Config:            controlplaneblackboxexporter.Config(),
			ScrapeConfigs:     controlplaneblackboxexporter.ScrapeConfig(b.Shoot.ControlPlaneNamespace, monitoringv1alpha1.Target("https://"+gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain)+"/healthz")),
			Replicas:          b.Shoot.GetReplicas(1),
		},
	)
}

// ReconcileBlackboxExporterControlPlane deploys or destroys the blackbox-exporter component depending on whether shoot
// monitoring is enabled or not.
func (b *Botanist) ReconcileBlackboxExporterControlPlane(ctx context.Context) error {
	if b.Operation.IsShootMonitoringEnabled() {
		return b.Shoot.Components.ControlPlane.BlackboxExporter.Deploy(ctx)
	}

	return b.Shoot.Components.ControlPlane.BlackboxExporter.Destroy(ctx)
}

// DefaultBlackboxExporterCluster returns a deployer for the blackbox-exporter.
func (b *Botanist) DefaultBlackboxExporterCluster() (component.DeployWaiter, error) {
	return sharedcomponent.NewBlackboxExporter(
		b.SeedClientSet.Client(),
		b.SecretsManager,
		b.Shoot.ControlPlaneNamespace,
		blackboxexporter.Values{
			ClusterType:       component.ClusterTypeShoot,
			VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			PodLabels: map[string]string{
				v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			},
			PriorityClassName: "system-cluster-critical",
			Config:            clusterblackboxexporter.Config(),
			ScrapeConfigs:     clusterblackboxexporter.ScrapeConfig(b.Shoot.ControlPlaneNamespace),
			PrometheusRules:   clusterblackboxexporter.PrometheusRule(b.Shoot.ControlPlaneNamespace),
			Replicas:          1,
		},
	)
}

// ReconcileBlackboxExporterCluster deploys or destroys the blackbox-exporter component depending on whether shoot
// monitoring is enabled or not.
func (b *Botanist) ReconcileBlackboxExporterCluster(ctx context.Context) error {
	if b.Operation.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.BlackboxExporter.Deploy(ctx)
	}

	return b.Shoot.Components.SystemComponents.BlackboxExporter.Destroy(ctx)
}
