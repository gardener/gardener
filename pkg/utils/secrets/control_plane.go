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

package secrets

import (
	"fmt"

	"github.com/gardener/gardener/pkg/utils"
)

const (
	// DataKeyKubeconfig is the key in a secret data holding the kubeconfig.
	DataKeyKubeconfig = "kubeconfig"
)

// ControlPlaneSecretConfig is a struct which inherits from CertificateSecretConfig and is extended with a couple of additional
// properties. A control plane secret will always contain a server/client certificate and optionally a kubeconfig.
type ControlPlaneSecretConfig struct {
	*CertificateSecretConfig

	BasicAuth *BasicAuth

	KubeConfigRequest *KubeConfigRequest
}

// KubeConfigRequest is a struct which holds information about a Kubeconfig to be generated.
type KubeConfigRequest struct {
	ClusterName  string
	APIServerURL string
}

// ControlPlane contains the certificate, and optionally the basic auth. information as well as a Kubeconfig.
type ControlPlane struct {
	Name string

	Certificate *Certificate
	BasicAuth   *BasicAuth
	Kubeconfig  []byte
}

// GetName returns the name of the secret.
func (s *ControlPlaneSecretConfig) GetName() string {
	return s.CertificateSecretConfig.Name
}

// Generate implements ConfigInterface.
func (s *ControlPlaneSecretConfig) Generate() (Interface, error) {
	return s.GenerateControlPlane()
}

// GenerateControlPlane computes a secret for a control plane component of the clusters managed by Gardener.
// It may include a Kubeconfig.
func (s *ControlPlaneSecretConfig) GenerateControlPlane() (*ControlPlane, error) {
	certificate, err := s.CertificateSecretConfig.GenerateCertificate()
	if err != nil {
		return nil, err
	}

	controlPlane := &ControlPlane{
		Name: s.Name,

		Certificate: certificate,
		BasicAuth:   s.BasicAuth,
	}

	if s.KubeConfigRequest != nil {
		kubeconfig, err := generateKubeconfig(s, certificate)
		if err != nil {
			return nil, err
		}
		controlPlane.Kubeconfig = kubeconfig
	}

	return controlPlane, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (c *ControlPlane) SecretData() map[string][]byte {
	data := map[string][]byte{
		DataKeyCertificateCA:          c.Certificate.CA.CertificatePEM,
		fmt.Sprintf("%s.key", c.Name): c.Certificate.PrivateKeyPEM,
		fmt.Sprintf("%s.crt", c.Name): c.Certificate.CertificatePEM,
	}

	if c.BasicAuth != nil {
		data[DataKeyUserName] = []byte(c.BasicAuth.Username)
		data[DataKeyPassword] = []byte(c.BasicAuth.Password)
	}

	if c.Kubeconfig != nil {
		data[DataKeyKubeconfig] = c.Kubeconfig
	}

	return data
}

// generateKubeconfig generates a Kubernetes Kubeconfig for communicating with the kube-apiserver by using
// a client certificate. If <basicAuthUser> and <basicAuthPass> are non-empty string, a second user object
// containing the Basic Authentication credentials is added to the Kubeconfig.
func generateKubeconfig(secret *ControlPlaneSecretConfig, certificate *Certificate) ([]byte, error) {
	values := map[string]interface{}{
		"APIServerURL": secret.KubeConfigRequest.APIServerURL,

		"CACertificate":     utils.EncodeBase64(secret.CertificateSecretConfig.SigningCA.CertificatePEM),
		"ClientCertificate": utils.EncodeBase64(certificate.CertificatePEM),
		"ClientKey":         utils.EncodeBase64(certificate.PrivateKeyPEM),
		"ClusterName":       secret.KubeConfigRequest.ClusterName,
	}

	if secret.BasicAuth != nil {
		values["BasicAuthUsername"] = secret.BasicAuth.Username
		values["BasicAuthPassword"] = secret.BasicAuth.Password
	}

	return utils.RenderLocalTemplate(kubeconfigTemplate, values)
}

const kubeconfigTemplate = `---
apiVersion: v1
kind: Config
current-context: {{ .ClusterName }}
clusters:
- name: {{ .ClusterName }}
  cluster:
    certificate-authority-data: {{ .CACertificate }}
    server: https://{{ .APIServerURL }}
contexts:
- name: {{ .ClusterName }}
  context:
    cluster: {{ .ClusterName }}
    user: {{ .ClusterName }}
users:
- name: {{ .ClusterName }}
  user:
    client-certificate-data: {{ .ClientCertificate }}
    client-key-data: {{ .ClientKey }}
{{- if and .BasicAuthUsername .BasicAuthPassword }}
- name: {{ .ClusterName }}-basic-auth
  user:
    username: {{ .BasicAuthUsername }}
    password: {{ .BasicAuthPassword }}
{{- end}}`
