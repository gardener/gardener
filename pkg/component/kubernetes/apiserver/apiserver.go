// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// SecretNameServerCert is the name of the kube-apiserver server certificate secret.
	SecretNameServerCert = "kube-apiserver"
	// ServicePortName is the name of the port in the service.
	ServicePortName = "kube-apiserver"
	// UserNameVPNSeedClient is the user name for the HA vpn-seed-client components (used as common name in its client certificate)
	UserNameVPNSeedClient = "vpn-seed-client"

	userName = "system:kube-apiserver:kubelet"
)

// Interface contains functions for a kube-apiserver deployer.
type Interface interface {
	apiserver.Interface
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
	// AppendAuthorizationWebhook appends an AuthorizationWebhook to AuthorizationWebhooks in the Values of the deployer.
	// TODO(oliver-goetz): Consider removing this method when we support Kubernetes version with structured authorization only.
	//  See https://github.com/gardener/gardener/pull/10682#discussion_r1816324389 for more information.
	AppendAuthorizationWebhook(AuthorizationWebhook) error
	// SetExternalHostname sets the ExternalHostname field in the Values of the deployer.
	SetExternalHostname(string)
	// SetExternalServer sets the ExternalServer field in the Values of the deployer.
	SetExternalServer(string)
	// SetNodeNetworkCIDRs sets the node CIDRs of the shoot network.
	SetNodeNetworkCIDRs([]net.IPNet)
	// SetServiceNetworkCIDRs sets the service CIDRs of the shoot network.
	SetServiceNetworkCIDRs([]net.IPNet)
	// SetPodNetworkCIDRs sets the pod CIDRs of the shoot network.
	SetPodNetworkCIDRs([]net.IPNet)
	// SetServerCertificateConfig sets the ServerCertificateConfig field in the Values of the deployer.
	SetServerCertificateConfig(ServerCertificateConfig)
	// SetServiceAccountConfig sets the ServiceAccount field in the Values of the deployer.
	SetServiceAccountConfig(ServiceAccountConfig)
	// SetSNIConfig sets the SNI field in the Values of the deployer.
	SetSNIConfig(SNIConfig)
}

// Values contains configuration values for the kube-apiserver resources.
type Values struct {
	apiserver.Values
	// AnonymousAuthenticationEnabled states whether anonymous authentication is enabled.
	AnonymousAuthenticationEnabled bool
	// APIAudiences are identifiers of the API. The service account token authenticator will validate that tokens used
	// against the API are bound to at least one of these audiences.
	APIAudiences []string
	// AuthenticationConfiguration contains authentication configuration.
	AuthenticationConfiguration *string
	// AuthenticationWebhook contains configuration for the authentication webhook.
	AuthenticationWebhook *AuthenticationWebhook
	// AuthorizationWebhook contains configuration for the authorization webhooks.
	AuthorizationWebhooks []AuthorizationWebhook
	// Autoscaling contains information for configuring autoscaling settings for the API server.
	Autoscaling AutoscalingConfig
	// DefaultNotReadyTolerationSeconds indicates the tolerationSeconds of the toleration for notReady:NoExecute
	// that is added by default to every pod that does not already have such a toleration (flag `--default-not-ready-toleration-seconds`).
	DefaultNotReadyTolerationSeconds *int64
	// DefaultUnreachableTolerationSeconds indicates the tolerationSeconds of the toleration for unreachable:NoExecute
	// that is added by default to every pod that does not already have such a toleration (flag `--default-unreachable-toleration-seconds`).
	DefaultUnreachableTolerationSeconds *int64
	// EventTTL is the amount of time to retain events.
	EventTTL *metav1.Duration
	// ExternalHostname is the external hostname which should be exposed by the kube-apiserver.
	ExternalHostname string
	// ExternalServer is the external server which should be used when generating the user kubeconfig.
	ExternalServer string
	// Images is a set of container images used for the containers of the kube-apiserver pods.
	Images Images
	// IsWorkerless specifies whether the cluster managed by this API server has worker nodes.
	IsWorkerless bool
	// NamePrefix is the prefix for the resource names.
	NamePrefix string
	// OIDC contains information for configuring OIDC settings for the kube-apiserver.
	OIDC *gardencorev1beta1.OIDCConfig
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// ResourcesToStoreInETCDEvents is a list of resources which should be stored in the etcd-events instead of the
	// etcd-main. The `events` resource in the `core` group is always stored in etcd-events.
	ResourcesToStoreInETCDEvents []schema.GroupResource
	// RuntimeConfig is the set of runtime configurations.
	RuntimeConfig map[string]bool
	// ServerCertificate contains configuration for the server certificate.
	ServerCertificate ServerCertificateConfig
	// ServiceAccount contains information for configuring ServiceAccount settings for the kube-apiserver.
	ServiceAccount ServiceAccountConfig
	// ServiceNetworkCIDRs are the CIDRs of the service network.
	ServiceNetworkCIDRs []net.IPNet
	// SNI contains information for configuring SNI settings for the kube-apiserver.
	SNI SNIConfig
	// Version is the Kubernetes version for the kube-apiserver.
	Version *semver.Version
	// VPN contains information for configuring the VPN settings for the kube-apiserver.
	VPN VPNConfig
}

// AuthenticationWebhook contains configuration for the authentication webhook.
type AuthenticationWebhook struct {
	// Kubeconfig contains the webhook configuration for token authentication in kubeconfig format. The API server will
	// query the remote service to determine authentication for bearer tokens.
	Kubeconfig []byte
	// CacheTTL is the duration to cache responses from the webhook token authenticator.
	CacheTTL *time.Duration
	// Version is the API version of the authentication.k8s.io TokenReview to send to and expect from the webhook.
	Version *string
}

// AuthorizationWebhook contains configuration for the authorization webhook.
type AuthorizationWebhook struct {
	// Name is the name of the webhook.
	Name string
	// Kubeconfig contains the webhook configuration in kubeconfig format. The API server will query the remote service
	// to determine access on the API server's secure port.
	Kubeconfig []byte
	// WebhookConfiguration is the actual webhook configuration.
	apiserverv1beta1.WebhookConfiguration
}

// AutoscalingConfig contains information for configuring autoscaling settings for the API server.
type AutoscalingConfig struct {
	// APIServerResources are the resource requirements for the API server container.
	APIServerResources corev1.ResourceRequirements
	// Replicas is the number of pod replicas for the API server.
	Replicas *int32
	// MinReplicas are the minimum Replicas for horizontal autoscaling.
	MinReplicas int32
	// MaxReplicas are the maximum Replicas for horizontal autoscaling.
	MaxReplicas int32
	// ScaleDownDisabled states whether scale-down shall be disabled.
	ScaleDownDisabled bool
	// MinAllowed are the minimum allowed resources for vertical autoscaling.
	MinAllowed corev1.ResourceList
}

// Images is a set of container images used for the containers of the kube-apiserver pods.
type Images struct {
	// KubeAPIServer is the container image for the kube-apiserver.
	KubeAPIServer string
	// VPNClient is the container image for the vpn-seed-client.
	VPNClient string
}

// VPNConfig contains information for configuring the VPN settings for the kube-apiserver.
type VPNConfig struct {
	// Enabled states whether VPN is enabled.
	Enabled bool
	// PodNetworkCIDRs are the CIDRs of the pod network.
	PodNetworkCIDRs []net.IPNet
	// NodeNetworkCIDRs are the CIDRs of the node network.
	NodeNetworkCIDRs []net.IPNet
	// HighAvailabilityEnabled states if VPN uses HA configuration.
	HighAvailabilityEnabled bool
	// HighAvailabilityNumberOfSeedServers is the number of VPN seed servers used for HA.
	HighAvailabilityNumberOfSeedServers int
	// HighAvailabilityNumberOfShootClients is the number of VPN shoot clients used for HA.
	HighAvailabilityNumberOfShootClients int
	// IPFamilies are the IPFamilies of the shoot.
	IPFamilies []gardencorev1beta1.IPFamily
}

// ServerCertificateConfig contains configuration for the server certificate.
type ServerCertificateConfig struct {
	// ExtraIPAddresses is a list of additional IP addresses to use for the SANS of the server certificate.
	ExtraIPAddresses []net.IP
	// ExtraDNSNames is a list of additional DNS names to use for the SANS of the server certificate.
	ExtraDNSNames []string
}

// ServiceAccountConfig contains information for configuring ServiceAccountConfig settings for the kube-apiserver.
type ServiceAccountConfig struct {
	// Issuer is the issuer of service accounts.
	Issuer string
	// AcceptedIssuers is an additional set of issuers that are used to determine which service account tokens are accepted.
	AcceptedIssuers []string
	// JWKSURI is used to overwrite the URI for the JSON Web Key Set in the discovery document served at /.well-known/openid-configuration.
	JWKSURI *string
	// ExtendTokenExpiration states whether the service account token expirations should be extended.
	ExtendTokenExpiration *bool
	// MaxTokenExpiration states what the maximal token expiration should be.
	MaxTokenExpiration *metav1.Duration
	// RotationPhase specifies the credentials rotation phase of the service account signing key.
	RotationPhase gardencorev1beta1.CredentialsRotationPhase
}

// SNIConfig contains information for configuring SNI settings for the kube-apiserver.
type SNIConfig struct {
	// Enabled states whether the SNI feature is enabled.
	Enabled bool
	// AdvertiseAddress is the address which should be advertised by the kube-apiserver.
	AdvertiseAddress string
	// TLS contains information for configuring the TLS SNI settings for the kube-apiserver.
	TLS []TLSSNIConfig
}

// TLSSNIConfig contains information for configuring the TLS SNI settings for the kube-apiserver.
type TLSSNIConfig struct {
	// SecretName is the name for an existing secret containing the TLS certificate and private key. Either this or both
	// Certificate and PrivateKey must be specified. If both is provided, SecretName is taking precedence.
	SecretName *string
	// Certificate is the TLS certificate. Either both this and PrivateKey, or SecretName must be specified. If both is
	// provided, SecretName is taking precedence.
	Certificate []byte
	// PrivateKey is the TLS certificate. Either both this and Certificate, or SecretName must be specified. If both is
	// provided, SecretName is taking precedence.
	PrivateKey []byte
	// DomainPatterns is an optional list of domain patterns which are fully qualified domain names, possibly with
	// prefixed wildcard segments. The domain patterns also allow IP addresses, but IPs should only be used if the
	// apiserver has visibility to the IP address requested by a client. If no domain patterns are provided, the names
	// of the certificate are extracted. Non-wildcard matches trump over wildcard matches, explicit domain patterns
	// trump over extracted names.
	DomainPatterns []string
}

// New creates a new instance of DeployWaiter for the kube-apiserver.
func New(client kubernetes.Interface, namespace string, secretsManager secretsmanager.Interface, values Values) Interface {
	return &kubeAPIServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type kubeAPIServer struct {
	client         kubernetes.Interface
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (k *kubeAPIServer) Deploy(ctx context.Context) error {
	var (
		deployment                             = k.emptyDeployment()
		podDisruptionBudget                    = k.emptyPodDisruptionBudget()
		horizontalPodAutoscaler                = k.emptyHorizontalPodAutoscaler()
		verticalPodAutoscaler                  = k.emptyVerticalPodAutoscaler()
		secretETCDEncryptionConfiguration      = k.emptySecret(v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration)
		secretOIDCCABundle                     = k.emptySecret(secretOIDCCABundleNamePrefix)
		secretAuditWebhookKubeconfig           = k.emptySecret(secretAuditWebhookKubeconfigNamePrefix)
		secretAuthenticationWebhookKubeconfig  = k.emptySecret(secretAuthenticationWebhookKubeconfigNamePrefix)
		secretAuthorizationWebhooksKubeconfigs = k.emptySecret(secretAuthorizationWebhooksKubeconfigsNamePrefix)
		configMapAdmissionConfigs              = k.emptyConfigMap(configMapAdmissionNamePrefix)
		secretAdmissionKubeconfigs             = k.emptySecret(secretAdmissionKubeconfigsNamePrefix)
		configMapAuditPolicy                   = k.emptyConfigMap(configMapAuditPolicyNamePrefix)
		configMapAuthenticationConfig          = k.emptyConfigMap(configMapAuthenticationConfigNamePrefix)
		configMapAuthorizationConfig           = k.emptyConfigMap(configMapAuthorizationConfigNamePrefix)
		configMapEgressSelector                = k.emptyConfigMap(configMapEgressSelectorNamePrefix)
	)

	if err := k.reconcilePodDisruptionBudget(ctx, podDisruptionBudget); err != nil {
		return err
	}

	if err := k.reconcileHorizontalPodAutoscaler(ctx, horizontalPodAutoscaler, deployment); err != nil {
		return err
	}

	if err := k.reconcileVerticalPodAutoscaler(ctx, verticalPodAutoscaler, deployment); err != nil {
		return err
	}

	if err := k.reconcileSecretETCDEncryptionConfiguration(ctx, secretETCDEncryptionConfiguration); err != nil {
		return err
	}

	if err := k.reconcileSecretOIDCCABundle(ctx, secretOIDCCABundle); err != nil {
		return err
	}

	if err := k.reconcileSecretAuthenticationWebhookKubeconfig(ctx, secretAuthenticationWebhookKubeconfig); err != nil {
		return err
	}

	if err := k.reconcileSecretAuthorizationWebhooksKubeconfigs(ctx, secretAuthorizationWebhooksKubeconfigs); err != nil {
		return err
	}

	secretServiceAccountKey, err := k.reconcileSecretServiceAccountKey(ctx)
	if err != nil {
		return err
	}

	secretHTTPProxy, err := k.reconcileSecretHTTPProxy(ctx)
	if err != nil {
		return err
	}

	secretKubeAggregator, err := k.reconcileSecretKubeAggregator(ctx)
	if err != nil {
		return err
	}

	secretKubeletClient, err := k.reconcileSecretKubeletClient(ctx)
	if err != nil {
		return err
	}

	secretServer, err := k.reconcileSecretServer(ctx)
	if err != nil {
		return err
	}

	secretStaticToken, err := k.reconcileSecretStaticToken(ctx)
	if err != nil {
		return err
	}

	if err := apiserver.ReconcileConfigMapAdmission(ctx, k.client.Client(), configMapAdmissionConfigs, k.values.Values); err != nil {
		return err
	}
	if err := apiserver.ReconcileSecretAdmissionKubeconfigs(ctx, k.client.Client(), secretAdmissionKubeconfigs, k.values.Values); err != nil {
		return err
	}

	if err := apiserver.ReconcileConfigMapAuditPolicy(ctx, k.client.Client(), configMapAuditPolicy, k.values.Audit); err != nil {
		return err
	}
	if err := apiserver.ReconcileSecretAuditWebhookKubeconfig(ctx, k.client.Client(), secretAuditWebhookKubeconfig, k.values.Audit); err != nil {
		return err
	}

	if err := k.reconcileConfigMapAuthenticationConfig(ctx, configMapAuthenticationConfig); err != nil {
		return err
	}

	if err := k.reconcileConfigMapAuthorizationConfig(ctx, configMapAuthorizationConfig); err != nil {
		return err
	}

	if err := k.reconcileConfigMapEgressSelector(ctx, configMapEgressSelector); err != nil {
		return err
	}

	secretHAVPNSeedClient, err := k.reconcileSecretHAVPNSeedClient(ctx)
	if err != nil {
		return err
	}

	secretHAVPNClientSeedTLSAuth, err := k.reconcileSecretHAVPNSeedClientTLSAuth(ctx)
	if err != nil {
		return err
	}

	tlsSNISecrets, err := k.reconcileTLSSNISecrets(ctx)
	if err != nil {
		return err
	}

	var serviceAccount *corev1.ServiceAccount
	if k.values.VPN.Enabled && k.values.VPN.HighAvailabilityEnabled {
		serviceAccount = k.emptyServiceAccount()
		if err := k.reconcileServiceAccount(ctx, serviceAccount); err != nil {
			return err
		}
		if err := k.reconcileRoleHAVPN(ctx); err != nil {
			return err
		}
		if err := k.reconcileRoleBindingHAVPN(ctx, serviceAccount); err != nil {
			return err
		}
	} else {
		if err := kubernetesutils.DeleteObjects(ctx, k.client.Client(),
			k.emptyServiceAccount(),
			k.emptyRoleHAVPN(),
			k.emptyRoleBindingHAVPN(),
		); err != nil {
			return err
		}
	}

	if err := k.reconcileDeployment(
		ctx,
		deployment,
		serviceAccount,
		configMapAuditPolicy,
		configMapAuthenticationConfig,
		configMapAuthorizationConfig,
		configMapAdmissionConfigs,
		secretAdmissionKubeconfigs,
		configMapEgressSelector,
		secretETCDEncryptionConfiguration,
		secretOIDCCABundle,
		secretServiceAccountKey,
		secretStaticToken,
		secretServer,
		secretKubeletClient,
		secretKubeAggregator,
		secretHTTPProxy,
		secretHAVPNSeedClient,
		secretHAVPNClientSeedTLSAuth,
		secretAuditWebhookKubeconfig,
		secretAuthenticationWebhookKubeconfig,
		secretAuthorizationWebhooksKubeconfigs,
		tlsSNISecrets,
	); err != nil {
		return err
	}

	if err := k.reconcileServiceMonitor(ctx, k.emptyServiceMonitor()); err != nil {
		return err
	}

	// apiserver deployed for shoot cluster
	if k.values.NamePrefix == "" {
		if err := k.reconcilePrometheusRule(ctx, k.emptyPrometheusRule()); err != nil {
			return err
		}
	}

	if !k.values.IsWorkerless {
		data, err := k.computeShootResourcesData()
		if err != nil {
			return err
		}

		return managedresources.CreateForShoot(ctx, k.client.Client(), k.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
	}

	return nil
}

func (k *kubeAPIServer) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, k.client.Client(),
		k.emptyManagedResource(),
		k.emptyHorizontalPodAutoscaler(),
		k.emptyVerticalPodAutoscaler(),
		k.emptyPodDisruptionBudget(),
		k.emptyDeployment(),
		k.emptyServiceAccount(),
		k.emptyRoleHAVPN(),
		k.emptyRoleBindingHAVPN(),
		k.emptyServiceMonitor(),
		k.emptyPrometheusRule(),
	)
}

var (
	// IntervalWaitForDeployment is the interval used while waiting for the Deployments to become healthy
	// or deleted.
	IntervalWaitForDeployment = 5 * time.Second
	// TimeoutWaitForDeployment is the timeout used while waiting for the Deployments to become healthy
	// or deleted.
	TimeoutWaitForDeployment = 5 * time.Minute
	// Until is an alias for retry.Until. Exposed for tests.
	Until = retry.Until
)

func (k *kubeAPIServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForDeployment)
	defer cancel()

	deployment := k.emptyDeployment()

	if err := Until(timeoutCtx, IntervalWaitForDeployment, health.IsDeploymentUpdated(k.client.APIReader(), deployment)); err != nil {
		var (
			retryError *retry.Error
			headBytes  *int64
			tailLines  = ptr.To[int64](10)
		)

		if !errors.As(err, &retryError) {
			return err
		}

		newestPod, err2 := kubernetesutils.NewestPodForDeployment(ctx, k.client.APIReader(), deployment)
		if err2 != nil {
			return fmt.Errorf("failure to find the newest pod for deployment to read the logs: %s: %w", err2.Error(), err)
		}
		if newestPod == nil {
			return err
		}

		logs, err2 := kubernetesutils.MostRecentCompleteLogs(ctx, k.client.Kubernetes().CoreV1().Pods(newestPod.Namespace), newestPod, ContainerNameKubeAPIServer, tailLines, headBytes)
		if err2 != nil {
			return fmt.Errorf("failure to read the logs: %s: %w", err2.Error(), err)
		}

		return fmt.Errorf("%s, logs of newest pod:\n%s", err.Error(), logs)
	}

	return nil
}

func (k *kubeAPIServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForDeployment)
	defer cancel()

	return retry.Until(timeoutCtx, IntervalWaitForDeployment, func(ctx context.Context) (done bool, err error) {
		deploy := k.emptyDeployment()
		err = k.client.Client().Get(ctx, client.ObjectKeyFromObject(deploy), deploy)

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

func (k *kubeAPIServer) GetValues() Values {
	return k.values
}

func (k *kubeAPIServer) SetAutoscalingAPIServerResources(resources corev1.ResourceRequirements) {
	k.values.Autoscaling.APIServerResources = resources
}

func (k *kubeAPIServer) GetAutoscalingReplicas() *int32 {
	return k.values.Autoscaling.Replicas
}

func (k *kubeAPIServer) AppendAuthorizationWebhook(webhook AuthorizationWebhook) error {
	for _, existingWebhook := range k.values.AuthorizationWebhooks {
		if existingWebhook.Name == webhook.Name {
			return fmt.Errorf("authorization webhook with name %q already exists", webhook.Name)
		}
	}

	k.values.AuthorizationWebhooks = append(k.values.AuthorizationWebhooks, webhook)

	return nil
}

func (k *kubeAPIServer) SetAutoscalingReplicas(replicas *int32) {
	k.values.Autoscaling.Replicas = replicas
}

func (k *kubeAPIServer) SetETCDEncryptionConfig(config apiserver.ETCDEncryptionConfig) {
	k.values.ETCDEncryption = config
}

func (k *kubeAPIServer) SetExternalHostname(hostname string) {
	k.values.ExternalHostname = hostname
}

func (k *kubeAPIServer) SetExternalServer(server string) {
	k.values.ExternalServer = server
}

func (k *kubeAPIServer) SetNodeNetworkCIDRs(nodes []net.IPNet) {
	k.values.VPN.NodeNetworkCIDRs = nodes
}

func (k *kubeAPIServer) SetPodNetworkCIDRs(pods []net.IPNet) {
	k.values.VPN.PodNetworkCIDRs = pods
}

func (k *kubeAPIServer) SetServiceNetworkCIDRs(services []net.IPNet) {
	k.values.ServiceNetworkCIDRs = services
}

func (k *kubeAPIServer) SetServerCertificateConfig(config ServerCertificateConfig) {
	k.values.ServerCertificate = config
}

func (k *kubeAPIServer) SetServiceAccountConfig(config ServiceAccountConfig) {
	k.values.ServiceAccount = config
}

func (k *kubeAPIServer) SetSNIConfig(config SNIConfig) {
	k.values.SNI = config
}

func (k *kubeAPIServer) prometheusAccessSecretName() string {
	if k.values.NamePrefix != "" {
		return garden.AccessSecretName
	}
	return shoot.AccessSecretName
}

func (k *kubeAPIServer) prometheusLabel() string {
	if k.values.NamePrefix != "" {
		return garden.Label
	}
	return shoot.Label
}

// GetLabels returns the labels for the kube-apiserver.
func GetLabels() map[string]string {
	return utils.MergeStringMaps(getLabels(), map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
	})
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}

// ComputeKubeAPIServerServiceAccountConfig computes the [ServiceAccountConfig]
// needed to configure a kube-apiserver.
func ComputeKubeAPIServerServiceAccountConfig(
	config *gardencorev1beta1.ServiceAccountConfig,
	externalHostname string,
	serviceAccountKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
) ServiceAccountConfig {
	var (
		defaultIssuer = "https://" + externalHostname
		out           = ServiceAccountConfig{
			Issuer:        defaultIssuer,
			RotationPhase: serviceAccountKeyRotationPhase,
		}
	)

	if config == nil {
		return out
	}

	out.ExtendTokenExpiration = config.ExtendTokenExpiration
	out.MaxTokenExpiration = config.MaxTokenExpiration

	if config.Issuer != nil {
		out.Issuer = *config.Issuer
	}
	out.AcceptedIssuers = config.AcceptedIssuers
	if out.Issuer != defaultIssuer && !slices.Contains(out.AcceptedIssuers, defaultIssuer) {
		out.AcceptedIssuers = append(out.AcceptedIssuers, defaultIssuer)
	}
	if config.Issuer == nil {
		// ensure defaultIssuer is not duplicated in the accepted issuers
		for i, val := range out.AcceptedIssuers {
			if val == defaultIssuer {
				out.AcceptedIssuers = append(out.AcceptedIssuers[:i], out.AcceptedIssuers[i+1:]...)
				break
			}
		}
	}

	return out
}

// ConfigCodec is the code for kube-apiserver configuration APIs.
var ConfigCodec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(apiserverv1alpha1.AddToScheme(scheme))
	utilruntime.Must(apiserverv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiserverv1.AddToScheme(scheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			apiserverv1alpha1.SchemeGroupVersion,
			apiserverv1alpha1.ConfigSchemeGroupVersion,
			apiserverv1beta1.SchemeGroupVersion,
			apiserverv1beta1.ConfigSchemeGroupVersion,
			apiserverv1.SchemeGroupVersion,
		})
	)

	ConfigCodec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}
