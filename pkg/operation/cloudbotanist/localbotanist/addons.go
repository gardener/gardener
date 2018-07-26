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

package localbotanist

import "github.com/gardener/gardener/pkg/operation/common"

// DeployKube2IAMResources - Not needed on Local.
func (b *LocalBotanist) DeployKube2IAMResources() error {
	return nil
}

// DestroyKube2IAMResources - Not needed on Local.
func (b *LocalBotanist) DestroyKube2IAMResources() error {
	return nil
}

// GenerateKube2IAMConfig - Not needed on Local.
func (b *LocalBotanist) GenerateKube2IAMConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, false), nil
}

// GenerateClusterAutoscalerConfig - Not needed on Local.
func (b *LocalBotanist) GenerateClusterAutoscalerConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, false), nil
}

// GenerateAdmissionControlConfig generates values which are required to render the chart admissions-controls properly.
func (b *LocalBotanist) GenerateAdmissionControlConfig() (map[string]interface{}, error) {
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
func (b *LocalBotanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(nil, b.Shoot.NginxIngressEnabled()), nil
}
