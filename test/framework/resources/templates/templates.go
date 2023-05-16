// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package templates

const (
	// SimpleLoadDeploymentName is the name of the simple load deployment template
	SimpleLoadDeploymentName = "simple-load-deployment.yaml.tpl"

	// NginxDaemonSetName is the name of the nginx daemonset template
	NginxDaemonSetName = "network-nginx-daemonset.yaml.tpl"

	// GuestbookAppName is the name if the guestbook app deployment template
	GuestbookAppName = "guestbook-app.yaml.tpl"

	// LoggerAppName is the name of the logger app deployment template
	LoggerAppName = "logger-app.yaml.tpl"

	// VPNTunnelDeploymentName is the name of the vpn deployment template
	VPNTunnelDeploymentName = "vpntunnel.yaml.tpl"

	// VPNTunnelCopyDeploymentName is the name of the vpn copy deployment template
	VPNTunnelCopyDeploymentName = "vpntunnel-copy.yaml.tpl"

	// PodAntiAffinityDeploymentName is the name of the pod anti affinity deployment template
	PodAntiAffinityDeploymentName = "pod-anti-affinity-deployment.yaml.tpl"

	// BlockValiValidatingWebhookConfiguration is the name of vali's ValidatingWebhookConfiguration
	BlockValiValidatingWebhookConfiguration = "block-vali-validatingwebhookconfiguration.yaml.tpl"
)
