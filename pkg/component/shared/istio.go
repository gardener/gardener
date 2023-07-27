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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ImageVector is an alias for imagevector.ImageVector(). Exposed for testing.
var ImageVector = imagevector.ImageVector()

// NewIstio returns a deployer for Istio.
func NewIstio(
	cl client.Client,
	chartRenderer chartrenderer.Interface,
	namePrefix string,
	ingressNamespace string,
	priorityClassName string,
	istiodEnabled bool,
	labels map[string]string,
	toKubeAPIServerPolicyLabel string,
	lbAnnotations map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType,
	serviceExternalIP *string,
	servicePorts []corev1.ServicePort,
	proxyProtocolEnabled bool,
	vpnEnabled bool,
	zones []string,
) (
	istio.Interface,
	error,
) {
	var (
		minReplicas *int
		maxReplicas *int
	)

	istiodImage, err := ImageVector.FindImage(imagevector.ImageNameIstioIstiod)
	if err != nil {
		return nil, err
	}

	igwImage, err := ImageVector.FindImage(imagevector.ImageNameIstioProxy)
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

	policyLabels := commonIstioIngressNetworkPolicyLabels(vpnEnabled)
	policyLabels[toKubeAPIServerPolicyLabel] = v1beta1constants.LabelNetworkPolicyAllowed

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
		NetworkPolicyLabels:   policyLabels,
		Namespace:             namePrefix + ingressNamespace,
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
	zone *string,
) error {
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
		Annotations:           utils.MergeStringMaps(annotations),
		Labels:                utils.MergeStringMaps(labels),
		NetworkPolicyLabels:   utils.MergeStringMaps(templateValues.NetworkPolicyLabels),
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

// GetIstioNamespaceForZone returns the namespace to use for a given zone.
// In case the zone name is too long the first five characters of the hash of the zone are used as zone identifiers.
func GetIstioNamespaceForZone(defaultNamespace string, zone string) string {
	const format = "%s--%s"
	if ns := fmt.Sprintf(format, defaultNamespace, zone); len(ns) <= validation.DNS1035LabelMaxLength {
		return ns
	}
	// Use the first five characters of the hash of the zone
	hashedZone := utils.ComputeSHA256Hex([]byte(zone))
	return fmt.Sprintf(format, defaultNamespace, hashedZone[:5])
}

const (
	alternativeZoneKey = v1beta1constants.GardenRole
	zoneInfix          = "--zone--"
)

// GetIstioZoneLabels returns the labels to be used for istio with the mandatory zone label set.
func GetIstioZoneLabels(labels map[string]string, zone *string) map[string]string {
	// Use "istio" for the default gateways and v1beta1constants.LabelExposureClassHandlerName for exposure classes
	zonekey := istio.DefaultZoneKey
	zoneValue := "ingressgateway"
	if value, ok := labels[zonekey]; ok {
		zoneValue = value
	} else if value, ok := labels[alternativeZoneKey]; ok {
		zonekey = alternativeZoneKey
		zoneValue = value
	}
	if zone != nil {
		zoneValue = fmt.Sprintf("%s%s%s", zoneValue, zoneInfix, *zone)
	}
	return utils.MergeStringMaps(labels, map[string]string{zonekey: zoneValue})
}

// IsZonalIstioExtension indicates whether the namespace related to the given labels is a zonal istio extension.
// It also returns the zone.
func IsZonalIstioExtension(labels map[string]string) (bool, string) {
	if v, ok := labels[istio.DefaultZoneKey]; ok {
		i := strings.Index(v, zoneInfix)
		if i < 0 {
			return false, ""
		}
		// There should be at least one character before and after the zone infix.
		return i > 0 && i < len(v)-len(zoneInfix), v[i+len(zoneInfix):]
	}
	if _, ok := labels[v1beta1constants.LabelExposureClassHandlerName]; ok {
		if v, ok := labels[alternativeZoneKey]; ok && strings.HasPrefix(v, v1beta1constants.GardenRoleExposureClassHandler) {
			i := strings.Index(v, zoneInfix)
			if i < 0 {
				return false, ""
			}
			// There should be at least v1beta1constants.GardenRoleExposureClassHandler characters before
			// and one after the zone infix.
			return i >= len(v1beta1constants.GardenRoleExposureClassHandler) && i < len(v)-len(zoneInfix), v[i+len(zoneInfix):]
		}
	}
	return false, ""
}

func commonIstioIngressNetworkPolicyLabels(vpnEnabled bool) map[string]string {
	labels := map[string]string{
		v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(v1beta1constants.IstioSystemNamespace+"-"+istio.IstiodServiceName, istio.IstiodPort): v1beta1constants.LabelNetworkPolicyAllowed,
	}
	if vpnEnabled {
		labels[gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-"+v1beta1constants.DeploymentNameVPNSeedServer, vpnseedserver.OpenVPNPort)] = v1beta1constants.LabelNetworkPolicyAllowed
		labels[gardenerutils.NetworkPolicyLabel(v1beta1constants.GardenNamespace+"-"+vpnauthzserver.Name, vpnauthzserver.ServerPort)] = v1beta1constants.LabelNetworkPolicyAllowed

		for i := 0; i < vpnseedserver.HighAvailabilityReplicaCount; i++ {
			labels[gardenerutils.NetworkPolicyLabel(fmt.Sprintf("%s-%s-%d", v1beta1constants.LabelNetworkPolicyShootNamespaceAlias, v1beta1constants.DeploymentNameVPNSeedServer, i), vpnseedserver.OpenVPNPort)] = v1beta1constants.LabelNetworkPolicyAllowed
		}
	}

	return labels
}
