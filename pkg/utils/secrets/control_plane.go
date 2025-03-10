// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// ControlPlaneSecretDataKeyCertificatePEM returns the data key inside a Secret of type ControlPlane whose value
// contains the certificate PEM.
func ControlPlaneSecretDataKeyCertificatePEM(name string) string { return name + ".crt" }

// ControlPlaneSecretDataKeyPrivateKey returns the data key inside a Secret of type ControlPlane whose value
// contains the private key PEM.
func ControlPlaneSecretDataKeyPrivateKey(name string) string { return name + ".key" }

// ControlPlaneSecretConfig is a struct which inherits from CertificateSecretConfig and is extended with a couple of additional
// properties. A control plane secret will always contain a server/client certificate and optionally a kubeconfig.
type ControlPlaneSecretConfig struct {
	Name string

	CertificateSecretConfig *CertificateSecretConfig

	BasicAuth *BasicAuth
	Token     *Token

	KubeConfigRequests []KubeConfigRequest
}

// KubeConfigRequest is a struct which holds information about a Kubeconfig to be generated.
type KubeConfigRequest struct {
	ClusterName   string
	APIServerHost string
	CAData        []byte
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
	return s.Name
}

// Generate implements ConfigInterface.
func (s *ControlPlaneSecretConfig) Generate() (DataInterface, error) {
	var certificate *Certificate

	if s.CertificateSecretConfig != nil {
		s.CertificateSecretConfig.Name = s.Name

		certData, err := s.CertificateSecretConfig.GenerateCertificate()
		if err != nil {
			return nil, err
		}
		certificate = certData
	}

	controlPlane := &ControlPlane{
		Name: s.Name,

		Certificate: certificate,
		BasicAuth:   s.BasicAuth,
		Token:       s.Token,
	}

	if len(s.KubeConfigRequests) > 0 {
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
	data := make(map[string][]byte)

	if c.Certificate != nil {
		if c.Certificate.CA != nil {
			data[DataKeyCertificateCA] = c.Certificate.CA.CertificatePEM
		}

		if c.Certificate.CertificatePEM != nil && c.Certificate.PrivateKeyPEM != nil {
			data[ControlPlaneSecretDataKeyPrivateKey(c.Name)] = c.Certificate.PrivateKeyPEM
			data[ControlPlaneSecretDataKeyCertificatePEM(c.Name)] = c.Certificate.CertificatePEM
		}
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

func generateKubeconfig(secret *ControlPlaneSecretConfig, certificate *Certificate) ([]byte, error) {
	if len(secret.KubeConfigRequests) == 0 {
		return nil, fmt.Errorf("missing kubeconfig request for %q", secret.Name)
	}

	var (
		name                 = secret.KubeConfigRequests[0].ClusterName
		authContextName      string
		authInfos            = []clientcmdv1.NamedAuthInfo{}
		tokenContextName     = fmt.Sprintf("%s-token", name)
		basicAuthContextName = fmt.Sprintf("%s-basic-auth", name)
	)

	if certificate != nil && certificate.CertificatePEM != nil && certificate.PrivateKeyPEM != nil {
		authContextName = name
	} else if secret.Token != nil {
		authContextName = tokenContextName
	} else if secret.BasicAuth != nil {
		authContextName = basicAuthContextName
	}

	if certificate != nil && certificate.CertificatePEM != nil && certificate.PrivateKeyPEM != nil {
		authInfos = append(authInfos, clientcmdv1.NamedAuthInfo{
			Name: name,
			AuthInfo: clientcmdv1.AuthInfo{
				ClientCertificateData: certificate.CertificatePEM,
				ClientKeyData:         certificate.PrivateKeyPEM,
			},
		})
	}

	if secret.Token != nil {
		authInfos = append(authInfos, clientcmdv1.NamedAuthInfo{
			Name: tokenContextName,
			AuthInfo: clientcmdv1.AuthInfo{
				Token: secret.Token.Token,
			},
		})
	}

	if secret.BasicAuth != nil {
		authInfos = append(authInfos, clientcmdv1.NamedAuthInfo{
			Name: basicAuthContextName,
			AuthInfo: clientcmdv1.AuthInfo{
				Username: secret.BasicAuth.Username,
				Password: secret.BasicAuth.Password,
			},
		})
	}

	config := &clientcmdv1.Config{
		CurrentContext: name,
		Clusters:       []clientcmdv1.NamedCluster{},
		Contexts:       []clientcmdv1.NamedContext{},
		AuthInfos:      authInfos,
	}

	for _, req := range secret.KubeConfigRequests {
		config.Clusters = append(config.Clusters, clientcmdv1.NamedCluster{
			Name: req.ClusterName,
			Cluster: clientcmdv1.Cluster{
				CertificateAuthorityData: req.CAData,
				Server:                   fmt.Sprintf("https://%s", req.APIServerHost),
			},
		})
		config.Contexts = append(config.Contexts, clientcmdv1.NamedContext{
			Name: req.ClusterName,
			Context: clientcmdv1.Context{
				Cluster:  req.ClusterName,
				AuthInfo: authContextName,
			},
		})
	}

	return runtime.Encode(clientcmdlatest.Codec, config)
}
