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

package awsbotanist

import (
	"github.com/gardener/gardener/pkg/operation/common"
)

// GenerateNginxIngressConfig generates values which are required to render the chart nginx-ingress properly.
func (b *AWSBotanist) GenerateNginxIngressConfig() (map[string]interface{}, error) {
	return common.GenerateAddonConfig(map[string]interface{}{
		"controller": map[string]interface{}{
			"config": map[string]interface{}{
				"use-proxy-protocol": "true",
			},
		},
	}, b.Shoot.NginxIngressEnabled()), nil
}

// GenerateVPNShootConfig generate cloud-specific vpn override - nothing unique for aws
func (b *AWSBotanist) GenerateVPNShootConfig() (map[string]interface{}, error) {
	return nil, nil
}
