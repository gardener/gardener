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

package botanist

import (
	"fmt"
	"net"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/konnectivity"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/common"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"k8s.io/apiserver/pkg/authentication/user"
)

var basicAuthSecretAPIServer = &secrets.BasicAuthSecretConfig{
	Name:           common.BasicAuthSecretName,
	Format:         secrets.BasicAuthFormatCSV,
	Username:       "admin",
	PasswordLength: 32,
}

func (b *Botanist) wantedCertificateAuthorities() map[string]*secrets.CertificateSecretConfig {
	wantedCertificateAuthorities := map[string]*secrets.CertificateSecretConfig{
		v1beta1constants.SecretNameCACluster: {
			Name:       v1beta1constants.SecretNameCACluster,
			CommonName: "kubernetes",
			CertType:   secrets.CACert,
		},
		v1beta1constants.SecretNameCAETCD: {
			Name:       etcd.SecretNameCA,
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
			Name:       metricsserver.SecretNameCA,
			CommonName: "metrics-server",
			CertType:   secrets.CACert,
		},
	}

	if b.Shoot.KonnectivityTunnelEnabled && b.APIServerSNIEnabled() {
		wantedCertificateAuthorities[konnectivity.SecretNameServerCA] = &secrets.CertificateSecretConfig{
			Name:       konnectivity.SecretNameServerCA,
			CommonName: konnectivity.ServerName,
			CertType:   secrets.CACert,
		}
	}

	return wantedCertificateAuthorities
}

var vpaSecrets = map[string]string{
	charts.ImageNameVpaAdmissionController: common.VpaAdmissionControllerName,
	charts.ImageNameVpaRecommender:         common.VpaRecommenderName,
	charts.ImageNameVpaUpdater:             common.VpaUpdaterName,
}

func (b *Botanist) generateStaticTokenConfig() *secrets.StaticTokenSecretConfig {
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
		},
	}

	if b.Shoot.KonnectivityTunnelEnabled {
		staticTokenConfig.Tokens[konnectivity.ServerAudience] = secrets.TokenConfig{
			Username: konnectivity.ServerAudience,
			UserID:   konnectivity.ServerAudience,
		}
	}

	if b.Shoot.WantsVerticalPodAutoscaler {
		for secretName, username := range vpaSecrets {
			staticTokenConfig.Tokens[secretName] = secrets.TokenConfig{
				Username: username,
				UserID:   secretName,
			}
		}
	}

	return staticTokenConfig
}

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config interface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func (b *Botanist) generateWantedSecretConfigs(basicAuthAPIServer *secrets.BasicAuth, staticToken *secrets.StaticToken, certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
	var (
		apiServerIPAddresses = []net.IP{
			net.ParseIP("127.0.0.1"),
			b.Shoot.Networks.APIServer,
		}
		apiServerCertDNSNames = append([]string{
			"kube-apiserver",
			fmt.Sprintf("kube-apiserver.%s", b.Shoot.SeedNamespace),
			fmt.Sprintf("kube-apiserver.%s.svc", b.Shoot.SeedNamespace),
			gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		}, kubernetes.DNSNamesForService("kubernetes", "default")...)

		kubeControllerManagerCertDNSNames = kubernetes.DNSNamesForService(kubecontrollermanager.ServiceName, b.Shoot.SeedNamespace)
		kubeSchedulerCertDNSNames         = kubernetes.DNSNamesForService(kubescheduler.ServiceName, b.Shoot.SeedNamespace)

		konnectivityServerDNSNames = append([]string{
			gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		}, kubernetes.DNSNamesForService(konnectivity.ServerName, b.Shoot.SeedNamespace)...)

		etcdCertDNSNames = append(
			b.Shoot.Components.ControlPlane.EtcdMain.ServiceDNSNames(),
			b.Shoot.Components.ControlPlane.EtcdEvents.ServiceDNSNames()...,
		)

		endUserCrtValidity = common.EndUserCrtValidity
	)

	if !b.Seed.Info.Spec.Settings.ShootDNS.Enabled {
		if addr := net.ParseIP(b.APIServerAddress); addr != nil {
			apiServerIPAddresses = append(apiServerIPAddresses, addr)
		} else {
			apiServerCertDNSNames = append(apiServerCertDNSNames, b.APIServerAddress)
		}
	}

	if b.Shoot.ExternalClusterDomain != nil {
		apiServerCertDNSNames = append(apiServerCertDNSNames, *(b.Shoot.Info.Spec.DNS.Domain), gutil.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain))
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
				Name: kubecontrollermanager.SecretName,

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
				Name: kubecontrollermanager.SecretNameServer,

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
				Name: kubescheduler.SecretName,

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
				Name: kubescheduler.SecretNameServer,

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
				Name: clusterautoscaler.SecretName,

				CommonName:   clusterautoscaler.UserName,
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
				Name: resourcemanager.SecretName,

				CommonName:   resourcemanager.UserName,
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
				Name: downloader.SecretName,

				CommonName:   downloader.SecretName,
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
			Name:       v1beta1constants.SecretNameServiceAccountKey,
			Bits:       4096,
			UsedForSSH: false,
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: etcd.SecretNameServer,

			CommonName:   "etcd-server",
			Organization: nil,
			DNSNames:     etcdCertDNSNames,
			IPAddresses:  nil,

			CertType:  secrets.ServerClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAETCD],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: etcd.SecretNameClient,

			CommonName:   "etcd-client",
			Organization: nil,
			DNSNames:     nil,
			IPAddresses:  nil,

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[v1beta1constants.SecretNameCAETCD],
		},

		// Secret definition for metrics-server
		&secrets.CertificateSecretConfig{
			Name: metricsserver.SecretNameServer,

			CommonName:   "metrics-server",
			Organization: nil,
			DNSNames:     b.Shoot.Components.SystemComponents.MetricsServer.ServiceDNSNames(),
			IPAddresses:  nil,

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
		var konnectivityServerToken *secrets.Token
		if staticToken != nil {
			var err error
			konnectivityServerToken, err = staticToken.GetTokenForUsername(konnectivity.ServerAudience)
			if err != nil {
				return nil, err
			}
		}
		// Secret definitions for konnectivity-server and konnectivity Agent
		secretList = append(secretList,
			&secrets.ControlPlaneSecretConfig{
				CertificateSecretConfig: &secrets.CertificateSecretConfig{
					Name:      konnectivity.SecretNameServerKubeconfig,
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
					Name:       konnectivity.SecretNameServerTLS,
					CommonName: konnectivity.SecretNameServerTLS,
					DNSNames:   konnectivityServerDNSNames,

					CertType:  secrets.ServerCert,
					SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
				},
			})

		if b.APIServerSNIEnabled() {
			secretList = append(secretList,
				&secrets.CertificateSecretConfig{
					Name: konnectivity.SecretNameServerTLSClient,

					CommonName:   "kube-apiserver",
					Organization: nil,
					DNSNames:     nil,
					IPAddresses:  nil,

					CertType:  secrets.ClientCert,
					SigningCA: certificateAuthorities[konnectivity.SecretNameServerCA],
				})
		}
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
			},

			&secrets.VPNTLSAuthConfig{
				Name: "vpn-seed-tlsauth",
			},
		)
	}

	if b.Shoot.WantsVerticalPodAutoscaler {
		var (
			commonName = fmt.Sprintf("vpa-webhook.%s.svc", b.Shoot.SeedNamespace)
			dnsNames   = []string{
				"vpa-webhook",
				fmt.Sprintf("vpa-webhook.%s", b.Shoot.SeedNamespace),
				commonName,
			}
		)

		secretList = append(secretList, &secrets.CertificateSecretConfig{
			Name:       common.VPASecretName,
			CommonName: commonName,
			DNSNames:   dnsNames,
			CertType:   secrets.ServerCert,
			SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCACluster],
		})
	}

	return secretList, nil
}
