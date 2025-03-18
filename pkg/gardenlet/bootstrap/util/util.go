// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// GetSeedName returns the seed name from the SeedConfig or the default Seed name
func GetSeedName(seedConfig *gardenletconfigv1alpha1.SeedConfig) string {
	if seedConfig != nil {
		return seedConfig.Name
	}
	return ""
}

// GetKubeconfigFromSecret tries to retrieve the kubeconfig bytes using the given client
// returns the kubeconfig or nil if it cannot be found
func GetKubeconfigFromSecret(ctx context.Context, seedClient client.Client, key client.ObjectKey) ([]byte, error) {
	kubeconfigSecret := &corev1.Secret{}
	if err := seedClient.Get(ctx, key, kubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	return kubeconfigSecret.Data[kubernetes.KubeConfig], nil
}

// UpdateGardenKubeconfigSecret updates the secret in the seed cluster that holds the kubeconfig of the Garden cluster.
func UpdateGardenKubeconfigSecret(ctx context.Context, certClientConfig *rest.Config, certData, privateKeyData []byte, seedClient client.Client, kubeconfigKey client.ObjectKey) ([]byte, error) {
	kubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, privateKeyData, certData)
	if err != nil {
		return nil, err
	}

	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigKey.Name,
			Namespace: kubeconfigKey.Namespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, kubeconfigSecret, func() error {
		delete(kubeconfigSecret.Annotations, v1beta1constants.GardenerOperation)
		kubeconfigSecret.Data = map[string][]byte{kubernetes.KubeConfig: kubeconfig}
		return nil
	}); err != nil {
		return nil, err
	}
	return kubeconfig, nil
}

// UpdateGardenKubeconfigCAIfChanged checks if the garden cluster CA given in the gardenClientConnection differs from the CA in the kubeconfig secret
// and updates the secret to contain the new CA if that's the case.
func UpdateGardenKubeconfigCAIfChanged(ctx context.Context, log logr.Logger, seedClient client.Client, kubeconfig []byte, gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection) ([]byte, error) {
	if kubeconfig == nil {
		return nil, fmt.Errorf("no kubeconfig given")
	}

	typedKubeconfig, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to parse kubeconfig: %w", err)
	}

	curContext, ok := typedKubeconfig.Contexts[typedKubeconfig.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("invalid kubeconfig: currently set context %s not found among contexts", typedKubeconfig.CurrentContext)
	}

	curCluster, ok := typedKubeconfig.Clusters[curContext.Cluster]
	if !ok {
		return nil, fmt.Errorf("invalid kubeconfig: currently set cluster %s not found among clusters", curContext.Cluster)
	}

	curAuth, ok := typedKubeconfig.AuthInfos[curContext.AuthInfo]
	if !ok {
		return nil, fmt.Errorf("invalid kubeconfig: currently set authinfo %s not found among authinfos", curContext.AuthInfo)
	}

	if bytes.Equal(curCluster.CertificateAuthorityData, gardenClientConnection.GardenClusterCACert) {
		// CAs are equal, nothing to do
		return kubeconfig, nil
	}

	kubeconfigKey := kubernetesutils.ObjectKeyFromSecretRef(*gardenClientConnection.KubeconfigSecret)
	log = log.WithValues("kubeconfigSecret", kubeconfigKey)
	log.Info("Updating kubeconfig secret as CA data has changed")

	if bytes.Equal(gardenClientConnection.GardenClusterCACert, []byte("none")) || bytes.Equal(gardenClientConnection.GardenClusterCACert, []byte("null")) {
		gardenClientConnection.GardenClusterCACert = []byte{}
	}

	// extract data from existing kubeconfig and reuse UpdateGardenKubeconfigSecret function
	return UpdateGardenKubeconfigSecret(ctx, &rest.Config{
		Host: curCluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: curCluster.InsecureSkipTLSVerify,
			CAData:   gardenClientConnection.GardenClusterCACert,
		},
	}, curAuth.ClientCertificateData, curAuth.ClientKeyData, seedClient, kubeconfigKey)
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
	if err := gardenClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: bootstraptokenutil.BootstrapTokenSecretName(tokenID)}, secret); client.IgnoreNotFound(err) != nil {
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
		bootstrapTokenSecret, err = bootstraptoken.ComputeBootstrapToken(ctx, gardenClient, tokenID, description, validity)
		if err != nil {
			return nil, err
		}
	}

	return CreateGardenletKubeconfigWithToken(gardenClientRestConfig, bootstraptoken.FromSecretData(bootstrapTokenSecret.Data))
}

// ComputeGardenletKubeconfigWithServiceAccountToken creates a kubeconfig containing the token of a service account
// Creates the required service account in the Garden cluster and puts the associated token into a Kubeconfig
// tailored to the Gardenlet
func ComputeGardenletKubeconfigWithServiceAccountToken(ctx context.Context, gardenClient client.Client, gardenClientRestConfig *rest.Config, serviceAccountName, serviceAccountNamespace string) ([]byte, error) {
	// Create a temporary service account
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: serviceAccountNamespace,
		},
	}
	if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, gardenClient, serviceAccount, func() error { return nil }); err != nil {
		return nil, err
	}

	// Get a token for this service account
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: ptr.To[int64](600),
		},
	}
	if err := gardenClient.SubResource("token").Create(ctx, serviceAccount, tokenRequest); err != nil {
		return nil, fmt.Errorf("failed creating a token for ServiceAccount %q: %w", client.ObjectKeyFromObject(serviceAccount), err)
	}

	// Create a ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterRoleBindingName(serviceAccount.Namespace, serviceAccount.Name),
		},
	}
	if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, gardenClient, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     GardenerSeedBootstrapper,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed creating a ClusterRoleBinding for ServiceAccount %q: %w", client.ObjectKeyFromObject(serviceAccount), err)
	}

	// Get bootstrap kubeconfig from service account secret
	return CreateGardenletKubeconfigWithToken(gardenClientRestConfig, tokenRequest.Status.Token)
}

// ClusterRoleBindingName concatenates the gardener seed bootstrapper group with the given name, separated by a colon.
func ClusterRoleBindingName(managedSeedNamespace, serviceAccountName string) string {
	return ClusterRoleBindingNamePrefix + managedSeedNamespace + clusterRoleBindingNameDelimiter + serviceAccountName
}

// ManagedSeedInfoFromClusterRoleBindingName returns the namespace and name of the related ManagedSeed for a given
// cluster role binding name.
func ManagedSeedInfoFromClusterRoleBindingName(clusterRoleBindingName string) (managedSeedNamespace, managedSeedName string) {
	var (
		metadata = strings.TrimPrefix(clusterRoleBindingName, ClusterRoleBindingNamePrefix)
		split    = strings.Split(metadata, clusterRoleBindingNameDelimiter)
	)

	managedSeedName = split[0]
	if len(split) > 1 {
		managedSeedNamespace = split[0]
		managedSeedName = split[1]
	}

	managedSeedName = strings.TrimPrefix(managedSeedName, ServiceAccountNamePrefix)
	return
}

// ServiceAccountName returns the name of a `ServiceAccount` for bootstrapping based on the given metadata.
func ServiceAccountName(name string) string {
	return ServiceAccountNamePrefix + name
}

const (
	// KindManagedSeed is a constant for the "managed seed" kind.
	KindManagedSeed = "seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource"
	// KindGardenlet is a constant for the "gardenlet" kind.
	KindGardenlet = "seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource"
	// ServiceAccountNamePrefix is the prefix used for service account names.
	ServiceAccountNamePrefix = "gardenlet-bootstrap-"
	// ClusterRoleBindingNamePrefix is the prefix used for cluster role binding names.
	ClusterRoleBindingNamePrefix = GardenerSeedBootstrapper + ":"
	// GardenerSeedBootstrapper is a constant for the gardener seed bootstrapper name.
	GardenerSeedBootstrapper = "gardener.cloud:system:seed-bootstrapper"

	clusterRoleBindingNameDelimiter = ":"
	descriptionMetadataDelimiter    = "/"
	descriptionSuffix               = "."
)

func metadataForNamespaceName(namespace, name string) string {
	if namespace != "" {
		return namespace + descriptionMetadataDelimiter + name
	}
	return name
}

func descriptionForKind(kind string) string {
	return fmt.Sprintf("A bootstrap token for the Gardenlet for %s ", kind)
}

// Description returns a description for a bootstrap token with the given kind/namespace/name information.
func Description(kind, namespace, name string) string {
	return descriptionForKind(kind) + metadataForNamespaceName(namespace, name) + descriptionSuffix
}

// MetadataFromDescription returns the namespace and name for a given description with a specific kind.
func MetadataFromDescription(description string) (kind, namespace, name string) {
	if strings.Contains(description, KindManagedSeed) {
		kind = KindManagedSeed
	} else if strings.Contains(description, KindGardenlet) {
		kind = KindGardenlet
	}

	var (
		metadata = strings.TrimPrefix(strings.TrimSuffix(description, descriptionSuffix), descriptionForKind(kind))
		split    = strings.Split(metadata, descriptionMetadataDelimiter)
	)

	if len(split) > 1 {
		namespace = split[0]
		name = split[1]
		return
	}

	name = split[0]
	return
}
