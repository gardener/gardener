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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
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
func (b *Botanist) generateWantedSecrets(basicAuthAPIServer *secrets.BasicAuth, certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
	var (
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

		kubeControllerManagerCertDNSNames = dnsNamesForService("kube-controller-manager", b.Shoot.SeedNamespace)
		kubeSchedulerCertDNSNames         = dnsNamesForService("kube-scheduler", b.Shoot.SeedNamespace)

		etcdCertDNSNames = dnsNamesForEtcd(b.Shoot.SeedNamespace)
	)

	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

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

				CommonName:   common.KubeControllerManagerDeploymentName,
				Organization: nil,
				DNSNames:     kubeControllerManagerCertDNSNames,
				IPAddresses:  nil,

				CertType:  secrets.ServerCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
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
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(true, false),
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

				CommonName:   common.KubeSchedulerDeploymentName,
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

		// Secret definition for kubecfg
		&secrets.ControlPlaneSecretConfig{
			CertificateSecretConfig: &secrets.CertificateSecretConfig{
				Name: "kubecfg",

				CommonName:   "system:cluster-admin",
				Organization: []string{user.SystemPrivilegedGroup},
				DNSNames:     nil,
				IPAddresses:  nil,

				CertType:  secrets.ClientCert,
				SigningCA: certificateAuthorities[gardencorev1alpha1.SecretNameCACluster],
			},

			BasicAuth: basicAuthAPIServer,

			KubeConfigRequest: &secrets.KubeConfigRequest{
				ClusterName:  b.Shoot.SeedNamespace,
				APIServerURL: b.Shoot.ComputeAPIServerURL(false, false),
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
func (b *Botanist) DeploySecrets() error {
	existingSecretsMap, err := b.fetchExistingSecrets()
	if err != nil {
		return err
	}

	// Migrate logging ingress admin credentials after exposing users logging.
	// This can be removed in a future Gardener version.
	loggingIngressAdminCredentials := existingSecretsMap[common.KibanaAdminIngressCredentialsSecretName]
	if loggingIngressAdminCredentials != nil && len(loggingIngressAdminCredentials.Data[secrets.DataKeyPasswordBcryptHash]) == 0 {
		if err := b.K8sSeedClient.DeleteSecret(b.Shoot.SeedNamespace, common.KibanaAdminIngressCredentialsSecretName); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		delete(existingSecretsMap, common.KibanaAdminIngressCredentialsSecretName)
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

	b.mutex.Lock()
	defer b.mutex.Unlock()

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
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
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

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[gardencorev1alpha1.SecretNameCloudProvider] = b.Shoot.Secret
	b.CheckSums[gardencorev1alpha1.SecretNameCloudProvider] = checksum

	return nil
}

// DeleteGardenSecrets deletes the Shoot-specific secrets from the project namespace in the Garden cluster.
// TODO: https://github.com/gardener/gardener/pull/353: This can be removed in a future version as we are now using owner
// references for the Garden secrets (also remove the actual invocation of the function in the deletion flow of a Shoot).
func (b *Botanist) DeleteGardenSecrets() error {
	if err := b.K8sGardenClient.DeleteSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, "kubeconfig")); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sGardenClient.DeleteSecret(b.Shoot.Info.Namespace, generateGardenSecretName(b.Shoot.Info.Name, gardencorev1alpha1.SecretNameSSHKeyPair)); err != nil && !apierrors.IsNotFound(err) {
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

		b.mutex.Lock()
		defer b.mutex.Unlock()

		b.Secrets[basicAuthSecretAPIServer.Name] = existingSecret

		return basicAuth, nil
	}

	basicAuth, err := basicAuthSecretAPIServer.Generate()
	if err != nil {
		return nil, err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

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
func (b *Botanist) SyncShootCredentialsToGarden() error {
	kubecfgURL := b.Shoot.InternalClusterDomain
	if b.Shoot.ExternalClusterDomain != nil {
		kubecfgURL = *b.Shoot.ExternalClusterDomain
	}

	projectSecrets := []projectSecret{
		{
			secretName:  "kubecfg",
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
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(b.Shoot.Info, gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")),
				},
				Annotations: projectSecret.annotations,
			},
			Type: corev1.SecretTypeOpaque,
			Data: b.Secrets[projectSecret.secretName].Data,
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

	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Secrets[name], err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, name, corev1.SecretTypeOpaque, data, false)
	return err
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
	}
}

func dnsNamesForEtcd(namespace string) []string {
	names := []string{
		fmt.Sprintf("%s-0", common.EtcdMainStatefulSetName),
		fmt.Sprintf("%s-0", common.EtcdEventsStatefulSetName),
	}
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", common.EtcdMainStatefulSetName), namespace)...)
	names = append(names, dnsNamesForService(fmt.Sprintf("%s-client", common.EtcdEventsStatefulSetName), namespace)...)
	return names
}
