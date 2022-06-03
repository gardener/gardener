// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	secretOIDCCABundleNamePrefix   = "kube-apiserver-oidc-cabundle"
	secretOIDCCABundleDataKeyCaCrt = "ca.crt"

	secretServiceAccountSigningKeyNamePrefix = "kube-apiserver-sa-signing-key"
	// SecretServiceAccountSigningKeyDataKeySigningKey is a constant for a key in the data map that contains the key
	// which is used to sign service accounts.
	SecretServiceAccountSigningKeyDataKeySigningKey = "signing-key"

	// SecretStaticTokenName is a constant for the name of the static-token secret.
	SecretStaticTokenName = "kube-apiserver-static-token"
	// SecretBasicAuthName is a constant for the name of the basic-auth secret.
	SecretBasicAuthName = "kube-apiserver-basic-auth"

	secretETCDEncryptionConfigurationDataKey = "encryption-configuration.yaml"

	userNameClusterAdmin = "system:cluster-admin"
	userNameHealthCheck  = "health-check"
)

func (k *kubeAPIServer) emptySecret(name string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileSecretOIDCCABundle(ctx context.Context, secret *corev1.Secret) error {
	if k.values.OIDC == nil || k.values.OIDC.CABundle == nil {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	secret.Data = map[string][]byte{secretOIDCCABundleDataKeyCaCrt: []byte(*k.values.OIDC.CABundle)}
	utilruntime.Must(kutil.MakeUnique(secret))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func (k *kubeAPIServer) reconcileSecretUserProvidedServiceAccountSigningKey(ctx context.Context, secret *corev1.Secret) error {
	if k.values.ServiceAccount.SigningKey == nil {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	secret.Data = map[string][]byte{SecretServiceAccountSigningKeyDataKeySigningKey: k.values.ServiceAccount.SigningKey}
	utilruntime.Must(kutil.MakeUnique(secret))

	return kutil.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func (k *kubeAPIServer) reconcileSecretServiceAccountKey(ctx context.Context) (*corev1.Secret, error) {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if k.values.ServiceAccount.RotationPhase == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	secret, err := k.secretsManager.Generate(ctx, &secretutils.RSASecretConfig{
		Name: v1beta1constants.SecretNameServiceAccountKey,
		Bits: 4096,
	}, options...)
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretBasicAuth(ctx context.Context) (*corev1.Secret, error) {
	var (
		secret *corev1.Secret
		err    error
	)

	if k.values.BasicAuthenticationEnabled {
		secret, err = k.secretsManager.Generate(ctx, &secretutils.BasicAuthSecretConfig{
			Name:           SecretBasicAuthName,
			Format:         secretutils.BasicAuthFormatCSV,
			Username:       "admin",
			PasswordLength: 32,
		}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace))
		if err != nil {
			return nil, err
		}
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-basic-auth", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretStaticToken(ctx context.Context) (*corev1.Secret, error) {
	staticTokenSecretConfig := &secretutils.StaticTokenSecretConfig{
		Name: SecretStaticTokenName,
		Tokens: map[string]secretutils.TokenConfig{
			userNameHealthCheck: {
				Username: userNameHealthCheck,
				UserID:   userNameHealthCheck,
			},
		},
	}

	if pointer.BoolDeref(k.values.StaticTokenKubeconfigEnabled, true) {
		staticTokenSecretConfig.Tokens[userNameClusterAdmin] = secretutils.TokenConfig{
			Username: userNameClusterAdmin,
			UserID:   userNameClusterAdmin,
			Groups:   []string{user.SystemPrivilegedGroup},
		}
	}

	secret, err := k.secretsManager.Generate(ctx, staticTokenSecretConfig, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "static-token", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretUserKubeconfig(ctx context.Context, secretStaticToken, secretBasicAuth *corev1.Secret) error {
	caBundleSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	var err error
	var basicAuth *secretutils.BasicAuth
	if secretBasicAuth != nil {
		basicAuth, err = secretutils.LoadBasicAuthFromCSV(SecretBasicAuthName, secretBasicAuth.Data[secretutils.DataKeyCSV])
		if err != nil {
			return err
		}
	}

	var token *secretutils.Token
	if secretStaticToken != nil {
		staticToken, err := secretutils.LoadStaticTokenFromCSV(SecretStaticTokenName, secretStaticToken.Data[secretutils.DataKeyStaticTokenCSV])
		if err != nil {
			return err
		}

		token, err = staticToken.GetTokenForUsername(userNameClusterAdmin)
		if err != nil {
			return err
		}
	}

	// TODO: In the future when we no longer support basic auth (dropped support for Kubernetes < 1.18) then we can
	//  switch from ControlPlaneSecretConfig to KubeconfigSecretConfig.
	if _, err := k.secretsManager.Generate(ctx, &secretutils.ControlPlaneSecretConfig{
		Name:      SecretNameUserKubeconfig,
		BasicAuth: basicAuth,
		Token:     token,
		KubeConfigRequests: []secretutils.KubeConfigRequest{{
			ClusterName:   k.namespace,
			APIServerHost: k.values.ExternalServer,
			CAData:        caBundleSecret.Data[secretutils.DataKeyCertificateBundle],
		}},
	}, secretsmanager.Rotate(secretsmanager.InPlace)); err != nil {
		return err
	}

	// TODO(rfranzke): Remove this in a future release.
	return kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kubecfg", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretETCDEncryptionConfiguration(ctx context.Context, secret *corev1.Secret) error {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if k.values.ETCDEncryption.RotationPhase == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	keySecret, err := k.secretsManager.Generate(ctx, &secretutils.ETCDEncryptionKeySecretConfig{
		Name:         v1beta1constants.SecretNameETCDEncryptionKey,
		SecretLength: 32,
	}, options...)
	if err != nil {
		return err
	}

	keySecretOld, _ := k.secretsManager.Get(v1beta1constants.SecretNameETCDEncryptionKey, secretsmanager.Old)

	encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{
		Resources: []apiserverconfigv1.ResourceConfiguration{{
			Resources: []string{
				"secrets",
			},
			Providers: []apiserverconfigv1.ProviderConfiguration{
				{
					AESCBC: &apiserverconfigv1.AESConfiguration{
						Keys: k.etcdEncryptionAESKeys(keySecret, keySecretOld),
					},
				},
				{
					Identity: &apiserverconfigv1.IdentityConfiguration{},
				},
			},
		}},
	}

	data, err := runtime.Encode(codec, encryptionConfiguration)
	if err != nil {
		return err
	}

	secret.Labels = map[string]string{v1beta1constants.LabelRole: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration}
	desiredLabels := utils.MergeStringMaps(secret.Labels) // copy
	secret.Data = map[string][]byte{secretETCDEncryptionConfigurationDataKey: data}
	utilruntime.Must(kutil.MakeUnique(secret))

	if err := k.client.Client().Create(ctx, secret); err == nil || !apierrors.IsAlreadyExists(err) {
		return err
	}

	// reconcile labels of existing secret
	if err := k.client.Client().Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		return err
	}
	patch := client.MergeFrom(secret.DeepCopy())
	secret.Labels = desiredLabels
	return k.client.Client().Patch(ctx, secret, patch)
}

func (k *kubeAPIServer) etcdEncryptionAESKeys(keySecretCurrent, keySecretOld *corev1.Secret) []apiserverconfigv1.Key {
	if keySecretOld == nil {
		return []apiserverconfigv1.Key{
			aesKeyFromSecretData(keySecretCurrent.Data),
		}
	}

	keyForEncryption, keyForDecryption := keySecretCurrent, keySecretOld
	if !k.values.ETCDEncryption.EncryptWithCurrentKey {
		keyForEncryption, keyForDecryption = keySecretOld, keySecretCurrent
	}

	return []apiserverconfigv1.Key{
		aesKeyFromSecretData(keyForEncryption.Data),
		aesKeyFromSecretData(keyForDecryption.Data),
	}
}

func aesKeyFromSecretData(data map[string][]byte) apiserverconfigv1.Key {
	return apiserverconfigv1.Key{
		Name:   string(data[secretutils.DataKeyEncryptionKeyName]),
		Secret: string(data[secretutils.DataKeyEncryptionSecret]),
	}
}

func (k *kubeAPIServer) reconcileSecretServer(ctx context.Context) (*corev1.Secret, error) {
	var (
		ipAddresses = append([]net.IP{
			net.ParseIP("127.0.0.1"),
		}, k.values.ServerCertificate.ExtraIPAddresses...)

		dnsNames = append([]string{
			v1beta1constants.DeploymentNameKubeAPIServer,
			fmt.Sprintf("%s.%s", v1beta1constants.DeploymentNameKubeAPIServer, k.namespace),
			fmt.Sprintf("%s.%s.svc", v1beta1constants.DeploymentNameKubeAPIServer, k.namespace),
		}, kutil.DNSNamesForService("kubernetes", metav1.NamespaceDefault)...)
	)

	secret, err := k.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  v1beta1constants.DeploymentNameKubeAPIServer,
		IPAddresses:                 append(ipAddresses, k.values.ServerCertificate.ExtraIPAddresses...),
		DNSNames:                    append(dnsNames, k.values.ServerCertificate.ExtraDNSNames...),
		CertType:                    secretutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretKubeletClient(ctx context.Context) (*corev1.Secret, error) {
	secret, err := k.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        secretNameKubeAPIServerToKubelet,
		CommonName:                  userName,
		CertType:                    secretutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAKubelet, secretsmanager.UseOldCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-kubelet", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretKubeAggregator(ctx context.Context) (*corev1.Secret, error) {
	secret, err := k.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        secretNameKubeAggregator,
		CommonName:                  "system:kube-aggregator",
		CertType:                    secretutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAFrontProxy), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-aggregator", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretHTTPProxy(ctx context.Context) (*corev1.Secret, error) {
	if !k.values.VPN.ReversedVPNEnabled {
		return nil, nil
	}

	secret, err := k.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        secretNameHTTPProxy,
		CommonName:                  "kube-apiserver-http-proxy",
		CertType:                    secretutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAVPN), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-http-proxy", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretLegacyVPNSeed(ctx context.Context) (*corev1.Secret, error) {
	if k.values.VPN.ReversedVPNEnabled {
		return nil, nil
	}

	secret, err := k.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        secretNameLegacyVPNSeed,
		CommonName:                  UserNameVPNSeed,
		CertType:                    secretutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAClient), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretLegacyVPNSeedTLSAuth(ctx context.Context) (*corev1.Secret, error) {
	if k.values.VPN.ReversedVPNEnabled {
		return nil, nil
	}

	secret, err := k.secretsManager.Generate(ctx, &secretutils.VPNTLSAuthConfig{
		Name: SecretNameVPNSeedTLSAuth,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kutil.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-tlsauth", Namespace: k.namespace}})
}
