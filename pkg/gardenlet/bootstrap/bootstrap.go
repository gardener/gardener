/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/certificate/bootstrap/bootstrap.go

Modifications Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
*/

package bootstrap

import (
	"context"
	"crypto"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	certificatesv1beta1client "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RequestSeedCertificate will create a certificate signing request for a seed
// (Organization and CommonName for the CSR will be set as expected for seed
// certificates) and send it to API server, then it will watch the object's
// status, once approved by API server, it will return the API server's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error. This is intended for use on seeds (gardenlet).
func RequestSeedCertificate(ctx context.Context, certificateClient certificatesv1beta1client.CertificateSigningRequestInterface, privateKeyData []byte, seedName string) (certData []byte, csrName string, err error) {
	return RequestCertificate(ctx, certificateClient, privateKeyData, "gardener.cloud:system:seed:"+string(seedName), []string{"gardener.cloud:system:seeds"})
}

// RequestCertificate will create a certificate signing request for a given
// organization and common name for the CSR will be set as expected for seed
// certificates) and send it to API server, then it will watch the object's
// status, once approved by API server, it will return the API server's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func RequestCertificate(ctx context.Context, certificateClient certificatesv1beta1client.CertificateSigningRequestInterface, privateKeyData []byte, commonName string, organization []string) (certData []byte, csrName string, err error) {
	subject := &pkix.Name{
		Organization: organization,
		CommonName:   commonName,
	}

	privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, "", fmt.Errorf("invalid private key for certificate request: %v", err)
	}
	csrData, err := certutil.MakeCSR(privateKey, subject, nil, nil)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate certificate request: %v", err)
	}

	usages := []certificatesv1beta1.KeyUsage{
		certificatesv1beta1.UsageDigitalSignature,
		certificatesv1beta1.UsageKeyEncipherment,
		certificatesv1beta1.UsageClientAuth,
	}

	// The Signer interface contains the Public() method to get the public key.
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, "", fmt.Errorf("private key does not implement crypto.Signer")
	}

	name, err := digestedName(signer.Public(), subject, usages)
	if err != nil {
		return nil, "", err
	}

	req, err := csr.RequestCertificate(certificateClient, csrData, name, usages, privateKey)
	if err != nil {
		return nil, "", err
	}

	childCtx, cancel := context.WithTimeout(ctx, 3600*time.Second)
	defer cancel()

	certData, err = csr.WaitForCertificate(childCtx, certificateClient, req)
	if err != nil {
		return nil, "", err
	}

	return certData, req.Name, nil
}

// This digest should include all the relevant pieces of the CSR we care about.
// We can't directly hash the serialized CSR because of random padding that we
// regenerate every loop and we include usages which are not contained in the
// CSR. This needs to be kept up to date as we add new fields to the node
// certificates and with ensureCompatible.
func digestedName(publicKey interface{}, subject *pkix.Name, usages []certificatesv1beta1.KeyUsage) (string, error) {
	hash := sha512.New512_256()

	// Here we make sure two different inputs can't write the same stream
	// to the hash. This delimiter is not in the base64.URLEncoding
	// alphabet so there is no way to have spill over collisions. Without
	// it 'CN:foo,ORG:bar' hashes to the same value as 'CN:foob,ORG:ar'
	const delimiter = '|'
	encode := base64.RawURLEncoding.EncodeToString

	write := func(data []byte) {
		hash.Write([]byte(encode(data)))
		hash.Write([]byte{delimiter})
	}

	publicKeyData, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	write(publicKeyData)

	write([]byte(subject.CommonName))
	for _, v := range subject.Organization {
		write([]byte(v))
	}
	for _, v := range usages {
		write([]byte(v))
	}

	return fmt.Sprintf("seed-csr-%s", encode(hash.Sum(nil))), nil
}

// MarshalKubeconfigWithClientCertificate marshals the kubeconfig derived from the bootstrapping process.
func MarshalKubeconfigWithClientCertificate(config *rest.Config, privateKeyData, certDat []byte) ([]byte, error) {
	return kubeconfigWithAuthInfo(config, &clientcmdapi.AuthInfo{
		ClientCertificateData: certDat,
		ClientKeyData:         privateKeyData,
	})
}

// MarshalKubeconfigWithToken marshals the kubeconfig derived with the given bootstrap token.
func MarshalKubeconfigWithToken(config *rest.Config, token string) ([]byte, error) {
	return kubeconfigWithAuthInfo(config, &clientcmdapi.AuthInfo{
		Token: token,
	})
}

func kubeconfigWithAuthInfo(config *rest.Config, authInfo *clientcmdapi.AuthInfo) ([]byte, error) {
	// Get the CA data from the bootstrap client config.
	caFile, caData := config.CAFile, []byte{}
	if len(caFile) == 0 {
		caData = config.CAData
	}

	return clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"gardenlet": {
			Server:                   config.Host,
			InsecureSkipTLSVerify:    config.Insecure,
			CertificateAuthority:     caFile,
			CertificateAuthorityData: caData,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"gardenlet": authInfo},
		Contexts: map[string]*clientcmdapi.Context{"gardenlet": {
			Cluster:  "gardenlet",
			AuthInfo: "gardenlet",
		}},
		CurrentContext: "gardenlet",
	})
}

// GardenerSeedBootstrapper is a constant for the gardener seed bootstrapper name.
const GardenerSeedBootstrapper = "gardener.cloud:system:seed-bootstrapper"

// BuildBootstrapperName concatenates the gardener seed bootstrapper group with the given name,
// separated by a colon.
func BuildBootstrapperName(name string) string {
	return fmt.Sprintf("%s:%s", GardenerSeedBootstrapper, name)
}

// DeleteBootstrapAuth checks which authentication mechanism was used to request a certificate
// (either a bootstrap token or a service account token was used). If the latter is true then it
// also deletes the corresponding ClusterRoleBinding.
func DeleteBootstrapAuth(ctx context.Context, c client.Client, csrName, seedName string) error {
	csr := &certificatesv1beta1.CertificateSigningRequest{}
	if err := c.Get(ctx, kutil.Key(csrName), csr); err != nil {
		return err
	}

	var resourcesToDelete []runtime.Object

	switch {
	case strings.HasPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix):
		resourcesToDelete = append(resourcesToDelete,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("bootstrap-token-%s", strings.TrimPrefix(csr.Spec.Username, "system:bootstrap:")),
					Namespace: metav1.NamespaceSystem,
				},
			},
		)

	case strings.HasPrefix(csr.Spec.Username, serviceaccount.ServiceAccountUsernamePrefix):
		namespace, name, err := serviceaccount.SplitUsername(csr.Spec.Username)
		if err != nil {
			return err
		}

		resourcesToDelete = append(resourcesToDelete,
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: BuildBootstrapperName(seedName),
				},
			},
		)
	}

	for _, obj := range resourcesToDelete {
		if err := c.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}
