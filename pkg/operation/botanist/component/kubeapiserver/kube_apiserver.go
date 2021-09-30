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

package kubeapiserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Port is the port exposed by the kube-apiserver.
	Port = 443
	// ServicePortName is the name of the port in the service.
	ServicePortName = "kube-apiserver"
	// UserName is the name of the kube-apiserver user when communicating with the kubelet.
	UserName = "system:kube-apiserver:kubelet"
)

// Interface contains functions for a kube-apiserver deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
	// SetSecrets sets the secrets.
	SetSecrets(Secrets)
	// SetAutoscalingAPIServerResources sets the APIServerResources field in the AutoscalingConfig of the Values of the
	// deployer.
	SetAutoscalingAPIServerResources(corev1.ResourceRequirements)
	// SetAutoscalingReplicas sets the Replicas field in the AutoscalingConfig of the Values of the deployer.
	SetAutoscalingReplicas(*int32)
	// SetServiceAccountConfig sets the ServiceAccount field in the Values of the deployer.
	SetServiceAccountConfig(ServiceAccountConfig)
	// SetSNIConfig sets the SNI field in the Values of the deployer.
	SetSNIConfig(SNIConfig)
	// SetProbeToken sets the ProbeToken field in the Values of the deployer.
	SetProbeToken(string)
	// SetExternalHostname sets the ExternalHostname field in the Values of the deployer.
	SetExternalHostname(string)
}

// Values contains configuration values for the kube-apiserver resources.
type Values struct {
	// AdmissionPlugins is the list of admission plugins with configuration for the kube-apiserver.
	AdmissionPlugins []gardencorev1beta1.AdmissionPlugin
	// AnonymousAuthenticationEnabled states whether anonymous authentication is enabled.
	AnonymousAuthenticationEnabled bool
	// APIAudiences are identifiers of the API. The service account token authenticator will validate that tokens used
	// against the API are bound to at least one of these audiences.
	APIAudiences []string
	// Audit contains information for configuring audit settings for the kube-apiserver.
	Audit *AuditConfig
	// Autoscaling contains information for configuring autoscaling settings for the kube-apiserver.
	Autoscaling AutoscalingConfig
	// BasicAuthenticationEnabled states whether basic authentication is enabled.
	BasicAuthenticationEnabled bool
	// ExternalHostname is the external hostname which should be exposed by the kube-apiserver.
	ExternalHostname string
	// FeatureGates is the set of feature gates.
	FeatureGates map[string]bool
	// Images is a set of container images used for the containers of the kube-apiserver pods.
	Images Images
	// OIDC contains information for configuring OIDC settings for the kube-apiserver.
	OIDC *gardencorev1beta1.OIDCConfig
	// ProbeToken is the JWT token used for {live,readi}ness probes of the kube-apiserver container.
	ProbeToken string
	// Requests contains configuration for the kube-apiserver requests.
	Requests *gardencorev1beta1.KubeAPIServerRequests
	// RuntimeConfig is the set of runtime configurations.
	RuntimeConfig map[string]bool
	// ServiceAccount contains information for configuring ServiceAccount settings for the kube-apiserver.
	ServiceAccount ServiceAccountConfig
	// SNI contains information for configuring SNI settings for the kube-apiserver.
	SNI SNIConfig
	// Version is the Kubernetes version for the kube-apiserver.
	Version *semver.Version
	// VPN contains information for configuring the VPN settings for the kube-apiserver.
	VPN VPNConfig
	// WatchCacheSizes are the configured sizes for the watch caches.
	WatchCacheSizes *gardencorev1beta1.WatchCacheSizes
}

// AuditConfig contains information for configuring audit settings for the kube-apiserver.
type AuditConfig struct {
	// Policy is the audit policy document in YAML format.
	Policy *string
}

// AutoscalingConfig contains information for configuring autoscaling settings for the kube-apiserver.
type AutoscalingConfig struct {
	// APIServerResources are the resource requirements for the kube-apiserver container.
	APIServerResources corev1.ResourceRequirements
	// HVPAEnabled states whether an HVPA object shall be deployed. If false, HPA and VPA will be used.
	HVPAEnabled bool
	// Replicas is the number of pod replicas for the kube-apiserver.
	Replicas *int32
	// MinReplicas are the minimum Replicas for horizontal autoscaling.
	MinReplicas int32
	// MaxReplicas are the maximum Replicas for horizontal autoscaling.
	MaxReplicas int32
	// UseMemoryMetricForHvpaHPA states whether the memory metric shall be used when the HPA is configured in an HVPA
	// resource.
	UseMemoryMetricForHvpaHPA bool
	// ScaleDownDisabledForHvpa states whether scale-down shall be disabled when HPA or VPA are configured in an HVPA
	// resource.
	ScaleDownDisabledForHvpa bool
}

// Images is a set of container images used for the containers of the kube-apiserver pods.
type Images struct {
	// AlpineIPTables is the container image for alpine-iptables.
	AlpineIPTables string
	// APIServerProxyPodWebhook is the container image for the apiserver-proxy-pod-webhook.
	APIServerProxyPodWebhook string
	// KubeAPIServer is the container image for the kube-apiserver.
	KubeAPIServer string
	// VPNSeed is the container image for the vpn-seed.
	VPNSeed string
}

// VPNConfig contains information for configuring the VPN settings for the kube-apiserver.
type VPNConfig struct {
	// ReversedVPNEnabled states whether the 'ReversedVPN' feature gate is enabled.
	ReversedVPNEnabled bool
	// PodNetworkCIDR is the CIDR of the pod network.
	PodNetworkCIDR string
	// ServiceNetworkCIDR is the CIDR of the service network.
	ServiceNetworkCIDR string
	// NodeNetworkCIDR is the CIDR of the node network.
	NodeNetworkCIDR *string
}

// ServiceAccountConfig contains information for configuring ServiceAccountConfig settings for the kube-apiserver.
type ServiceAccountConfig struct {
	// Issuer is the issuer of service accounts.
	Issuer string
	// SigningKey is the key used when service accounts are signed.
	SigningKey []byte
	// ExtendTokenExpiration states whether the service account token expirations should be extended.
	ExtendTokenExpiration *bool
	// MaxTokenExpiration states what the maximal token expiration should be.
	MaxTokenExpiration *metav1.Duration
}

// SNIConfig contains information for configuring SNI settings for the kube-apiserver.
type SNIConfig struct {
	// Enabled states whether the SNI feature is enabled.
	Enabled bool
	// PodMutatorEnabled states whether the pod mutator is enabled.
	PodMutatorEnabled bool
	// APIServerFQDN is the fully qualified domain name for the kube-apiserver.
	APIServerFQDN string
	// AdvertiseAddress is the address which should be advertised by the kube-apiserver.
	AdvertiseAddress string
}

// New creates a new instance of DeployWaiter for the kube-apiserver.
func New(client kubernetes.Interface, namespace string, values Values) Interface {
	return &kubeAPIServer{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type kubeAPIServer struct {
	client    kubernetes.Interface
	namespace string
	values    Values
	secrets   Secrets
}

func (k *kubeAPIServer) Deploy(ctx context.Context) error {
	if err := k.secrets.verify(k.values); err != nil {
		return err
	}

	var (
		deployment                           = k.emptyDeployment()
		podDisruptionBudget                  = k.emptyPodDisruptionBudget()
		horizontalPodAutoscaler              = k.emptyHorizontalPodAutoscaler()
		verticalPodAutoscaler                = k.emptyVerticalPodAutoscaler()
		hvpa                                 = k.emptyHVPA()
		networkPolicyAllowFromShootAPIServer = k.emptyNetworkPolicy(networkPolicyNameAllowFromShootAPIServer)
		networkPolicyAllowToShootAPIServer   = k.emptyNetworkPolicy(networkPolicyNameAllowToShootAPIServer)
		networkPolicyAllowKubeAPIServer      = k.emptyNetworkPolicy(networkPolicyNameAllowKubeAPIServer)
		secretOIDCCABundle                   = k.emptySecret(secretOIDCCABundleNamePrefix)
		secretServiceAccountSigningKey       = k.emptySecret(secretServiceAccountSigningKeyNamePrefix)
		configMapAdmission                   = k.emptyConfigMap(configMapAdmissionNamePrefix)
		configMapAuditPolicy                 = k.emptyConfigMap(configMapAuditPolicyNamePrefix)
		configMapEgressSelector              = k.emptyConfigMap(configMapEgressSelectorNamePrefix)
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

	if err := k.reconcileHVPA(ctx, hvpa, deployment); err != nil {
		return err
	}

	if err := k.reconcileNetworkPolicyAllowFromShootAPIServer(ctx, networkPolicyAllowFromShootAPIServer); err != nil {
		return err
	}

	if err := k.reconcileNetworkPolicyAllowToShootAPIServer(ctx, networkPolicyAllowToShootAPIServer); err != nil {
		return err
	}

	if err := k.reconcileNetworkPolicyAllowKubeAPIServer(ctx, networkPolicyAllowKubeAPIServer); err != nil {
		return err
	}

	if err := k.reconcileSecretOIDCCABundle(ctx, secretOIDCCABundle); err != nil {
		return err
	}

	if err := k.reconcileSecretServiceAccountSigningKey(ctx, secretServiceAccountSigningKey); err != nil {
		return err
	}

	if err := k.reconcileConfigMapAdmission(ctx, configMapAdmission); err != nil {
		return err
	}

	if err := k.reconcileConfigMapAuditPolicy(ctx, configMapAuditPolicy); err != nil {
		return err
	}

	if err := k.reconcileConfigMapEgressSelector(ctx, configMapEgressSelector); err != nil {
		return err
	}

	if err := k.reconcileDeployment(ctx, deployment, configMapAuditPolicy, configMapAdmission, configMapEgressSelector, secretOIDCCABundle, secretServiceAccountSigningKey); err != nil {
		return err
	}

	data, err := k.computeShootResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, k.client.Client(), k.namespace, ManagedResourceName, false, data)
}

func (k *kubeAPIServer) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(ctx, k.client.Client(),
		k.emptyManagedResource(),
		k.emptyManagedResourceSecret(),
		k.emptyHorizontalPodAutoscaler(),
		k.emptyVerticalPodAutoscaler(),
		k.emptyHVPA(),
		k.emptyPodDisruptionBudget(),
		k.emptyDeployment(),
		k.emptyNetworkPolicy(networkPolicyNameAllowFromShootAPIServer),
		k.emptyNetworkPolicy(networkPolicyNameAllowToShootAPIServer),
		k.emptyNetworkPolicy(networkPolicyNameAllowKubeAPIServer),
	)
}

var (
	// IntervalWaitForDeployment is the interval used while waiting for the Deployments to become healthy
	// or deleted.
	IntervalWaitForDeployment = 5 * time.Second
	// TimeoutWaitForDeployment is the timeout used while waiting for the Deployments to become healthy
	// or deleted.
	TimeoutWaitForDeployment = 5 * time.Minute
)

func (k *kubeAPIServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForDeployment)
	defer cancel()

	deployment := k.emptyDeployment()

	if err := retry.Until(timeoutCtx, IntervalWaitForDeployment, func(ctx context.Context) (done bool, err error) {
		if err2 := k.client.APIReader().Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err2 != nil {
			return retry.SevereError(err2)
		}

		if deployment.Generation != deployment.Status.ObservedGeneration {
			return retry.MinorError(fmt.Errorf("kube-apiserver not observed at latest generation (%d/%d)",
				deployment.Status.ObservedGeneration, deployment.Generation))
		}

		replicas := int32(0)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}
		if replicas != deployment.Status.Replicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver deployment has outdated replicas"))
		}
		if replicas != deployment.Status.UpdatedReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough updated replicas (%d/%d)",
				deployment.Status.UpdatedReplicas, replicas))
		}
		if replicas != deployment.Status.AvailableReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough available replicas (%d/%d",
				deployment.Status.AvailableReplicas, replicas))
		}

		return retry.Ok()
	}); err != nil {
		var (
			retryError *retry.Error
			headBytes  *int64
			tailLines  = pointer.Int64Ptr(10)
		)

		if !errors.As(err, &retryError) {
			return err
		}

		newestPod, err2 := kutil.NewestPodForDeployment(ctx, k.client.APIReader(), deployment)
		if err2 != nil {
			return fmt.Errorf("failure to find the newest pod for deployment to read the logs: %s: %w", err2.Error(), err)
		}
		if newestPod == nil {
			return err
		}

		if version.ConstraintK8sLess119.Check(k.values.Version) {
			headBytes = pointer.Int64Ptr(1024)
		}

		logs, err2 := kutil.MostRecentCompleteLogs(ctx, k.client.Kubernetes().CoreV1().Pods(newestPod.Namespace), newestPod, ContainerNameKubeAPIServer, tailLines, headBytes)
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

func (k *kubeAPIServer) SetAutoscalingReplicas(replicas *int32) {
	k.values.Autoscaling.Replicas = replicas
}

func (k *kubeAPIServer) SetSecrets(secrets Secrets) {
	k.secrets = secrets
}

func (k *kubeAPIServer) SetServiceAccountConfig(config ServiceAccountConfig) {
	k.values.ServiceAccount = config
}

func (k *kubeAPIServer) SetSNIConfig(config SNIConfig) {
	k.values.SNI = config
}

func (k *kubeAPIServer) SetProbeToken(token string) {
	k.values.ProbeToken = token
}

func (k *kubeAPIServer) SetExternalHostname(hostname string) {
	k.values.ExternalHostname = hostname
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

// Secrets is collection of secrets for the kube-apiserver.
type Secrets struct {
	// BasicAuthentication contains the basic authentication credentials.
	// Only relevant if BasicAuthenticationEnabled is true.
	BasicAuthentication *component.Secret
	// CA is the cluster's certificate authority.
	CA component.Secret
	// CAEtcd is the certificate authority for the etcd.
	CAEtcd component.Secret
	// CAFrontProxy is the certificate authority for the front-proxy.
	CAFrontProxy component.Secret
	// Etcd is the client certificate for the kube-apiserver to talk to etcd.
	Etcd component.Secret
	// EtcdEncryptionConfig is the configuration containing information how to encrypt the etcd data.
	EtcdEncryptionConfig component.Secret
	// HTTPProxy is the client certificate for the http proxy to talk to the kube-apiserver..
	// Only relevant if VPNConfig.ReversedVPNEnabled is true.
	HTTPProxy *component.Secret
	// KubeAggregator is the client certificate for the kube-aggregator to talk to the kube-apiserver.
	KubeAggregator component.Secret
	// KubeAPIServerToKubelet is the client certificate for the kube-apiserver to talk to kubelets.
	KubeAPIServerToKubelet component.Secret
	// Server is the server certificate and key for the HTTP server of kube-apiserver.
	Server component.Secret
	// ServiceAccountKey is key for service accounts.
	ServiceAccountKey component.Secret
	// StaticToken is the static token secret.
	StaticToken component.Secret
	// VPNSeed is the client certificate for the vpn-seed to talk to the kube-apiserver.
	// Only relevant if VPNConfig.ReversedVPNEnabled is false.
	VPNSeed *component.Secret
	// VPNSeedTLSAuth is the TLS auth information for the vpn-seed.
	// Only relevant if VPNConfig.ReversedVPNEnabled is false.
	VPNSeedTLSAuth *component.Secret
	// VPNSeedServerTLSAuth is the TLS auth information for the vpn-seed server.
	// Only relevant if VPNConfig.ReversedVPNEnabled is true.
	VPNSeedServerTLSAuth *component.Secret
}

type secret struct {
	*component.Secret
	isRequired func(Values) bool
}

func (s *Secrets) all() map[string]secret {
	return map[string]secret{
		"BasicAuthentication":    {Secret: s.BasicAuthentication, isRequired: func(v Values) bool { return v.BasicAuthenticationEnabled }},
		"CA":                     {Secret: &s.CA},
		"CAEtcd":                 {Secret: &s.CAEtcd},
		"CAFrontProxy":           {Secret: &s.CAFrontProxy},
		"Etcd":                   {Secret: &s.Etcd},
		"EtcdEncryptionConfig":   {Secret: &s.EtcdEncryptionConfig},
		"HTTPProxy":              {Secret: s.HTTPProxy, isRequired: func(v Values) bool { return v.VPN.ReversedVPNEnabled }},
		"KubeAggregator":         {Secret: &s.KubeAggregator},
		"KubeAPIServerToKubelet": {Secret: &s.KubeAPIServerToKubelet},
		"Server":                 {Secret: &s.Server},
		"ServiceAccountKey":      {Secret: &s.ServiceAccountKey},
		"StaticToken":            {Secret: &s.StaticToken},
		"VPNSeed":                {Secret: s.VPNSeed, isRequired: func(v Values) bool { return !v.VPN.ReversedVPNEnabled }},
		"VPNSeedTLSAuth":         {Secret: s.VPNSeedTLSAuth, isRequired: func(v Values) bool { return !v.VPN.ReversedVPNEnabled }},
		"VPNSeedServerTLSAuth":   {Secret: s.VPNSeedServerTLSAuth, isRequired: func(v Values) bool { return v.VPN.ReversedVPNEnabled }},
	}
}

func (s *Secrets) verify(values Values) error {
	for name, secret := range s.all() {
		if secret.isRequired != nil && !secret.isRequired(values) {
			continue
		}

		if secret.Secret == nil || secret.Name == "" || secret.Checksum == "" {
			return fmt.Errorf("missing information for required secret %s", name)
		}
	}

	return nil
}
