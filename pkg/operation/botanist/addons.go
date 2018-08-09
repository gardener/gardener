// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureIngressDNSRecord creates the respective wildcard DNS record for the nginx-ingress-controller.
func (b *Botanist) EnsureIngressDNSRecord() error {
	if !b.Shoot.NginxIngressEnabled() || b.Shoot.Hibernated {
		return b.DestroyIngressDNSRecord()
	}

	loadBalancerIngress, _, err := common.GetLoadBalancerIngress(b.K8sShootClient, metav1.NamespaceSystem, "addons-nginx-ingress-controller")
	if err != nil {
		return err
	}
	return b.DeployDNSRecord("ingress", b.Shoot.GetIngressFQDN("*"), loadBalancerIngress, false)
}

// DestroyIngressDNSRecord destroys the nginx-ingress resources created by Terraform.
func (b *Botanist) DestroyIngressDNSRecord() error {
	return b.DestroyDNSRecord("ingress", false)
}

// GenerateKubernetesDashboardConfig generates the values which are required to render the chart of
// the kubernetes-dashboard properly.
func (b *Botanist) GenerateKubernetesDashboardConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, b.Shoot.KubernetesDashboardEnabled()), nil
}

// GenerateClusterAutoscalerConfig generates the values which are required to render the chart of
// the cluster-autoscaler properly.
func (b *Botanist) GenerateClusterAutoscalerConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, b.Shoot.ClusterAutoscalerEnabled()), nil
}

// GenerateKubeLegoConfig generates the values which are required to render the chart of
// kube-lego properly.
func (b *Botanist) GenerateKubeLegoConfig() (map[string]interface{}, error) {
	var (
		enabled = b.Shoot.KubeLegoEnabled()
		values  map[string]interface{}
	)

	if enabled {
		values = map[string]interface{}{
			"config": map[string]interface{}{
				"LEGO_EMAIL": b.Shoot.Info.Spec.Addons.KubeLego.Mail,
			},
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// GenerateMonocularConfig generates the values which are required to render the chart of
// monocular properly.
func (b *Botanist) GenerateMonocularConfig() (map[string]interface{}, error) {
	var (
		enabled = b.Shoot.MonocularEnabled()
		values  map[string]interface{}
	)

	if enabled {
		var (
			name          = "monocular-tls"
			monocularHost = b.Shoot.GetIngressFQDN("monocular")
			kubecfgSecret = b.Secrets["kubecfg"]
			basicAuth     = utils.CreateSHA1Secret(kubecfgSecret.Data["username"], kubecfgSecret.Data["password"])
		)

		if _, err := b.K8sShootClient.CreateSecret(metav1.NamespaceSystem, name, corev1.SecretTypeTLS, b.Secrets[name].Data, true); err != nil {
			return nil, err
		}

		values = map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"hosts":           []string{monocularHost},
			},
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// GenerateHeapsterConfig generates the values which are required to render the chart of
// heapster properly.
func (b *Botanist) GenerateHeapsterConfig() (map[string]interface{}, error) {
	var (
		enabled = b.Shoot.HeapsterEnabled()
		values  map[string]interface{}
	)

	if enabled {
		addonManagerLabels := map[string]interface{}{
			"addonmanager.kubernetes.io/mode": "Reconcile",
		}
		values = map[string]interface{}{
			"labels": addonManagerLabels,
			"service": map[string]interface{}{
				"labels": addonManagerLabels,
			},
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
}

// GenerateHelmTillerConfig generates the values which are required to render the chart of
// helm-tiller properly.
func (b *Botanist) GenerateHelmTillerConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, b.Shoot.MonocularEnabled()), nil
}
