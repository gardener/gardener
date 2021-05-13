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
)

type certsAndKeys struct {
	certPEM, privateKeyPEM string
	saPubPEM, saPrivPEM    string
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
	caConfig := &secrets.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCACluster,
		CommonName: "kubernetes-ca",
		CertType:   secrets.CACert,
	}
	certificate, err := caConfig.GenerateCertificate()
	if err != nil {
		return nil, err
	}
	return certificate, nil
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
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: kubeAPIServerCertName,

				CommonName:   kubeAPIServerCertName,
				Organization: nil,

				DNSNames:    CertificatesDNSNames[kubeAPIServerCertNamesKey],
				IPAddresses: CertificatesIPAdresses[kubeAPIServerIPAddressesKey],

				CertType:  secrets.ServerClientCert,
				SigningCA: caCertificate,
			},
		},
		// Secret definition for kube-etcd
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: etcdCertNAME,

				CommonName:   etcdCertNAME,
				Organization: nil,

				DNSNames:    CertificatesDNSNames[etcdCertNamesKey],
				IPAddresses: CertificatesIPAdresses[etcdIPAddressesKey],

				CertType:  secrets.ServerClientCert,
				SigningCA: caCertificate,
			},
		},
		// Secret definition for kube-etcd-peer
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: etcdPeerCertName,

				CommonName:   etcdPeerCertName,
				Organization: nil,

				DNSNames:    CertificatesDNSNames[etcdPeerCertNamesKey],
				IPAddresses: CertificatesIPAdresses[etcdPeerIPAddressesKey],

				CertType:  secrets.ServerClientCert,
				SigningCA: caCertificate,
			},
		},
		// Secret definition for kube-etcd-healthcheck-client
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: etcdHealthCheckClientCertName,

				CommonName:   etcdHealthCheckClientCertName,
				Organization: nil,

				CertType:  secrets.ClientCert,
				SigningCA: caCertificate,
			},
		},
		// Secret definition for kube-apiserver-etcd-client
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: kubeAPIServerETCDClientCertName,

				CommonName:   kubeAPIServerETCDClientCertName,
				Organization: []string{user.SystemPrivilegedGroup},

				CertType:  secrets.ClientCert,
				SigningCA: caCertificate,
			},
		},
		// Secret definition for front-proxy
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: frontProxyCertName,

				CommonName:   frontProxyCertName,
				Organization: nil,

				CertType:  secrets.ClientCert,
				SigningCA: caCertificate,
			},
		},
		// Secret definition for admin kubeconfig
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "default-admin",

				CommonName:   kubeAdminConfCertName,
				Organization: []string{user.SystemPrivilegedGroup},

				CertType:  secrets.ClientCert,
				SigningCA: caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
		// Secret definition for kube-controller-manager kubeconfig
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "default-kube-controller-manager",

				CommonName:   user.KubeControllerManager,
				Organization: []string{user.SystemPrivilegedGroup},

				CertType:  secrets.ClientCert,
				SigningCA: caCertificate,
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
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name:       "gardener-admission-controller",
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
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name:       "gardener-apiserver",
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
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name:       "gardener-controller-manager",
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
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name:       "gardener-scheduler",
				CommonName: gardenerSchedulerName,
				CertType:   secrets.ClientCert,
				SigningCA:  caCertificate,
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   "local-garden",
				APIServerHost: "localhost:2443",
			}},
		},
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name:         "gardenlet",
				CommonName:   v1beta1constants.SeedUserNamePrefix + v1beta1constants.SeedUserNameSuffixAmbiguous,
				Organization: []string{v1beta1constants.SeedsGroup},
				CertType:     secrets.ClientCert,
				SigningCA:    caCertificate,
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
			credentials[s.GetName()] = certsAndKeys{
				saPubPEM:  saPubPEMString,
				saPrivPEM: saPrivPEMString,
				credType:  sas,
			}
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
			if err := saveToFile(filepath.Join(certFilesPath, credentialName+".pub"), credential.saPubPEM); err != nil {
				fmt.Println(err.Error())
			}

			if err := saveToFile(filepath.Join(keyFilePath, credentialName+".key"), credential.saPrivPEM); err != nil {
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
