// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio

import (
	"embed"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
)

var (
	//go:embed charts/istio/istio-ingress
	chartIngress     embed.FS
	chartPathIngress = filepath.Join("charts", "istio", "istio-ingress")
)

// IngressGatewayValues holds values for the istio-ingress chart.
// The only opened port is 15021.
type IngressGatewayValues struct {
	Annotations           map[string]string
	Labels                map[string]string
	ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType
	Image                 string
	IstiodNamespace       string
	LoadBalancerIP        *string
	MaxReplicas           *int
	MinReplicas           *int
	Namespace             string
	TrustDomain           string
	ProxyProtocolEnabled  bool
	VPNEnabled            bool
	Zones                 []string

	// Ports is a list of all Ports the istio-ingress gateways is listening on.
	// Port 15021 and 15000 cannot be used.
	Ports []corev1.ServicePort
}

func (i *istiod) generateIstioIngressGatewayChart() (*chartrenderer.RenderedChart, error) {
	renderedChart := &chartrenderer.RenderedChart{}

	for _, istioIngressGateway := range i.values.IngressGateway {
		values := map[string]interface{}{
			"trustDomain":           istioIngressGateway.TrustDomain,
			"labels":                istioIngressGateway.Labels,
			"annotations":           istioIngressGateway.Annotations,
			"externalTrafficPolicy": istioIngressGateway.ExternalTrafficPolicy,
			"deployNamespace":       false,
			"priorityClassName":     "istio-ingressgateway",
			"ports":                 istioIngressGateway.Ports,
			"image":                 istioIngressGateway.Image,
			"istiodNamespace":       istioIngressGateway.IstiodNamespace,
			"loadBalancerIP":        istioIngressGateway.LoadBalancerIP,
			"serviceName":           v1beta1constants.DefaultSNIIngressServiceName,
			"proxyProtocolEnabled":  istioIngressGateway.ProxyProtocolEnabled,
			"vpn": map[string]interface{}{
				"enabled": istioIngressGateway.VPNEnabled,
				// Always pass replicas here since every seed can potentially host shoot clusters with
				// highly available control-planes.
				"highAvailabilityReplicas": vpnseedserver.HighAvailabilityReplicaCount,
			},
		}

		if istioIngressGateway.MinReplicas != nil {
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
