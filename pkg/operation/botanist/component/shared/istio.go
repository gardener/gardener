// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewIstio returns a deployer for Istio.
func NewIstio(
	cl client.Client,
	imageVector imagevector.ImageVector,
	chartRenderer chartrenderer.Interface,
	namePrefix string,
	ingressNamespace string,
	priorityClassName string,
	istiodEnabled bool,
	labels map[string]string,
	lbAnnotations map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType,
	serviceExternalIP *string,
	servicePorts []corev1.ServicePort,
	proxyProtocolEnabled bool,
	vpnEnabled bool,
	zones []string) (istio.Interface, error) {
	var (
		minReplicas *int
		maxReplicas *int
	)

	istiodImage, err := imageVector.FindImage(images.ImageNameIstioIstiod)
	if err != nil {
		return nil, err
	}

	igwImage, err := imageVector.FindImage(images.ImageNameIstioProxy)
	if err != nil {
		return nil, err
	}

	if len(zones) > 1 {
		// Each availability zone should have at least 2 replicas as on some infrastructures each
		// zonal load balancer is exposed individually via its own IP address. Therefore, having
		// just one replica may negatively affect availability.
		minReplicas = pointer.Int(len(zones) * 2)
		// The default configuration without availability zones has 5 as the maximum amount of
		// replicas, which apparently works in all known Gardener scenarios. Reducing it to less
		// per zone gives some room for autoscaling while it is assumed to never reach the maximum.
		maxReplicas = pointer.Int(len(zones) * 4)
	}

	ingressGatewayNamespace := fmt.Sprintf("%s%s", namePrefix, ingressNamespace)

	defaultIngressGatewayConfig := istio.IngressGatewayValues{
		TrustDomain:           gardencorev1beta1.DefaultDomain,
		Image:                 igwImage.String(),
		IstiodNamespace:       v1beta1constants.IstioSystemNamespace,
		Annotations:           lbAnnotations,
		ExternalTrafficPolicy: externalTrafficPolicy,
		MinReplicas:           minReplicas,
		MaxReplicas:           maxReplicas,
		Ports:                 servicePorts,
		LoadBalancerIP:        serviceExternalIP,
		Labels:                labels,
		Namespace:             ingressGatewayNamespace,
		PriorityClassName:     priorityClassName,
		ProxyProtocolEnabled:  proxyProtocolEnabled,
		VPNEnabled:            vpnEnabled,
	}

	return istio.NewIstio(
		cl,
		chartRenderer,
		istio.Values{
			Istiod: istio.IstiodValues{
				Enabled:           istiodEnabled,
				Image:             istiodImage.String(),
				Namespace:         v1beta1constants.IstioSystemNamespace,
				PriorityClassName: priorityClassName,
				TrustDomain:       gardencorev1beta1.DefaultDomain,
				Zones:             zones,
			},
			IngressGateway: []istio.IngressGatewayValues{
				defaultIngressGatewayConfig,
			},
			NamePrefix: namePrefix,
		},
	), nil
}

// AddIstioIngressGateway adds an Istio ingress gateway to the given deployer. It uses the first Ingress Gateway
// to fill out common chart values. Hence, it is assumed that at least one Ingress Gateway was added to the given
// `istioDeployer` before calling this function.
func AddIstioIngressGateway(
	istioDeployer istio.Interface,
	namespace string,
	annotations map[string]string,
	labels map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType,
	serviceExternalIP *string,
	zone *string) error {
	gatewayValues := istioDeployer.GetValues().IngressGateway
	if len(gatewayValues) < 1 {
		return fmt.Errorf("at least one ingress gateway must be present before adding further ones")
	}

	// Take the first ingress gateway values to create additional gateways
	templateValues := gatewayValues[0]

	var (
		zones       []string
		minReplicas *int
		maxReplicas *int
	)

	if zone == nil {
		minReplicas = templateValues.MinReplicas
		maxReplicas = templateValues.MaxReplicas
	} else {
		zones = []string{*zone}
	}

	istioDeployer.AddIngressGateway(istio.IngressGatewayValues{
		Annotations:           annotations,
		Labels:                labels,
		Namespace:             namespace,
		MinReplicas:           minReplicas,
		MaxReplicas:           maxReplicas,
		ExternalTrafficPolicy: externalTrafficPolicy,
		LoadBalancerIP:        serviceExternalIP,
		Image:                 templateValues.Image,
		IstiodNamespace:       v1beta1constants.IstioSystemNamespace,
		Ports:                 templateValues.Ports,
		PriorityClassName:     templateValues.PriorityClassName,
		ProxyProtocolEnabled:  templateValues.ProxyProtocolEnabled,
		TrustDomain:           gardencorev1beta1.DefaultDomain,
		VPNEnabled:            templateValues.VPNEnabled,
		Zones:                 zones,
	})

	return nil
}
