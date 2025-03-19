// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/apiserver"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	// SecretStaticTokenName is a constant for the name of the static-token secret.
	SecretStaticTokenName = "kube-apiserver-static-token" // #nosec G101 -- No credential.
	// SecretNameUserKubeconfig is the name for the user kubeconfig.
	SecretNameUserKubeconfig = "user-kubeconfig" // #nosec G101 -- No credential.

	secretOIDCCABundleNamePrefix   = "kube-apiserver-oidc-cabundle" // #nosec G101 -- No credential.
	secretOIDCCABundleDataKeyCaCrt = "ca.crt"

	secretAuditWebhookKubeconfigNamePrefix           = "kube-apiserver-audit-webhook-kubeconfig"           // #nosec G101 -- No credential.
	secretAuthenticationWebhookKubeconfigNamePrefix  = "kube-apiserver-authentication-webhook-kubeconfig"  // #nosec G101 -- No credential.
	secretAuthorizationWebhooksKubeconfigsNamePrefix = "kube-apiserver-authorization-webhooks-kubeconfigs" // #nosec G101 -- No credential.
	secretAdmissionKubeconfigsNamePrefix             = "kube-apiserver-admission-kubeconfigs"              // #nosec G101 -- No credential.

	// UserNameClusterAdmin is the user name for the static admin token.
	UserNameClusterAdmin = "system:cluster-admin"
	userNameHealthCheck  = "health-check"
)

func (k *kubeAPIServer) emptySecret(name string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileSecretOIDCCABundle(ctx context.Context, secret *corev1.Secret) error {
	if value, ok := k.values.FeatureGates["StructuredAuthenticationConfiguration"]; k.values.OIDC == nil ||
		(versionutils.ConstraintK8sGreaterEqual130.Check(k.values.Version) && (!ok || value)) ||
		k.values.OIDC.CABundle == nil {
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

	return k.secretsManager.Generate(ctx, &secretsutils.RSASecretConfig{
		Name: v1beta1constants.SecretNameServiceAccountKey,
		Bits: 4096,
	}, options...)
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

	if ptr.Deref(k.values.StaticTokenKubeconfigEnabled, false) {
		staticTokenSecretConfig.Tokens[UserNameClusterAdmin] = secretsutils.TokenConfig{
			Username: UserNameClusterAdmin,
			UserID:   UserNameClusterAdmin,
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

	staticToken, err := secretsutils.LoadStaticTokenFromCSV(SecretStaticTokenName, secretStaticToken.Data[secretsutils.DataKeyStaticTokenCSV])
	if err != nil {
		return err
	}

	token, err := staticToken.GetTokenForUsername(UserNameClusterAdmin)
	if err != nil {
		return err
	}

	_, err = k.secretsManager.Generate(ctx, &secretsutils.KubeconfigSecretConfig{
		Name:        SecretNameUserKubeconfig,
		ContextName: k.namespace,
		Cluster: clientcmdv1.Cluster{
			Server:                   "localhost",
			CertificateAuthorityData: caBundleSecret.Data[secretsutils.DataKeyCertificateBundle],
		},
		AuthInfo: clientcmdv1.AuthInfo{
			Token: token.Token,
		},
	}, secretsmanager.Rotate(secretsmanager.InPlace))
	return err
}

func (k *kubeAPIServer) reconcileSecretETCDEncryptionConfiguration(ctx context.Context, secret *corev1.Secret) error {
	return apiserver.ReconcileSecretETCDEncryptionConfiguration(
		ctx,
		k.client.Client(),
		k.secretsManager,
		k.values.ETCDEncryption,
		secret,
		v1beta1constants.SecretNameETCDEncryptionKey,
		v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration,
	)
}

func (k *kubeAPIServer) reconcileSecretServer(ctx context.Context) (*corev1.Secret, error) {
	var (
		ipAddresses    = append([]net.IP{}, k.values.ServerCertificate.ExtraIPAddresses...)
		deploymentName = k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer
		dnsNames       = kubernetesutils.DNSNamesForService(deploymentName, k.namespace)
	)

	if k.values.RunsAsStaticPod || k.values.SNI.Enabled || (k.values.VPN.Enabled && k.values.VPN.HighAvailabilityEnabled) {
		dnsNames = append(dnsNames, "localhost")
		ipAddresses = append(ipAddresses, net.ParseIP("127.0.0.1"))
		ipAddresses = append(ipAddresses, net.ParseIP("::1"))
	}

	if !k.values.IsWorkerless {
		dnsNames = append(dnsNames, kubernetesutils.DNSNamesForService("kubernetes", metav1.NamespaceDefault)...)
	}

	return k.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                              SecretNameServerCert,
		CommonName:                        deploymentName,
		IPAddresses:                       append(ipAddresses, k.values.ServerCertificate.ExtraIPAddresses...),
		DNSNames:                          append(dnsNames, k.values.ServerCertificate.ExtraDNSNames...),
		CertType:                          secretsutils.ServerCert,
		SkipPublishingCACertificate:       true,
		IncludeCACertificateInServerChain: true,
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

func (k *kubeAPIServer) reconcileSecretAuthenticationWebhookKubeconfig(ctx context.Context, secret *corev1.Secret) error {
	if k.values.AuthenticationWebhook == nil || len(k.values.AuthenticationWebhook.Kubeconfig) == 0 {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	return apiserver.ReconcileSecretWebhookKubeconfig(ctx, k.client.Client(), secret, k.values.AuthenticationWebhook.Kubeconfig)
}

func (k *kubeAPIServer) reconcileSecretAuthorizationWebhooksKubeconfigs(ctx context.Context, secret *corev1.Secret) error {
	if len(k.values.AuthorizationWebhooks) == 0 {
		// We don't delete the secret here as we don't know its name (as it's unique). Instead, we rely on the usual
		// garbage collection for unique secrets/configmaps.
		return nil
	}

	secret.Data = make(map[string][]byte)

	for _, webhook := range k.values.AuthorizationWebhooks {
		if len(webhook.Kubeconfig) != 0 {
			secret.Data[authorizationWebhookKubeconfigFilename(webhook.Name)] = webhook.Kubeconfig
		}
	}

	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, secret))
}

func authorizationWebhookKubeconfigFilename(name string) string {
	return strings.ToLower(name) + "-kubeconfig.yaml"
}
