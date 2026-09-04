// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	vpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kuberneteshealth "github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// DefaultVPNShoot returns a deployer for the VPNShoot
func (b *Botanist) DefaultVPNShoot() (vpnshoot.Interface, error) {
	if b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted() {
		return nil, nil
	}

	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpnClient, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return vpnshoot.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		vpnshoot.Values{
			Image:             image.String(),
			VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
			VPAUpdateDisabled: b.Shoot.VPNVPAUpdateDisabled,
			ReversedVPN: vpnshoot.ReversedVPNValues{
				Header:     "outbound|1194||" + vpnseedserver.ServiceName + "." + b.Shoot.ControlPlaneNamespace + ".svc.cluster.local",
				Endpoint:   b.outOfClusterAPIServerFQDN(),
				IPFamilies: b.Shoot.GetInfo().Spec.Networking.IPFamilies,
			},
			HighAvailabilityEnabled:              b.Shoot.VPNHighAvailabilityEnabled,
			HighAvailabilityNumberOfSeedServers:  b.Shoot.VPNHighAvailabilityNumberOfSeedServers,
			HighAvailabilityNumberOfShootClients: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
			SeedPodNetwork:                       b.Seed.GetInfo().Spec.Networks.Pods,
			AutoMTU:                              b.Shoot.VPNAutoMTU,
		},
	), nil
}

// DeployVPNShoot deploys the vpn-shoot.
func (b *Botanist) DeployVPNShoot(ctx context.Context) error {
	b.Shoot.Components.SystemComponents.VPNShoot.SetPodNetworkCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.SystemComponents.VPNShoot.SetServiceNetworkCIDRs(b.Shoot.Networks.Services)
	b.Shoot.Components.SystemComponents.VPNShoot.SetNodeNetworkCIDRs(b.Shoot.Networks.Nodes)

	if err := b.Shoot.Components.SystemComponents.VPNShoot.Deploy(ctx); err != nil {
		return err
	}

	return b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
		condition := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort)
		condition = v1beta1helper.UpdatedConditionWithClock(b.Clock, condition, gardencorev1beta1.ConditionTrue, "ShootUsesUnifiedHTTPProxyPort", fmt.Sprintf("Shoot uses http-proxy port %d for VPN", vpnseedserver.HTTPProxyGatewayPort))
		shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, condition)
		return nil
	})
}

// RecoverStaleVPNShootPods deletes not-ready vpn-shoot pods so they get recreated, triggering a fresh CNI
// call. This recovers from stale CNI state that can occur during HA zone updates when pods are scheduled
// during a transient kube-apiserver outage and the CNI plugin never programs their network interfaces.
func (b *Botanist) RecoverStaleVPNShootPods(ctx context.Context) error {
	if !b.Shoot.VPNHighAvailabilityEnabled {
		return nil
	}

	podList := &corev1.PodList{}
	if err := b.ShootClientSet.Client().List(ctx, podList,
		client.InNamespace(metav1.NamespaceSystem),
		client.MatchingLabels{v1beta1constants.LabelApp: "vpn-shoot"},
	); err != nil {
		return fmt.Errorf("failed to list vpn-shoot pods: %w", err)
	}

	b.Logger.Info("Readiness of vpn-shoot timed out, deleting pods to recover from potential stale state", "podCount", len(podList.Items))
	for _, pod := range podList.Items {
		if kuberneteshealth.IsPodReady(&pod) {
			continue
		}
		if err := b.ShootClientSet.Client().Delete(ctx, pod.DeepCopy()); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete vpn-shoot pod %s: %w", pod.Name, err)
		}
		b.Logger.Info("Deleted vpn-shoot pod for network reprogramming", "pod", pod.Name)
	}

	return b.Shoot.Components.SystemComponents.VPNShoot.Wait(ctx)
}
