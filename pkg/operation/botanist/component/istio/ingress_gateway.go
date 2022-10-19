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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"

	corev1 "k8s.io/api/core/v1"
)

const (
	istioIngressGatewayServicePortNameStatus = "status-port"
)

var (
	//go:embed charts/istio/istio-ingress
	chartIngress     embed.FS
	chartPathIngress = filepath.Join("charts", "istio", "istio-ingress")
)

// IngressGateway is a set of configuration values for the istio-ingress chart.
type IngressGateway struct {
	Values    IngressValues
	Namespace string
}

// IngressValues holds values for the istio-ingress chart.
// The only opened port is 15021.
type IngressValues struct {
	TrustDomain     string            `json:"trustDomain,omitempty"`
	Image           string            `json:"image,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	IstiodNamespace string            `json:"istiodNamespace,omitempty"`
	LoadBalancerIP  *string           `json:"loadBalancerIP,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	// Ports is a list of all Ports the istio-ingress gateways is listening on.
	// Port 15021 and 15000 cannot be used.
	Ports []corev1.ServicePort `json:"ports,omitempty"`
}

func (i *istiod) generateIstioIngressGatewayChart() (*chartrenderer.RenderedChart, error) {
	renderedChart := &chartrenderer.RenderedChart{}

	for _, istioIngressGateway := range i.istioIngressGatewayValues {
		values := map[string]interface{}{
			"trustDomain":       istioIngressGateway.Values.TrustDomain,
			"labels":            istioIngressGateway.Values.Labels,
			"annotations":       istioIngressGateway.Values.Annotations,
			"deployNamespace":   false,
			"priorityClassName": "istio-ingressgateway",
			"ports":             istioIngressGateway.Values.Ports,
			"image":             istioIngressGateway.Values.Image,
			"istiodNamespace":   istioIngressGateway.Values.IstiodNamespace,
			"loadBalancerIP":    istioIngressGateway.Values.LoadBalancerIP,
			"serviceName":       v1beta1constants.DefaultSNIIngressServiceName,
			"portsNames": map[string]interface{}{
				"status": istioIngressGatewayServicePortNameStatus,
			},
		}

		renderedIngressChart, err := i.chartRenderer.RenderEmbeddedFS(chartIngress, chartPathIngress, ManagedResourceControlName, istioIngressGateway.Namespace, values)
		if err != nil {
			return nil, err
		}

		addSuffixToManifestsName(renderedIngressChart, istioIngressGateway.Namespace)

		renderedChart.ChartName = renderedIngressChart.ChartName
		renderedChart.Manifests = append(renderedChart.Manifests, renderedIngressChart.Manifests...)
	}

	return renderedChart, nil
}

func getIngressGatewayNamespaceLabels(labels map[string]string) map[string]string {
	var namespaceLabels = map[string]string{
		"istio-operator-managed": "Reconcile",
		"istio-injection":        "disabled",
	}

	if value, ok := labels[v1beta1constants.GardenRole]; ok && value == v1beta1constants.GardenRoleExposureClassHandler {
		namespaceLabels[v1beta1constants.GardenRole] = v1beta1constants.GardenRoleExposureClassHandler
	}
	if value, ok := labels[v1beta1constants.LabelExposureClassHandlerName]; ok {
		namespaceLabels[v1beta1constants.LabelExposureClassHandlerName] = value
	}

	return namespaceLabels
}

func addSuffixToManifestsName(charts *chartrenderer.RenderedChart, suffix string) {
	for i := 0; i < len(charts.Manifests); i++ {
		charts.Manifests[i].Name = strings.TrimSuffix(charts.Manifests[i].Name, ".yaml")
		charts.Manifests[i].Name = charts.Manifests[i].Name + "/" + suffix + ".yaml"
	}
}
