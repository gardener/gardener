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

package vagrantbotanist

import "github.com/gardener/gardener/pkg/operation/common"

// DeployKube2IAMResources - Not needed on Vagrant
func (b *VagrantBotanist) DeployKube2IAMResources() error {
	return nil
}

// DestroyKube2IAMResources - Not needed on Vagrant.
func (b *VagrantBotanist) DestroyKube2IAMResources() error {
	return nil
}

// GenerateKube2IAMConfig - Not needed on Vagrant.
func (b *VagrantBotanist) GenerateKube2IAMConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, false), nil
}

// GenerateClusterAutoscalerConfig - Not needed on Vagrant.
func (b *VagrantBotanist) GenerateClusterAutoscalerConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, false), nil
}

// GenerateAdmissionControlConfig generates values which are required to render the chart admissions-controls properly.
func (b *VagrantBotanist) GenerateAdmissionControlConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"StorageClasses": []map[string]interface{}{
			{
				"Name":           "default",
				"IsDefaultClass": true,
				"Provisioner":    "k8s.io/minikube-hostpath",
				"Parameters":     map[string]interface{}{},
			},
		},
	}, nil
}

// GenerateNginxIngressConfig generates values which are required to render the chart nginx-ingress properly.
func (b *VagrantBotanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, b.Shoot.Info.Spec.Addons.NginxIngress.Enabled), nil
}
