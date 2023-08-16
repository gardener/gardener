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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// NodeAgentBaseDir is the directory on the worker node that contains gardener-node-agent
	// relevant files.
	NodeAgentBaseDir = "/var/lib/gardener-node-agent"
	// NodeAgentConfigPath is the file path on the worker node that contains the configuration
	// of the gardener-node-agent.
	NodeAgentConfigPath = NodeAgentBaseDir + "/configuration.yaml"
	// NodeAgentInitScriptPath is the file path on the worker node that contains the init script
	// of the gardener-node-agent.
	NodeAgentInitScriptPath = NodeAgentBaseDir + "/gardener-node-init.sh"

	// NodeInitUnitName is the name of the gardener-node-init systemd service.
	NodeInitUnitName = "gardener-node-init.service"
	// NodeAgentUnitName is the name of the gardener-node-agent systemd service.
	NodeAgentUnitName = "gardener-node-agent.service"

	// NodeAgentOSCSecretKey is the key inside the gardener-node-agent osc secret to access
	// the encoded osc.
	NodeAgentOSCSecretKey = "gardener-node-agent"
	// NodeAgentOSCOldConfigPath is the file path on the worker node that contains the
	// previous content of the osc
	NodeAgentOSCOldConfigPath = NodeAgentBaseDir + "/previous-osc.yaml"

	// NodeAgentTokenFilePath is the file path on the worker node that contains the shoot access
	// token of the gardener-node-agent.
	NodeAgentTokenFilePath = NodeAgentBaseDir + "/token"
	// NodeAgentTokenSecretName is the name of the secret that contains the shoot access
	// token of the gardener-node-agent.
	NodeAgentTokenSecretName = "gardener-node-agent"
	// NodeAgentTokenSecretKey is the key inside the gardener-node-agent token secret to access
	// the token.
	NodeAgentTokenSecretKey = "token"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeAgentConfiguration defines the configuration for the gardener-node-agent.
type NodeAgentConfiguration struct {
	metav1.TypeMeta

	// APIServer contains the connection configuration for the gardener-node-agent to
	// access the shoot api server.
	APIServer APIServer `json:"apiServer"`

	// SecretName defines the name of the secret in the shoot cluster control plane, which contains
	// the OSC for the gardener-node-agent.
	OSCSecretName string `json:"oscSecretName"`
	// TokenSecretName defines the name of the secret in the shoot cluster control plane, which contains
	// the `kube-apiserver` access token for the gardener-node-agent.
	TokenSecretName string `json:"tokenSecretName"`

	// Image is the container image reference to the gardener-node-agent.
	Image string `json:"image"`
	// HyperkubeImage is the container image reference to the hyperkube containing kubelet.
	HyperkubeImage string `json:"hyperkubeImage"`

	// KubernetesVersion contains the kubernetes version of the kubelet, used for annotating
	// the corresponding node resource with a kubernetes version annotation.
	KubernetesVersion string `json:"kubernetesVersion"`

	// KubeletDataVolumeSize sets the data volume size of an unformatted disk on the worker node,
	// which is used for /var/lib on the worker.
	// +optional
	KubeletDataVolumeSize *int64 `json:"KubeletDataVolumeSize,omitempty"`
}

// APIServer contains the connection configuration for the gardener-node-agent to
type APIServer struct {
	// URL is the url to the api server.
	URL string `json:"url"`
	// CA is the ca certificate for the api server.
	CA string `json:"ca"`
	// BootstrapToken is the initial token to fetch the shoot access token for
	// kubelet and the gardener-node-agent.
	BootstrapToken string `json:"bootstrapToken"`
}
