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

	"github.com/gardener/gardener/pkg/chartrenderer"
)

var (
	//go:embed charts/istio/istio-proxy-protocol
	chartProxyProtocol     embed.FS
	chartPathProxyProtocol = filepath.Join("charts", "istio", "istio-proxy-protocol")
)

// ProxyProtocol is a set of configuration values for the istio-proxy-protocol chart.
type ProxyProtocol struct {
	Values    ProxyValues
	Namespace string
}

// ProxyValues holds values for the istio-proxy-protocol chart.
type ProxyValues struct {
	Labels map[string]string `json:"labels,omitempty"`
}

func (i *istiod) generateIstioProxyProtocolChart() (*chartrenderer.RenderedChart, error) {
	renderedChart := &chartrenderer.RenderedChart{}

	for _, istioProxyProtocol := range i.istioProxyProtocolValues {
		values := map[string]interface{}{
			"labels": istioProxyProtocol.Values.Labels,
		}

		renderedProxyProtocolChart, err := i.chartRenderer.RenderEmbeddedFS(chartProxyProtocol, chartPathProxyProtocol, ManagedResourceControlName, istioProxyProtocol.Namespace, values)
		if err != nil {
			return nil, err
		}

		addSuffixToManifestsName(renderedProxyProtocolChart, istioProxyProtocol.Namespace)

		renderedChart.ChartName = renderedProxyProtocolChart.ChartName
		renderedChart.Manifests = append(renderedChart.Manifests, renderedProxyProtocolChart.Manifests...)
	}

	return renderedChart, nil
}
