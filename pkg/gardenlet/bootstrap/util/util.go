// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package util

import (
	"context"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// GardenerSeedBootstrapper is a constant for the gardener seed bootstrapper name.
	GardenerSeedBootstrapper = "gardener.cloud:system:seed-bootstrapper"
	// DefaultSeedName is the default seed name in case the gardenlet config.SeedConfig is not set
	DefaultSeedName = "<ambiguous>"
	// DedicatedSeedKubeconfig is a constant for the target cluster name when the gardenlet is using a dedicated seed kubeconfig
	DedicatedSeedKubeconfig = "configured in .SeedClientConnection.Kubeconfig"
	// InCluster is a constant for the target cluster name  when the gardenlet is running in a Kubernetes cluster
	// and is using the mounted service account token of that cluster
	InCluster = "in cluster"
)

// BuildBootstrapperName concatenates the gardener seed bootstrapper group with the given name,
// separated by a colon.
func BuildBootstrapperName(name string) string {
	return fmt.Sprintf("%s:%s", GardenerSeedBootstrapper, name)
}

// GetSeedName returns the seed name from the SeedConfig or the default Seed name
func GetSeedName(seedConfig *config.SeedConfig) string {
	if seedConfig != nil {
		return seedConfig.Name
	}
	return constants.SeedUserNameSuffixAmbiguous
}

// GetTargetClusterName returns the target cluster of the gardenlet based on the SeedClientConnection.
// This is either the cluster configured by .SeedClientConnection.Kubeconfig, or when running in Kubernetes,
// the local cluster it is deployed to (by using a mounted service account token)
func GetTargetClusterName(config *config.SeedClientConnection) string {
	if config != nil && len(config.Kubeconfig) != 0 {
		return DedicatedSeedKubeconfig
	}
	return InCluster
}

// GetKubeconfigFromSecret tries to retrieve the kubeconfig bytes using the given client
// returns the kubeconfig or nil if it cannot be found
func GetKubeconfigFromSecret(ctx context.Context, seedClient client.Client, namespace, name string) ([]byte, error) {
	kubeconfigSecret := &corev1.Secret{}
	if err := seedClient.Get(ctx, kutil.Key(namespace, name), kubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	return kubeconfigSecret.Data[kubernetes.KubeConfig], nil
}

// UpdateGardenKubeconfigSecret updates the secret in the seed cluster that holds the kubeconfig of the Garden cluster.
func UpdateGardenKubeconfigSecret(ctx context.Context, certClientConfig *rest.Config, certData, privateKeyData []byte, seedClient client.Client, gardenClientConnection *config.GardenClientConnection) ([]byte, error) {
	kubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, privateKeyData, certData)
	if err != nil {
		return nil, err
	}

	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenClientConnection.KubeconfigSecret.Name,
			Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, seedClient, kubeconfigSecret, func() error {
		kubeconfigSecret.Data = map[string][]byte{kubernetes.KubeConfig: kubeconfig}
		return nil
	}); err != nil {
		return nil, err
	}
	return kubeconfig, nil
}

// CreateGardenletKubeconfigWithClientCertificate creates a kubeconfig for the Gardenlet with the given client certificate.
func CreateGardenletKubeconfigWithClientCertificate(config *rest.Config, privateKeyData, certDat []byte) ([]byte, error) {
	return kubeconfigWithAuthInfo(config, &clientcmdapi.AuthInfo{
		ClientCertificateData: certDat,
		ClientKeyData:         privateKeyData,
	})
}

// CreateGardenletKubeconfigWithToken creates a kubeconfig for the Gardenlet with the given bootstrap token.
func CreateGardenletKubeconfigWithToken(config *rest.Config, token string) ([]byte, error) {
	return kubeconfigWithAuthInfo(config, &clientcmdapi.AuthInfo{
		Token: token,
	})
}

// DigestedName is a digest that should include all the relevant pieces of the CSR we care about.
// We can't directly hash the serialized CSR because of random padding that we
// regenerate every loop and we include usages which are not contained in the
// CSR. This needs to be kept up to date as we add new fields to the node
// certificates and with ensureCompatible.
func DigestedName(publicKey interface{}, subject *pkix.Name, usages []certificatesv1.KeyUsage) (string, error) {
	hash := sha512.New512_256()

	// Here we make sure two different inputs can't write the same stream
	// to the hash. This delimiter is not in the base64.URLEncoding
	// alphabet so there is no way to have spill over collisions. Without
	// it 'CN:foo,ORG:bar' hashes to the same value as 'CN:foob,ORG:ar'
	const delimiter = '|'
	encode := base64.RawURLEncoding.EncodeToString

	write := func(data []byte) {
		_, _ = hash.Write([]byte(encode(data)))
		_, _ = hash.Write([]byte{delimiter})
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

// ComputeGardenletKubeconfigWithBootstrapToken creates a kubeconfig containing a valid bootstrap token as client credentials
// Creates the required bootstrap token secret in the Garden cluster and puts it into a Kubeconfig
// tailored to the Gardenlet
func ComputeGardenletKubeconfigWithBootstrapToken(ctx context.Context, gardenClient client.Client, gardenClientRestConfig *rest.Config, tokenID, description string, validity time.Duration) ([]byte, error) {
	var (
		refreshBootstrapToken = true
		bootstrapTokenSecret  *corev1.Secret
		err                   error
	)

	secret := &corev1.Secret{}
	if err := gardenClient.Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstraptokenutil.BootstrapTokenSecretName(tokenID)), secret); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if expirationTime, ok := secret.Data[bootstraptokenapi.BootstrapTokenExpirationKey]; ok {
		t, err := time.Parse(time.RFC3339, string(expirationTime))
		if err != nil {
			return nil, err
		}

		if !t.Before(metav1.Now().UTC()) {
			bootstrapTokenSecret = secret
			refreshBootstrapToken = false
		}
	}

	if refreshBootstrapToken {
		bootstrapTokenSecret, err = kutil.ComputeBootstrapToken(ctx, gardenClient, tokenID, description, validity)
		if err != nil {
			return nil, err
		}
	}

	return CreateGardenletKubeconfigWithToken(gardenClientRestConfig, kutil.BootstrapTokenFrom(bootstrapTokenSecret.Data))
}

// ComputeGardenletKubeconfigWithBootstrapToken creates a kubeconfig containing the token of a service account
// Creates the required service account in the Garden cluster and puts the associated token into a Kubeconfig
// tailored to the Gardenlet
func ComputeGardenletKubeconfigWithServiceAccountToken(ctx context.Context, gardenClient client.Client, gardenClientRestConfig *rest.Config, serviceAccountName string) ([]byte, error) {
	// Create a temporary service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, sa, func() error { return nil }); err != nil {
		return nil, err
	}

	// Get the service account secret
	if len(sa.Secrets) == 0 {
		return nil, fmt.Errorf("service account token controller has not yet created a secret for the service account")
	}
	saSecret := &corev1.Secret{}
	if err := gardenClient.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, sa.Secrets[0].Name), saSecret); err != nil {
		return nil, err
	}

	// Create a ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: BuildBootstrapperName(serviceAccountName),
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     GardenerSeedBootstrapper,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: v1beta1constants.GardenNamespace,
			},
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Get bootstrap kubeconfig from service account secret
	return CreateGardenletKubeconfigWithToken(gardenClientRestConfig, string(saSecret.Data[corev1.ServiceAccountTokenKey]))
}
