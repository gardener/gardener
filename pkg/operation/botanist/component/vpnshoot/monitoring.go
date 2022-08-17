// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

const (
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
`
)

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (v *vpnShoot) ScrapeConfigs() ([]string, error) {
	return nil, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (v *vpnShoot) AlertingRules() (map[string]string, error) {
	return map[string]string{"vpn.rules.yaml": monitoringAlertingRules}, nil
}
