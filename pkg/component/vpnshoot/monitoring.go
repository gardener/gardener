// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpnshoot

import (
	"strconv"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
)

const (
	monitoringPrometheusJobName = "tunnel-probe-apiserver-proxy"

	monitoringMetricProbeHttpStatusCode = "probe_http_status_code"
	monitoringMetricProbeSuccess        = "probe_success"

	monitoringAlertingRules = `groups:
- name: vpn.rules
  rules:
  - alert: VPNShootNoPods
    expr: kube_deployment_status_replicas_available{deployment="` + deploymentName + `"} == 0
    for: 30m
    labels:
      service: vpn
      severity: critical
      type: shoot
      visibility: operator
    annotations:
      description: vpn-shoot deployment in Shoot cluster has 0 available pods. VPN won't work.
      summary: VPN Shoot deployment no pods

  - alert: VPNProbeAPIServerProxyFailed
    expr: absent(probe_success{job="tunnel-probe-apiserver-proxy"}) == 1 or probe_success{job="tunnel-probe-apiserver-proxy"} == 0 or probe_http_status_code{job="tunnel-probe-apiserver-proxy"} != 200
    for: 30m
    labels:
      service: vpn-test
      severity: critical
      type: shoot
      visibility: all
    annotations:
      description: The API Server proxy functionality is not working. Probably the vpn connection from an API Server pod to the vpn-shoot endpoint on the Shoot workers does not work.
      summary: API Server Proxy not usable
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricProbeHttpStatusCode,
		monitoringMetricProbeSuccess,
	}

	monitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
honor_labels: false
metrics_path: /probe
params:
  module:
  - http_apiserver
follow_redirects: false
kubernetes_sd_configs:
- role: pod
  namespaces:
    names: [ kube-system ]
  api_server: https://` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserverconstants.Port) + `
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
relabel_configs:
- target_label: type
  replacement: seed
- source_labels: [ __meta_kubernetes_pod_name,__meta_kubernetes_pod_container_name ]
  action: keep
  regex: vpn-shoot-(0|.+-.+);vpn-shoot-init
- source_labels: [__meta_kubernetes_pod_name,__meta_kubernetes_pod_container_name]
  target_label: __param_target
  regex: (.+);(.+)
  replacement: https://` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserverconstants.Port) + `/api/v1/namespaces/kube-system/pods/${1}/log?container=${2}&tailLines=1
  action: replace
- source_labels: [ __param_target ]
  target_label: instance
  action: replace
- target_label: __address__
  replacement: 127.0.0.1:9115
  action: replace
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
`
)

// ScrapeConfigs returns the scrape configurations for vpn-shoot.
func (v *vpnShoot) ScrapeConfigs() ([]string, error) {
	return []string{monitoringScrapeConfig}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (v *vpnShoot) AlertingRules() (map[string]string, error) {
	return map[string]string{"vpn.rules.yaml": monitoringAlertingRules}, nil
}
