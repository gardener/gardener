// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"net"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	corednsconstants "github.com/gardener/gardener/pkg/component/networking/coredns/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultCoreDNS returns a deployer for the CoreDNS.
func (b *Botanist) DefaultCoreDNS() (coredns.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameCoredns, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := coredns.Values{
		// resolve conformance test issue (https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44)
		// before changing
		ClusterDomain:                   gardencorev1beta1.DefaultDomain,
		Image:                           image.String(),
		AutoscalingMode:                 gardencorev1beta1.CoreDNSAutoscalingModeHorizontal,
		SearchPathRewriteCommonSuffixes: getCommonSuffixesForRewriting(b.Shoot.GetInfo().Spec.SystemComponents),
		// Pod/node network CIDRs and cluster IPs are set on deployment to handle dynamic network CIDRs
	}

	if b.ShootUsesDNS() {
		values.APIServerHost = ptr.To(b.outOfClusterAPIServerFQDN())
	}

	if v1beta1helper.IsCoreDNSAutoscalingModeUsed(b.Shoot.GetInfo().Spec.SystemComponents, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional) {
		image, err = imagevector.Containers().FindImage(imagevector.ContainerImageNameClusterProportionalAutoscaler, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
		if err != nil {
			return nil, err
		}
		values.ClusterProportionalAutoscalerImage = image.String()
		values.AutoscalingMode = gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional
		values.WantsVerticalPodAutoscaler = b.Shoot.WantsVerticalPodAutoscaler
	}

	return coredns.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, values), nil
}

// DeployCoreDNS deploys the CoreDNS system component.
func (b *Botanist) DeployCoreDNS(ctx context.Context) error {
	restartedAtAnnotations, err := b.getCoreDNSRestartedAtAnnotations(ctx)
	if err != nil {
		return err
	}

	shootIPFamilies := b.Shoot.GetInfo().Spec.Networking.IPFamilies
	if len(shootIPFamilies) == 0 {
		return fmt.Errorf("no IP families configured in shoot spec")
	}

	if len(b.Shoot.Networks.CoreDNS) == 0 {
		return fmt.Errorf("no CoreDNS cluster IPs available")
	}

	nodesCondition := v1beta1helper.GetCondition(b.Shoot.GetInfo().Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
	dnsCondition := v1beta1helper.GetCondition(b.Shoot.GetInfo().Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
	if (dnsCondition != nil && dnsCondition.Status != gardencorev1beta1.ConditionTrue) || (nodesCondition != nil) {
		b.Shoot.Components.SystemComponents.CoreDNS.SetIPFamilies([]gardencorev1beta1.IPFamily{shootIPFamilies[0]})
		b.Shoot.Components.SystemComponents.CoreDNS.SetClusterIPs([]net.IP{b.Shoot.Networks.CoreDNS[0]})
	} else {
		b.Shoot.Components.SystemComponents.CoreDNS.SetIPFamilies(shootIPFamilies)
		b.Shoot.Components.SystemComponents.CoreDNS.SetClusterIPs(b.Shoot.Networks.CoreDNS)
	}

	b.Shoot.Components.SystemComponents.CoreDNS.SetNodeNetworkCIDRs(b.Shoot.Networks.Nodes)
	b.Shoot.Components.SystemComponents.CoreDNS.SetPodNetworkCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.SystemComponents.CoreDNS.SetPodAnnotations(restartedAtAnnotations)

	return b.Shoot.Components.SystemComponents.CoreDNS.Deploy(ctx)
}

// NowFunc is a function returning the current time.
// Exposed for testing.
var NowFunc = time.Now

func (b *Botanist) getCoreDNSRestartedAtAnnotations(ctx context.Context) (map[string]string, error) {
	const key = "gardener.cloud/restarted-at"

	if controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskRestartCoreAddons) {
		return map[string]string{key: NowFunc().UTC().Format(time.RFC3339)}, nil
	}

	minPodIPCount, err := b.getMinCoreDNSPodIPCount(ctx)
	if err != nil {
		return nil, err
	}

	constraint := v1beta1helper.GetCondition(b.Shoot.GetInfo().Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
	if constraint != nil &&
		constraint.Status == gardencorev1beta1.ConditionProgressing &&
		minPodIPCount < len(b.Shoot.GetInfo().Spec.Networking.IPFamilies) {
		return map[string]string{key: NowFunc().UTC().Format(time.RFC3339)}, nil
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: coredns.DeploymentName, Namespace: metav1.NamespaceSystem}}
	if err := b.ShootClientSet.Client().Get(ctx, client.ObjectKeyFromObject(deployment), deployment); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if val, ok := deployment.Spec.Template.Annotations[key]; ok {
		return map[string]string{key: val}, nil
	}

	return nil, nil
}

func (b *Botanist) getMinCoreDNSPodIPCount(ctx context.Context) (int, error) {
	podList := &corev1.PodList{}
	if err := b.ShootClientSet.Client().List(ctx, podList,
		client.InNamespace(metav1.NamespaceSystem),
		client.MatchingLabels{corednsconstants.LabelKey: corednsconstants.LabelValue}); err != nil {
		return 0, err
	}

	if len(podList.Items) == 0 {
		return 0, nil
	}

	minIPCount := 2
	for _, pod := range podList.Items {
		minIPCount = min(minIPCount, len(pod.Status.PodIPs))
	}

	return minIPCount, nil
}

func getCommonSuffixesForRewriting(systemComponents *gardencorev1beta1.SystemComponents) []string {
	if systemComponents != nil && systemComponents.CoreDNS != nil && systemComponents.CoreDNS.Rewriting != nil {
		return systemComponents.CoreDNS.Rewriting.CommonSuffixes
	}
	return []string{}
}
