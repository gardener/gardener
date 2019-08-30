// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"fmt"
	"net"
	"os/exec"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenv1beta1helper "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var wantedCertificateAuthorities = map[string]*secrets.CertificateSecretConfig{
	gardencorev1alpha1.SecretNameCACluster: {
		Name:       gardencorev1alpha1.SecretNameCACluster,
		CommonName: "kubernetes",
		CertType:   secrets.CACert,
	},
	gardencorev1alpha1.SecretNameCAETCD: {
		Name:       gardencorev1alpha1.SecretNameCAETCD,
		CommonName: "etcd",
		CertType:   secrets.CACert,
	},
	gardencorev1alpha1.SecretNameCAFrontProxy: {
		Name:       gardencorev1alpha1.SecretNameCAFrontProxy,
		CommonName: "front-proxy",
		CertType:   secrets.CACert,
	},
	gardencorev1alpha1.SecretNameCAKubelet: {
		Name:       gardencorev1alpha1.SecretNameCAKubelet,
		CommonName: "kubelet",
		CertType:   secrets.CACert,
	},
	gardencorev1alpha1.SecretNameCAMetricsServer: {
		Name:       gardencorev1alpha1.SecretNameCAMetricsServer,
		CommonName: "metrics-server",
		CertType:   secrets.CACert,
	},
}

const (
	certificateETCDServer = "etcd-server-tls"
	certificateETCDClient = "etcd-client-tls"
)

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func (b *Botanist) generateWantedSecrets(basicAuthAPIServer *secrets.BasicAuth, staticToken *secrets.StaticToken, certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
	var (
		apiServerIPAddresses = []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP(common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 1)),
		}
		apiServerCertDNSNames = append([]string{
			"kube-apiserver",
			fmt.Sprintf("kube-apiserver.%s", b.Shoot.SeedNamespace),
			fmt.Sprintf("kube-apiserver.%s.svc", b.Shoot.SeedNamespace),
			common.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		}, dnsNamesForService("kubernetes", "default")...)

		kubeControllerManagerCertDNSNames = dnsNamesForService("kube-controller-manager", b.Shoot.SeedNamespace)
		kubeSchedulerCertDNSNames         = dnsNamesForService("kube-scheduler", b.Shoot.SeedNamespace)

		etcdCertDNSNames = dnsNamesForEtcd(b.Shoot.SeedNamespace)
	)

	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	if b.Shoot.ExternalClusterDomain != nil {
		apiServerCertDNSNames = append(apiServerCertDNSNames, *(b.Shoot.Info.Spec.DNS.Domain), common.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain))
	}

	secretList := []secrets.ConfigInterface{
		// Secret definition for kube-apiserver
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-apiserver",

				CommonName:   user.APIServerUser,
				Organization: nil,
				DNSNames:     apiServerCertDNSNames,
				IPAddresses:  apiServerIPAddresses,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},
		},

		// Secret definition for kube-apiserver to kubelets communication
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-apiserver-kubelet",

				CommonName:   "system:kube-apiserver:kubelet",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCAKubelet],
			},
		},

		// Secret definition for kube-aggregator
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-aggregator",

				CommonName:   "system:kube-aggregator",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCAFrontProxy],
			},
		},

		// Secret definition for kube-controller-manager
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-controller-manager",

				CommonName:   user.KubeControllerManager,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},
			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
			},
		},

		// Secret definition for kube-controller-manager server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.KubeControllerManagerServerName,

				CommonName:   gardencorev1alpha1.DeploymentNameKubeControllerManager,
				Organization: nil,
				DNSNames:     kubeControllerManagerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},
		},

		// Secret definition for kube-scheduler
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-scheduler",

				CommonName:   user.KubeScheduler,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
			},
		},

		// Secret definition for kube-scheduler server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.KubeSchedulerServerName,

				CommonName:   gardencorev1alpha1.DeploymentNameKubeScheduler,
				Organization: nil,
				DNSNames:     kubeSchedulerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},
		},

		// Secret definition for cluster-autoscaler
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: gardencorev1alpha1.DeploymentNameClusterAutoscaler,

				CommonName:   "system:cluster-autoscaler",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
			},
		},

		// Secret definition for gardener-resource-manager
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "gardener-resource-manager",

				CommonName:   "gardener.cloud:system:gardener-resource-manager",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
			},
		},

		// Secret definition for kube-proxy
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-proxy",

				CommonName:   user.KubeProxy,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, true),
			},
		},

		// Secret definition for kube-state-metrics
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-state-metrics",

				CommonName:   fmt.Sprintf("%s:monitoring:kube-state-metrics", garden.GroupName),
				Organization: []string{fmt.Sprintf("%s:monitoring", garden.GroupName)},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
			},
		},

		// Secret definition for prometheus
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "prometheus",

				CommonName:   fmt.Sprintf("%s:monitoring:prometheus", garden.GroupName),
				Organization: []string{fmt.Sprintf("%s:monitoring", garden.GroupName)},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
			},
		},

		// Secret definition for prometheus to kubelets communication
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "prometheus-kubelet",

				CommonName:   fmt.Sprintf("%s:monitoring:prometheus", garden.GroupName),
				Organization: []string{fmt.Sprintf("%s:monitoring", garden.GroupName)},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCAKubelet],
			},
		},

		// Secret definition for gardener
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: gardencorev1alpha1.SecretNameGardener,

				CommonName:   gardenv1beta1.GardenerName,
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, true),
			},
		},

		// Secret definition for cloud-config-downloader
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "cloud-config-downloader",

				CommonName:   "cloud-config-downloader",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, true),
			},
		},

		// Secret definition for monitoring
		&secrets.BasicAuthSecretConfig{
			Name:   "monitoring-ingress-credentials",
			Format: secrets.BasicAuthFormatNormal,

			Username:       "admin",
			PasswordLength: 32,
		},

		// Secret definition for monitoring for shoot owners
		&secrets.BasicAuthSecretConfig{
			Name:   "monitoring-ingress-credentials-users",
			Format: secrets.BasicAuthFormatNormal,

			Username:       "admin",
			PasswordLength: 32,
		},

		// Secret definition for ssh-keypair
		&secrets.RSASecretConfig{
			Name:       gardencorev1alpha1.SecretNameSSHKeyPair,
			Bits:       4096,
			UsedForSSH: true,
		},

		// Secret definition for service-account-key
		&secrets.RSASecretConfig{
			Name:       "service-account-key",
			Bits:       4096,
			UsedForSSH: false,
		},

		// Secret definition for vpn-shoot (OpenVPN server side)
		&secrets.CertificateSecretConfig{
			Name: "vpn-shoot",

			CommonName:   "vpn-shoot",
			Organization: nil,
			DNSNames:     []string{},
			IPAddresses:  []net.IP{},

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
		},

		// Secret definition for vpn-seed (OpenVPN client side)
		&secrets.CertificateSecretConfig{
			Name: "vpn-seed",

			CommonName:   "vpn-seed",
			Organization: nil,
			DNSNames:     []string{},
			IPAddresses:  []net.IP{},

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: certificateETCDServer,

			CommonName:   "etcd-server",
			Organization: nil,
			DNSNames:     etcdCertDNSNames,
			IPAddresses:  nil,

			CertType:  secrets.ServerClientCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCAETCD],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: certificateETCDClient,

			CommonName:   "etcd-client",
			Organization: nil,
			DNSNames:     nil,
			IPAddresses:  nil,

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCAETCD],
		},

		// Secret definition for metrics-server
		&secrets.CertificateSecretConfig{
			Name: "metrics-server",

			CommonName:   "metrics-server",
			Organization: nil,
			DNSNames: []string{
				"metrics-server",
				fmt.Sprintf("metrics-server.%s", metav1.NamespaceSystem),
				fmt.Sprintf("metrics-server.%s.svc", metav1.NamespaceSystem),
			},
			IPAddresses: nil,

			CertType:  secrets.ServerClientCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCAMetricsServer],
		},

		// Secret definition for alertmanager (ingress)
		&secrets.CertificateSecretConfig{
			Name: "alertmanager-tls",

			CommonName:   "alertmanager",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{b.ComputeAlertManagerHost()},
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
		},

		// Secret definition for grafana (ingress)
		&secrets.CertificateSecretConfig{
			Name: "grafana-tls",

			CommonName:   "grafana",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     b.ComputeGrafanaHosts(),
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
		},

		// Secret definition for prometheus (ingress)
		&secrets.CertificateSecretConfig{
			Name: "prometheus-tls",

			CommonName:   "prometheus",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{b.ComputePrometheusHost()},
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
		},
	}

	// Secret definition for kubecfg
	kubecfgToken, err := staticToken.GetTokenForUsername(common.KubecfgUsername)
	if err != nil {
		return nil, err
	}

	secretList = append(secretList, &secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name:      common.KubecfgSecretName,
			SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
		},

		BasicAuth: basicAuthAPIServer,
		Token:     kubecfgToken,

		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: b.Shoot.ComputeAPIServerURL(false, false),
		},
	})

	loggingEnabled := controllermanagerfeatures.FeatureGate.Enabled(features.Logging)
	if loggingEnabled {
		elasticsearchHosts := []string{"elasticsearch-logging",
			fmt.Sprintf("elasticsearch-logging.%s", b.Shoot.SeedNamespace),
			fmt.Sprintf("elasticsearch-logging.%s.svc", b.Shoot.SeedNamespace),
		}
		secretList = append(secretList,
			&secrets.CertificateSecretConfig{
				Name: "kibana-tls",

				CommonName:   "kibana",
				Organization: []string{fmt.Sprintf("%s:logging:ingress", garden.GroupName)},
				DNSNames:     []string{b.ComputeKibanaHost()},
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},
			// Secret for elasticsearch
			&secrets.CertificateSecretConfig{
				Name: "elasticsearch-logging-server",

				CommonName:   "elasticsearch",
				Organization: nil,
				DNSNames:     elasticsearchHosts,
				IPAddresses:  nil,

				CertType:  secrets.ServerClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
				PKCS:      secrets.PKCS8,
			},
			// Secret definition for logging
			&secrets.BasicAuthSecretConfig{
				Name:   "logging-ingress-credentials-users",
				Format: secrets.BasicAuthFormatNormal,

				Username:                  "user",
				PasswordLength:            32,
				BcryptPasswordHashRequest: true,
			},
			&secrets.BasicAuthSecretConfig{
				Name:   "logging-ingress-credentials",
				Format: secrets.BasicAuthFormatNormal,

				Username:                  "admin",
				PasswordLength:            32,
				BcryptPasswordHashRequest: true,
			},
			&secrets.CertificateSecretConfig{
				Name: "sg-admin-client",

				CommonName:   "elasticsearch-logging",
				Organization: nil,
				DNSNames:     elasticsearchHosts,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
				PKCS:      secrets.PKCS8,
			},
			&secrets.BasicAuthSecretConfig{
				Name:   "kibana-logging-sg-credentials",
				Format: secrets.BasicAuthFormatNormal,

				Username:                  "kibanaserver",
				PasswordLength:            32,
				BcryptPasswordHashRequest: true,
			},
			&secrets.BasicAuthSecretConfig{
				Name:   "curator-sg-credentials",
				Format: secrets.BasicAuthFormatNormal,

				Username:                  "curator",
				PasswordLength:            32,
				BcryptPasswordHashRequest: true,
			},
			&secrets.BasicAuthSecretConfig{
				Name:   "admin-sg-credentials",
				Format: secrets.BasicAuthFormatNormal,

				Username:                  "admin",
				PasswordLength:            32,
				BcryptPasswordHashRequest: true,
			},
		)
	}

	return secretList, nil
}

// DeploySecrets creates a CA certificate for the Shoot cluster and uses it to sign the server certificate
// used by the kube-apiserver, and all client certificates used for communcation. It also creates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
func (b *Botanist) DeploySecrets(ctx context.Context) error {
	// If the rotate-kubeconfig operation annotation is set then we delete the existing kubecfg and basic-auth
	// secrets. This will trigger the regeneration, incorporating new credentials. After successful deletion of all
	// old secrets we remove the operation annotation.
	if kutil.HasMetaDataAnnotation(b.Shoot.Info, common.ShootOperation, common.ShootOperationRotateKubeconfigCredentials) {
		b.Logger.Infof("Rotating kubeconfig credentials")

		for _, secretName := range []string{common.StaticTokenSecretName, common.BasicAuthSecretName, common.KubecfgSecretName} {
			if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
				return err
			}
		}

		if _, err := kutil.TryUpdateShootAnnotations(b.K8sGardenClient.Garden(), retry.DefaultRetry, b.Shoot.Info.ObjectMeta, func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			delete(shoot.Annotations, common.ShootOperation)
			return shoot, nil
		}); err != nil {
			return err
		}
	}

	// Basic authentication can be enabled or disabled. In both cases we have to check whether the basic auth secret in the shoot
	// namespace in the seed exists because we might need to regenerate the end-users kubecfg that is used to communicate with the
	// shoot cluster. If basic auth is enabled then we want to store the credentials inside the kubeconfig, if it's disabled then
	// we want to remove old credentials out of it.
	// Thus, if the basic-auth secret is not found and basic auth is disabled then we don't need to refresh anything. If it's found
	// then we have to delete it and refresh the kubecfg (which is triggered by deleting the kubecfg secret). The other cases are
	// the opposite: Basic auth is enabled and basic-auth secret found: no deletion required. If the secret is not found then we
	// generate a new one and want to refresh the kubecfg.
	mustDeleteUserCredentialSecrets := !gardenv1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info)
	basicAuthSecret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, common.BasicAuthSecretName), basicAuthSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		mustDeleteUserCredentialSecrets = gardenv1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info)
	}
	if mustDeleteUserCredentialSecrets {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.BasicAuthSecretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.KubecfgSecretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	existingSecretsMap, err := b.fetchExistingSecrets(ctx)
	if err != nil {
		return err
	}

	certificateAuthorities, err := b.generateCertificateAuthorities(existingSecretsMap)
	if err != nil {
		return err
	}

	var basicAuthAPIServer *secrets.BasicAuth
	if gardenv1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		basicAuthAPIServer, err = b.generateBasicAuthAPIServer(ctx, existingSecretsMap)
		if err != nil {
			return err
		}
	}

	staticToken, err := b.generateStaticToken(ctx, existingSecretsMap)
	if err != nil {
		return err
	}

	if err := b.deployOpenVPNTLSAuthSecret(ctx, existingSecretsMap); err != nil {
		return err
	}

	wantedSecretsList, err := b.generateWantedSecrets(basicAuthAPIServer, staticToken, certificateAuthorities)
	if err != nil {
		return err
	}

	if err := b.generateShootSecrets(ctx, existingSecretsMap, wantedSecretsList); err != nil {
		return err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	for name, secret := range b.Secrets {
		b.CheckSums[name] = common.ComputeSecretCheckSum(secret.Data)
	}

	return nil
}

// DeployCloudProviderSecret creates or updates the cloud provider secret in the Shoot namespace
// in the Seed cluster.
func (b *Botanist) DeployCloudProviderSecret(ctx context.Context) error {
	var (
		checksum = common.ComputeSecretCheckSum(b.Shoot.Secret.Data)
		secret   = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	if err := kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
		secret.Annotations = map[string]string{
			"checksum/data": checksum,
		}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = b.Shoot.Secret.Data
		return nil
	}); err != nil {
		return err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[gardencorev1alpha1.SecretNameCloudProvider] = b.Shoot.Secret
	b.CheckSums[gardencorev1alpha1.SecretNameCloudProvider] = checksum

	return nil
}

func (b *Botanist) fetchExistingSecrets(ctx context.Context) (map[string]*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	if err := b.K8sSeedClient.Client().List(ctx, secretList, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return nil, err
	}

	existingSecretsMap := make(map[string]*corev1.Secret, len(secretList.Items))
	for _, secret := range secretList.Items {
		secretObj := secret
		existingSecretsMap[secret.Name] = &secretObj
	}

	return existingSecretsMap, nil
}

func (b *Botanist) generateCertificateAuthorities(existingSecretsMap map[string]*corev1.Secret) (map[string]*secrets.Certificate, error) {
	generatedSecrets, certificateAuthorities, err := secrets.GenerateCertificateAuthorities(b.K8sSeedClient, existingSecretsMap, wantedCertificateAuthorities, b.Shoot.SeedNamespace)
	if err != nil {
		return nil, err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	for secretName, caSecret := range generatedSecrets {
		b.Secrets[secretName] = caSecret
	}

	return certificateAuthorities, nil
}

func (b *Botanist) generateBasicAuthAPIServer(ctx context.Context, existingSecretsMap map[string]*corev1.Secret) (*secrets.BasicAuth, error) {
	basicAuthSecretAPIServer := &secrets.BasicAuthSecretConfig{
		Name:           common.BasicAuthSecretName,
		Format:         secrets.BasicAuthFormatCSV,
		Username:       "admin",
		PasswordLength: 32,
	}

	if existingSecret, ok := existingSecretsMap[basicAuthSecretAPIServer.Name]; ok {
		basicAuth, err := secrets.LoadBasicAuthFromCSV(basicAuthSecretAPIServer.Name, existingSecret.Data[secrets.DataKeyCSV])
		if err != nil {
			return nil, err
		}

		b.mutex.Lock()
		defer b.mutex.Unlock()

		b.Secrets[basicAuthSecretAPIServer.Name] = existingSecret

		return basicAuth, nil
	}

	basicAuth, err := basicAuthSecretAPIServer.Generate()
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      basicAuthSecretAPIServer.Name,
			Namespace: b.Shoot.SeedNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: basicAuth.SecretData(),
	}
	if err := b.K8sSeedClient.Client().Create(ctx, secret); err != nil {
		return nil, err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[basicAuthSecretAPIServer.Name] = secret

	return basicAuth.(*secrets.BasicAuth), nil
}

func (b *Botanist) generateStaticToken(ctx context.Context, existingSecretsMap map[string]*corev1.Secret) (*secrets.StaticToken, error) {
	staticTokenConfig := &secrets.StaticTokenSecretConfig{
		Name: common.StaticTokenSecretName,
		Tokens: []secrets.TokenConfig{
			{
				Username: common.KubecfgUsername,
				UserID:   common.KubecfgUsername,
				Groups:   []string{user.SystemPrivilegedGroup},
			},
			{
				Username: common.KubeAPIServerHealthCheck,
				UserID:   common.KubeAPIServerHealthCheck,
			},
		},
	}

	if existingSecret, ok := existingSecretsMap[staticTokenConfig.Name]; ok {
		staticToken, err := secrets.LoadStaticTokenFromCSV(staticTokenConfig.Name, existingSecret.Data[secrets.DataKeyStaticTokenCSV])
		if err != nil {
			return nil, err
		}

		b.mutex.Lock()
		defer b.mutex.Unlock()

		b.Secrets[staticTokenConfig.Name] = existingSecret

		if err := b.storeAPIServerHealthCheckToken(staticToken); err != nil {
			return nil, err
		}

		return staticToken, nil
	}

	staticToken, err := staticTokenConfig.Generate()
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      staticTokenConfig.Name,
			Namespace: b.Shoot.SeedNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: staticToken.SecretData(),
	}
	if err := b.K8sSeedClient.Client().Create(ctx, secret); err != nil {
		return nil, err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[staticTokenConfig.Name] = secret

	staticTokenObj := staticToken.(*secrets.StaticToken)

	if err := b.storeAPIServerHealthCheckToken(staticTokenObj); err != nil {
		return nil, err
	}

	return staticTokenObj, nil
}

func (b *Botanist) storeAPIServerHealthCheckToken(staticToken *secrets.StaticToken) error {
	kubeAPIServerHealthCheckToken, err := staticToken.GetTokenForUsername(common.KubeAPIServerHealthCheck)
	if err != nil {
		return err
	}

	b.APIServerHealthCheckToken = kubeAPIServerHealthCheckToken.Token
	return nil
}

func (b *Botanist) generateShootSecrets(ctx context.Context, existingSecretsMap map[string]*corev1.Secret, wantedSecretsList []secrets.ConfigInterface) error {
	deployedClusterSecrets, err := secrets.GenerateClusterSecrets(ctx, b.K8sSeedClient, existingSecretsMap, wantedSecretsList, b.Shoot.SeedNamespace)
	if err != nil {
		return err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	for secretName, secret := range deployedClusterSecrets {
		b.Secrets[secretName] = secret
	}

	return nil
}

const (
	secretSuffixKubeConfig = "kubeconfig"
	secretSuffixSSHKeyPair = gardencorev1alpha1.SecretNameSSHKeyPair
	secretSuffixMonitoring = "monitoring"
	secretSuffixLogging    = "logging"
)

func computeProjectSecretName(shootName, suffix string) string {
	return fmt.Sprintf("%s.%s", shootName, suffix)
}

type projectSecret struct {
	secretName  string
	suffix      string
	annotations map[string]string
}

// SyncShootCredentialsToGarden copies the kubeconfig generated for the user, the SSH keypair to
// the project namespace in the Garden cluster and the monitoring credentials for the
// user-facing monitoring stack are also copied.
func (b *Botanist) SyncShootCredentialsToGarden(ctx context.Context) error {
	kubecfgURL := common.GetAPIServerDomain(b.Shoot.InternalClusterDomain)
	if b.Shoot.ExternalClusterDomain != nil {
		kubecfgURL = common.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain)
	}

	projectSecrets := []projectSecret{
		{
			secretName:  common.KubecfgSecretName,
			suffix:      secretSuffixKubeConfig,
			annotations: map[string]string{"url": "https://" + kubecfgURL},
		},
		{
			secretName: gardencorev1alpha1.SecretNameSSHKeyPair,
			suffix:     secretSuffixSSHKeyPair,
		},
		{
			secretName:  "monitoring-ingress-credentials-users",
			suffix:      secretSuffixMonitoring,
			annotations: map[string]string{"url": "https://" + b.ComputeGrafanaUsersHost()},
		},
	}

	if controllermanagerfeatures.FeatureGate.Enabled(features.Logging) {
		projectSecrets = append(projectSecrets, projectSecret{
			secretName:  "logging-ingress-credentials-users",
			suffix:      secretSuffixLogging,
			annotations: map[string]string{"url": "https://" + b.ComputeKibanaHost()},
		})
	}

	for _, projectSecret := range projectSecrets {
		secretObj := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      computeProjectSecretName(b.Shoot.Info.Name, projectSecret.suffix),
				Namespace: b.Shoot.Info.Namespace,
			},
		}

		if err := kutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), secretObj, func() error {
			secretObj.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(b.Shoot.Info, gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")),
			}
			secretObj.Annotations = projectSecret.annotations
			secretObj.Type = corev1.SecretTypeOpaque
			secretObj.Data = b.Secrets[projectSecret.secretName].Data
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (b *Botanist) deployOpenVPNTLSAuthSecret(ctx context.Context, existingSecretsMap map[string]*corev1.Secret) error {
	name := "vpn-seed-tlsauth"
	if tlsAuthSecret, ok := existingSecretsMap[name]; ok {
		b.mutex.Lock()
		defer b.mutex.Unlock()

		b.Secrets[name] = tlsAuthSecret
		return nil
	}

	tlsAuthKey, err := generateOpenVPNTLSAuth()
	if err != nil {
		return fmt.Errorf("error while creating openvpn tls auth secret: %v", err)
	}

	data := map[string][]byte{
		"vpn.tlsauth": tlsAuthKey,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: b.Shoot.SeedNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := b.K8sSeedClient.Client().Create(ctx, secret); err != nil {
		return err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[name] = secret

	return nil
}

func generateOpenVPNTLSAuth() ([]byte, error) {
	var (
		out bytes.Buffer
		cmd = exec.Command("openvpn", "--genkey", "--secret", "/dev/stdout")
	)

	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func gardenEtcdEncryptionSecretName(shootName string) string {
	return fmt.Sprintf("%s.%s", shootName, common.EtcdEncryptionSecretName)
}

func dnsNamesForService(name, namespace string) []string {
	return []string{
		name,
		fmt.Sprintf("%s.%s", name, namespace),
		fmt.Sprintf("%s.%s.svc", name, namespace),
		fmt.Sprintf("%s.%s.svc.%s", name, namespace, gardenv1beta1.DefaultDomain),
	}
}

func dnsNamesForEtcd(namespace string) []string {
	names := []string{
		fmt.Sprintf("%s-0", gardencorev1alpha1.StatefulSetNameETCDMain),
		fmt.Sprintf("%s-0", gardencorev1alpha1.StatefulSetNameETCDEvents),
	}
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", gardencorev1alpha1.StatefulSetNameETCDMain), namespace)...)
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", gardencorev1alpha1.StatefulSetNameETCDEvents), namespace)...)
	return names
}
