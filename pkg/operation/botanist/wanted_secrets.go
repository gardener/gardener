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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config interface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func (b *Botanist) generateWantedSecretConfigs(certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
	var (
		etcdCertDNSNames = append(
			b.Shoot.Components.ControlPlane.EtcdMain.ServiceDNSNames(),
			b.Shoot.Components.ControlPlane.EtcdEvents.ServiceDNSNames()...,
		)

		endUserCrtValidity = common.EndUserCrtValidity
	)

	secretList := []secrets.ConfigInterface{
		// Secret definition for prometheus
		// TODO(rfranzke): Delete this in a future release once all monitoring configurations of extensions have been
		// adapted.
		&secrets.ControlPlaneSecretConfig{
			Name: "prometheus",
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   "gardener.cloud:monitoring:prometheus",
				Organization: []string{"gardener.cloud:monitoring"},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   b.Shoot.SeedNamespace,
				APIServerHost: b.Shoot.ComputeInClusterAPIServerAddress(true),
			}},
		},

		// Secret definition for monitoring
		&secrets.BasicAuthSecretConfig{
			Name:   common.MonitoringIngressCredentials,
			Format: secrets.BasicAuthFormatNormal,

			Username:       "admin",
			PasswordLength: 32,
		},

		// Secret definition for monitoring for shoot owners
		&secrets.BasicAuthSecretConfig{
			Name:   common.MonitoringIngressCredentialsUsers,
			Format: secrets.BasicAuthFormatNormal,

			Username:       "admin",
			PasswordLength: 32,
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
	}

	if gardencorev1beta1helper.SeedSettingDependencyWatchdogProbeEnabled(b.Seed.GetInfo().Spec.Settings) {
		// Secret definitions for dependency-watchdog-internal and external probes
		secretList = append(secretList, &secrets.ControlPlaneSecretConfig{
			Name: kubeapiserver.DependencyWatchdogInternalProbeSecretName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   dependencywatchdog.UserName,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   b.Shoot.SeedNamespace,
				APIServerHost: b.Shoot.ComputeInClusterAPIServerAddress(false),
			}},
		}, &secrets.ControlPlaneSecretConfig{
			Name: kubeapiserver.DependencyWatchdogExternalProbeSecretName,
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				CommonName:   dependencywatchdog.UserName,
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},
			KubeConfigRequests: []secrets.KubeConfigRequest{{
				ClusterName:   b.Shoot.SeedNamespace,
				APIServerHost: b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
			}},
		})
	}

	if b.Shoot.ReversedVPNEnabled {
		secretList = append(secretList,
			// Secret definition for vpn-shoot (OpenVPN client side)
			&secrets.CertificateSecretConfig{
				Name:       vpnshoot.SecretNameVPNShootClient,
				CommonName: "vpn-shoot-client",
				CertType:   secrets.ClientCert,
				SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCAVPN],
			},

			// Secret definition for vpn-seed-server (OpenVPN server side)
			&secrets.CertificateSecretConfig{
				Name:       "vpn-seed-server",
				CommonName: "vpn-seed-server",
				DNSNames:   kubernetes.DNSNamesForService(vpnseedserver.ServiceName, b.Shoot.SeedNamespace),
				CertType:   secrets.ServerCert,
				SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCAVPN],
			},

			&secrets.VPNTLSAuthConfig{
				Name: vpnseedserver.VpnSeedServerTLSAuth,
			},
		)
	} else {
		secretList = append(secretList,
			// Secret definition for vpn-shoot (OpenVPN server side)
			&secrets.CertificateSecretConfig{
				Name:       vpnshoot.SecretNameVPNShoot,
				CommonName: "vpn-shoot",
				CertType:   secrets.ServerCert,
				SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			// Secret definition for vpn-seed (OpenVPN client side)
			&secrets.CertificateSecretConfig{
				Name:       kubeapiserver.SecretNameVPNSeed,
				CommonName: kubeapiserver.UserNameVPNSeed,
				CertType:   secrets.ClientCert,
				SigningCA:  certificateAuthorities[v1beta1constants.SecretNameCACluster],
			},

			&secrets.VPNTLSAuthConfig{
				Name: kubeapiserver.SecretNameVPNSeedTLSAuth,
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
