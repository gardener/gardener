// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// SecretStaticTokenName is a constant for the name of the static-token secret.
	SecretStaticTokenName = "kube-apiserver-static-token"

	secretOIDCCABundleNamePrefix   = "kube-apiserver-oidc-cabundle"
	secretOIDCCABundleDataKeyCaCrt = "ca.crt"

	secretAuditWebhookKubeconfigNamePrefix          = "kube-apiserver-audit-webhook-kubeconfig"
	secretAuthenticationWebhookKubeconfigNamePrefix = "kube-apiserver-authentication-webhook-kubeconfig"
	secretAuthorizationWebhookKubeconfigNamePrefix  = "kube-apiserver-authorization-webhook-kubeconfig"
	secretWebhookKubeconfigDataKey                  = "kubeconfig.yaml"

	secretETCDEncryptionConfigurationDataKey = "encryption-configuration.yaml"
	secretAdmissionKubeconfigsNamePrefix     = "kube-apiserver-admission-kubeconfigs"

	userNameClusterAdmin = "system:cluster-admin"
	userNameHealthCheck  = "health-check"
)

func (k *kubeAPIServer) emptySecret(name string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileSecretAdmissionKubeconfigs(ctx context.Context, secret *corev1.Secret) error {
	secret.Data = make(map[string][]byte)

	for _, plugin := range k.values.EnabledAdmissionPlugins {
		if len(plugin.Kubeconfig) != 0 {
			secret.Data[admissionPluginsKubeconfigFilename(plugin.Name)] = plugin.Kubeconfig
		}
	}

	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func admissionPluginsKubeconfigFilename(name string) string {
	return strings.ToLower(name) + "-kubeconfig.yaml"
}

func (k *kubeAPIServer) reconcileSecretOIDCCABundle(ctx context.Context, secret *corev1.Secret) error {
	if k.values.OIDC == nil || k.values.OIDC.CABundle == nil {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	secret.Data = map[string][]byte{secretOIDCCABundleDataKeyCaCrt: []byte(*k.values.OIDC.CABundle)}
	utilruntime.Must(kubernetesutils.MakeUnique(secret))

	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func (k *kubeAPIServer) reconcileSecretServiceAccountKey(ctx context.Context) (*corev1.Secret, error) {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if k.values.ServiceAccount.RotationPhase == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	secret, err := k.secretsManager.Generate(ctx, &secretsutils.RSASecretConfig{
		Name: v1beta1constants.SecretNameServiceAccountKey,
		Bits: 4096,
	}, options...)
	if err != nil {
		return nil, err
	}

	// TODO(rfranzke): Remove this in a future release.
	return secret, kubernetesutils.DeleteObject(ctx, k.client.Client(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key", Namespace: k.namespace}})
}

func (k *kubeAPIServer) reconcileSecretStaticToken(ctx context.Context) (*corev1.Secret, error) {
	staticTokenSecretConfig := &secretsutils.StaticTokenSecretConfig{
		Name: SecretStaticTokenName,
		Tokens: map[string]secretsutils.TokenConfig{
			userNameHealthCheck: {
				Username: userNameHealthCheck,
				UserID:   userNameHealthCheck,
			},
		},
	}

	if pointer.BoolDeref(k.values.StaticTokenKubeconfigEnabled, true) {
		staticTokenSecretConfig.Tokens[userNameClusterAdmin] = secretsutils.TokenConfig{
			Username: userNameClusterAdmin,
			UserID:   userNameClusterAdmin,
			Groups:   []string{user.SystemPrivilegedGroup},
		}
	}

	return k.secretsManager.Generate(ctx, staticTokenSecretConfig, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (k *kubeAPIServer) reconcileSecretUserKubeconfig(ctx context.Context, secretStaticToken *corev1.Secret) error {
	caBundleSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	var err error
	var token *secretsutils.Token
	if secretStaticToken != nil {
		staticToken, err := secretsutils.LoadStaticTokenFromCSV(SecretStaticTokenName, secretStaticToken.Data[secretsutils.DataKeyStaticTokenCSV])
		if err != nil {
			return err
		}

		token, err = staticToken.GetTokenForUsername(userNameClusterAdmin)
		if err != nil {
			return err
		}
	}

	_, err = k.secretsManager.Generate(ctx, &secretsutils.KubeconfigSecretConfig{
		Name:        SecretNameUserKubeconfig,
		ContextName: k.namespace,
		Cluster: clientcmdv1.Cluster{
			Server:                   k.values.ExternalServer,
			CertificateAuthorityData: caBundleSecret.Data[secretsutils.DataKeyCertificateBundle],
		},
		AuthInfo: clientcmdv1.AuthInfo{
			Token: token.Token,
		},
	}, secretsmanager.Rotate(secretsmanager.InPlace))
	return err
}

func (k *kubeAPIServer) reconcileSecretETCDEncryptionConfiguration(ctx context.Context, secret *corev1.Secret) error {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if k.values.ETCDEncryption.RotationPhase == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	keySecret, err := k.secretsManager.Generate(ctx, &secretsutils.ETCDEncryptionKeySecretConfig{
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
	secret.Data = map[string][]byte{secretETCDEncryptionConfigurationDataKey: data}
	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	desiredLabels := utils.MergeStringMaps(secret.Labels) // copy

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
		Name:   string(data[secretsutils.DataKeyEncryptionKeyName]),
		Secret: string(data[secretsutils.DataKeyEncryptionSecret]),
	}
}

func (k *kubeAPIServer) reconcileSecretServer(ctx context.Context) (*corev1.Secret, error) {
	var (
		ipAddresses    = append([]net.IP{}, k.values.ServerCertificate.ExtraIPAddresses...)
		deploymentName = k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer
		dnsNames       = kubernetesutils.DNSNamesForService(deploymentName, k.namespace)
	)

	if k.values.SNI.PodMutatorEnabled || (k.values.VPN.Enabled && k.values.VPN.HighAvailabilityEnabled) {
		ipAddresses = append(ipAddresses, net.ParseIP("127.0.0.1"))
	}

	if !k.values.IsWorkerless {
		dnsNames = append(dnsNames, kubernetesutils.DNSNamesForService("kubernetes", metav1.NamespaceDefault)...)
	}

	return k.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  deploymentName,
		IPAddresses:                 append(ipAddresses, k.values.ServerCertificate.ExtraIPAddresses...),
		DNSNames:                    append(dnsNames, k.values.ServerCertificate.ExtraDNSNames...),
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (k *kubeAPIServer) reconcileSecretKubeletClient(ctx context.Context) (*corev1.Secret, error) {
	if k.values.IsWorkerless {
		return nil, nil
	}

	return k.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameKubeAPIServerToKubelet,
		CommonName:                  userName,
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAKubelet, secretsmanager.UseOldCA), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (k *kubeAPIServer) reconcileSecretKubeAggregator(ctx context.Context) (*corev1.Secret, error) {
	return k.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameKubeAggregator,
		CommonName:                  "system:kube-aggregator",
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAFrontProxy), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (k *kubeAPIServer) reconcileSecretHTTPProxy(ctx context.Context) (*corev1.Secret, error) {
	if !k.values.VPN.Enabled || k.values.VPN.HighAvailabilityEnabled {
		return nil, nil
	}

	return k.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameHTTPProxy,
		CommonName:                  "kube-apiserver-http-proxy",
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAVPN), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (k *kubeAPIServer) reconcileSecretHAVPNSeedClient(ctx context.Context) (*corev1.Secret, error) {
	if !k.values.VPN.Enabled || !k.values.VPN.HighAvailabilityEnabled {
		return nil, nil
	}

	return k.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameHAVPNSeedClient,
		CommonName:                  UserNameVPNSeedClient,
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAVPN), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (k *kubeAPIServer) reconcileSecretHAVPNSeedClientTLSAuth(ctx context.Context) (*corev1.Secret, error) {
	if !k.values.VPN.Enabled || !k.values.VPN.HighAvailabilityEnabled {
		return nil, nil
	}

	return k.secretsManager.Generate(ctx, &secretsutils.VPNTLSAuthConfig{
		Name: vpnseedserver.SecretNameTLSAuth,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
}

type tlsSNISecret struct {
	secretName     string
	domainPatterns []string
}

func (k *kubeAPIServer) reconcileTLSSNISecrets(ctx context.Context) ([]tlsSNISecret, error) {
	var out []tlsSNISecret

	for i, sni := range k.values.SNI.TLS {
		switch {
		case sni.SecretName != nil:
			out = append(out, tlsSNISecret{secretName: *sni.SecretName, domainPatterns: sni.DomainPatterns})

		case len(sni.Certificate) > 0 && len(sni.PrivateKey) > 0:
			secret := k.emptySecret(fmt.Sprintf("kube-apiserver-tls-sni-%d", i))

			secret.Data = map[string][]byte{
				corev1.TLSCertKey:       sni.Certificate,
				corev1.TLSPrivateKeyKey: sni.PrivateKey,
			}
			utilruntime.Must(kubernetesutils.MakeUnique(secret))

			if err := client.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret)); err != nil {
				return nil, err
			}

			out = append(out, tlsSNISecret{secretName: secret.Name, domainPatterns: sni.DomainPatterns})

		default:
			return nil, fmt.Errorf("either the name of an existing secret or both certificate and private key must be provided for TLS SNI config")
		}
	}

	return out, nil
}

func (k *kubeAPIServer) reconcileSecretWebhookKubeconfig(ctx context.Context, secret *corev1.Secret, kubeconfig []byte) error {
	secret.Data = map[string][]byte{secretWebhookKubeconfigDataKey: kubeconfig}
	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func (k *kubeAPIServer) reconcileSecretAuditWebhookKubeconfig(ctx context.Context, secret *corev1.Secret) error {
	if k.values.Audit == nil || k.values.Audit.Webhook == nil || len(k.values.Audit.Webhook.Kubeconfig) == 0 {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	return k.reconcileSecretWebhookKubeconfig(ctx, secret, k.values.Audit.Webhook.Kubeconfig)
}

func (k *kubeAPIServer) reconcileSecretAuthenticationWebhookKubeconfig(ctx context.Context, secret *corev1.Secret) error {
	if k.values.AuthenticationWebhook == nil || len(k.values.AuthenticationWebhook.Kubeconfig) == 0 {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	return k.reconcileSecretWebhookKubeconfig(ctx, secret, k.values.AuthenticationWebhook.Kubeconfig)
}

func (k *kubeAPIServer) reconcileSecretAuthorizationWebhookKubeconfig(ctx context.Context, secret *corev1.Secret) error {
	if k.values.AuthorizationWebhook == nil || len(k.values.AuthorizationWebhook.Kubeconfig) == 0 {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	return k.reconcileSecretWebhookKubeconfig(ctx, secret, k.values.AuthorizationWebhook.Kubeconfig)
}
