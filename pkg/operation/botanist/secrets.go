// Copyright 2018 The Gardener Authors.
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

package botanist

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// Secret is a struct which contains a name and is used to be inherited from for more advanced secrets.
// * DoNotApply is a boolean value which can be used to prevent creating the Secret in the Seed cluster.
//   This can be useful to generate secrets which will be used in the Shoot cluster (whose API server
//   might not be available yet).
type Secret struct {
	Name       string
	DoNotApply bool
}

// RSASecret is a struct which inherits from Secret (i.e., it gets a name) and which allows specifying the
// number of bits which should be used for the to-be-created RSA private key. A RSASecret always contains
// the corresponding public key as well.
// * Bits is the number of bits for the RSA private key.
type RSASecret struct {
	Secret
	Bits int
}

// TLSSecret is a struct which inherits from Secret (i.e., it gets a name) and which allows specifying the
// required properties for the to-be-created certificate. It always contains a 2048-bit RSA private key
// and can be either a server of a client certificate.
// * CommonName is the common name used in the certificate.
// * Organization is a list of organizations used in the certificate.
// * DNSNames is a list of DNS names for the Subject Alternate Names list.
// * IPAddresses is a list of IP addresses for the Subject Alternate Names list.
// * IsServerCert specifies whether the certificate should be a server certificate (if not, a client certificate
//   will be created).
type TLSSecret struct {
	Secret
	CommonName   string
	Organization []string
	DNSNames     []string
	IPAddresses  []net.IP
	IsServerCert bool
}

// ControlPlaneSecret is a struct which inherits from TLSSecret and is extended with a couple of additional
// properties. A control plane secret will always contain a client certificate and optionally a kubeconfig.
// * KubeconfigRequired specifies whether a Kubeconfig should be created or not.
// * KubeconfigWithBasicAuth specifies whether the generated Kubeconfig should contain the basic authentication
//   credentials (beneath the client certificate).
// * KubeconfigUseInternalClusterDomain specifies whether the technical load balancer address or the cluster domain
//   should be used in the Kubeconfig.
// * RunsInSeed specifies whether the component using the generated Kubeconfig runs in the Seed cluster (which
//   means it can communicate with the kube-apiserver locally).
type ControlPlaneSecret struct {
	TLSSecret
	KubeconfigRequired                 bool
	KubeconfigWithBasicAuth            bool
	KubeconfigUseInternalClusterDomain bool
	RunsInSeed                         bool
}

// DeploySecrets creates a CA certificate for the Shoot cluster and uses it to sign the server certificate
// used by the kube-apiserver, and all client certificates used for communcation. It also creates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
func (b *Botanist) DeploySecrets() error {
	var (
		name                  string
		err                   error
		secretsMap            = map[string]*corev1.Secret{}
		data                  map[string][]byte
		basicAuthData         map[string]string
		CAPrivateKey          *rsa.PrivateKey
		CACertificateTemplate *x509.Certificate
		CACertificatePEM      []byte
	)

	secrets, err := b.K8sSeedClient.ListSecrets(b.Shoot.SeedNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		secretObj := secret
		secretsMap[secret.ObjectMeta.Name] = &secretObj
	}

	// First we have to generate a CA certificate in order to sign the remainining server/client certificates.
	name = "ca"
	if val, ok := secretsMap[name]; ok {
		b.Secrets[name] = val
		CAPrivateKey, err = utils.DecodePrivateKey(val.Data["ca.key"])
		if err != nil {
			return err
		}
		CACertificateTemplate, err = utils.DecodeCertificate(val.Data["ca.crt"])
		if err != nil {
			return err
		}
		CACertificatePEM, err = signCertificate(CACertificateTemplate, CACertificateTemplate, CAPrivateKey, CAPrivateKey)
		if err != nil {
			return err
		}
	} else {
		CAPrivateKey, err = generateRSAPrivateKey(2048)
		if err != nil {
			return err
		}
		CACertificateTemplate = generateCertificateTemplate("kubernetes", nil, nil, nil, true, false)
		CACertificatePEM, err = signCertificate(CACertificateTemplate, CACertificateTemplate, CAPrivateKey, CAPrivateKey)
		if err != nil {
			return err
		}
		data = map[string][]byte{
			"ca.key": utils.EncodePrivateKey(CAPrivateKey),
			"ca.crt": CACertificatePEM,
		}
		b.Secrets[name], err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, name, corev1.SecretTypeOpaque, data, false)
		if err != nil {
			return err
		}
	}

	// Second we create Basic Authentication credentials which will be used for the kube-apiserver as well
	// as the Ingress resources.
	name = "kube-apiserver-basic-auth"
	dataKey := "basic_auth.csv"
	if val, ok := secretsMap[name]; ok {
		b.Secrets[name] = val
		csv := strings.Split(string(val.Data[dataKey]), ",")
		basicAuthData = map[string]string{
			"username": csv[1],
			"password": csv[0],
		}
	} else {
		basicAuthUsername, basicAuthPassword := generateBasicAuthData()
		basicAuthData = map[string]string{
			"username": basicAuthUsername,
			"password": basicAuthPassword,
		}
		data = map[string][]byte{
			dataKey: []byte(fmt.Sprintf("%s,%s,%s,%s", basicAuthPassword, basicAuthUsername, basicAuthUsername, user.SystemPrivilegedGroup)),
		}
		b.Secrets[name], err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, name, corev1.SecretTypeOpaque, data, false)
		if err != nil {
			return err
		}
	}

	// Third we create the cloudprovider secret which contains the credentials for the cloud provider.
	name = "cloudprovider"
	_, err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, name, corev1.SecretTypeOpaque, b.Shoot.Secret.Data, true)
	if err != nil {
		return err
	}
	b.Secrets[name] = b.Shoot.Secret

	// Now we are prepared enough to generate the remaining secrets, i.e. server certificates, client certificates,
	// and SSH key pairs.
	secretList, err := b.generateSecrets()
	if err != nil {
		return err
	}
	var (
		errorChan     = make(chan error)
		secretMapLock = sync.RWMutex{}
	)
	for _, s := range secretList {
		switch secret := s.(type) {
		case RSASecret:
			go func() {
				secretMapLock.Lock()
				defer secretMapLock.Unlock()
				if val, ok := secretsMap[secret.Name]; ok {
					b.Secrets[secret.Name] = val
					err = nil
				} else {
					b.Secrets[secret.Name], err = b.createRSASecret(secret)
				}
				errorChan <- err
			}()
		case TLSSecret:
			go func() {
				secretMapLock.Lock()
				defer secretMapLock.Unlock()
				if val, ok := secretsMap[secret.Name]; ok {
					b.Secrets[secret.Name] = val
					err = nil
				} else {
					b.Secrets[secret.Name], err = b.createTLSSecret(secret, CACertificateTemplate, CAPrivateKey)
				}
				errorChan <- err
			}()
		case ControlPlaneSecret:
			go func() {
				secretMapLock.Lock()
				defer secretMapLock.Unlock()
				if val, ok := secretsMap[secret.Name]; ok {
					b.Secrets[secret.Name] = val
					err = nil
				} else {
					b.Secrets[secret.Name], err = b.createControlPlaneSecret(secret, CACertificatePEM, CACertificateTemplate, CAPrivateKey, basicAuthData)
				}
				errorChan <- err
			}()
		}
	}

	// Check wether an error occurred during the parallel processing of the Secret creation.
	var e []error
	for i := 0; i < len(secretList); i++ {
		select {
		case err := <-errorChan:
			if err != nil {
				e = append(e, err)
			}
		}
	}
	if len(e) > 0 {
		return fmt.Errorf("Errors occurred during secret generation: %+v", e)
	}

	// Create kubeconfig and ssh-keypair secrets also in the project namespace in the Garden cluster
	for key, value := range map[string]string{"kubeconfig": "kubecfg", "ssh-keypair": "ssh-keypair"} {
		_, err = b.K8sGardenClient.CreateSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, key), corev1.SecretTypeOpaque, b.Secrets[value].Data, true)
		if err != nil {
			return err
		}
	}

	return b.computeSecretsCheckSums()
}

// DeleteGardenSecrets deletes the Shoot-specific secrets from the project namespace in the Garden cluster.
// TODO: Switch to putting an ownerReference of the Shoot into the Secret's metadata once garbage collection works properly.
func (b *Botanist) DeleteGardenSecrets() error {
	err := b.K8sGardenClient.DeleteSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, "kubeconfig"))
	if apierrors.IsNotFound(err) {
		return nil
	}
	err = b.K8sGardenClient.DeleteSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, "ssh-keypair"))
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// createRSASecret takes a RSASecret object, and it generates a new RSA private key using the specified
// number of bits. It also computes the corresponding public key. The computed secrets will be created
// as a Secret object in the Seed cluster and the created Secret object will be returned.
func (b *Botanist) createRSASecret(secret RSASecret) (*corev1.Secret, error) {
	privateKey, err := generateRSAPrivateKey(secret.Bits)
	if err != nil {
		return nil, err
	}
	publicKey, err := generateRSAPublicKey(privateKey)
	if err != nil {
		return nil, err
	}
	publicKey = append(publicKey, []byte(" "+secret.Name)...)
	data := map[string][]byte{
		"id_rsa":     utils.EncodePrivateKey(privateKey),
		"id_rsa.pub": publicKey,
	}

	if secret.DoNotApply {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}, nil
	}
	return b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, secret.Name, corev1.SecretTypeOpaque, data, false)
}

// createTLSSecret takes a TLSSecret object, the CA certificate template and the CA private key, and it
// generates a new 2048-bit RSA private key along with a X509 certificate which will be signed by the given
// CA. The computed secrets will be created as a Secret object in the Seed cluster and the created Secret
// object will be returned.
func (b *Botanist) createTLSSecret(secret TLSSecret, CACertificateTemplate *x509.Certificate, CAPrivateKey *rsa.PrivateKey) (*corev1.Secret, error) {
	privateKeyPEM, certificatePEM, err := generateCertificate(secret, CACertificateTemplate, CAPrivateKey)
	if err != nil {
		return nil, err
	}
	data := map[string][]byte{
		"tls.key": privateKeyPEM,
		"tls.crt": certificatePEM,
	}

	if secret.DoNotApply {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: data,
		}, nil
	}
	return b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, secret.Name, corev1.SecretTypeTLS, data, false)
}

// createControlPlaneSecret takes a ControlPlaneSecret object, the CA certificate template and the CA private key,
// the PEM-encoded CA certificate and the basic authentication credentials, and it generates a new 2048-bit RSA
// private key along with a X509 certificate which will be signed by the given CA. Moreover, depending on the settings
// on the secret object, a Kubeconfig with the basic authentication credentials will be created. The computed secrets
// will be created as a Secret object in the Seed cluster and the created Secret object will be returned.
func (b *Botanist) createControlPlaneSecret(secret ControlPlaneSecret, CACertificatePEM []byte, CACertificateTemplate *x509.Certificate, CAPrivateKey *rsa.PrivateKey, basicAuthData map[string]string) (*corev1.Secret, error) {
	if secret.Name == "kube-apiserver" {
		secret.IPAddresses, secret.DNSNames = b.appendLoadBalancerIngresses(secret.IPAddresses, secret.DNSNames)
	}
	privateKeyPEM, certificatePEM, err := generateCertificate(secret.TLSSecret, CACertificateTemplate, CAPrivateKey)
	if err != nil {
		return nil, err
	}
	data := map[string][]byte{
		"ca.crt":             CACertificatePEM,
		secret.Name + ".key": privateKeyPEM,
		secret.Name + ".crt": certificatePEM,
	}
	if secret.KubeconfigRequired {
		var (
			basicAuthUser = ""
			basicAuthPass = ""
		)
		if secret.KubeconfigWithBasicAuth {
			basicAuthUser = basicAuthData["username"]
			basicAuthPass = basicAuthData["password"]
			data["username"] = []byte(basicAuthData["username"])
			data["password"] = []byte(basicAuthData["password"])
		}
		apiServerURL := b.computeAPIServerURL(secret.RunsInSeed, secret.KubeconfigUseInternalClusterDomain)
		kubeconfig, err := generateKubeconfig(b.Shoot.SeedNamespace, apiServerURL, utils.EncodeBase64(CACertificatePEM), utils.EncodeBase64(certificatePEM), utils.EncodeBase64(privateKeyPEM), basicAuthUser, basicAuthPass)
		if err != nil {
			return nil, err
		}
		data["kubeconfig"] = kubeconfig
	}

	if secret.DoNotApply {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}, nil
	}
	return b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, secret.Name, corev1.SecretTypeOpaque, data, false)
}

// generateCertificate takes a TLSSecret object, the CA certificate template and the CA private key, and it
// generates a new 2048-bit RSA private key along with a X509 certificate which will be signed by the given
// CA. The private key as well as the certificate will be returned PEM-encoded.
func generateCertificate(secret TLSSecret, CACertificateTemplate *x509.Certificate, CAPrivateKey *rsa.PrivateKey) ([]byte, []byte, error) {
	privateKey, err := generateRSAPrivateKey(2048)
	if err != nil {
		return nil, nil, err
	}
	privateKeyPEM := utils.EncodePrivateKey(privateKey)
	certificateTemplate := generateCertificateTemplate(secret.CommonName, secret.Organization, secret.DNSNames, secret.IPAddresses, false, secret.IsServerCert)
	certificatePEM, err := signCertificate(certificateTemplate, CACertificateTemplate, privateKey, CAPrivateKey)
	if err != nil {
		return nil, nil, err
	}
	return privateKeyPEM, certificatePEM, nil
}

// generateRSAPrivateKey generates a RSA private for the given number of <bits>.
func generateRSAPrivateKey(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// generateRSAPublicKey takes a RSA private key <privateKey> and generates the corresponding public key.
// It serializes the public key for inclusion in an OpenSSH `authorized_keys` file and it trims the new-
// line at the end.
func generateRSAPublicKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicKey := ssh.MarshalAuthorizedKey(pubKey)
	return bytes.Trim(publicKey, "\x0a"), nil
}

// generateCertificateTemplate creates a X509 Certificate object based on the provided information regarding
// common name, organization, SANs (DNS names and IP addresses). It can create a server certificate if the
// <isServerCert> flag is true, otherwise it creates client certificate. If <isCACert> is true, then a CA
// certificate is being created. The certificates a valid for 10 years.
func generateCertificateTemplate(commonName string, organization, dnsNames []string, ipAddresses []net.IP, isCA, isServerCert bool) *x509.Certificate {
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		IsCA: isCA,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // + 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		if isServerCert {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		} else {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
	}
	return template
}

// signCertificate takes a <certificateTemplate> and a <certificateTemplateSigner> which is used to sign
// the first. It also requires the corresponding private keys of both certificates. The created certificate
// is returned as byte slice.
func signCertificate(certificateTemplate, certificateTemplateSigner *x509.Certificate, privateKey, privateKeySigner *rsa.PrivateKey) ([]byte, error) {
	certificate, err := x509.CreateCertificate(rand.Reader, certificateTemplate, certificateTemplateSigner, &privateKey.PublicKey, privateKeySigner)
	if err != nil {
		return nil, err
	}
	return utils.EncodeCertificate(certificate), nil
}

// generateBasicAuthData computes a username/password keypair. It uses "admin" as username and generates a
// random password of length 32.
func generateBasicAuthData() (string, string) {
	return "admin", utils.GenerateRandomString(32)
}

// generateKubeconfig generates a Kubernetes Kubeconfig for communicating with the kube-apiserver by using
// a client certificate. If <basicAuthUser> and <basicAuthPass> are non-empty string, a second user object
// containing the Basic Authentication credentials is added to the Kubeconfig.
func generateKubeconfig(clusterName, serverURL, caCertificate, clientCertificate, clientKey, basicAuthUser, basicAuthPass string) ([]byte, error) {
	return utils.RenderLocalTemplate(kubeconfigTemplate, map[string]interface{}{
		"APIServerURL":      serverURL,
		"BasicAuthUsername": basicAuthUser,
		"BasicAuthPassword": basicAuthPass,
		"CACertificate":     caCertificate,
		"ClientCertificate": clientCertificate,
		"ClientKey":         clientKey,
		"ClusterName":       clusterName,
	})
}

// appendLoadBalancerIngresses takes a list of IP addresses <ipAddresses> and a list of DNS names <dnsNames>
// and appends all ingresses of the load balancer pointing to the kube-apiserver to the lists.
func (b *Botanist) appendLoadBalancerIngresses(ipAddresses []net.IP, dnsNames []string) ([]net.IP, []string) {
	for _, ingress := range b.APIServerIngresses {
		if ingress.IP != "" {
			ipAddresses = append([]net.IP{net.ParseIP(ingress.IP)}, ipAddresses...)
		} else if ingress.Hostname != "" {
			dnsNames = append([]string{ingress.Hostname}, dnsNames...)
		} else {
			b.Logger.Warn("Could not add kube-apiserver ingress to the certificate's SANs because it does neither contain an IP nor a hostname.")
		}
	}
	return ipAddresses, dnsNames
}

// computeAPIServerURL takes a boolean value identifying whether the component connecting to the API server
// runs in the Seed cluster <runsInSeed>, and a boolean value <useInternalClusterDomain> which determines whether the
// internal or the external cluster domain should be used.
func (b *Botanist) computeAPIServerURL(runsInSeed, useInternalClusterDomain bool) string {
	if runsInSeed {
		return "kube-apiserver"
	}
	dnsProvider := b.Shoot.Info.Spec.DNS.Provider
	if dnsProvider == gardenv1beta1.DNSUnmanaged || (dnsProvider != gardenv1beta1.DNSUnmanaged && useInternalClusterDomain) {
		return b.Shoot.InternalClusterDomain
	}
	return *(b.Shoot.ExternalClusterDomain)
}

// computeSecretsCheckSums computes sha256 checksums for Secrets or ConfigMaps which will be injected
// into a Pod template (to establish automatic pod restart on changes).
func (b *Botanist) computeSecretsCheckSums() error {
	for name, secret := range b.Secrets {
		jsonString, err := json.Marshal(secret.Data)
		if err != nil {
			return err
		}
		b.CheckSums[name] = utils.ComputeSHA256Hex(jsonString)
	}
	return nil
}

func generateGardenSecretName(shootName, secretName string) string {
	return fmt.Sprintf("%s.%s", shootName, secretName)
}

// generateSecrets returns a list of Secret object types defined above which contain each its specific
// configuration for the creation of certificates (server/client), RSA key pairs, basic authentication
// credentials, etc.
func (b *Botanist) generateSecrets() ([]interface{}, error) {
	alertManagerHost, err := b.Seed.GetIngressFQDN("a", b.Shoot.Info.Name, b.Garden.ProjectName)
	if err != nil {
		return nil, err
	}
	grafanaHost, err := b.Seed.GetIngressFQDN("g", b.Shoot.Info.Name, b.Garden.ProjectName)
	if err != nil {
		return nil, err
	}
	prometheusHost, err := b.Seed.GetIngressFQDN("p", b.Shoot.Info.Name, b.Garden.ProjectName)
	if err != nil {
		return nil, err
	}

	apiServerCertDNSNames := []string{
		fmt.Sprintf("kube-apiserver.%s", b.Shoot.SeedNamespace),
		fmt.Sprintf("kube-apiserver.%s.svc", b.Shoot.SeedNamespace),
		fmt.Sprintf("kube-apiserver.%s.svc.cluster.local", b.Shoot.SeedNamespace),
		"kube-apiserver",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
		b.Shoot.InternalClusterDomain,
	}
	if b.Shoot.ExternalClusterDomain != nil {
		apiServerCertDNSNames = append(apiServerCertDNSNames, *(b.Shoot.Info.Spec.DNS.Domain), *(b.Shoot.ExternalClusterDomain))
	}

	secretList := []interface{}{
		// Secret definition for kube-apiserver
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-apiserver",
				},
				CommonName:   user.APIServerUser,
				Organization: nil,
				DNSNames:     apiServerCertDNSNames,
				IPAddresses: []net.IP{
					net.ParseIP("127.0.0.1"),
					net.ParseIP(common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 1)),
				},
				IsServerCert: true,
			},
			KubeconfigRequired: false,
			RunsInSeed:         true,
		},

		// Secret definition for kube-apiserver to kubelets communication
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-apiserver-kubelet",
				},
				CommonName:   "system:kube-apiserver:kubelet",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: false,
			RunsInSeed:         false,
		},

		// Secret definition for kube-aggregator
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-aggregator",
				},
				CommonName:   "system:kube-aggregator",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: false,
			RunsInSeed:         false,
		},

		// Secret definition for kube-controller-manager
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-controller-manager",
				},
				CommonName:   user.KubeControllerManager,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: true,
			RunsInSeed:         true,
		},

		// Secret definition for kube-scheduler
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-scheduler",
				},
				CommonName:   user.KubeScheduler,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: true,
			RunsInSeed:         true,
		},

		// Secret definition for kube-addon-manager
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-addon-manager",
				},
				CommonName:   "system:kube-addon-manager",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: true,
			RunsInSeed:         true,
		},

		// Secret definition for kube-proxy
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-proxy",
				},
				CommonName:   user.KubeProxy,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired:                 true,
			KubeconfigUseInternalClusterDomain: true,
			RunsInSeed:                         false,
		},

		// Secret definition for kubelet
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kubelet",
				},
				CommonName:   "kubelet",
				Organization: []string{user.NodesGroup},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired:                 true,
			KubeconfigUseInternalClusterDomain: true,
			RunsInSeed:                         false,
		},

		// Secret definition for auto-node-repair
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "auto-node-repair",
				},
				CommonName:   fmt.Sprintf("%s:monitoring:auto-node-repair", garden.GroupName),
				Organization: []string{fmt.Sprintf("%s:monitoring", garden.GroupName)},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: true,
			RunsInSeed:         true,
		},

		// Secret definition for kube-state-metrics
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kube-state-metrics",
				},
				CommonName:   fmt.Sprintf("%s:monitoring:kube-state-metrics", garden.GroupName),
				Organization: []string{fmt.Sprintf("%s:monitoring", garden.GroupName)},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: true,
			RunsInSeed:         true,
		},

		// Secret definition for prometheus
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "prometheus",
				},
				CommonName:   fmt.Sprintf("%s:monitoring:prometheus", garden.GroupName),
				Organization: []string{fmt.Sprintf("%s:monitoring", garden.GroupName)},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired: true,
			RunsInSeed:         true,
		},

		// Secret definition for kubecfg
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "kubecfg",
				},
				CommonName:   "system:cluster-admin",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired:                 true,
			KubeconfigWithBasicAuth:            true,
			KubeconfigUseInternalClusterDomain: false,
			RunsInSeed:                         false,
		},

		// Secret definition for gardener
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: gardenv1beta1.GardenerName,
				},
				CommonName:   gardenv1beta1.GardenerName,
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired:                 true,
			KubeconfigWithBasicAuth:            true,
			KubeconfigUseInternalClusterDomain: true,
			RunsInSeed:                         false,
		},

		// Secret definition for cloud-config-downloader
		ControlPlaneSecret{
			TLSSecret: TLSSecret{
				Secret: Secret{
					Name: "cloud-config-downloader",
				},
				CommonName:   "cloud-config-downloader",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,
				IsServerCert: false,
			},
			KubeconfigRequired:                 true,
			KubeconfigWithBasicAuth:            false,
			KubeconfigUseInternalClusterDomain: true,
			RunsInSeed:                         false,
		},

		// Secret definition for ssh-keypair
		RSASecret{
			Secret: Secret{
				Name: "ssh-keypair",
			},
			Bits: 4096,
		},

		// Secret definition for vpn-ssh-keypair
		RSASecret{
			Secret: Secret{
				Name: "vpn-ssh-keypair",
			},
			Bits: 4096,
		},

		// Secret definition for alertmanager (ingress)
		TLSSecret{
			Secret: Secret{
				Name: "alertmanager-tls",
			},
			CommonName:   alertManagerHost,
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{alertManagerHost},
			IPAddresses:  nil,
			IsServerCert: true,
		},

		// Secret definition for grafana (ingress)
		TLSSecret{
			Secret: Secret{
				Name: "grafana-tls",
			},
			CommonName:   grafanaHost,
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{grafanaHost},
			IPAddresses:  nil,
			IsServerCert: true,
		},

		// Secret definition for prometheus (ingress)
		TLSSecret{
			Secret: Secret{
				Name: "prometheus-tls",
			},
			CommonName:   prometheusHost,
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{prometheusHost},
			IPAddresses:  nil,
			IsServerCert: true,
		},
	}

	if b.Shoot.Info.Spec.Addons.Monocular.Enabled && b.Shoot.Info.Spec.DNS.Domain != nil {
		monocularHost, err := b.Shoot.GetIngressFQDN("monocular")
		if err != nil {
			return nil, err
		}
		secretList = append(secretList, TLSSecret{
			Secret: Secret{
				Name:       "monocular-tls",
				DoNotApply: true,
			},
			CommonName:   monocularHost,
			Organization: nil,
			DNSNames:     []string{monocularHost},
			IPAddresses:  nil,
			IsServerCert: true,
		})
	}

	return secretList, nil
}

const kubeconfigTemplate = `---
apiVersion: v1
kind: Config
current-context: {{.ClusterName}}
clusters:
- name: {{.ClusterName}}
  cluster:
    certificate-authority-data: {{.CACertificate}}
    server: https://{{.APIServerURL}}
contexts:
- name: {{.ClusterName}}
  context:
    cluster: {{.ClusterName}}
    user: {{.ClusterName}}
users:
- name: {{.ClusterName}}
  user:
    client-certificate-data: {{.ClientCertificate}}
    client-key-data: {{.ClientKey}}
{{- if and (gt (len .BasicAuthUsername) 0) (gt (len .BasicAuthPassword) 0)}}
- name: {{.ClusterName}}-basic-auth
  user:
    username: {{.BasicAuthUsername}}
    password: {{.BasicAuthPassword}}
{{- end}}`
