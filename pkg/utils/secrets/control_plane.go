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

	"github.com/gardener/gardener/pkg/utils/infodata"

	"k8s.io/apimachinery/pkg/runtime"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	// DataKeyKubeconfig is the key in a secret data holding the kubeconfig.
	DataKeyKubeconfig = "kubeconfig"
)

// ControlPlaneSecretDataKeyCertificatePEM returns the data key inside a Secret of type ControlPlane whose value
// contains the certificate PEM.
func ControlPlaneSecretDataKeyCertificatePEM(name string) string { return fmt.Sprintf("%s.crt", name) }

// ControlPlaneSecretDataKeyPrivateKey returns the data key inside a Secret of type ControlPlane whose value
// contains the private key PEM.
func ControlPlaneSecretDataKeyPrivateKey(name string) string { return fmt.Sprintf("%s.key", name) }

// ControlPlaneSecretConfig is a struct which inherits from CertificateSecretConfig and is extended with a couple of additional
// properties. A control plane secret will always contain a server/client certificate and optionally a kubeconfig.
type ControlPlaneSecretConfig struct {
	*CertificateSecretConfig

	BasicAuth *BasicAuth
	Token     *Token

	KubeConfigRequests []KubeConfigRequest
}

// KubeConfigRequest is a struct which holds information about a Kubeconfig to be generated.
type KubeConfigRequest struct {
	ClusterName   string
	APIServerHost string
}

// ControlPlane contains the certificate, and optionally the basic auth. information as well as a Kubeconfig.
type ControlPlane struct {
	Name string

	Certificate *Certificate
	BasicAuth   *BasicAuth
	Token       *Token
	Kubeconfig  []byte
}

// GetName returns the name of the secret.
func (s *ControlPlaneSecretConfig) GetName() string {
	return s.CertificateSecretConfig.Name
}

// Generate implements ConfigInterface.
func (s *ControlPlaneSecretConfig) Generate() (DataInterface, error) {
	return s.GenerateControlPlane()
}

// GenerateInfoData implements ConfigInterface
func (s *ControlPlaneSecretConfig) GenerateInfoData() (infodata.InfoData, error) {
	cert, err := s.CertificateSecretConfig.GenerateCertificate()
	if err != nil {
		return nil, err
	}

	if len(cert.PrivateKeyPEM) == 0 && len(cert.CertificatePEM) == 0 {
		return infodata.EmptyInfoData, nil
	}

	return NewCertificateInfoData(cert.PrivateKeyPEM, cert.CertificatePEM), nil
}

// GenerateFromInfoData implements ConfigInterface
func (s *ControlPlaneSecretConfig) GenerateFromInfoData(infoData infodata.InfoData) (DataInterface, error) {
	data, ok := infoData.(*CertificateInfoData)
	if !ok {
		return nil, fmt.Errorf("could not convert InfoData entry %s to CertificateInfoData", s.Name)
	}

	certificate := &Certificate{
		Name: s.Name,
		CA:   s.SigningCA,

		PrivateKeyPEM:  data.PrivateKey,
		CertificatePEM: data.Certificate,
	}

	controlPlane := &ControlPlane{
		Name: s.Name,

		Certificate: certificate,
		BasicAuth:   s.BasicAuth,
		Token:       s.Token,
	}

	if len(s.KubeConfigRequests) > 0 {
		kubeconfig, err := GenerateKubeconfig(s, certificate)
		if err != nil {
			return nil, err
		}
		controlPlane.Kubeconfig = kubeconfig
	}

	return controlPlane, nil
}

// LoadFromSecretData implements infodata.Loader
func (s *ControlPlaneSecretConfig) LoadFromSecretData(secretData map[string][]byte) (infodata.InfoData, error) {
	privateKeyPEM := secretData[ControlPlaneSecretDataKeyPrivateKey(s.Name)]
	certificatePEM := secretData[ControlPlaneSecretDataKeyCertificatePEM(s.Name)]

	if len(privateKeyPEM) == 0 && len(certificatePEM) == 0 {
		return infodata.EmptyInfoData, nil
	}

	return NewCertificateInfoData(privateKeyPEM, certificatePEM), nil
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
		Token:       s.Token,
	}

	if len(s.KubeConfigRequests) > 0 {
		kubeconfig, err := GenerateKubeconfig(s, certificate)
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
		DataKeyCertificateCA: c.Certificate.CA.CertificatePEM,
	}

	if c.Certificate.CertificatePEM != nil && c.Certificate.PrivateKeyPEM != nil {
		data[ControlPlaneSecretDataKeyPrivateKey(c.Name)] = c.Certificate.PrivateKeyPEM
		data[ControlPlaneSecretDataKeyCertificatePEM(c.Name)] = c.Certificate.CertificatePEM
	}

	if c.BasicAuth != nil {
		data[DataKeyUserName] = []byte(c.BasicAuth.Username)
		data[DataKeyPassword] = []byte(c.BasicAuth.Password)
	}

	if c.Token != nil {
		data[DataKeyToken] = []byte(c.Token.Token)
	}

	if c.Kubeconfig != nil {
		data[DataKeyKubeconfig] = c.Kubeconfig
	}

	return data
}

// GenerateKubeconfig generates a Kubernetes Kubeconfig for communicating with the kube-apiserver by using
// a client certificate. If <basicAuthUser> and <basicAuthPass> are non-empty string, a second user object
// containing the Basic Authentication credentials is added to the Kubeconfig.
func GenerateKubeconfig(secret *ControlPlaneSecretConfig, certificate *Certificate) ([]byte, error) {
	if len(secret.KubeConfigRequests) == 0 {
		return nil, fmt.Errorf("missing kubeconfig request for %q", secret.Name)
	}

	var (
		name                 = secret.KubeConfigRequests[0].ClusterName
		authContextName      string
		authInfos            = []configv1.NamedAuthInfo{}
		tokenContextName     = fmt.Sprintf("%s-token", name)
		basicAuthContextName = fmt.Sprintf("%s-basic-auth", name)
	)

	if certificate.CertificatePEM != nil && certificate.PrivateKeyPEM != nil {
		authContextName = name
	} else if secret.Token != nil {
		authContextName = tokenContextName
	} else if secret.BasicAuth != nil {
		authContextName = basicAuthContextName
	}

	if certificate.CertificatePEM != nil && certificate.PrivateKeyPEM != nil {
		authInfos = append(authInfos, configv1.NamedAuthInfo{
			Name: name,
			AuthInfo: configv1.AuthInfo{
				ClientCertificateData: certificate.CertificatePEM,
				ClientKeyData:         certificate.PrivateKeyPEM,
			},
		})
	}

	if secret.Token != nil {
		authInfos = append(authInfos, configv1.NamedAuthInfo{
			Name: tokenContextName,
			AuthInfo: configv1.AuthInfo{
				Token: secret.Token.Token,
			},
		})
	}

	if secret.BasicAuth != nil {
		authInfos = append(authInfos, configv1.NamedAuthInfo{
			Name: basicAuthContextName,
			AuthInfo: configv1.AuthInfo{
				Username: secret.BasicAuth.Username,
				Password: secret.BasicAuth.Password,
			},
		})
	}

	config := &configv1.Config{
		CurrentContext: name,
		Clusters:       []configv1.NamedCluster{},
		Contexts:       []configv1.NamedContext{},
		AuthInfos:      authInfos,
	}

	for _, req := range secret.KubeConfigRequests {
		config.Clusters = append(config.Clusters, configv1.NamedCluster{
			Name: req.ClusterName,
			Cluster: configv1.Cluster{
				CertificateAuthorityData: certificate.CA.CertificatePEM,
				Server:                   fmt.Sprintf("https://%s", req.APIServerHost),
			},
		})
		config.Contexts = append(config.Contexts, configv1.NamedContext{
			Name: req.ClusterName,
			Context: configv1.Context{
				Cluster:  req.ClusterName,
				AuthInfo: authContextName,
			},
		})
	}

	return runtime.Encode(configlatest.Codec, config)
}
