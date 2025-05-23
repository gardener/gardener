// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ImageVector is an alias for imagevector.Containers(). Exposed for testing.
var ImageVector = imagevector.Containers()

// NewIstio returns a deployer for Istio.
func NewIstio(
	ctx context.Context,
	cl client.Client,
	chartRenderer chartrenderer.Interface,
	namePrefix string,
	ingressNamespace string,
	priorityClassName string,
	istiodEnabled bool,
	labels map[string]string,
	toKubeAPIServerPolicyLabel string,
	lbAnnotations map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicy,
	serviceExternalIP *string,
	servicePorts []corev1.ServicePort,
	proxyProtocolEnabled bool,
	terminateLoadBalancerProxyProtocol *bool,
	vpnEnabled bool,
	zones []string,
	dualStack bool,
) (
	istio.Interface,
	error,
) {
	var (
		minReplicas *int
		maxReplicas *int
	)

	istiodImage, err := ImageVector.FindImage(imagevector.ContainerImageNameIstioIstiod)
	if err != nil {
		return nil, err
	}

	igwImage, err := ImageVector.FindImage(imagevector.ContainerImageNameIstioProxy)
	if err != nil {
		return nil, err
	}

	if len(zones) > 1 {
		// Each availability zone should have at least 2 replicas as on some infrastructures each
		// zonal load balancer is exposed individually via its own IP address. Therefore, having
		// just one replica may negatively affect availability.
		minReplicas = ptr.To(len(zones) * 2)
		// The default configuration without availability zones has 9 as the maximum amount of
		// replicas, which apparently works in all known Gardener scenarios. Reducing it to less
		// per zone gives some room for autoscaling while it is assumed to never reach the maximum.
		maxReplicas = ptr.To(len(zones) * 6)
	}

	policyLabels := commonIstioIngressNetworkPolicyLabels(vpnEnabled)
	policyLabels[toKubeAPIServerPolicyLabel] = v1beta1constants.LabelNetworkPolicyAllowed
	// In case the cluster's API server should be exposed via ingress domain for the dashboard terminal scenario,
	// istio ingress gateway needs to be able to directly forward traffic to the runtime API server.
	policyLabels[v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer] = v1beta1constants.LabelNetworkPolicyAllowed

	enforceSpreadAcrossHosts, err := ShouldEnforceSpreadAcrossHosts(ctx, cl, zones)
	if err != nil {
		return nil, err
	}

	defaultIngressGatewayConfig := istio.IngressGatewayValues{
		TrustDomain:                        gardencorev1beta1.DefaultDomain,
		Image:                              igwImage.String(),
		IstiodNamespace:                    v1beta1constants.IstioSystemNamespace,
		Annotations:                        lbAnnotations,
		ExternalTrafficPolicy:              externalTrafficPolicy,
		MinReplicas:                        minReplicas,
		MaxReplicas:                        maxReplicas,
		Ports:                              servicePorts,
		LoadBalancerIP:                     serviceExternalIP,
		Labels:                             labels,
		NetworkPolicyLabels:                policyLabels,
		Namespace:                          namePrefix + ingressNamespace,
		PriorityClassName:                  priorityClassName,
		ProxyProtocolEnabled:               proxyProtocolEnabled,
		TerminateLoadBalancerProxyProtocol: ptr.Deref(terminateLoadBalancerProxyProtocol, false),
		VPNEnabled:                         vpnEnabled,
		DualStack:                          dualStack,
		EnforceSpreadAcrossHosts:           enforceSpreadAcrossHosts,
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
				DualStack:         dualStack,
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
	ctx context.Context,
	cl client.Client,
	istioDeployer istio.Interface,
	namespace string,
	annotations map[string]string,
	labels map[string]string,
	externalTrafficPolicy *corev1.ServiceExternalTrafficPolicy,
	serviceExternalIP *string,
	zone *string,
	dualStack bool,
	terminateLoadBalancerProxyProtocol *bool,
) error {
	gatewayValues := istioDeployer.GetValues().IngressGateway
	if len(gatewayValues) < 1 {
		return errors.New("at least one ingress gateway must be present before adding further ones")
	}

	// Take the first ingress gateway values to create additional gateways
	templateValues := gatewayValues[0]

	var (
		zones                    []string
		minReplicas              *int
		maxReplicas              *int
		enforceSpreadAcrossHosts bool
		err                      error
	)

	if zone == nil {
		minReplicas = templateValues.MinReplicas
		maxReplicas = templateValues.MaxReplicas
		enforceSpreadAcrossHosts = templateValues.EnforceSpreadAcrossHosts
	} else {
		zones = []string{*zone}

		enforceSpreadAcrossHosts, err = ShouldEnforceSpreadAcrossHosts(ctx, cl, []string{*zone})
		if err != nil {
			return err
		}
	}

	istioDeployer.AddIngressGateway(istio.IngressGatewayValues{
		Annotations:                        utils.MergeStringMaps(annotations),
		Labels:                             utils.MergeStringMaps(labels),
		NetworkPolicyLabels:                utils.MergeStringMaps(templateValues.NetworkPolicyLabels),
		Namespace:                          namespace,
		MinReplicas:                        minReplicas,
		MaxReplicas:                        maxReplicas,
		ExternalTrafficPolicy:              externalTrafficPolicy,
		LoadBalancerIP:                     serviceExternalIP,
		Image:                              templateValues.Image,
		IstiodNamespace:                    v1beta1constants.IstioSystemNamespace,
		Ports:                              templateValues.Ports,
		PriorityClassName:                  templateValues.PriorityClassName,
		ProxyProtocolEnabled:               templateValues.ProxyProtocolEnabled,
		TerminateLoadBalancerProxyProtocol: ptr.Deref(terminateLoadBalancerProxyProtocol, templateValues.TerminateLoadBalancerProxyProtocol),
		TrustDomain:                        gardencorev1beta1.DefaultDomain,
		VPNEnabled:                         templateValues.VPNEnabled,
		Zones:                              zones,
		DualStack:                          dualStack,
		EnforceSpreadAcrossHosts:           enforceSpreadAcrossHosts,
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

// ShouldEnforceSpreadAcrossHosts checks whether all given zones have at least two nodes so that Istio can be spread across hosts in each zone.
func ShouldEnforceSpreadAcrossHosts(ctx context.Context, cl client.Client, zones []string) (bool, error) {
	// If there are multiple zones, losing multiple Istio ingress replicas on one node is not a big problem since there are also replicas in two other zones.
	// Hence, we do not need to enforce spreading across hosts. This helps to save resources in small HA clusters like the runtime cluster.
	if len(zones) > 1 {
		return false, nil
	}

	const targetNodeCount = 2
	nodeList := &corev1.NodeList{}
	if err := cl.List(ctx, nodeList); err != nil {
		return false, err
	}
	nodesPerZone := make([]int, len(zones))
	zonesIncomplete := len(zones)
forNode:
	for _, node := range nodeList.Items {
		// Skip nodes with taints since Istio pods cannot be scheduled on them.
		if len(node.Spec.Taints) > 0 {
			continue
		}

		nodeZone := node.Labels[corev1.LabelTopologyZone]
		for i, zone := range zones {
			// In theory, this should be an equals check, but cloud provider handle regions/zones differently so that we might end up
			// with a more complete label value on the node compared to what we might get as parameter, i.e. <region>-<zone> vs. <zone>.
			// Hence, we do a best effort match here using only the suffix. If it fails, the spreading is not enforced, which is acceptable.
			if strings.HasSuffix(nodeZone, zone) {
				if nodesPerZone[i] < targetNodeCount {
					nodesPerZone[i] = nodesPerZone[i] + 1
					if nodesPerZone[i] >= targetNodeCount {
						zonesIncomplete = zonesIncomplete - 1
						if zonesIncomplete == 0 {
							break forNode
						}
					}
				}
				break
			}
		}
	}
	return zonesIncomplete == 0 && len(zones) > 0, nil
}

func commonIstioIngressNetworkPolicyLabels(vpnEnabled bool) map[string]string {
	labels := map[string]string{
		v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(v1beta1constants.IstioSystemNamespace+"-"+istio.IstiodServiceName, istio.IstiodPort):                         v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(v1beta1constants.GardenNamespace+"-"+nginxingress.GetServiceName(), nginxingress.ServicePortControllerHttps): v1beta1constants.LabelNetworkPolicyAllowed,
	}
	if vpnEnabled {
		labels[gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-"+v1beta1constants.DeploymentNameVPNSeedServer, vpnseedserver.OpenVPNPort)] = v1beta1constants.LabelNetworkPolicyAllowed

		for i := 0; i < vpnseedserver.HighAvailabilityReplicaCount; i++ {
			labels[gardenerutils.NetworkPolicyLabel(fmt.Sprintf("%s-%s-%d", v1beta1constants.LabelNetworkPolicyShootNamespaceAlias, v1beta1constants.DeploymentNameVPNSeedServer, i), vpnseedserver.OpenVPNPort)] = v1beta1constants.LabelNetworkPolicyAllowed
		}
	}

	return labels
}
