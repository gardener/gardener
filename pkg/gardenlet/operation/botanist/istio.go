// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/networking/istiobasicauthserver"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

// DefaultIstio returns a deployer for the Istio installation that runs inside self-hosted shoot clusters.
func (b *Botanist) DefaultIstio(ctx context.Context) (istio.Interface, error) {
	if !b.Shoot.IsSelfHosted() {
		return nil, nil
	}

	// Ownership precedence for istiod: garden > self-hosted-shoot > seed.
	shootIsGarden, err := gardenletutils.ClusterIsGarden(ctx, b.SeedClientSet.Client())
	if err != nil {
		return nil, fmt.Errorf("failed checking whether self-hosted shoot is also the garden cluster: %w", err)
	}

	var (
		istiodEnabled         = !shootIsGarden
		annotations           map[string]string
		externalTrafficPolicy *corev1.ServiceExternalTrafficPolicy
		loadBalancerClass     *string
		proxyProtocol         *bool
		zones                 []string
	)

	if pool := v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo().Spec.Provider.Workers); pool != nil {
		zones = pool.Zones

		if pool.ControlPlane != nil && pool.ControlPlane.LoadBalancerServices != nil {
			lb := pool.ControlPlane.LoadBalancerServices
			annotations = lb.Annotations
			externalTrafficPolicy = lb.ExternalTrafficPolicy
			loadBalancerClass = lb.Class
			if lb.ProxyProtocol != nil {
				proxyProtocol = &lb.ProxyProtocol.Allowed
			}
		}
	}

	return sharedcomponent.NewIstio(
		ctx,
		b.SeedClientSet.Client(),
		b.SeedClientSet.ChartRenderer(),
		// Use a dedicated name prefix so this Istio installation can co-exist with a seed/virtual-garden one.
		"self-hosted-shoot-",
		v1beta1constants.DefaultSNIIngressNamespace,
		v1beta1constants.PriorityClassNameShootControlPlane100,
		istiodEnabled,
		map[string]string{
			istio.DefaultZoneKey: "ingressgateway",
			istio.RoleKey:        istio.RoleShoot,
		},
		[]string{
			gardenerutils.NetworkPolicyLabel(v1beta1constants.GardenNamespace+"-"+v1beta1constants.DeploymentNameIstioBasicAuthServer, istiobasicauthserver.Port),
		},
		annotations,
		loadBalancerClass,
		externalTrafficPolicy,
		nil,
		[]corev1.ServicePort{{Name: "tcp", Port: 443, TargetPort: intstr.FromInt32(9443)}},
		proxyProtocol,
		false,
		false,
		zones,
		len(b.Shoot.GetInfo().Spec.Networking.IPFamilies) == 2,
		b.Shoot.KubernetesVersion,
	)
}

// DeployIstio deploys the Istio installation (istiod + ingress gateway) into a self-hosted shoot.
func (b *Botanist) DeployIstio(ctx context.Context) error {
	if b.Shoot.Components.ControlPlane.Istio == nil {
		return nil
	}
	return b.Shoot.Components.ControlPlane.Istio.Deploy(ctx)
}
