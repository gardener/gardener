// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DataKeyKubeconfig is the key in a secret data holding the kubeconfig.
const DataKeyKubeconfig = "kubeconfig"

// KubeconfigSecretConfig is configuration for kubeconfig secrets.
type KubeconfigSecretConfig struct {
	Name        string
	ContextName string
	Cluster     clientcmdv1.Cluster
	AuthInfo    clientcmdv1.AuthInfo
	Namespace   string
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
	kubeconfig := kubernetesutils.NewKubeconfig(s.ContextName, s.Cluster, s.AuthInfo)
	kubeconfig.Contexts[0].Context.Namespace = s.Namespace

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
