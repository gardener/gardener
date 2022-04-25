// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secrets

import (
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// DataKeyKubeconfig is the key in a secret data holding the kubeconfig.
const DataKeyKubeconfig = "kubeconfig"

// KubeconfigSecretConfig is configuration for kubeconfig secrets.
type KubeconfigSecretConfig struct {
	Name        string
	ContextName string
	Cluster     clientcmdv1.Cluster
	AuthInfo    clientcmdv1.AuthInfo
}

// Kubeconfig contains the name and the generated kubeconfig.
type Kubeconfig struct {
	Name          string
	Kubeconfig    *clientcmdv1.Config
	kubeconfigRaw []byte
}

// GetName returns the name of the secret.
func (s *KubeconfigSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *KubeconfigSecretConfig) Generate() (DataInterface, error) {
	kubeconfig := kutil.NewKubeconfig(s.ContextName, s.Cluster, s.AuthInfo)

	raw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
	if err != nil {
		return nil, err
	}

	return &Kubeconfig{
		Name:          s.Name,
		Kubeconfig:    kubeconfig,
		kubeconfigRaw: raw,
	}, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (v *Kubeconfig) SecretData() map[string][]byte {
	return map[string][]byte{DataKeyKubeconfig: v.kubeconfigRaw}
}
