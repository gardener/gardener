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

package deployment

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/Masterminds/semver"
	errorspkg "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SecretNameCAFrontProxy is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the kube-aggregator of a shoot cluster.
	SecretNameCAFrontProxy = "ca-front-proxy"
	// SecretNameKubeAPIserverKubelet is a constant for the name of a
	// Kubernetes secret object that contains the client certificate and key for requests to the kubelet.
	SecretNameKubeAPIserverKubelet = "kube-apiserver-kubelet"
	// SecretNameKubeAggregator is a constant for the name of a Kubernetes
	// secret object for the kube-apiserver's aggregator.
	SecretNameKubeAggregator = "kube-aggregator"
	// StaticTokenSecretName is the name of the secret containing static tokens for the kube-apiserver.
	StaticTokenSecretName = "static-token"
	// BasicAuthSecretName is the name of the secret containing basic
	// authentication credentials for the kube-apiserver.
	BasicAuthSecretName = "kube-apiserver-basic-auth"
	// SecretNameVPNSeedTLSAuth is a constant for the name of a Kubernetes secret object that contains
	// the vpn TLS authentication keys.
	SecretNameVPNSeedTLSAuth = "vpn-seed-tlsauth"
	// SecretNameVPNShoot is a constant for the name of a Kubernetes secret object that contains the
	// TLS server certificate for the VPN server in the Shoot cluster.
	SecretNameVPNShoot = "vpn-shoot"
	// SecretNameVPNSeed is a constant for the name of a Kubernetes secret object
	// that contains the CA and client certificate plus key
	// used to communicate with the Shoot API server.
	SecretNameVPNSeed = "vpn-seed"
	// SecretNameTLSServer is a constant for the name of a Kubernetes secret object that
	// contains the default x509 TLS server Certificate of the kube-apiserver.
	SecretNameTLSServer = "kube-apiserver"
	// secretNameOIDC is a constant for the name of a Kubernetes secret object in the Seed cluster that contains the CA bundle for the Shoot API server OIDC.
	secretNameOIDC = "kube-apiserver-oidc-cabundle"
	// secretNameServiceAccountSigningKey is a constant for the name of a Kubernetes secret in the Seed cluster object that contains key used to sign service accounts.
	secretNameServiceAccountSigningKey = "kube-apiserver-service-account-signing-key"

	// labelRole is a constant for the value of a label with key 'role' whose value is 'apiserver'.
	labelRole = "apiserver"
	// containerNameKubeAPIServer is the name of the kube apiserver container
	containerNameKubeAPIServer = "kube-apiserver"
	// containerNameKubeAPIServer is the name of the vpn sidecar container
	containerNameVPNSeed = "vpn-seed"
	// containerNameKonnectivityServer is the name of the konnectivity sidecar container
	containerNameKonnectivityServer = "konnectivity-server"
	// containerNameApiserverProxyPodMutator is the name of the apiserver proxy pod mutator sidecar container
	containerNameApiserverProxyPodMutator = "apiserver-proxy-pod-mutator"

	// cmNameKonnectivityEgressSelector is the name of the config map containing the egress selector config file for the konnectivity tunnel.
	cmNameKonnectivityEgressSelector = "kube-apiserver-egress-selector-configuration"
	// cmNameAPIServerAdmissionConfig is the name of the config map containing the admission plugin configuration file.
	cmNameAPIServerAdmissionConfig = "kube-apiserver-admission-config"
	// cmNameAuditPolicyConfig is the name of the config map containing the audit policy file.
	cmNameAuditPolicyConfig = "audit-policy-config"

	// fileNameAdmissionPluginConfiguration is the filename for the admission configuration file containing the
	// configuration for the enabled admission plugins.
	fileNameAdmissionPluginConfiguration = "admission-configuration.yaml"
	// fileNameAuditPolicyConfig is the filename for the audit policy file.
	fileNameAuditPolicyConfig = "audit-policy.yaml"
	// fileNameServiceAccountSigning is the filename for the service account signing key.
	fileNameServiceAccountSigning = "signing-key"
	// fileNameKonnectivityEgressSelector is the filename for the egress selector config file for the konnectivity tunnel.
	fileNameKonnectivityEgressSelector = "egress-selector-configuration.yaml"
	// fileNameSecretOIDCCert is the filename for the OIDC CA Bundle.
	fileNameSecretOIDCCert = "ca.crt"

	// volumeMountNameCA is a constant for the volume mount name for the secret containing the CA certificate of the kube-apiserver.
	volumeMountNameCA = "ca"
	// volumeMountNameCAEtcd is a constant for the volume mount name for the secret containing the CA certificate of the etcd.
	volumeMountNameCAEtcd = "ca-etcd"
	// volumeMountNameCAFrontProxy is a constant for the volume mount name for the secret containing the Root certificate
	// bundle used to verify client certificates.
	volumeMountNameCAFrontProxy = "ca-front-proxy"
	// volumeMountNameTLSServer is a constant for the volume mount name for for the secret containing he default x509
	// TLS server Certificate of the kube-apiserver.
	volumeMountNameTLSServer = "kube-apiserver"
	// volumeMountNameEtcdClientTLS is a constant for the volume mount name for for the secret containing the client
	// certificate and key for communication with etcd.
	volumeMountNameEtcdClientTLS = "etcd-client-tls"
	// volumeMountNameServiceAccountKey is a constant for the volume mount name for for the secret containing the keys
	// used to verify ServiceAccount tokens.
	volumeMountNameServiceAccountKey = "service-account-key"
	// volumeMountNameBasicAuth is a constant for the volume mount name for for the secret containing the basic auth
	// credentials for the API Server.
	volumeMountNameBasicAuth = "kube-apiserver-basic-auth"
	// volumeMountNameStaticToken is a constant for the volume mount name for for the secret containing static tokens for
	// the kube-apiserver.
	volumeMountNameStaticToken = "static-token"
	// volumeMountNameKubeAPIServerKubelet is a constant for the volume mount name for for the secret containing
	// the client certificate and key for requests to the kubelet.
	volumeMountNameKubeAPIServerKubelet = "kube-apiserver-kubelet"
	// volumeMountNameKubeAggregator is a constant for the volume mount name for for the secret containing
	// the client certificate and key used to prove the identity of the aggregator or kube-apiserver when it must
	// call out during a request.
	volumeMountNameKubeAggregator = "kube-aggregator"
	// volumeMountNameEgressSelectionConfig is a constant for the volume mount name of the konnectivity tunnel
	// egress selection config map.
	volumeMountNameEgressSelectionConfig = "egress-selection-config"
	// volumeMountNameKonnectivityUDS is a constant for the volume mount name for the konnectivity tunnel feature gate
	// mounting an empty directory for the Unix Domain Socket.
	volumeMountNameKonnectivityUDS = "konnectivity-uds"
	// volumeMountNameKonnectivityClientTLS is a constant for the volume mount name for the konnectivity tunnel
	// mounting the client cert to talk to the external konnectivity deployment (only used when SNI enabled)
	volumeMountNameKonnectivityClientTLS = "konnectivity-server-client-tls"
	// volumeMountNameAuditPolicyConfig is a constant for the volume mount name of the audit policy config map.
	volumeMountNameAuditPolicyConfig = "audit-policy-config"
	// volumeMountNameServiceAccountSigningKey is a constant for the volume mount name for the service account signing secret.
	volumeMountNameServiceAccountSigningKey = "kube-apiserver-service-account-signing-key"
	// volumeMountNameAdmissionConfig is a constant for the volume mount name for the configuration file of the admission plugins.
	volumeMountNameAdmissionConfig = "kube-apiserver-admission-config"
	// volumeMountNameKonnectivityServerCerts is a constant for the volume mount name for the TLS serving certificate and key of the konnectivity server.
	volumeMountNameKonnectivityServerCerts = "konnectivity-server-certs"
	// volumeMountNameKonnectivityServerKubeconfig is a constant for the volume mount name for the kubeconfig of the konnectivity server.
	volumeMountNameKonnectivityServerKubeconfig = "konnectivity-server-kubeconfig"
	// volumeMountPathVPNModules is a constant for the volume mount name for the init container required by the VPN mounting /lib/modules.
	volumeMountNameVPNModules = "modules"
	// volumeMountNameVPNSeed is a constant for the volume mount name for the secret containing the vpn TLS authentication keys.
	volumeMountNameVPNSeedTLSAuth = "vpn-seed-tlsauth"
	// volumeMountNameVPNSeed is a constant for the volume mount name for the directory containing the CA and client certificate plus key used to communicate with the Shoot API server.
	volumeMountNameVPNSeed = "vpn-seed"
	// volumeMountNameEtcdEncryption is a constant for the volume mount name for the etcd-encryption secret.
	volumeMountNameEtcdEncryption = "etcd-encryption-secret"

	// volumeMountPathAdmissionPluginConfig is the volume mount path for the admission config file.
	volumeMountPathAdmissionPluginConfig = "/etc/kubernetes/admission"
	// volumeMountPathAuditPolicyConfig is the volume mount path for the audit policy file.
	volumeMountPathAuditPolicyConfig = "/etc/kubernetes/audit"
	// volumeMountPathKonnectivityEgressSelector is the volume mount path for the egress selector file for the konnectivity tunnel.
	volumeMountPathKonnectivityEgressSelector = "/etc/kubernetes/konnectivity"
	// volumeMountPathKonnectivityUDS is the volume mount path for the empty directory for the Unix Domain Socket file for the konnectivity tunnel.
	volumeMountPathKonnectivityUDS = "/etc/srv/kubernetes/konnectivity-server"
	// volumeMountPathKonnectivityClientTLS is the volume mount path for the konnectivity tunnel
	// mounting the client cert to talk to the external konnectivity deployment (only used when SNI enabled)
	volumeMountPathKonnectivityClientTLS = "/etc/srv/kubernetes/konnectivity-server-client-tls"
	// volumeMountPathBasicAuth is the volume mount path for the basic auth file.
	volumeMountPathBasicAuth = "/srv/kubernetes/auth"
	// volumeMountPathCA is the volume mount path for the CA certificate used by the kube-apiserver.
	volumeMountPathCA = "/srv/kubernetes/ca"
	// volumeMountPathETCDCA is the volume mount path for the ETCD CA certificate used by the kube-apiserver.
	volumeMountPathETCDCA = "/srv/kubernetes/etcd/ca"
	// volumeMountPathETCDClient is the volume mount path for the ETCD client certificate and key used by the kube-apiserver.
	volumeMountPathETCDClient = "/srv/kubernetes/etcd/client"
	// volumeMountPathETCDEncryptionSecret is the volume mount path for the ETCD encryption secret.
	volumeMountPathETCDEncryptionSecret = "/etc/kubernetes/etcd-encryption-secret"
	// volumeMountPathKubeletSecret is the volume mount path for the kubelet secret.
	volumeMountPathKubeletSecret = "/srv/kubernetes/apiserver-kubelet"
	// volumeMountPathKubeletSecret is the volume mount path for the kube-aggregator secret.
	volumeMountPathKubeAggregator = "/srv/kubernetes/aggregator"
	// volumeMountPathKubeletSecret is the volume mount path for the kube-aggregator secret.
	volumeMountPathCAFrontProxy = "/srv/kubernetes/ca-front-proxy"
	// volumeMountNameOIDCBundle is a constant for the volume mount name for the optional OIDC CA Bundle.
	volumeMountNameOIDCBundle = "kube-apiserver-oidc-cabundle"
	// volumeMountPathServiceAccountKey is the volume mount path for the service account key that is a PEM-encoded private
	// RSA or ECDSA key used to sign service account tokens.
	volumeMountPathServiceAccountKey = "/srv/kubernetes/service-account-key"
	// volumeMountPathStaticTokenAuth is the volume mount path for the static token file.
	volumeMountPathStaticTokenAuth = "/srv/kubernetes/token"
	// volumeMountPathTLS is the volume mount path for the default x509 TLS server Certificate of the kube-apiserver.
	volumeMountPathTLS = "/srv/kubernetes/apiserver"
	// volumeMountPathServiceAccountSigning is the volume mount path for the service account signing secret.
	volumeMountPathServiceAccountSigning = "/srv/kubernetes/service-account-signing-key"
	// volumeMountPathOIDCCABundle is the volume mount path for the OIDC CA bundle.
	volumeMountPathOIDCCABundle = "/srv/kubernetes/oidc"
	// volumeMountPathKonnectivityServerCerts is the volume mount path for the TLS serving certificate and key for the konnectivity server.
	volumeMountPathKonnectivityServerCerts = "/certs/konnectivity-server"
	// volumeMountPathKonnectivityServerKubeconfig is the volume mount path for the TLS serving certificate and key of the konnectivity server.
	volumeMountPathKonnectivityServerKubeconfig = "/etc/srv/kubernetes/konnectivity-server-kubeconfig"
	volumeMountPathVPNSeed                      = "/srv/secrets/vpn-seed"
	volumeMountPathVPNSeedTLSAuth               = "/srv/secrets/tlsauth"
	// volumeMountPathVPNModules is the volume mount path for the init container required by the VPN mounting /lib/modules.
	volumeMountPathVPNModules = "/lib/modules"

	// defaultAPIServerReplicas is the default replicas for the kube-apiserver
	defaultAPIServerReplicas = 1
	// konnectivityUDSName is the Unix Domain Socket name used for communication of the Shoot API server to
	// konnectivity server sidecar for traffic that meant for components in the Shoot VPC.
	konnectivityUDSName = "konnectivity-server.socket"
	// portNameHTTPS is a constant for the name of the HTTPS port of the kube-apiserver.
	portNameHTTPS = "https"
	// vpnClientPort is a constant for the TCP tunnel port of the vpn client.
	vpnClientPort = int32(1194)
	// vpnPort is a constant for the OpenVPN port of the vpn server in the Shoot.
	vpnPort = "4314"

	// constants for CA bundle mounts
	volumeMountNameCABundleFedoraRHEL6Openelec = "fedora-rhel6-openelec-cabundle"
	volumeMountPathCABundleFedoraRHEL6Openelec = "/etc/pki/tls"
	volumeMountNameCABundleCentOSRHEl7         = "centos-rhel7-cabundle"
	volumeMountPathCABundleCentOSRHEl7Dir      = "/etc/pki/ca-trust/extracted/pem"
	volumeMountNameCABundleEtcSSL              = "etc-ssl"
	volumeMountPathCABundleEtcSSL              = "/etc/ssl"
	volumeMountNameCABundleUsrShareCacerts     = "usr-share-cacerts"
	volumeMountPathCABundleUsrShareCacerts     = "/usr/share/ca-certificates"
	volumeMountNameCABundleDebianFamily        = "debian-family-cabundle"
	volumeMountPathCABundleDebianFamily        = "/etc/ssl/certs/ca-certificates.crt"
	volumeMountNameCABundleFedoraRHEL6         = "fedora-rhel6-cabundle"
	volumeMountPathCABundleFedoraRHEL6         = "/etc/pki/tls/certs/ca-bundle.crt"
	volumeMountNameCABundleOpensuse            = "opensuse-cabundle"
	volumeMountPathCABundleOpensuse            = "/etc/ssl/ca-bundle.pem"
	volumeMountNameCABundleOpenelec            = "openelec-cabundle"
	volumeMountPathCABundleOpenelec            = "/etc/pki/tls/cacert.pem"
	volumeMountPathCABundleCentOSRHEL7File     = "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
	volumeMountNameCABundleAlpine              = "alpine-linux-cabundle"
	volumeMountPathCABundleAlpine              = "/etc/ssl/cert.pem"
)

// KubeAPIServer contains functions for a kube-apiserver deployer.
type KubeAPIServer interface {
	component.DeployWaiter
	component.MonitoringComponent
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
	// SetHealthCheckToken sets the health check token for the kube-apiserver
	SetHealthCheckToken(string)
	// SetShootAPIServerClusterIP sets the Cluster IP of the service of the shoot apiserver
	SetShootAPIServerClusterIP(string)
	// SetShootOutOfClusterAPIServerAddress sets the internal API server domain of the Shoot API Server
	// used by Gardener system components
	SetShootOutOfClusterAPIServerAddress(string)
}

// New creates a new instance of DeployWaiter for the kube-apiserver.
func New(
	config *gardencorev1beta1.KubeAPIServerConfig,
	managedSeedAPIServer *gardencorev1beta1helper.ShootedSeedAPIServer,
	seedClient kubernetes.Interface,
	gardenClient client.Client,
	shootKubernetesVersion *semver.Version,
	seedNamespace string,
	gardenNamespace string,
	hibernationEnabled bool,
	konnectivityTunnelEnabled bool,
	etcdEncryptionEnabled bool,
	basicAuthenticationEnabled bool,
	hvpaEnabled bool,
	mountHostCADirectories bool,
	shootHasDeletionTimestamp bool,
	serviceNetwork *net.IPNet,
	podNetwork *net.IPNet,
	nodeNetwork *net.IPNet,
	minNodeCount int32,
	maxNodeCount int32,
	shootAnnotations map[string]string,
	maintenanceWindow *gardencorev1beta1.MaintenanceTimeWindow,
	sniValues APIServerSNIValues,
	images APIServerImages,
) KubeAPIServer {
	return &kubeAPIServer{
		config:                     config,
		managedSeed:                managedSeedAPIServer,
		seedClient:                 seedClient,
		gardenClient:               gardenClient,
		shootKubernetesVersion:     shootKubernetesVersion,
		seedNamespace:              seedNamespace,
		gardenNamespace:            gardenNamespace,
		hibernationEnabled:         hibernationEnabled,
		konnectivityTunnelEnabled:  konnectivityTunnelEnabled,
		etcdEncryptionEnabled:      etcdEncryptionEnabled,
		basicAuthenticationEnabled: basicAuthenticationEnabled,
		hvpaEnabled:                hvpaEnabled,
		mountHostCADirectories:     mountHostCADirectories,
		shootHasDeletionTimestamp:  shootHasDeletionTimestamp,
		serviceNetwork:             serviceNetwork,
		podNetwork:                 podNetwork,
		nodeNetwork:                nodeNetwork,
		minNodeCount:               minNodeCount,
		maxNodeCount:               maxNodeCount,
		shootAnnotations:           shootAnnotations,
		maintenanceWindow:          maintenanceWindow,
		sniValues:                  sniValues,
		images:                     images,
	}
}

// APIServerSNIValues contains the SNI related values of the kube-apiserver deployment
type APIServerSNIValues struct {
	// SNIEnabled indicates if SNI is enabled for this cluster
	SNIEnabled bool
	// SNIPodMutatorEnabled indicates if the SNI pod mutator webhook is enabled
	SNIPodMutatorEnabled bool
	// shootAPIServerClusterIP is the Cluster IP of the service of the shoot apiserver
	// apiserver explicitly advertises the cluster ip when using SNI
	shootAPIServerClusterIP string
}

// APIServerImages contains the images required to deploy the kube-apiserver
type APIServerImages struct {
	// KubeAPIServerImageName is the name of the kube-apiserver image
	KubeAPIServerImageName string
	// AlpineIptablesImageName is the name of the image that sets the IPTable rules for the VPN
	// not required when the Konnectivity feature gate is enabled
	AlpineIptablesImageName string
	// VPNSeedImageName is the name of the vpn-client image
	// not required when the Konnectivity feature gate is enabled
	VPNSeedImageName string
	// KonnectivityServerTunnelImageName is the name of the konnectivity-server image
	// required when the Konnectivity server feature gate is enabled
	KonnectivityServerTunnelImageName string
	// ApiServerProxyPodMutatorWebhookImageName is the name of the image of the api server proxy pod mutator webhook
	// required when the SNI feature gate is enabled
	ApiServerProxyPodMutatorWebhookImageName string
}

type kubeAPIServer struct {
	config       *gardencorev1beta1.KubeAPIServerConfig
	managedSeed  *gardencorev1beta1helper.ShootedSeedAPIServer
	seedClient   kubernetes.Interface
	gardenClient client.Client

	hibernationEnabled         bool
	konnectivityTunnelEnabled  bool
	etcdEncryptionEnabled      bool
	basicAuthenticationEnabled bool
	hvpaEnabled                bool
	mountHostCADirectories     bool
	shootHasDeletionTimestamp  bool

	shootKubernetesVersion *semver.Version
	seedNamespace          string
	gardenNamespace        string
	serviceNetwork         *net.IPNet
	podNetwork             *net.IPNet
	nodeNetwork            *net.IPNet

	// the sum of all 'minimum' fields of all worker groups of the Shoot.
	minNodeCount int32
	// the sum of all 'maximum' fields of all worker groups of the Shoot.
	maxNodeCount int32
	// the annotations on the Shoot resource in the Garden cluster.
	shootAnnotations map[string]string
	// maintenanceWindow is the daily maintenance time window of the Shoot cluster.
	maintenanceWindow *gardencorev1beta1.MaintenanceTimeWindow
	// healthCheckToken is the base64 encoded token for the liveness probe of the kube-apiserver pod.
	healthCheckToken string
	// shootOutOfClusterAPIServerAddress is the API server domain (or ip if unmanaged DNS)
	// of the Shoot cluster (internal) used by Gardener system components.
	// In case of SNI, the DNS entry points to the ingress domain/ip of the SNI loadbalancer
	// in the Seed cluster (e.g. of the istio-ingressgateway)
	shootOutOfClusterAPIServerAddress string
	// deploymentReplicas is a computed field that specifies the replicas of the kube-apiserver
	// deployment
	deploymentReplicas *int32

	sniValues APIServerSNIValues
	images    APIServerImages
	secrets   Secrets
}

func (k *kubeAPIServer) SetShootAPIServerClusterIP(s string) {
	k.sniValues.shootAPIServerClusterIP = s
}

func (k *kubeAPIServer) SetShootOutOfClusterAPIServerAddress(s string) {
	k.shootOutOfClusterAPIServerAddress = s
}

func (k *kubeAPIServer) Deploy(ctx context.Context) error {
	// validate configuration that is set during deploy time
	err := k.validateSecrets()
	if err != nil {
		return err
	}

	if len(k.shootOutOfClusterAPIServerAddress) == 0 {
		return fmt.Errorf("the ingress of the SNI loadbalancer has to be set to calculate the API Server health check token and to confgiure the SNI pod mutator")
	}

	if len(k.healthCheckToken) == 0 {
		return fmt.Errorf("the API Server health check token has to be set")
	}

	if k.sniValues.SNIEnabled && len(k.sniValues.shootAPIServerClusterIP) == 0 {
		return fmt.Errorf("the cluster IP of the service of the apiserver has to be set when SNI is enabled")
	}

	var (
		apiServerAdmissionPlugins = k.getAdmissionPlugins()
		command                   = k.computeKubeAPIServerCommand(apiServerAdmissionPlugins)

		// checksum for resources deployed by this component
		checksumServiceAccountSigningKey,
		checksumConfigMapEgressSelection,
		checksumConfigMapAuditPolicy,
		checksumSecretOIDCCABundle,
		checksumConfigMapAdmissionConfig *string
	)

	if k.config != nil && k.config.OIDCConfig != nil && k.config.OIDCConfig.CABundle != nil {
		checksumSecretOIDCCABundle, err = k.deploySecretOIDCBundle(ctx)
		if err := err; err != nil {
			return err
		}
	}

	if k.config != nil && k.config.ServiceAccountConfig != nil && k.config.ServiceAccountConfig.SigningKeySecret != nil {
		checksumServiceAccountSigningKey, err = k.deploySecretServiceAccountSigningKey(ctx)
		if err != nil {
			return err
		}
	}

	if k.konnectivityTunnelEnabled {
		checksumConfigMapEgressSelection, err = k.deployEgressSelectorConfigMap(ctx)
		if err := err; err != nil {
			return err
		}

		if err := k.deployServiceAccount(ctx); err != nil {
			return err
		}

		if !k.sniValues.SNIEnabled {
			if err := k.deployRBAC(ctx); err != nil {
				return err
			}
		}
	}

	checksumConfigMapAdmissionConfig, err = k.deployAdmissionConfigMap(ctx, apiServerAdmissionPlugins)
	if err != nil {
		return err
	}

	checksumConfigMapAuditPolicy, err = k.deployAuditPolicyConfigMap(ctx)
	if err != nil {
		return err
	}

	if err = k.deployNetworkPolicies(ctx); err != nil {
		return err
	}

	if err = k.deployPodDisruptionBudget(ctx); err != nil {
		return err
	}

	if err = k.deployKubeAPIServerDeployment(ctx, command, checksumServiceAccountSigningKey, checksumConfigMapEgressSelection, checksumConfigMapAuditPolicy, checksumSecretOIDCCABundle, checksumConfigMapAdmissionConfig); err != nil {
		return err
	}

	// deploy after the kube-apiserver deployment because it requires the
	// up-to date number of replicas
	if err := k.deployAutoscaler(ctx); err != nil {
		return err
	}

	return nil
}

// validateSecrets validates that all required secrets are set
func (k *kubeAPIServer) validateSecrets() error {
	if k.secrets.CA.Name == "" || k.secrets.CA.Checksum == "" {
		return fmt.Errorf("missing CA secret information")
	}

	if k.secrets.CAFrontProxy.Name == "" || k.secrets.CAFrontProxy.Checksum == "" {
		return fmt.Errorf("missing CA front-proxy secret information")
	}

	if k.secrets.TLSServer.Name == "" || k.secrets.TLSServer.Checksum == "" {
		return fmt.Errorf("missing TLS server secret information")
	}

	if k.secrets.KubeAggregator.Name == "" || k.secrets.KubeAggregator.Checksum == "" {
		return fmt.Errorf("missing kube aggregator secret information")
	}

	if k.secrets.KubeAPIServerKubelet.Name == "" || k.secrets.KubeAPIServerKubelet.Checksum == "" {
		return fmt.Errorf("missing kubelet secret information")
	}

	if k.secrets.StaticToken.Name == "" || k.secrets.StaticToken.Checksum == "" {
		return fmt.Errorf("missing staticToken secret information")
	}

	if k.secrets.ServiceAccountKey.Name == "" || k.secrets.ServiceAccountKey.Checksum == "" {
		return fmt.Errorf("missing service account key secret information")
	}

	if k.secrets.EtcdCA.Name == "" || k.secrets.EtcdCA.Checksum == "" {
		return fmt.Errorf("missing etcd CA secret information")
	}

	if k.secrets.EtcdClientTLS.Name == "" || k.secrets.EtcdClientTLS.Checksum == "" {
		return fmt.Errorf("missing etcd client TLS secret information")
	}

	if k.basicAuthenticationEnabled && (k.secrets.BasicAuth.Name == "" || k.secrets.BasicAuth.Checksum == "") {
		return fmt.Errorf("missing basic auth secret information")
	}

	if k.konnectivityTunnelEnabled && !k.sniValues.SNIEnabled && (k.secrets.KonnectivityServerCerts.Name == "" || k.secrets.KonnectivityServerCerts.Checksum == "") {
		return fmt.Errorf("missing konnectivity server certificate secret information")
	}

	if k.konnectivityTunnelEnabled && !k.sniValues.SNIEnabled && (k.secrets.KonnectivityServerKubeconfig.Name == "" || k.secrets.KonnectivityServerKubeconfig.Checksum == "") {
		return fmt.Errorf("missing konnectivity server kubeconfig secret information")
	}

	if k.konnectivityTunnelEnabled && k.sniValues.SNIEnabled && (k.secrets.KonnectivityServerClientTLS.Name == "" || k.secrets.KonnectivityServerClientTLS.Checksum == "") {
		return fmt.Errorf("missing konnectivity server client certificate secret information")
	}

	if !k.konnectivityTunnelEnabled && (k.secrets.VpnSeed.Name == "" || k.secrets.VpnSeed.Checksum == "") {
		return fmt.Errorf("missing vpn seed secret information")
	}

	if !k.konnectivityTunnelEnabled && (k.secrets.VpnSeedTLSAuth.Name == "" || k.secrets.VpnSeedTLSAuth.Checksum == "") {
		return fmt.Errorf("missing vpn seed  TLS auth secret information")
	}

	if k.etcdEncryptionEnabled && (k.secrets.EtcdEncryption.Name == "" || k.secrets.EtcdEncryption.Checksum == "") {
		return fmt.Errorf("missing etcd encryption secret information")
	}
	return nil
}

func (k *kubeAPIServer) getAdmissionPlugins() []gardencorev1beta1.AdmissionPlugin {
	admissionPlugins := kubernetes.GetAdmissionPluginsForVersion(k.shootKubernetesVersion.String())
	if k.config != nil {
		for _, plugin := range k.config.AdmissionPlugins {
			pluginOverwritesDefault := false

			for i, defaultPlugin := range admissionPlugins {
				if defaultPlugin.Name == plugin.Name {
					pluginOverwritesDefault = true
					admissionPlugins[i] = plugin
					break
				}
			}

			if !pluginOverwritesDefault {
				admissionPlugins = append(admissionPlugins, plugin)
			}
		}
	}
	return admissionPlugins
}

func (k *kubeAPIServer) Destroy(ctx context.Context) error {
	toDelete := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: k.seedNamespace,
		},
	}

	return client.IgnoreNotFound(k.seedClient.Client().Delete(ctx, toDelete, kubernetes.DefaultDeleteOptions...))
}

func (k *kubeAPIServer) Wait(ctx context.Context) error {
	deployment := &appsv1.Deployment{}

	if err := retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := k.seedClient.DirectClient().Get(ctx, kutil.Key(k.seedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil {
			return retry.SevereError(err)
		}
		if deployment.Generation != deployment.Status.ObservedGeneration {
			return retry.MinorError(fmt.Errorf("kube-apiserver not observed at latest generation (%d/%d)",
				deployment.Status.ObservedGeneration, deployment.Generation))
		}

		replicas := int32(0)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}
		if replicas != deployment.Status.UpdatedReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough updated replicas (%d/%d)",
				deployment.Status.UpdatedReplicas, replicas))
		}
		if replicas != deployment.Status.Replicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver deployment has outdated replicas"))
		}
		if replicas != deployment.Status.AvailableReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough available replicas (%d/%d)",
				deployment.Status.AvailableReplicas, replicas))
		}

		return retry.Ok()
	}); err != nil {
		var retryError *retry.Error
		if !errors.As(err, &retryError) {
			return err
		}

		newestPod, err2 := kutil.NewestPodForDeployment(ctx, k.seedClient.DirectClient(), deployment)
		if err2 != nil {
			return errorspkg.Wrapf(err, "failure to find the newest pod for deployment to read the logs: %s", err2.Error())
		}
		if newestPod == nil {
			return err
		}

		logs, err2 := kutil.MostRecentCompleteLogs(ctx, k.seedClient.Kubernetes().CoreV1().Pods(newestPod.Namespace), newestPod, "kube-apiserver", pointer.Int64Ptr(10))
		if err2 != nil {
			return errorspkg.Wrapf(err, "failure to read the logs: %s", err2.Error())
		}

		errWithLogs := fmt.Errorf("%s, logs of newest pod:\n%s", err.Error(), logs)
		return gardencorev1beta1helper.DetermineError(errWithLogs, errWithLogs.Error())
	}

	return nil
}

func (k *kubeAPIServer) WaitCleanup(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {
		deploy := &appsv1.Deployment{}
		err = k.seedClient.Client().Get(ctx, kutil.Key(k.seedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deploy)
		switch {
		case apierrors.IsNotFound(err):
			return retry.Ok()
		case err == nil:
			return retry.MinorError(err)
		default:
			return retry.SevereError(err)
		}
	})
}

func (k *kubeAPIServer) SetSecrets(secrets Secrets) {
	k.secrets = secrets
}

func (k *kubeAPIServer) SetHealthCheckToken(s string) {
	k.healthCheckToken = s
}

// Secrets is collection of secrets for the kube-apiserver.
type Secrets struct {
	// CA is a secret containing the CA certificate of the kube-apiserver.
	// used for kube-apiserver deployment flag: --client-CA-file
	// any request presenting a client certificate signed by one of the authorities in the client-CA-file
	// is authenticated with an identity corresponding to the CommonName of the client certificate
	CA component.Secret
	// CAFrontProxy is a secret containing the Root certificate bundle to use to verify client certificates on incoming
	// requests before trusting usernames in headers specified by --requestheader-username-headers
	// used for kube-apiserver deployment flag:  --requestheader-client-CA-file
	CAFrontProxy component.Secret
	// TLSServer is a secret containing the default x509 TLS server Certificate of the kube-apiserver.
	// used for kube-apiserver deployment flags: --tls-cert-file & --tls-private-key-file
	TLSServer component.Secret
	// KubeAggregator is a secret containing the client certificate and key used to prove the identity of the
	// aggregator or kube-apiserver when it must call out during a request.
	// This includes proxying requests to a user api-server and calling out to webhook admission plugins.
	// used for kube-apiserver deployment flags: --proxy-client-cert-file & -proxy-client-key-file
	KubeAggregator component.Secret
	// KubeAPIServerKubelet is a secret containing the client certificate and key for requests to the kubelet.
	// used for kube-apiserver deployment flags: --kubelet-client-certificate & --kubelet-client-key
	KubeAPIServerKubelet component.Secret
	// KubeAPIServerKubelet is a secret containing static tokens for the kube-apiserver
	// used to secure the secure port of the API server via token authentication.
	// used for kube-apiserver deployment flag: --token-auth-file
	StaticToken component.Secret
	// ServiceAccountKey is a secret containing the PEM-encoded x509 RSA or ECDSA private or public keys, used to verify ServiceAccount tokens.
	// used for kube-apiserver deployment flag: --service-account-key-file
	ServiceAccountKey component.Secret
	// EtcdCA is a secret containing the CA certificate of the etcd.
	// used for kube-apiserver deployment flag: --etcd-cafile
	EtcdCA component.Secret
	// EtcdClientTLS is a secret containing the client certificate and key for communication with etcd.
	// used for kube-apiserver deployment flags: --etcd-certfile && --etcd-keyfile
	EtcdClientTLS component.Secret
	// BasicAuth is a secret containing the basic auth credentials for the API Server.
	BasicAuth component.Secret
	// KonnectivityServerCerts is a secret containing the default x509 TLS server Certificate of the konnectivity server.
	KonnectivityServerCerts component.Secret
	// KonnectivityServerKubeconfig is a secret containing the kubeconfig for the konnectivity server to talk to the Shoot API Server.
	KonnectivityServerKubeconfig component.Secret
	// KonnectivityServerCerts is a secret containing the client certificate to talk to the external the konnectivity server.
	// only required if konnectivity + SNI enabled
	KonnectivityServerClientTLS component.Secret
	// VpnSeed is a secret containing the CA and client certificate plus key used to communicate with the Shoot API server.
	VpnSeed component.Secret
	// VpnSeedTLSAuth is a secret containing the vpn tls authentication keys.
	VpnSeedTLSAuth component.Secret
	// EtcdEncryption is a secret containing the etcd encryption key.
	EtcdEncryption component.Secret
}

var (
	encoderCoreV1 = kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, corev1.SchemeGroupVersion)

	versionConstraintK8sSmaller111      *semver.Constraints
	versionConstraintK8sEqual111        *semver.Constraints
	versionConstraintK8sSmaller112      *semver.Constraints
	versionConstraintK8sGreaterEqual112 *semver.Constraints
	versionConstraintK8sSmaller113      *semver.Constraints
	versionConstraintK8sSmaller114      *semver.Constraints
	versionConstraintK8sGreaterEqual115 *semver.Constraints
	versionConstraintK8sSmaller116      *semver.Constraints
	versionConstraintK8sGreaterEqual116 *semver.Constraints
	versionConstraintK8sGreaterEqual117 *semver.Constraints
	versionConstraintK8sSmaller120      *semver.Constraints
)

func init() {
	var err error

	versionConstraintK8sSmaller111, err = semver.NewConstraint("< 1.11")
	utilruntime.Must(err)
	versionConstraintK8sEqual111, err = semver.NewConstraint("~ 1.11")
	utilruntime.Must(err)
	versionConstraintK8sSmaller112, err = semver.NewConstraint("< 1.12")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual112, err = semver.NewConstraint(">= 1.12")
	utilruntime.Must(err)
	versionConstraintK8sSmaller113, err = semver.NewConstraint("< 1.13")
	utilruntime.Must(err)
	versionConstraintK8sSmaller114, err = semver.NewConstraint("< 1.14")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual115, err = semver.NewConstraint(">= 1.15")
	utilruntime.Must(err)
	versionConstraintK8sSmaller116, err = semver.NewConstraint("< 1.16")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual116, err = semver.NewConstraint(">= 1.16")
	utilruntime.Must(err)
	versionConstraintK8sGreaterEqual117, err = semver.NewConstraint(">= 1.17")
	utilruntime.Must(err)
	versionConstraintK8sSmaller120, err = semver.NewConstraint("< 1.20")
	utilruntime.Must(err)
}
