// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"embed"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	"github.com/gardener/gardener/pkg/features"
)

var (
	//go:embed charts/istio/istio-ingress
	chartIngress     embed.FS
	chartPathIngress = filepath.Join("charts", "istio", "istio-ingress")
)

// IngressGatewayValues holds values for the istio-ingress chart.
// The only opened port is 15021.
type IngressGatewayValues struct {
	Annotations                        map[string]string
	Labels                             map[string]string
	NetworkPolicyLabels                map[string]string
	ExternalTrafficPolicy              *corev1.ServiceExternalTrafficPolicy
	Image                              string
	IstiodNamespace                    string
	LoadBalancerIP                     *string
	MaxReplicas                        *int
	MinReplicas                        *int
	Namespace                          string
	PriorityClassName                  string
	TrustDomain                        string
	ProxyProtocolEnabled               bool
	TerminateLoadBalancerProxyProtocol bool
	VPNEnabled                         bool
	Zones                              []string
	DualStack                          bool
	EnforceSpreadAcrossHosts           bool

	// Ports is a list of all Ports the istio-ingress gateways is listening on.
	// Port 15021 and 15000 cannot be used.
	Ports []corev1.ServicePort
}

func (i *istiod) generateIstioIngressGatewayChart() (*chartrenderer.RenderedChart, error) {
	renderedChart := &chartrenderer.RenderedChart{}

	for _, istioIngressGateway := range i.values.IngressGateway {
		values := map[string]any{
			"trustDomain":                        istioIngressGateway.TrustDomain,
			"labels":                             istioIngressGateway.Labels,
			"networkPolicyLabels":                istioIngressGateway.NetworkPolicyLabels,
			"annotations":                        istioIngressGateway.Annotations,
			"externalTrafficPolicy":              istioIngressGateway.ExternalTrafficPolicy,
			"dualStack":                          istioIngressGateway.DualStack,
			"deployNamespace":                    false,
			"priorityClassName":                  istioIngressGateway.PriorityClassName,
			"ports":                              istioIngressGateway.Ports,
			"image":                              istioIngressGateway.Image,
			"istiodNamespace":                    istioIngressGateway.IstiodNamespace,
			"loadBalancerIP":                     istioIngressGateway.LoadBalancerIP,
			"serviceName":                        v1beta1constants.DefaultSNIIngressServiceName,
			"proxyProtocolEnabled":               istioIngressGateway.ProxyProtocolEnabled,
			"terminateLoadBalancerProxyProtocol": istioIngressGateway.TerminateLoadBalancerProxyProtocol,
			"terminateLoadBalancerAPIServer":     features.DefaultFeatureGate.Enabled(features.IstioTLSTermination),
			"vpn": map[string]any{
				"enabled": istioIngressGateway.VPNEnabled,
			},
			"enforceSpreadAcrossHosts":                  istioIngressGateway.EnforceSpreadAcrossHosts,
			"apiServerRequestHeaderUserName":            kubeapiserverconstants.RequestHeaderUserName,
			"apiServerRequestHeaderGroup":               kubeapiserverconstants.RequestHeaderGroup,
			"apiServerAuthenticationDynamicMetadataKey": apiserverexposure.AuthenticationDynamicMetadataKey,
		}

		if istioIngressGateway.MinReplicas != nil {
			// Apply minReplicas here to deploy the Ingress-Gateway with the intended number of replicas from the beginning (creation).
			// Otherwise, we would need to wait until HPA scales up the deployment which then again can trigger unnecessary rolling updates
			// when additional configuration is added by registered webhooks, e.g. high-availability-config webhook.
			values["replicas"] = *istioIngressGateway.MinReplicas
			values["minReplicas"] = *istioIngressGateway.MinReplicas
		}
		if istioIngressGateway.MaxReplicas != nil {
			values["maxReplicas"] = *istioIngressGateway.MaxReplicas
		}

		renderedIngressChart, err := i.chartRenderer.RenderEmbeddedFS(chartIngress, chartPathIngress, releaseName, istioIngressGateway.Namespace, values)
		if err != nil {
			return nil, err
		}

		addSuffixToManifestsName(renderedIngressChart, istioIngressGateway.Namespace)

		renderedChart.ChartName = renderedIngressChart.ChartName
		renderedChart.Manifests = append(renderedChart.Manifests, renderedIngressChart.Manifests...)
	}

	return renderedChart, nil
}

func addSuffixToManifestsName(charts *chartrenderer.RenderedChart, suffix string) {
	for i := 0; i < len(charts.Manifests); i++ {
		charts.Manifests[i].Name = strings.TrimSuffix(charts.Manifests[i].Name, ".yaml")
		charts.Manifests[i].Name = charts.Manifests[i].Name + "/" + suffix + ".yaml"
	}
}
