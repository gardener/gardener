// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component"
)

// Interface contains functions for a deployer for an API server built with k8s.io/apiserver.
type Interface interface {
	component.DeployWaiter
	// GetAutoscalingReplicas gets the Replicas field in the AutoscalingConfig of the Values of the deployer.
	GetAutoscalingReplicas() *int32
	// SetAutoscalingAPIServerResources sets the APIServerResources field in the AutoscalingConfig of the Values of the
	// deployer.
	SetAutoscalingAPIServerResources(corev1.ResourceRequirements)
	// SetAutoscalingReplicas sets the Replicas field in the AutoscalingConfig of the Values of the deployer.
	SetAutoscalingReplicas(*int32)
	// SetETCDEncryptionConfig sets the ETCDEncryptionConfig field in the Values of the deployer.
	SetETCDEncryptionConfig(ETCDEncryptionConfig)
}

// Values contains configuration values for the API server resources.
type Values struct {
	// EnabledAdmissionPlugins is the list of admission plugins that should be enabled with configuration for the API server.
	EnabledAdmissionPlugins []AdmissionPluginConfig
	// DisabledAdmissionPlugins is the list of admission plugins that should be disabled for the API server.
	DisabledAdmissionPlugins []gardencorev1beta1.AdmissionPlugin
	// Audit contains information for configuring audit settings for the API server.
	Audit *AuditConfig
	// Autoscaling contains information for configuring autoscaling settings for the API server.
	Autoscaling AutoscalingConfig
	// ETCDEncryption contains configuration for the encryption of resources in etcd.
	ETCDEncryption ETCDEncryptionConfig
	// FeatureGates is the set of feature gates.
	FeatureGates map[string]bool
	// Logging contains configuration settings for the log and access logging verbosity
	Logging *gardencorev1beta1.APIServerLogging
	// Requests contains configuration for the API server requests.
	Requests *gardencorev1beta1.APIServerRequests
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// WatchCacheSizes are the configured sizes for the watch caches.
	WatchCacheSizes *gardencorev1beta1.WatchCacheSizes
}

// AdmissionPluginConfig contains information about a specific admission plugin and its corresponding configuration.
type AdmissionPluginConfig struct {
	gardencorev1beta1.AdmissionPlugin
	// Kubeconfig is an optional API server connection configuration of this admission plugin. The configs for some
	// admission plugins like `ImagePolicyWebhook` or `ValidatingAdmissionWebhook` can take a reference to an API server
	Kubeconfig []byte
}

// AuditConfig contains information for configuring audit settings for the API server.
type AuditConfig struct {
	// Policy is the audit policy document in YAML format.
	Policy *string
	// Webhook contains configuration for the audit webhook.
	Webhook *AuditWebhook
}

// AuditWebhook contains configuration for the audit webhook.
type AuditWebhook struct {
	// Kubeconfig contains the API server file that defines the audit webhook configuration.
	Kubeconfig []byte
	// BatchMaxSize is the maximum size of a batch.
	BatchMaxSize *int32
	// Version is the API group and version used for serializing audit events written to webhook.
	Version *string
}

// AutoscalingMode represents the different autoscaling modes for an API Server.
type AutoscalingMode int8

const (
	// AutoscalingModeBaseline differs substantially between kube-apiserver and gardener-apiserver.
	// For kube-apiserver, it is active HPA and VPA in passive, recommend-only mode.
	// For gardener-apiserver, it is VPA only.
	AutoscalingModeBaseline = 0 + iota
	// AutoscalingModeHVPA uses Gardener's custom HVPA autoscaler.
	AutoscalingModeHVPA
	// AutoscalingModeVPAAndHPA uses VPA on CPU utilization and HPA on CPU usage.
	AutoscalingModeVPAAndHPA
)

// AutoscalingConfig contains information for configuring autoscaling settings for the API server.
type AutoscalingConfig struct {
	// Mode is the strategy for scaling the API server.
	// Defaults to AutoscalingModeBaseline.
	Mode AutoscalingMode
	// APIServerResources are the resource requirements for the API server container.
	APIServerResources corev1.ResourceRequirements
	// Replicas is the number of pod replicas for the API server.
	Replicas *int32
	// MinReplicas are the minimum Replicas for horizontal autoscaling.
	MinReplicas int32
	// MaxReplicas are the maximum Replicas for horizontal autoscaling.
	MaxReplicas int32
	// UseMemoryMetricForHvpaHPA states whether the memory metric shall be used when the HPA is configured in an HVPA
	// resource.
	UseMemoryMetricForHvpaHPA bool
	// ScaleDownDisabled states whether scale-down shall be disabled.
	// Only HVPA and VPAAndHPA autoscaling modes support disabling scale-down.
	ScaleDownDisabled bool
	// VPAMaxAllowed specifies the maximum amount of resources that will be recommended by the Vertical Pod Autoscaler.
	VPAMaxAllowed corev1.ResourceList
}

// ETCDEncryptionConfig contains configuration for the encryption of resources in etcd.
type ETCDEncryptionConfig struct {
	// RotationPhase specifies the credentials rotation phase of the encryption key.
	RotationPhase gardencorev1beta1.CredentialsRotationPhase
	// EncryptWithCurrentKey specifies whether the current encryption key should be used for encryption. If this is
	// false and if there are two keys then the old key will be used for encryption while the current/new key will only
	// be used for decryption.
	EncryptWithCurrentKey bool
	// ResourcesToEncrypt are the resources which should be encrypted.
	ResourcesToEncrypt []string
	// EncryptedResources are the resources which are currently encrypted.
	EncryptedResources []string
}
