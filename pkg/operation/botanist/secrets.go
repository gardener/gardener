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
	"encoding/json"
	"fmt"
	"net"
	"os/exec"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	caCluster       = "ca"
	caETCD          = "ca-etcd"
	caFrontProxy    = "ca-front-proxy"
	caKubelet       = "ca-kubelet"
	caMetricsServer = "ca-metrics-server"
)

var wantedCertificateAuthorities = map[string]*secrets.CertificateSecretConfig{
	caCluster: &secrets.CertificateSecretConfig{
		Name:       caCluster,
		CommonName: "kubernetes",
		CertType:   secrets.CACert,
	},
	caETCD: &secrets.CertificateSecretConfig{
		Name:       caETCD,
		CommonName: "etcd",
		CertType:   secrets.CACert,
	},
	caFrontProxy: &secrets.CertificateSecretConfig{
		Name:       caFrontProxy,
		CommonName: "front-proxy",
		CertType:   secrets.CACert,
	},
	caKubelet: &secrets.CertificateSecretConfig{
		Name:       caKubelet,
		CommonName: "kubelet",
		CertType:   secrets.CACert,
	},
	caMetricsServer: &secrets.CertificateSecretConfig{
		Name:       caMetricsServer,
		CommonName: "metrics-server",
		CertType:   secrets.CACert,
	},
}

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func (b *Botanist) generateWantedSecrets(basicAuthAPIServer *secrets.BasicAuth, certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
	var (
		alertManagerHost = b.Seed.GetIngressFQDN("a", b.Shoot.Info.Name, b.Garden.Project.Name)
		grafanaHost      = b.Seed.GetIngressFQDN("g", b.Shoot.Info.Name, b.Garden.Project.Name)
		prometheusHost   = b.ComputePrometheusIngressFQDN()

		apiServerIPAddresses = []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP(common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 1)),
		}
		apiServerCertDNSNames = append([]string{
			"kube-apiserver",
			fmt.Sprintf("kube-apiserver.%s", b.Shoot.SeedNamespace),
			fmt.Sprintf("kube-apiserver.%s.svc", b.Shoot.SeedNamespace),
			b.Shoot.InternalClusterDomain,
		}, dnsNamesForService("kubernetes", "default")...)

		cloudControllerManagerCertDNSNames = dnsNamesForService("cloud-controller-manager", b.Shoot.SeedNamespace)
		kubeControllerManagerCertDNSNames  = dnsNamesForService("kube-controller-manager", b.Shoot.SeedNamespace)
		kubeSchedulerCertDNSNames          = dnsNamesForService("kube-scheduler", b.Shoot.SeedNamespace)

		etcdCertDNSNames = []string{
			fmt.Sprintf("etcd-%s-0", common.EtcdRoleMain),
			fmt.Sprintf("etcd-%s-0", common.EtcdRoleEvents),
			fmt.Sprintf("etcd-%s-client.%s.svc", common.EtcdRoleMain, b.Shoot.SeedNamespace),
			fmt.Sprintf("etcd-%s-client.%s.svc", common.EtcdRoleEvents, b.Shoot.SeedNamespace),
		}
	)

	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	apiServerIPAddresses, apiServerCertDNSNames = b.appendLoadBalancerIngresses(apiServerIPAddresses, apiServerCertDNSNames)

	if b.Shoot.ExternalClusterDomain != nil {
		apiServerCertDNSNames = append(apiServerCertDNSNames, *(b.Shoot.Info.Spec.DNS.Domain), *(b.Shoot.ExternalClusterDomain))
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
				SigningCA: certificateAuthorities[caCluster],
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
				SigningCA: certificateAuthorities[caKubelet],
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
				SigningCA: certificateAuthorities[caFrontProxy],
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
				SigningCA: certificateAuthorities[caCluster],
			},
			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
			},
		},

		// Secret definition for kube-controller-manager server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.KubeControllerManagerServerName,

				CommonName:   common.KubeControllerManagerDeploymentName,
				Organization: nil,
				DNSNames:     kubeControllerManagerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[caCluster],
			},
		},

		// Secret definition for cloud-controller-manager
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "cloud-controller-manager",

				CommonName:   "system:cloud-controller-manager",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
			},
		},

		// Secret definition for cloud-controller-manager server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.CloudControllerManagerServerName,

				CommonName:   common.CloudControllerManagerDeploymentName,
				Organization: nil,
				DNSNames:     cloudControllerManagerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[caCluster],
			},
		},

		// Secret definition for the aws-lb-readvertiser
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "aws-lb-readvertiser",

				CommonName:   "aws-lb-readvertiser",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
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
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
			},
		},

		// Secret definition for kube-scheduler server
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: common.KubeSchedulerServerName,

				CommonName:   common.KubeSchedulerDeploymentName,
				Organization: nil,
				DNSNames:     kubeSchedulerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[caCluster],
			},
		},

		// Secret definition for machine-controller-manager
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "machine-controller-manager",

				CommonName:   "system:machine-controller-manager",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
			},
		},

		// Secret definition for cluster-autoscaler
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "cluster-autoscaler",

				CommonName:   "system:cluster-autoscaler",
				Organization: nil,
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
			},
		},

		// Secret definition for kube-addon-manager
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kube-addon-manager",

				CommonName:   "system:kube-addon-manager",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
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
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(false, true),
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
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
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
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(true, false),
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
				SigningCA: certificateAuthorities[caKubelet],
			},
		},

		// Secret definition for kubecfg
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kubecfg",

				CommonName:   "system:cluster-admin",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			BasicAuth: basicAuthAPIServer,

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(false, false),
			},
		},

		// Secret definition for gardener
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: gardenv1beta1.GardenerName,

				CommonName:   gardenv1beta1.GardenerName,
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(false, true),
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
				SigningCA: certificateAuthorities[caCluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.computeAPIServerURL(false, true),
			},
		},

		// Secret definition for monitoring
		&secrets.BasicAuthSecretConfig{
			Name:   "monitoring-ingress-credentials",
			Format: secrets.BasicAuthFormatNormal,

			Username:       "admin",
			PasswordLength: 32,
		},

		// Secret definition for ssh-keypair
		&secrets.RSASecretConfig{
			Name:       "ssh-keypair",
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
			SigningCA: certificateAuthorities[caCluster],
		},

		// Secret definition for vpn-seed (OpenVPN client side)
		&secrets.CertificateSecretConfig{
			Name: "vpn-seed",

			CommonName:   "vpn-seed",
			Organization: nil,
			DNSNames:     []string{},
			IPAddresses:  []net.IP{},

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[caCluster],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: "etcd-server-tls",

			CommonName:   "etcd-server",
			Organization: nil,
			DNSNames:     etcdCertDNSNames,
			IPAddresses:  nil,

			CertType:  secrets.ServerClientCert,
			SigningCA: certificateAuthorities[caETCD],
		},

		// Secret definition for etcd server
		&secrets.CertificateSecretConfig{
			Name: "etcd-client-tls",

			CommonName:   "etcd-client",
			Organization: nil,
			DNSNames:     nil,
			IPAddresses:  nil,

			CertType:  secrets.ClientCert,
			SigningCA: certificateAuthorities[caETCD],
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
			SigningCA: certificateAuthorities[caMetricsServer],
		},

		// Secret definition for alertmanager (ingress)
		&secrets.CertificateSecretConfig{
			Name: "alertmanager-tls",

			CommonName:   "alertmanager",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{alertManagerHost},
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[caCluster],
		},

		// Secret definition for grafana (ingress)
		&secrets.CertificateSecretConfig{
			Name: "grafana-tls",

			CommonName:   "grafana",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{grafanaHost},
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[caCluster],
		},

		// Secret definition for prometheus (ingress)
		&secrets.CertificateSecretConfig{
			Name: "prometheus-tls",

			CommonName:   "prometheus",
			Organization: []string{fmt.Sprintf("%s:monitoring:ingress", garden.GroupName)},
			DNSNames:     []string{prometheusHost},
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[caCluster],
		},
	}

	if b.Shoot.MonocularEnabled() && b.Shoot.Info.Spec.DNS.Domain != nil {
		secretList = append(secretList, &secrets.CertificateSecretConfig{
			Name: "monocular-tls",

			CommonName:   "monocular",
			Organization: nil,
			DNSNames:     []string{b.Shoot.GetIngressFQDN("monocular")},
			IPAddresses:  nil,

			CertType:  secrets.ServerCert,
			SigningCA: certificateAuthorities[caCluster],
		})
	}

	loggingEnabled := features.ControllerFeatureGate.Enabled(features.Logging)
	if loggingEnabled {
		kibanaHost := b.Seed.GetIngressFQDN("k", b.Shoot.Info.Name, b.Garden.Project.Name)
		secretList = append(secretList,
			&secrets.CertificateSecretConfig{
				Name: "kibana-tls",

				CommonName:   "kibana",
				Organization: []string{fmt.Sprintf("%s:logging:ingress", garden.GroupName)},
				DNSNames:     []string{kibanaHost},
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[caCluster],
			},
			// Secret definition for logging
			&secrets.BasicAuthSecretConfig{
				Name:   "logging-ingress-credentials",
				Format: secrets.BasicAuthFormatNormal,

				Username:       "admin",
				PasswordLength: 32,
			},
		)
	}

	certManagementEnabled := features.ControllerFeatureGate.Enabled(features.CertificateManagement)
	if certManagementEnabled {
		secretList = append(secretList,
			&secrets.ControlPlaneSecretConfig{
				CertificateSecretConfig: &secrets.CertificateSecretConfig{
					Name: common.CertBrokerResourceName,

					CommonName: "garden.sapcloud.io:system:cert-broker",
					CertType:   secrets.ClientCert,
					SigningCA:  certificateAuthorities[caCluster],
				},

				KubeConfigRequest: &secrets.KubeConfigRequest{
					ClusterName:  b.Shoot.SeedNamespace,
					APIServerURL: b.computeAPIServerURL(true, true),
				},
			})
	}

	return secretList, nil
}

// DeploySecrets creates a CA certificate for the Shoot cluster and uses it to sign the server certificate
// used by the kube-apiserver, and all client certificates used for communcation. It also creates RSA key
// pairs for SSH connections to the nodes/VMs and for the VPN tunnel. Moreover, basic authentication
// credentials are computed which will be used to secure the Ingress resources and the kube-apiserver itself.
// Server certificates for the exposed monitoring endpoints (via Ingress) are generated as well.
func (b *Botanist) DeploySecrets() error {
	existingSecretsMap, err := b.fetchExistingSecrets()
	if err != nil {
		return err
	}

	if err := b.deleteOldCertificates(existingSecretsMap); err != nil {
		return err
	}

	certificateAuthorities, err := b.generateCertificateAuthorities(existingSecretsMap)
	if err != nil {
		return err
	}

	basicAuthAPIServer, err := b.generateBasicAuthAPIServer(existingSecretsMap)
	if err != nil {
		return err
	}

	if err := b.deployOpenVPNTLSAuthSecret(existingSecretsMap); err != nil {
		return err
	}

	wantedSecretsList, err := b.generateWantedSecrets(basicAuthAPIServer, certificateAuthorities)
	if err != nil {
		return err
	}

	if err := b.generateShootSecrets(existingSecretsMap, wantedSecretsList); err != nil {
		return err
	}

	for name, secret := range b.Secrets {
		b.CheckSums[name] = computeSecretCheckSum(secret.Data)
	}

	return nil
}

// DeployCloudProviderSecret creates or updates the cloud provider secret in the Shoot namespace
// in the Seed cluster.
func (b *Botanist) DeployCloudProviderSecret() error {
	var (
		checksum = computeSecretCheckSum(b.Shoot.Secret.Data)
		secret   = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.CloudProviderSecretName,
				Namespace: b.Shoot.SeedNamespace,
				Annotations: map[string]string{
					"checksum/data": checksum,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: b.Shoot.Secret.Data,
		}
	)

	if _, err := b.K8sSeedClient.CreateSecretObject(secret, true); err != nil {
		return err
	}

	b.Secrets[common.CloudProviderSecretName] = b.Shoot.Secret
	b.CheckSums[common.CloudProviderSecretName] = checksum
	return nil
}

// DeleteGardenSecrets deletes the Shoot-specific secrets from the project namespace in the Garden cluster.
// TODO: https://github.com/gardener/gardener/pull/353: This can be removed in a future version as we are now using owner
// references for the Garden secrets (also remove the actual invocation of the function in the deletion flow of a Shoot).
func (b *Botanist) DeleteGardenSecrets() error {
	if err := b.K8sGardenClient.DeleteSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, "kubeconfig")); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sGardenClient.DeleteSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, "ssh-keypair")); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (b *Botanist) fetchExistingSecrets() (map[string]*corev1.Secret, error) {
	secretList, err := b.K8sSeedClient.ListSecrets(b.Shoot.SeedNamespace, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	existingSecretsMap := make(map[string]*corev1.Secret, len(secretList.Items))
	for _, secret := range secretList.Items {
		secretObj := secret
		existingSecretsMap[secret.Name] = &secretObj
	}

	return existingSecretsMap, nil
}

// Previously, we have used the same certificate authority in all places. Now, we are using dedicated CAs for the
// different components. Gardener does not re-create/generate certificates/secrets if they already exist. Hence,
// we have to delete the existing certificates to let them re-created and re-signed by a new CA.
// This can be removed in a future version. See details: https://github.com/gardener/gardener/pull/353
func (b *Botanist) deleteOldCertificates(existingSecretsMap map[string]*corev1.Secret) error {
	var dedicatedCASecrets = map[string][]string{
		caETCD:       []string{"etcd-server-tls", "etcd-client-tls"},
		caFrontProxy: []string{"kube-aggregator"},
		caKubelet:    []string{"kube-apiserver-kubelet"},
	}
	for caName, secrets := range dedicatedCASecrets {
		for _, secretName := range secrets {
			if _, ok := existingSecretsMap[caName]; !ok {
				if err := b.K8sSeedClient.DeleteSecret(b.Shoot.SeedNamespace, secretName); err != nil && !apierrors.IsNotFound(err) {
					return err
				}
				delete(existingSecretsMap, secretName)
			}
		}
	}

	return nil
}

func (b *Botanist) generateCertificateAuthorities(existingSecretsMap map[string]*corev1.Secret) (map[string]*secrets.Certificate, error) {
	generatedSecrets, certificateAuthorities, err := secrets.GenerateCertificateAuthorities(b.K8sSeedClient, existingSecretsMap, wantedCertificateAuthorities, b.Shoot.SeedNamespace)
	if err != nil {
		return nil, err
	}

	for secretName, caSecret := range generatedSecrets {
		b.Secrets[secretName] = caSecret
	}

	return certificateAuthorities, nil
}

func (b *Botanist) generateBasicAuthAPIServer(existingSecretsMap map[string]*corev1.Secret) (*secrets.BasicAuth, error) {
	basicAuthSecretAPIServer := &secrets.BasicAuthSecretConfig{
		Name:           "kube-apiserver-basic-auth",
		Format:         secrets.BasicAuthFormatCSV,
		Username:       "admin",
		PasswordLength: 32,
	}

	if existingSecret, ok := existingSecretsMap[basicAuthSecretAPIServer.Name]; ok {
		basicAuth, err := secrets.LoadBasicAuthFromCSV(basicAuthSecretAPIServer.Name, existingSecret.Data[secrets.DataKeyCSV])
		if err != nil {
			return nil, err
		}

		b.Secrets[basicAuthSecretAPIServer.Name] = existingSecret
		return basicAuth, nil
	}

	basicAuth, err := basicAuthSecretAPIServer.Generate()
	if err != nil {
		return nil, err
	}

	b.Secrets[basicAuthSecretAPIServer.Name], err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, basicAuthSecretAPIServer.Name, corev1.SecretTypeOpaque, basicAuth.SecretData(), false)
	if err != nil {
		return nil, err
	}

	return basicAuth.(*secrets.BasicAuth), nil
}

func (b *Botanist) generateShootSecrets(existingSecretsMap map[string]*corev1.Secret, wantedSecretsList []secrets.ConfigInterface) error {
	deployedClusterSecrets, err := secrets.GenerateClusterSecrets(b.K8sSeedClient, existingSecretsMap, wantedSecretsList, b.Shoot.SeedNamespace)
	if err != nil {
		return err
	}

	for secretName, secret := range deployedClusterSecrets {
		b.Secrets[secretName] = secret
	}

	return nil
}

// SyncShootCredentialsToGarden copies the kubeconfig generated for the user as well as the SSH keypair to
// the project namespace in the Garden cluster.
func (b *Botanist) SyncShootCredentialsToGarden() error {
	for key, value := range map[string]string{"kubeconfig": "kubecfg", "ssh-keypair": "ssh-keypair"} {
		secretObj := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%s", b.Shoot.Info.Name, key),
				Namespace: b.Shoot.Info.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(b.Shoot.Info, gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")),
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: b.Secrets[value].Data,
		}
		if _, err := b.K8sGardenClient.CreateSecretObject(secretObj, true); err != nil {
			return err
		}
	}

	return nil
}

func (b *Botanist) deployOpenVPNTLSAuthSecret(existingSecretsMap map[string]*corev1.Secret) error {
	name := "vpn-seed-tlsauth"
	if tlsAuthSecret, ok := existingSecretsMap[name]; ok {
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
	b.Secrets[name], err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, name, corev1.SecretTypeOpaque, data, false)
	return err
}

// appendLoadBalancerIngresses takes a list of IP addresses <ipAddresses> and a list of DNS names <dnsNames>
// and appends all ingresses of the load balancer pointing to the kube-apiserver to the lists.
func (b *Botanist) appendLoadBalancerIngresses(ipAddresses []net.IP, dnsNames []string) ([]net.IP, []string) {
	for _, ingress := range b.APIServerIngresses {
		switch {
		case ingress.IP != "":
			ipAddresses = append([]net.IP{net.ParseIP(ingress.IP)}, ipAddresses...)
		case ingress.Hostname != "":
			dnsNames = append([]string{ingress.Hostname}, dnsNames...)
		default:
			b.Logger.Warnf("Could not add kube-apiserver ingress '%+v' to the certificate's SANs because it does neither contain an IP nor a hostname.", ingress)
		}
	}
	return ipAddresses, dnsNames
}

// computeAPIServerURL takes a boolean value identifying whether the component connecting to the API server
// runs in the Seed cluster <runsInSeed>, and a boolean value <useInternalClusterDomain> which determines whether the
// internal or the external cluster domain should be used.
func (b *Botanist) computeAPIServerURL(runsInSeed, useInternalClusterDomain bool) string {
	if runsInSeed {
		return "kube-apiserver"
	}
	dnsProvider := b.Shoot.Info.Spec.DNS.Provider
	if dnsProvider == gardenv1beta1.DNSUnmanaged || (dnsProvider != gardenv1beta1.DNSUnmanaged && useInternalClusterDomain) {
		return b.Shoot.InternalClusterDomain
	}
	return *(b.Shoot.ExternalClusterDomain)
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

func computeSecretCheckSum(data map[string][]byte) string {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return utils.ComputeSHA256Hex(jsonString)
}

func generateGardenSecretName(shootName, secretName string) string {
	return fmt.Sprintf("%s.%s", shootName, secretName)
}

func dnsNamesForService(name, namespace string) []string {
	return []string{
		name,
		fmt.Sprintf("%s.%s", name, namespace),
		fmt.Sprintf("%s.%s.svc", name, namespace),
		fmt.Sprintf("%s.%s.svc.%s", name, namespace, gardenv1beta1.DefaultDomain),
		// TODO: Determine Seed cluster's domain that is configured for kubelet and kube-dns/coredns
		// fmt.Sprintf("%s.%s.svc.%s", name, namespace, seed-kube-domain),
	}
}
