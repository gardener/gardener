// Copyright 2018 The Gardener Authors.
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

package openstackbotanist

import "github.com/gardener/gardener/pkg/operation/common"

// DeployKube2IAMResources - Not needed on OpenStack
func (b *OpenStackBotanist) DeployKube2IAMResources() error {
	return nil
}

// DestroyKube2IAMResources - Not needed on OpenStack.
func (b *OpenStackBotanist) DestroyKube2IAMResources() error {
	return nil
}

// GenerateKube2IAMConfig - Not needed on OpenStack.
func (b *OpenStackBotanist) GenerateKube2IAMConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, false), nil
}

// GenerateClusterAutoscalerConfig - Not needed on OpenStack.
func (b *OpenStackBotanist) GenerateClusterAutoscalerConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, false), nil
}

// GenerateAdmissionControlConfig generates values which are required to render the chart admissions-controls properly.
func (b *OpenStackBotanist) GenerateAdmissionControlConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"StorageClasses": []map[string]interface{}{
			{
				"Name":           "default",
				"IsDefaultClass": true,
				"Provisioner":    "kubernetes.io/cinder",
				"Parameters": map[string]interface{}{
					"availability": b.Shoot.Info.Spec.Cloud.OpenStack.Zones[0],
					"type":         "default",
				},
			},
		},
	}, nil
}

// GenerateCalicoConfig generates values which are required to render the chart calico properly.
func (b *OpenStackBotanist) GenerateCalicoConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"cloudProvider": b.Shoot.CloudProvider,
		"enabled":       true,
	}, nil
}

// GenerateNginxIngressConfig generates values which are required to render the chart nginx-ingress properly.
func (b *OpenStackBotanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, b.Shoot.Info.Spec.Addons.NginxIngress.Enabled), nil
}
