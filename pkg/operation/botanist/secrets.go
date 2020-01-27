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
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var wantedCertificateAuthorityConfigs = map[string]*secrets.CertificateSecretConfig{
	v1beta1constants.SecretNameCACluster: {
		Name:       v1beta1constants.SecretNameCACluster,
		CommonName: "kubernetes",
		CertType:   secrets.CACert,
	},
	v1beta1constants.SecretNameCAETCD: {
		Name:       v1beta1constants.SecretNameCAETCD,
		CommonName: "etcd",
		CertType:   secrets.CACert,
	},
	v1beta1constants.SecretNameCAFrontProxy: {
		Name:       v1beta1constants.SecretNameCAFrontProxy,
		CommonName: "front-proxy",
		CertType:   secrets.CACert,
	},
	v1beta1constants.SecretNameCAKubelet: {
		Name:       v1beta1constants.SecretNameCAKubelet,
		CommonName: "kubelet",
		CertType:   secrets.CACert,
	},
	v1beta1constants.SecretNameCAMetricsServer: {
		Name:       v1beta1constants.SecretNameCAMetricsServer,
		CommonName: "metrics-server",
		CertType:   secrets.CACert,
	},
}

var basicAuthSecretAPIServer = &secrets.BasicAuthSecretConfig{
	Name:           common.BasicAuthSecretName,
	Format:         secrets.BasicAuthFormatCSV,
	Username:       "admin",
	PasswordLength: 32,
}

var staticTokenConfig = &secrets.StaticTokenSecretConfig{
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

var browserCerts = sets.NewString(common.GrafanaTLS, common.KibanaTLS, common.PrometheusTLS, common.AlertManagerTLS)

const (
	certificateETCDServer = "etcd-server-tls"
	certificateETCDClient = "etcd-client-tls"

	renewedLabel    = "cert.gardener.cloud/renewed-endpoint"
	oldRenewedLabel = "cert.gardener.cloud/renewed"
)

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func (b *Botanist) generateWantedSecrets(basicAuthAPIServer *secrets.BasicAuth, staticToken *secrets.StaticToken, checkCertificateAuthorities bool, certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
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

		endUserCrtValidity = common.EndUserCrtValidity
	)

	if gardencorev1beta1helper.TaintsHave(b.Seed.Info.Spec.Taints, gardencorev1beta1.SeedTaintDisableDNS) {
		if addr := net.ParseIP(b.APIServerAddress); addr != nil {
			apiServerIPAddresses = append(apiServerIPAddresses, addr)
		} else {
			apiServerCertDNSNames = append(apiServerCertDNSNames, b.APIServerAddress)
		}
	}

	if checkCertificateAuthorities && len(certificateAuthorities) != len(wantedCertificateAuthorityConfigs) {
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAKubelet],
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAFrontProxy],
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},
			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false, b.APIServerAddress),
			},
		},

		// Secret definition for kube-controller-manager server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.KubeControllerManagerServerName,

				CommonName:   v1beta1constants.DeploymentNameKubeControllerManager,
				Organization: nil,
				DNSNames:     kubeControllerManagerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false, b.APIServerAddress),
			},
		},

		// Secret definition for kube-scheduler server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.KubeSchedulerServerName,

				CommonName:   v1beta1constants.DeploymentNameKubeScheduler,
				Organization: nil,
				DNSNames:     kubeSchedulerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},
		},

		// Secret definition for cluster-autoscaler
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: v1beta1constants.DeploymentNameClusterAutoscaler,

				CommonName:   "system:cluster-autoscaler",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false, b.APIServerAddress),
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false, b.APIServerAddress),
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, true, b.APIServerAddress),
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false, b.APIServerAddress),
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false, b.APIServerAddress),
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAKubelet],
			},
		},

		// Secret definition for gardener
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: v1beta1constants.SecretNameGardener,

				CommonName:   gardencorev1beta1.GardenerName,
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, true, b.APIServerAddress),
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, true, b.APIServerAddress),
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
			Name:       v1beta1constants.SecretNameSSHKeyPair,
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
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},

		// Secret definition for vpn-seed (OpenVPN client side)
		&secrets.CertificateSecretConfig{
			Name: "vpn-seed",

			CommonName:   "vpn-seed",
			Organization: nil,
			DNSNames:     []string{},
			IPAddresses:  []net.IP{},

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: certificateETCDServer,

			CommonName:   "etcd-server",
			Organization: nil,
			DNSNames:     etcdCertDNSNames,
			IPAddresses:  nil,

			CertType:  secrets.ServerClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAETCD],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: certificateETCDClient,

			CommonName:   "etcd-client",
			Organization: nil,
			DNSNames:     nil,
			IPAddresses:  nil,

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAETCD],
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
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAMetricsServer],
		},

		// Secret definition for alertmanager (ingress)
		&secrets.CertificateSecretConfig{
			Name: common.AlertManagerTLS,

			CommonName:   "alertmanager",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     b.ComputeAlertManagerHosts(),
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			Validity:  &endUserCrtValidity,
		},

		// Secret definition for grafana (ingress)
		&secrets.CertificateSecretConfig{
			Name: common.GrafanaTLS,

			CommonName:   "grafana",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     b.ComputeGrafanaHosts(),
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			Validity:  &endUserCrtValidity,
		},

		// Secret definition for prometheus (ingress)
		&secrets.CertificateSecretConfig{
			Name: common.PrometheusTLS,

			CommonName:   "prometheus",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     b.ComputePrometheusHosts(),
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			Validity:  &endUserCrtValidity,
		},
	}

	// Secret definition for kubecfg
	var kubecfgToken *secrets.Token
	if staticToken != nil {
		var err error
		kubecfgToken, err = staticToken.GetTokenForUsername(common.KubecfgUsername)
		if err != nil {
			return nil, err
		}
	}

	secretList = append(secretList, &secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name:      common.KubecfgSecretName,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},

		BasicAuth: basicAuthAPIServer,
		Token:     kubecfgToken,

		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: b.Shoot.ComputeAPIServerURL(false, false, b.APIServerAddress),
		},
	})

	// Secret definition for dependency-watchdog-internal-probe
	secretList = append(secretList, &secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name:      common.DependencyWatchdogInternalProbeSecretName,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},

		BasicAuth: basicAuthAPIServer,
		Token:     kubecfgToken,

		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: fmt.Sprintf("%s.%s.svc", v1beta1constants.DeploymentNameKubeAPIServer, b.Shoot.SeedNamespace),
		},
	})

	// Secret definition for dependency-watchdog-external-probe
	secretList = append(secretList, &secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name:      common.DependencyWatchdogExternalProbeSecretName,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},

		BasicAuth: basicAuthAPIServer,
		Token:     kubecfgToken,

		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: b.Shoot.ComputeAPIServerURL(false, true, b.APIServerAddress),
		},
	})

	loggingEnabled := gardenletfeatures.FeatureGate.Enabled(features.Logging)
	if loggingEnabled {
		elasticsearchHosts := []string{"elasticsearch-logging",
			fmt.Sprintf("elasticsearch-logging.%s", b.Shoot.SeedNamespace),
			fmt.Sprintf("elasticsearch-logging.%s.svc", b.Shoot.SeedNamespace),
		}
		secretList = append(secretList,
			&secrets.CertificateSecretConfig{
				Name: common.KibanaTLS,

				CommonName:   "kibana",
				Organization: []string{fmt.Sprintf("%s:logging:ingress", garden.GroupName)},
				DNSNames:     b.ComputeKibanaHosts(),
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
				Validity:  &endUserCrtValidity,
			},
			// Secret for elasticsearch
			&secrets.CertificateSecretConfig{
				Name: "elasticsearch-logging-server",

				CommonName:   "elasticsearch",
				Organization: nil,
				DNSNames:     elasticsearchHosts,
				IPAddresses:  nil,

				CertType:  secrets.ServerClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
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
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
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

// DeploySecrets loads data from the ShootState for secret resources and uses it to deploy kubernetes secret objects in the Shoot's control plane.
func (b *Botanist) DeploySecrets(ctx context.Context) error {
	shootState, err := b.K8sGardenClient.GardenCore().CoreV1alpha1().ShootStates(b.Shoot.Info.Namespace).Get(b.Shoot.Info.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	gardenerResourceDataMap, err := gardencorev1alpha1helper.CreateGardenerResourceDataMap(shootState.Spec.Gardener)
	if err != nil {
		return err
	}
	existingSecretsMap, err := b.fetchExistingSecrets(ctx)
	if err != nil {
		return err
	}

	_, err = b.loadAndDeployCertificateAuthorities(gardenerResourceDataMap, existingSecretsMap, wantedCertificateAuthorityConfigs)
	if err != nil {
		return err
	}

	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		_, err := b.loadAndDeployCSVSecret(ctx, gardenerResourceDataMap, existingSecretsMap, secrets.DataKeyCSV, basicAuthSecretAPIServer, func(name string, data []byte) (secrets.Interface, error) {
			return secrets.LoadBasicAuthFromCSV(name, data)
		})
		if err != nil {
			return err
		}
	}

	staticTokenSecret, err := b.loadAndDeployCSVSecret(ctx, gardenerResourceDataMap, existingSecretsMap, secrets.DataKeyStaticTokenCSV, staticTokenConfig, func(name string, data []byte) (secrets.Interface, error) {
		return secrets.LoadStaticTokenFromCSV(name, data)
	})
	if err != nil {
		return err
	}
	staticToken := staticTokenSecret.(*secrets.StaticToken)

	if err := b.storeAPIServerHealthCheckToken(staticToken); err != nil {
		return err
	}

	if err := b.deployOpenVPNTLSAuthSecret(ctx, existingSecretsMap); err != nil {
		return err
	}

	wantedSecretsList, err := b.generateWantedSecrets(nil, nil, false, nil)
	if err != nil {
		return err
	}

	if err := b.deployShootSecrets(ctx, gardenerResourceDataMap, existingSecretsMap, wantedSecretsList); err != nil {
		return err
	}

	wildcardCert, err := seed.GetWildcardCertificate(ctx, b.K8sSeedClient.Client())
	if err != nil {
		return err
	}

	if wildcardCert != nil {
		// Copy certificate to shoot namespace
		crt := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      wildcardCert.GetName(),
				Namespace: b.Shoot.SeedNamespace,
			},
		}
		if err := kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), crt, func() error {
			crt.Data = wildcardCert.Data
			return nil
		}); err != nil {
			return err
		}
		b.ControlPlaneWildcardCert = crt
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	for name, secret := range b.Secrets {
		b.CheckSums[name] = common.ComputeSecretCheckSum(secret.Data)
	}

	return nil
}

// LoadExistingSecretsData loads data from already existing certificate authorities and secrets in the Shoot's control plane.
// TODO: This function is only necessary because of the addition of the ShootState resource so that data for secrets of existing shoots does not get regenerated
// but is instead loaded into the ShootState. It can be removed later on
func (b *Botanist) LoadExistingSecretsData(ctx context.Context) error {
	shootState, err := b.K8sGardenClient.GardenCore().CoreV1alpha1().ShootStates(b.Shoot.Info.Namespace).Get(b.Shoot.Info.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	gardenerResourceList := shootState.Spec.Gardener

	existingSecrets, err := b.fetchExistingSecrets(ctx)
	if err != nil {
		return err
	}

	if basicAuthSecret, ok := existingSecrets[basicAuthSecretAPIServer.Name]; ok {
		if _, existingResourceData := gardencorev1alpha1helper.GetGardenerResourceData(gardenerResourceList, basicAuthSecretAPIServer.Name); existingResourceData == nil {
			gardenerResourceList = append(gardenerResourceList, gardencorev1alpha1.GardenerResourceData{
				Name: basicAuthSecretAPIServer.Name,
				Data: basicAuthSecret.Data,
			})
		}
	}

	if staticTokenSecret, ok := existingSecrets[staticTokenConfig.Name]; ok {
		if _, existingResourceData := gardencorev1alpha1helper.GetGardenerResourceData(gardenerResourceList, staticTokenConfig.Name); existingResourceData == nil {
			gardenerResourceList = append(gardenerResourceList, gardencorev1alpha1.GardenerResourceData{
				Name: staticTokenConfig.Name,
				Data: staticTokenSecret.Data,
			})
		}
	}

	gardenerResourceList = loadCertificateAuthorities(gardenerResourceList, existingSecrets, wantedCertificateAuthorityConfigs)
	wantedShootSecrets, err := b.generateWantedSecrets(nil, nil, false, nil)
	if err != nil {
		return err
	}

	gardenerResourceList = loadSecrets(gardenerResourceList, existingSecrets, wantedShootSecrets)

	err = kutil.TryUpdate(ctx, retry.DefaultBackoff, b.K8sGardenClient.Client(), shootState, func() error {
		shootState.Spec.Gardener = gardenerResourceList
		return nil
	})

	return err
}

// GenerateSecrets generates Certificate Authorities for the Shoot cluster and uses them
// to sign certificates used by components in the Shoot's control plane. It also generates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
// All of this data is saved in the ShootState resource as it must be persisted for eventual Shoot Control Plane migration
func (b *Botanist) GenerateSecrets(ctx context.Context) error {
	shootState, err := b.K8sGardenClient.GardenCore().CoreV1alpha1().ShootStates(b.Shoot.Info.Namespace).Get(b.Shoot.Info.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	gardenerResourceList := shootState.Spec.Gardener
	gardenerResourceDataMap, err := gardencorev1alpha1helper.CreateGardenerResourceDataMap(gardenerResourceList)
	if err != nil {
		return err
	}

	// If the rotate-kubeconfig operation annotation is set then we delete the existing kubecfg and basic-auth
	// secrets and we also remove their data from gardenerResourceDataMap so that it can be regenerated and re-added to the ShootState.
	// This will trigger the regeneration, incorporating new credentials. After successful deletion of all
	// old secrets we remove the operation annotation.
	if kutil.HasMetaDataAnnotation(b.Shoot.Info, common.ShootOperation, common.ShootOperationRotateKubeconfigCredentials) {
		b.Logger.Infof("Rotating kubeconfig credentials")
		secretsToRegenerate := []string{common.StaticTokenSecretName, common.BasicAuthSecretName, common.KubecfgSecretName}
		for _, secretName := range secretsToRegenerate {
			if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
				return err
			}
			delete(gardenerResourceDataMap, secretName)
		}

		if _, err := kutil.TryUpdateShootAnnotations(b.K8sGardenClient.GardenCore(), retry.DefaultRetry, b.Shoot.Info.ObjectMeta, func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
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
	// generate a new one and want to refresh the kubecfg. If we delete the basic auth secret, we must also remove it from the secrets in the ShootState
	mustDeleteUserCredentialSecrets := !gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info)
	basicAuthSecret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, common.BasicAuthSecretName), basicAuthSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		mustDeleteUserCredentialSecrets = gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info)
	}
	if mustDeleteUserCredentialSecrets {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.BasicAuthSecretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		gardenerResourceList = gardencorev1alpha1helper.RemoveGardenerResourceData(gardenerResourceList, common.BasicAuthSecretName)
		delete(gardenerResourceDataMap, common.BasicAuthSecretName)

		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.KubecfgSecretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
		delete(gardenerResourceDataMap, common.KubecfgSecretName)
	}

	var basicAuthAPIServer *secrets.BasicAuth
	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		basicAuth, err := generateCSVSecretData(gardenerResourceDataMap, secrets.DataKeyCSV, basicAuthSecretAPIServer, func(name string, data []byte) (secrets.Interface, error) {
			return secrets.LoadBasicAuthFromCSV(name, data)
		})
		if err != nil {
			return err
		}
		basicAuthAPIServer = basicAuth.(*secrets.BasicAuth)
	}

	staticToken, err := generateCSVSecretData(gardenerResourceDataMap, secrets.DataKeyStaticTokenCSV, staticTokenConfig, func(name string, data []byte) (secrets.Interface, error) {
		return secrets.LoadStaticTokenFromCSV(name, data)
	})
	if err != nil {
		return err
	}
	staticTokenSecret := staticToken.(*secrets.StaticToken)

	certificateAuthorities, err := secrets.GenerateCertificateAuthoritiesData(gardenerResourceDataMap, wantedCertificateAuthorityConfigs)
	if err != nil {
		return err
	}

	wantedSecretsConfig, err := b.generateWantedSecrets(basicAuthAPIServer, staticTokenSecret, true, certificateAuthorities)
	if err != nil {
		return err
	}

	err = secrets.GeneratePersistedSecrets(gardenerResourceDataMap, wantedSecretsConfig)
	if err != nil {
		return err
	}

	err = kutil.TryUpdate(ctx, retry.DefaultBackoff, b.K8sGardenClient.Client(), shootState, func() error {
		shootState.Spec.Gardener = gardencorev1alpha1helper.AddGardenerResourceDataFromMap(gardenerResourceList, gardenerResourceDataMap)
		return nil
	})

	return err
}

// DeployCloudProviderSecret creates or updates the cloud provider secret in the Shoot namespace
// in the Seed cluster.
func (b *Botanist) DeployCloudProviderSecret(ctx context.Context) error {
	var (
		checksum = common.ComputeSecretCheckSum(b.Shoot.Secret.Data)
		secret   = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
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

	b.Secrets[v1beta1constants.SecretNameCloudProvider] = b.Shoot.Secret
	b.CheckSums[v1beta1constants.SecretNameCloudProvider] = checksum

	return nil
}

func (b *Botanist) loadAndDeployCertificateAuthorities(gardenerResourceDataMap map[string]gardencorev1alpha1.GardenerResourceData, existingSecretsMap map[string]*corev1.Secret, wantedCertificateAuthorityConfigs map[string]*secrets.CertificateSecretConfig) (map[string]*secrets.Certificate, error) {
	generatedSecrets, certificateAuthorities, err := secrets.LoadAndDeployCertificateAuthorities(gardenerResourceDataMap, existingSecretsMap, wantedCertificateAuthorityConfigs, b.Shoot.SeedNamespace, b.K8sSeedClient)
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

func (b *Botanist) storeAPIServerHealthCheckToken(staticToken *secrets.StaticToken) error {
	kubeAPIServerHealthCheckToken, err := staticToken.GetTokenForUsername(common.KubeAPIServerHealthCheck)
	if err != nil {
		return err
	}

	b.APIServerHealthCheckToken = kubeAPIServerHealthCheckToken.Token
	return nil
}

func (b *Botanist) deployShootSecrets(ctx context.Context, gardenerResourceDataMap map[string]gardencorev1alpha1.GardenerResourceData, existingSecretsMap map[string]*corev1.Secret, wantedSecretsList []secrets.ConfigInterface) error {
	deployedClusterSecrets, err := secrets.DeployPersistedSecrets(ctx, gardenerResourceDataMap, wantedSecretsList, existingSecretsMap, b.Shoot.SeedNamespace, b.K8sSeedClient)
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
	secretSuffixSSHKeyPair = v1beta1constants.SecretNameSSHKeyPair
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
			secretName: v1beta1constants.SecretNameSSHKeyPair,
			suffix:     secretSuffixSSHKeyPair,
		},
		{
			secretName:  "monitoring-ingress-credentials-users",
			suffix:      secretSuffixMonitoring,
			annotations: map[string]string{"url": "https://" + b.ComputeGrafanaUsersHost()},
		},
	}

	if gardenletfeatures.FeatureGate.Enabled(features.Logging) {
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
				*metav1.NewControllerRef(b.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
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

func loadCertificateAuthorities(gardenerReourceDataList []gardencorev1alpha1.GardenerResourceData, existingSecretsMap map[string]*corev1.Secret, wantedCertificateAuthorityConfigs map[string]*secrets.CertificateSecretConfig) []gardencorev1alpha1.GardenerResourceData {
	for name := range wantedCertificateAuthorityConfigs {
		if val, ok := existingSecretsMap[name]; ok {
			if _, existingResourceData := gardencorev1alpha1helper.GetGardenerResourceData(gardenerReourceDataList, name); existingResourceData == nil {
				gardenerReourceDataList = append(gardenerReourceDataList, gardencorev1alpha1.GardenerResourceData{
					Name: name,
					Data: val.Data,
				})
			}
		}
	}
	return gardenerReourceDataList
}

func loadSecrets(gardenerReourceDataList []gardencorev1alpha1.GardenerResourceData, existingSecretsMap map[string]*corev1.Secret, wantedSecretsList []secrets.ConfigInterface) []gardencorev1alpha1.GardenerResourceData {
	for _, secreteConfig := range wantedSecretsList {
		name := secreteConfig.GetName()
		if val, ok := existingSecretsMap[name]; ok {
			if _, existingResourceData := gardencorev1alpha1helper.GetGardenerResourceData(gardenerReourceDataList, name); existingResourceData == nil {
				gardenerReourceDataList = append(gardenerReourceDataList, gardencorev1alpha1.GardenerResourceData{
					Name: name,
					Data: val.Data,
				})
			}
		}
	}

	return gardenerReourceDataList
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

func dnsNamesForService(name, namespace string) []string {
	return []string{
		name,
		fmt.Sprintf("%s.%s", name, namespace),
		fmt.Sprintf("%s.%s.svc", name, namespace),
		fmt.Sprintf("%s.%s.svc.%s", name, namespace, gardencorev1beta1.DefaultDomain),
	}
}

func dnsNamesForEtcd(namespace string) []string {
	names := []string{
		fmt.Sprintf("%s-0", v1beta1constants.StatefulSetNameETCDMain),
		fmt.Sprintf("%s-0", v1beta1constants.StatefulSetNameETCDEvents),
	}
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", v1beta1constants.StatefulSetNameETCDMain), namespace)...)
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", v1beta1constants.StatefulSetNameETCDEvents), namespace)...)
	return names
}

func markSecretDataForRegeneration(gardenerResourceDataMap map[string]gardencorev1alpha1.GardenerResourceData, name string) {

}

type loadDataFromCSV func(name string, data []byte) (secrets.Interface, error)

func generateCSVSecretData(gardenerResourceDataMap map[string]gardencorev1alpha1.GardenerResourceData, dataKey string, config secrets.ConfigInterface, loadDataFunc loadDataFromCSV) (secrets.Interface, error) {
	name := config.GetName()

	if existingResourceData, ok := gardenerResourceDataMap[name]; ok {
		secretInterface, err := loadDataFunc(name, existingResourceData.Data[dataKey])
		if err != nil {
			return nil, err
		}
		return secretInterface, nil
	}

	csvSecretDataObj, err := config.Generate()
	if err != nil {
		return nil, err
	}
	csvSecretData := csvSecretDataObj.SecretData()
	gardenerResourceData := gardencorev1alpha1.GardenerResourceData{Name: name, Data: csvSecretData}
	gardenerResourceDataMap[name] = gardenerResourceData

	secretInterface, err := loadDataFunc(name, csvSecretData[dataKey])
	if err != nil {
		return nil, err
	}
	return secretInterface, nil
}

func (b *Botanist) loadAndDeployCSVSecret(ctx context.Context, gardenerResourceDataMap map[string]gardencorev1alpha1.GardenerResourceData, existingSecretsMap map[string]*corev1.Secret, dataKey string, config secrets.ConfigInterface, loadDataFunc loadDataFromCSV) (secrets.Interface, error) {
	name := config.GetName()
	existingResourceData, ok := gardenerResourceDataMap[name]
	if !ok {
		return nil, fmt.Errorf("missing data for %s secret", name)
	}

	secretInterface, err := loadDataFunc(name, existingResourceData.Data[dataKey])
	if err != nil {
		return nil, err
	}

	if existingSecret, ok := existingSecretsMap[name]; ok {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.Secrets[name] = existingSecret

		return secretInterface, nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: b.Shoot.SeedNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: existingResourceData.Data,
	}
	if err := b.K8sSeedClient.Client().Create(ctx, secret); err != nil {
		return nil, err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.Secrets[name] = secret

	return secretInterface, nil
}
