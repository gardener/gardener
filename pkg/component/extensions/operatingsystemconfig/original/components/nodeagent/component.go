// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeagent

import (
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// UnitContent returns the systemd unit content for the gardener-node-agent unit.
func UnitContent() string {
	return `[Unit]
Description=Gardener Node Agent
After=network.target

[Service]
LimitMEMLOCK=infinity
ExecStart=` + nodeagentv1alpha1.BinaryDir + `/gardener-node-agent --config=` + nodeagentv1alpha1.ConfigFilePath + `
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target`
}
