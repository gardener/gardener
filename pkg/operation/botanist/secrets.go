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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var wantedCertificateAuthorities = map[string]*secrets.CertificateSecretConfig{
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

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func (b *Botanist) generateWantedSecrets(basicAuthAPIServer *secrets.BasicAuth, staticToken *secrets.StaticToken, certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
	var (
		apiServerIPAddresses = []net.IP{
			net.ParseIP("127.0.0.1"),
			b.Shoot.Networks.APIServer,
		}
		apiServerCertDNSNames = append([]string{
			"kube-apiserver",
			fmt.Sprintf("kube-apiserver.%s", b.Shoot.SeedNamespace),
			fmt.Sprintf("kube-apiserver.%s.svc", b.Shoot.SeedNamespace),
			common.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		}, dnsNamesForService("kubernetes", "default")...)

		kubeControllerManagerCertDNSNames = dnsNamesForService("kube-controller-manager", b.Shoot.SeedNamespace)
		kubeSchedulerCertDNSNames         = dnsNamesForService("kube-scheduler", b.Shoot.SeedNamespace)

		konnectivityServerDNSNames = append([]string{
			common.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		}, dnsNamesForService(common.KonnectivityServerCertName, b.Shoot.SeedNamespace)...)

		etcdCertDNSNames = dnsNamesForEtcd(b.Shoot.SeedNamespace)

		endUserCrtValidity = common.EndUserCrtValidity
	)

	if !b.Seed.Info.Spec.Settings.ShootDNS.Enabled {
		if addr := net.ParseIP(b.APIServerAddress); addr != nil {
			apiServerIPAddresses = append(apiServerIPAddresses, addr)
		} else {
			apiServerCertDNSNames = append(apiServerCertDNSNames, b.APIServerAddress)
		}
	}

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
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(true),
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
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(true),
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
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(true),
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
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(true),
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
				APIServerURL: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
			},
		},

		// Secret definition for kube-state-metrics
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-state-metrics",

				CommonName:   "gardener.cloud:monitoring:kube-state-metrics",
				Organization: []string{"gardener.cloud:monitoring"},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(true),
			},
		},

		// Secret definition for prometheus
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "prometheus",

				CommonName:   "gardener.cloud:monitoring:prometheus",
				Organization: []string{"gardener.cloud:monitoring"},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(true),
			},
		},

		// Secret definition for prometheus to kubelets communication
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "prometheus-kubelet",

				CommonName:   "gardener.cloud:monitoring:prometheus",
				Organization: []string{"gardener.cloud:monitoring"},
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
				APIServerURL: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
			},
		},
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: v1beta1constants.SecretNameGardenerInternal,

				CommonName:   gardencorev1beta1.GardenerName,
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(false),
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
				APIServerURL: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
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

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: common.EtcdServerTLS,

			CommonName:   "etcd-server",
			Organization: nil,
			DNSNames:     etcdCertDNSNames,
			IPAddresses:  nil,

			CertType:  secrets.ServerClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAETCD],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: common.EtcdClientTLS,

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
			Organization: []string{"gardener.cloud:monitoring:ingress"},
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
			Organization: []string{"gardener.cloud:monitoring:ingress"},
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
			Organization: []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:     b.ComputePrometheusHosts(),
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			Validity:  &endUserCrtValidity,
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
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},

		BasicAuth: basicAuthAPIServer,
		Token:     kubecfgToken,

		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, false),
		},
	})

	// Secret definitions for dependency-watchdog-internal and external probes
	secretList = append(secretList, &secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name: common.DependencyWatchdogInternalProbeSecretName,

			CommonName:   common.DependencyWatchdogUserName,
			Organization: nil,
			DNSNames:     nil,
			IPAddresses:  nil,

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},
		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: b.Shoot.ComputeInClusterAPIServerAddress(false),
		},
	}, &secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &secrets.CertificateSecretConfig{
			Name: common.DependencyWatchdogExternalProbeSecretName,

			CommonName:   common.DependencyWatchdogUserName,
			Organization: nil,
			DNSNames:     nil,
			IPAddresses:  nil,

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
		},
		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  b.Shoot.SeedNamespace,
			APIServerURL: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
		},
	})

	if b.Shoot.KonnectivityTunnelEnabled {
		konnectivityServerToken, err := staticToken.GetTokenForUsername(common.KonnectivityServerUserName)
		if err != nil {
			return nil, err
		}

		// Secret definitions for konnectivity-server and konnectivity Agent
		secretList = append(secretList,
			&secrets.ControlPlaneSecretConfig{
				CertificateSecretConfig: &secrets.CertificateSecretConfig{
					Name:      common.KonnectivityServerKubeconfig,
					SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
				},

				BasicAuth: basicAuthAPIServer,
				Token:     konnectivityServerToken,

				KubeConfigRequest: &secrets.KubeConfigRequest{
					ClusterName:  b.Shoot.SeedNamespace,
					APIServerURL: fmt.Sprintf("%s.%s", v1beta1constants.DeploymentNameKubeAPIServer, b.Shoot.SeedNamespace),
				},
			},
			&secrets.ControlPlaneSecretConfig{
				CertificateSecretConfig: &secrets.CertificateSecretConfig{
					Name:       common.KonnectivityServerCertName,
					CommonName: common.KonnectivityServerCertName,
					DNSNames:   konnectivityServerDNSNames,

					CertType:  secrets.ServerCert,
					SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
				},
			})
	} else {
		secretList = append(secretList,
			// Secret definition for vpn-shoot (OpenVPN server side)
			&secrets.CertificateSecretConfig{
				Name:       "vpn-shoot",
				CommonName: "vpn-shoot",
				CertType:   secrets.ServerCert,
				SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			// Secret definition for vpn-seed (OpenVPN client side)
			&secrets.CertificateSecretConfig{
				Name:       "vpn-seed",
				CommonName: "vpn-seed",
				CertType:   secrets.ClientCert,
				SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCACluster],
			})
	}

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
				Organization: []string{"gardener.cloud:logging:ingress"},
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

// DeploySecrets creates a CA certificate for the Shoot cluster and uses it to sign the server certificate
// used by the kube-apiserver, and all client certificates used for communication. It also creates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
func (b *Botanist) DeploySecrets(ctx context.Context) error {
	// If the rotate-kubeconfig operation annotation is set then we delete the existing kubecfg and basic-auth
	// secrets. This will trigger the regeneration, incorporating new credentials. After successful deletion of all
	// old secrets we remove the operation annotation.
	if val, ok := common.GetShootOperationAnnotation(b.Shoot.Info.Annotations); ok && val == common.ShootOperationRotateKubeconfigCredentials {
		b.Logger.Infof("Rotating kubeconfig credentials")

		for _, secretName := range []string{common.StaticTokenSecretName, common.BasicAuthSecretName, common.KubecfgSecretName} {
			if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
				return err
			}
		}

		if _, err := kutil.TryUpdateShootAnnotations(b.K8sGardenClient.GardenCore(), retry.DefaultRetry, b.Shoot.Info.ObjectMeta, func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			delete(shoot.Annotations, v1beta1constants.GardenerOperation)
			delete(shoot.Annotations, common.ShootOperationDeprecated)
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
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: common.KubecfgSecretName, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	// The kubeconfig secrets for the dependency-watchdog probes have now stopped using token to avoid the need for reconciliation
	// on rotation. The old secrets containing tokens must be removed so that new secrets without token can be generated.
	for _, secretName := range []string{common.DependencyWatchdogInternalProbeSecretName, common.DependencyWatchdogExternalProbeSecretName} {
		secret := &corev1.Secret{}
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, secretName), secret); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if _, ok := secret.Data[secrets.DataKeyToken]; ok {
			// The kubeconfig uses bearer token. Delete it to regenerate the new kubeconfig without bearer token.
			if err := b.K8sSeedClient.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
				return err
			}
		}

		// The kubeconfig does not use bearer token. No change required.
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
	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		basicAuthAPIServer, err = b.generateBasicAuthAPIServer(ctx, existingSecretsMap)
		if err != nil {
			return err
		}
	}

	staticToken, err := b.generateStaticToken(ctx, existingSecretsMap)
	if err != nil {
		return err
	}

	if b.Shoot.KonnectivityTunnelEnabled {
		err = b.cleanupVPNSecrets(ctx)
	} else {
		err = b.deployOpenVPNTLSAuthSecret(ctx, existingSecretsMap)
	}

	if err != nil {
		return err
	}

	wantedSecretsList, err := b.generateWantedSecrets(basicAuthAPIServer, staticToken, certificateAuthorities)
	if err != nil {
		return err
	}

	// Only necessary to renew certificates for Alertmanager, Grafana, Kibana, Prometheus
	// TODO: (timuthy) remove in future version.
	var (
		oldRenewedLabel = "cert.gardener.cloud/renewed"
		renewedLabel    = "cert.gardener.cloud/renewed-endpoint"
		browserCerts    = sets.NewString(common.GrafanaTLS, common.KibanaTLS, common.PrometheusTLS, common.AlertManagerTLS)
	)
	for name, secret := range existingSecretsMap {
		_, ok := secret.Labels[renewedLabel]
		if browserCerts.Has(name) && !ok {
			if err := b.K8sSeedClient.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
				return err
			}
			delete(existingSecretsMap, name)
		}

		if name == "etcd-server-tls" {
			if err := b.K8sSeedClient.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
				return err
			}
			delete(existingSecretsMap, name)
		}
	}

	if err := b.generateShootSecrets(ctx, existingSecretsMap, wantedSecretsList); err != nil {
		return err
	}

	// Only necessary to renew certificates for Alertmanager, Grafana, Kibana, Prometheus
	// TODO: (timuthy) remove in future version.
	for name, secret := range b.Secrets {
		_, ok := secret.Labels[renewedLabel]
		if browserCerts.Has(name) && !ok {
			if secret.Labels == nil {
				secret.Labels = make(map[string]string)
			}
			delete(secret.Labels, oldRenewedLabel)
			secret.Labels[renewedLabel] = "true"

			if err := b.K8sSeedClient.Client().Update(ctx, secret); err != nil {
				return err
			}
		}
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	for name, secret := range b.Secrets {
		b.CheckSums[name] = common.ComputeSecretCheckSum(secret.Data)
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

		if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), crt, func() error {
			crt.Data = wildcardCert.Data
			return nil
		}); err != nil {
			return err
		}

		b.ControlPlaneWildcardCert = crt
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
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
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
		Tokens: map[string]secrets.TokenConfig{
			common.KubecfgUsername: {
				Username: common.KubecfgUsername,
				UserID:   common.KubecfgUsername,
				Groups:   []string{user.SystemPrivilegedGroup},
			},
			common.KubeAPIServerHealthCheck: {
				Username: common.KubeAPIServerHealthCheck,
				UserID:   common.KubeAPIServerHealthCheck,
			},
			common.KonnectivityServerUserName: {
				Username: common.KonnectivityServerUserName,
				UserID:   common.KonnectivityServerUserName,
			},
		}}

	var (
		newStaticTokenConfig = secrets.StaticTokenSecretConfig{
			Name:   common.StaticTokenSecretName,
			Tokens: make(map[string]secrets.TokenConfig),
		}
		staticToken *secrets.StaticToken
	)
	if existingSecret, ok := existingSecretsMap[staticTokenConfig.Name]; ok {
		var err error
		staticToken, err = secrets.LoadStaticTokenFromCSV(staticTokenConfig.Name, existingSecret.Data[secrets.DataKeyStaticTokenCSV])
		if err != nil {
			return nil, err
		}

		var tokenConfigSet, tokenSet = sets.NewString(), sets.NewString()
		for _, t := range staticTokenConfig.Tokens {
			tokenConfigSet.Insert(t.Username)
		}
		for _, t := range staticToken.Tokens {
			tokenSet.Insert(t.Username)
		}

		if diff := tokenConfigSet.Difference(tokenSet); diff.Len() > 0 {
			for _, tokenKey := range diff.UnsortedList() {
				newStaticTokenConfig.Tokens[tokenKey] = staticTokenConfig.Tokens[tokenKey]
			}
		} else {
			b.mutex.Lock()
			defer b.mutex.Unlock()

			b.Secrets[staticTokenConfig.Name] = existingSecret

			if err := b.storeAPIServerHealthCheckToken(staticToken); err != nil {
				return nil, err
			}

			return staticToken, nil
		}
	}

	var err error
	if len(newStaticTokenConfig.Tokens) > 0 {
		staticToken, err = newStaticTokenConfig.AppendStaticToken(staticToken)
	} else {
		staticToken, err = staticTokenConfig.GenerateStaticToken()
	}

	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      staticTokenConfig.Name,
			Namespace: b.Shoot.SeedNamespace,
		},
		Type: corev1.SecretTypeOpaque,
	}

	_, err = controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), secret, func() error {
		secret.Data = staticToken.SecretData()
		return nil
	})
	if err != nil {
		return nil, err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[staticTokenConfig.Name] = secret

	staticTokenObj := staticToken

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

		if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), secretObj, func() error {
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

func (b *Botanist) cleanupVPNSecrets(ctx context.Context) error {
	// TODO: remove when all Gardener supported versions are >= 1.18
	vpnSecretNamesToDelete := []string{"vpn-seed", "vpn-seed-tlsauth", "vpn-shoot"}
	for _, secret := range vpnSecretNamesToDelete {
		if err := b.K8sSeedClient.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil {
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
		fmt.Sprintf("%s-local", v1beta1constants.ETCDMain),
		fmt.Sprintf("%s-local", v1beta1constants.ETCDEvents),
	}
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", v1beta1constants.ETCDMain), namespace)...)
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", v1beta1constants.ETCDEvents), namespace)...)
	return names
}
