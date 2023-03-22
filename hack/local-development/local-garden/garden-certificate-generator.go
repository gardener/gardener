// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// usr/bin/env go run $0 "$@"; exit

package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"k8s.io/apiserver/pkg/authentication/user"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	etcdHealthCheckClientCertName   = "kube-etcd-healthcheck-client"
	kubeAPIServerETCDClientCertName = "kube-apiserver-etcd-client"
	frontProxyCertName              = "front-proxy-client"
	kubeAdminConfCertName           = "kubernetes-admin"

	etcdCertNamesKey   = "etcdCertDNSNames"
	etcdIPAddressesKey = "etcdIPAddresses"
	etcdCertNAME       = "kube-etcd"

	etcdPeerCertNamesKey   = "etcdPeerCertDNSNames"
	etcdPeerIPAddressesKey = "etcdPeerIPAddresses"
	etcdPeerCertName       = "kube-etcd-peer"

	kubeAPIServerCertName       = "kube-apiserver"
	kubeAPIServerIPAddressesKey = "kubeAPIServerIPAddresses"
	kubeAPIServerCertNamesKey   = "kubeAPIServerDNSNames"

	gardenerAdmissionControllerName = "gardener.cloud:system:admission-controller"
	gardenerAPIServerName           = "gardener.cloud:system:apiserver"
	gardenerControllerManagerName   = "gardener.cloud:system:controller-manager"
	gardenerSchedulerName           = "gardener.cloud:system:scheduler"

	serviceAccountKey = "sa"
)

type credentialsType string

var (
	certs credentialsType = "certs"
	sas   credentialsType = "sa"
	rsas  credentialsType = "rsa"
)

type certsAndKeys struct {
	certPEM, privateKeyPEM string
	rsaPubPEM, rsaPrivPEM  string
	kubeConfig             string

	credType credentialsType
}

func convertPrivateKeyToPEM(key *rsa.PrivateKey) (string, error) {
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	var privateKeyRowBuffer bytes.Buffer
	if err := pem.Encode(&privateKeyRowBuffer, pemBlock); err != nil {
		return "", err
	}

	return privateKeyRowBuffer.String(), nil
}

func convertPublicKeyToPEM(key rsa.PublicKey) (string, error) {
	asn1Bytes, err := asn1.Marshal(key)
	if err != nil {
		return "", err
	}
	pemBlock := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: asn1Bytes,
	}

	var publicKeyRowBuffer bytes.Buffer
	if err := pem.Encode(&publicKeyRowBuffer, pemBlock); err != nil {
		return "", err
	}
	return publicKeyRowBuffer.String(), nil
}

func createCACertificate() (*secrets.Certificate, error) {
	return (&secrets.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCACluster,
		CommonName: "kubernetes-ca",
		CertType:   secrets.CACert,
	}).GenerateCertificate()
}

func createClusterCertificatesAndKeys(caCertificate *secrets.Certificate) (map[string]certsAndKeys, error) {
	var (
		localhostIP   = net.ParseIP("127.0.0.1")
		localhostName = "localhost"
	)

	CertificatesDNSNames := map[string][]string{
		etcdCertNamesKey: {
			localhostName,
			"etcd",
		},
		etcdPeerCertNamesKey: {
			localhostName,
			"etcd",
		},
		kubeAPIServerCertNamesKey: {
			localhostName,
			"host.docker.internal",
			"kube-apiserver",
			"kubernetes",
			"kubernetes.default,kubernetes.default.svc",
			"kubernetes.default.svc.cluster",
			"kubernetes.default.svc.cluster.local",
		},
	}

	CertificatesIPAdresses := map[string][]net.IP{
		etcdIPAddressesKey: {
			localhostIP,
		},
		etcdPeerIPAddressesKey: {
			localhostIP,
		},
		kubeAPIServerIPAddressesKey: {
			localhostIP,
		},
	}

	secretList := []secrets.ConfigInterface{
		// Secret definition for kube-apiserver
		&secrets.ControlPlaneSecretConfig{
			Name: kubeAPIServerCertName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:  kubeAPIServerCertName,
				DNSNames:    CertificatesDNSNames[kubeAPIServerCertNamesKey],
				IPAddresses: CertificatesIPAdresses[kubeAPIServerIPAddressesKey],
				CertType:    secrets.ServerClientCert,
				SigningCA:   caCertificate,
			},
		},
		// Secret definition for kube-etcd
		&secrets.ControlPlaneSecretConfig{
			Name: etcdCertNAME,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:  etcdCertNAME,
				DNSNames:    CertificatesDNSNames[etcdCertNamesKey],
				IPAddresses: CertificatesIPAdresses[etcdIPAddressesKey],
				CertType:    secrets.ServerClientCert,
				SigningCA:   caCertificate,
			},
		},
		// Secret definition for kube-etcd-peer
		&secrets.ControlPlaneSecretConfig{
			Name: etcdPeerCertName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:  etcdPeerCertName,
				DNSNames:    CertificatesDNSNames[etcdPeerCertNamesKey],
				IPAddresses: CertificatesIPAdresses[etcdPeerIPAddressesKey],
				CertType:    secrets.ServerClientCert,
				SigningCA:   caCertificate,
			},
		},
		// Secret definition for kube-etcd-healthcheck-client
		&secrets.ControlPlaneSecretConfig{
			Name: etcdHealthCheckClientCertName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName: etcdHealthCheckClientCertName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
		},
		// Secret definition for kube-apiserver-etcd-client
		&secrets.ControlPlaneSecretConfig{
			Name: kubeAPIServerETCDClientCertName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   kubeAPIServerETCDClientCertName,
				Organization: []string{user.SystemPrivilegedGroup},
				CertType:     secrets.ClientCert,
				SigningCA:    caCertificate,
			},
		},
		// Secret definition for front-proxy
		&secrets.ControlPlaneSecretConfig{
			Name: frontProxyCertName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName: frontProxyCertName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
		},
		// Secret definition for admin kubeconfig
		&secrets.ControlPlaneSecretConfig{
			Name: "default-admin",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   kubeAdminConfCertName,
				Organization: []string{user.SystemPrivilegedGroup},
				CertType:     secrets.ClientCert,
				SigningCA:    caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
		// Secret definition for kube-controller-manager kubeconfig
		&secrets.ControlPlaneSecretConfig{
			Name: "default-kube-controller-manager",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   user.KubeControllerManager,
				Organization: []string{user.SystemPrivilegedGroup},
				CertType:     secrets.ClientCert,
				SigningCA:    caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "kube-apiserver:2443",
			}},
		},
		// Secret definition for service account (used for kube-apiserver and kube-controller-manager)
		&secrets.RSASecretConfig{
			Name:       serviceAccountKey,
			Bits:       4096,
			UsedForSSH: false,
		},
		// Secret definitions for gardener components
		&secrets.ControlPlaneSecretConfig{
			Name: "gardener-admission-controller",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName: gardenerAdmissionControllerName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
		&secrets.ControlPlaneSecretConfig{
			Name: "gardener-apiserver",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName: gardenerAPIServerName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
		&secrets.ControlPlaneSecretConfig{
			Name: "gardener-controller-manager",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName: gardenerControllerManagerName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
		&secrets.ControlPlaneSecretConfig{
			Name: "gardener-scheduler",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName: gardenerSchedulerName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
	}

	credentials := make(map[string]certsAndKeys)
	for _, s := range secretList {
		obj, err := s.Generate()
		if err != nil {
			return nil, err
		}

		if controlPlaneCredentials, ok := obj.(*secrets.ControlPlane); ok {
			ck := certsAndKeys{}

			ck.certPEM = string(controlPlaneCredentials.Certificate.CertificatePEM)
			ck.privateKeyPEM = string(controlPlaneCredentials.Certificate.PrivateKeyPEM)
			ck.credType = certs
			if controlPlaneCredentials.Kubeconfig != nil {
				ck.kubeConfig = string(controlPlaneCredentials.Kubeconfig)
			}

			credentials[s.GetName()] = ck

		}

		if saCredentials, ok := obj.(*secrets.RSAKeys); ok {
			saPubPEMString, err := convertPublicKeyToPEM(*saCredentials.PublicKey)
			if err != nil {
				return nil, err
			}
			saPrivPEMString, err := convertPrivateKeyToPEM(saCredentials.PrivateKey)
			if err != nil {
				return nil, err
			}

			val := certsAndKeys{
				rsaPubPEM:  saPubPEMString,
				rsaPrivPEM: saPrivPEMString,
				credType:   rsas,
			}

			if saCredentials.Name == "" {
				val.credType = sas
			}

			credentials[s.GetName()] = val
		}
	}

	return credentials, nil
}

func saveToFile(fileName string, key string) error {
	outFile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if _, err = outFile.WriteString(key); err != nil {
		return err
	}

	return nil
}

func main() {
	var keyFilePath, certFilesPath, kubeConfigFilesPath string

	flag.StringVar(&keyFilePath, "keys-path", "./certificates/keys", "path to certs-directory")
	flag.StringVar(&certFilesPath, "certs-path", "./certificates/certs", "path to keys directory")
	flag.StringVar(&kubeConfigFilesPath, "kubeconfigs-path", "./kubeconfigs", "path to kubeconfigs")

	flag.Parse()

	certificate, err := createCACertificate()
	if err != nil {
		fmt.Println(err.Error())
	}

	if err := saveToFile(filepath.Join(certFilesPath, certificate.Name+".crt"), string(certificate.CertificatePEM)); err != nil {
		fmt.Println(err.Error())
	}

	if err := saveToFile(filepath.Join(keyFilePath, certificate.Name+".key"), string(certificate.PrivateKeyPEM)); err != nil {
		fmt.Println(err.Error())
	}
	credentials, err := createClusterCertificatesAndKeys(certificate)
	if err != nil {
		fmt.Println(err.Error())
	}

	for credentialName, credential := range credentials {
		switch {
		case credential.credType == sas:
			if err := saveToFile(filepath.Join(certFilesPath, credentialName+".pub"), credential.rsaPubPEM); err != nil {
				fmt.Println(err.Error())
			}

			if err := saveToFile(filepath.Join(keyFilePath, credentialName+".key"), credential.rsaPrivPEM); err != nil {
				fmt.Println(err.Error())
			}

		case credential.credType == rsas:
			if err := saveToFile(filepath.Join(keyFilePath, credentialName+".key"), credential.rsaPrivPEM); err != nil {
				fmt.Println(err.Error())
			}

		case credential.credType == certs:
			if err := saveToFile(filepath.Join(certFilesPath, credentialName+".crt"), credential.certPEM); err != nil {
				fmt.Println(err.Error())
			}

			if err := saveToFile(filepath.Join(keyFilePath, credentialName+".key"), credential.privateKeyPEM); err != nil {
				fmt.Println(err.Error())
			}

			if len(credential.kubeConfig) > 0 {
				if err := saveToFile(filepath.Join(kubeConfigFilesPath, credentialName+".conf"), credential.kubeConfig); err != nil {
					fmt.Println(err.Error())
				}
			}
		}
	}
}
