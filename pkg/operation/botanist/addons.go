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
	"context"

	"github.com/gardener/gardener/pkg/operation/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSPurposeIngress is a constant for a DNS record used for the ingress domain name.
const DNSPurposeIngress = "ingress"

// EnsureIngressDNSRecord creates the respective wildcard DNS record for the nginx-ingress-controller.
func (b *Botanist) EnsureIngressDNSRecord(ctx context.Context) error {
	if !b.Shoot.NginxIngressEnabled() || b.Shoot.IsHibernated {
		return b.DestroyIngressDNSRecord(ctx)
	}

	loadBalancerIngress, err := common.GetLoadBalancerIngress(ctx, b.K8sShootClient.Client(), metav1.NamespaceSystem, "addons-nginx-ingress-controller")
	if err != nil {
		return err
	}

	if err := b.waitUntilDNSProviderReady(ctx, DNSPurposeExternal); err != nil {
		return err
	}

	if err := b.deployDNSEntry(ctx, DNSPurposeIngress, b.Shoot.GetIngressFQDN("*"), loadBalancerIngress); err != nil {
		return err
	}

	return b.deleteLegacyTerraformDNSResources(ctx, common.TerraformerPurposeIngressDNSDeprecated)
}

// DestroyIngressDNSRecord destroys the nginx-ingress resources created by Terraform.
func (b *Botanist) DestroyIngressDNSRecord(ctx context.Context) error {
	return b.deleteDNSEntry(ctx, DNSPurposeIngress)
}

// GenerateKubernetesDashboardConfig generates the values which are required to render the chart of
// the kubernetes-dashboard properly.
func (b *Botanist) GenerateKubernetesDashboardConfig() (map[string]interface{}, error) {
	var (
		enabled = b.Shoot.KubernetesDashboardEnabled()
		values  map[string]interface{}
	)

	if enabled && b.Shoot.Info.Spec.Addons.KubernetesDashboard.AuthenticationMode != nil {
		values = map[string]interface{}{
			"authenticationMode": *b.Shoot.Info.Spec.Addons.KubernetesDashboard.AuthenticationMode,
		}
	}

	return common.GenerateAddonConfig(values, enabled), nil
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
