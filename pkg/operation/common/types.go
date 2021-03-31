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

package common

import (
	"time"
)

const (
	// VPNTunnel dictates that VPN is used as a tunnel between seed and shoot networks.
	VPNTunnel string = "vpn-shoot"

	// BasicAuthSecretName is the name of the secret containing basic authentication credentials for the kube-apiserver.
	BasicAuthSecretName = "kube-apiserver-basic-auth"

	// ConfirmationDeletion is an annotation on a Shoot and Project resources whose value must be set to "true" in order to
	// allow deleting the resource (if the annotation is not set any DELETE request will be denied).
	ConfirmationDeletion = "confirmation.gardener.cloud/deletion"

	// DNSProvider is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	DNSProvider = "dns.gardener.cloud/provider"

	// DNSDomain is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	DNSDomain = "dns.gardener.cloud/domain"

	// DNSIncludeZones is the key for an annotation on a Kubernetes Secret object whose value must point to a list
	// of zones that shall be included.
	DNSIncludeZones = "dns.gardener.cloud/include-zones"

	// DNSExcludeZones is the key for an annotation on a Kubernetes Secret object whose value must point to a list
	// of zones that shall be excluded.
	DNSExcludeZones = "dns.gardener.cloud/exclude-zones"

	// EtcdEncryptionSecretName is the name of the shoot-specific secret which contains
	// that shoot's EncryptionConfiguration. The EncryptionConfiguration contains a key
	// which the shoot's apiserver uses for encrypting selected etcd content.
	// Should match charts/seed-controlplane/charts/kube-apiserver/templates/deployment.yaml
	EtcdEncryptionSecretName = "etcd-encryption-secret"

	// EtcdEncryptionSecretFileName is the name of the file within the EncryptionConfiguration
	// which is made available as volume mount to the shoot's apiserver.
	// Should match charts/seed-controlplane/charts/kube-apiserver/templates/deployment.yaml
	EtcdEncryptionSecretFileName = "encryption-configuration.yaml"

	// EtcdEncryptionChecksumLabelName is the name of the label which is added to the shoot
	// secrets after rewriting them to ensure that successfully rewritten secrets are not
	// (unnecessarily) rewritten during each reconciliation.
	EtcdEncryptionChecksumLabelName = "shoot.gardener.cloud/etcd-encryption-configuration-checksum"

	// EtcdEncryptionForcePlaintextAnnotationName is the name of the annotation with which to annotate
	// the EncryptionConfiguration secret to force the decryption of shoot secrets
	EtcdEncryptionForcePlaintextAnnotationName = "shoot.gardener.cloud/etcd-encryption-force-plaintext-secrets"

	// EtcdEncryptionEncryptedResourceSecrets is the name of the secret resource to be encrypted
	EtcdEncryptionEncryptedResourceSecrets = "secrets"

	// EtcdEncryptionKeyPrefix is the prefix for the key name of the EncryptionConfiguration's key
	EtcdEncryptionKeyPrefix = "key"

	// EtcdEncryptionKeySecretLen is the expected length in bytes of the EncryptionConfiguration's key
	EtcdEncryptionKeySecretLen = 32

	// GardenerDeletionProtected is a label on CustomResourceDefinitions indicating that the deletion is protected, i.e.
	// it must be confirmed with the `confirmation.gardener.cloud/deletion=true` annotation before a `DELETE` call
	// is accepted.
	GardenerDeletionProtected = "gardener.cloud/deletion-protected"

	// ETCDEncryptionConfigDataName is the name of ShootState data entry holding the current key and encryption state used to encrypt shoot resources
	ETCDEncryptionConfigDataName = "etcdEncryptionConfiguration"

	// GardenCreatedBy is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	GardenCreatedBy = "gardener.cloud/created-by"

	// GrafanaOperatorsPrefix is a constant for a prefix used for the operators Grafana instance.
	GrafanaOperatorsPrefix = "go"

	// GrafanaUsersPrefix is a constant for a prefix used for the users Grafana instance.
	GrafanaUsersPrefix = "gu"

	// GrafanaOperatorsRole is a constant for the operators role.
	GrafanaOperatorsRole = "operators"

	// GrafanaUsersRole is a constant for the users role.
	GrafanaUsersRole = "users"

	// PrometheusPrefix is a constant for a prefix used for the Prometheus instance.
	PrometheusPrefix = "p"

	// AlertManagerPrefix is a constant for a prefix used for the AlertManager instance.
	AlertManagerPrefix = "au"

	// IngressPrefix is the part of a FQDN which will be used to construct the domain name for an ingress controller of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the ingress domain would be
	// '*.<IngressPrefix>.cluster.example.com'.
	IngressPrefix = "ingress"

	// APIServerPrefix is the part of a FQDN which will be used to construct the domain name for the kube-apiserver of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the apiserver domain would be
	// 'api.cluster.example.com'.
	APIServerPrefix = "api"

	// InternalDomainKey is a key which must be present in an internal domain constructed for a Shoot cluster. If the
	// configured internal domain already contains it, it won't be added twice. If it does not contain it, it will be
	// appended.
	InternalDomainKey = "internal"

	// CoreDNSDeploymentName is the name of the coredns deployment.
	CoreDNSDeploymentName = "coredns"

	// KubecfgUsername is the username for the token used for the kubeconfig the shoot.
	KubecfgUsername = "system:cluster-admin"

	// KubecfgSecretName is the name of the kubecfg secret.
	KubecfgSecretName = "kubecfg"

	// DependencyWatchdogExternalProbeSecretName is the name of the kubecfg secret with internal DNS for external access.
	DependencyWatchdogExternalProbeSecretName = "dependency-watchdog-external-probe"

	// DependencyWatchdogInternalProbeSecretName is the name of the kubecfg secret with cluster IP access.
	DependencyWatchdogInternalProbeSecretName = "dependency-watchdog-internal-probe"

	// DependencyWatchdogUserName is the user name of the dependency-watchdog.
	DependencyWatchdogUserName = "gardener.cloud:system:dependency-watchdog"

	// KubeAPIServerHealthCheck is a key for the kube-apiserver-health-check user.
	KubeAPIServerHealthCheck = "kube-apiserver-health-check"

	// StaticTokenSecretName is the name of the secret containing static tokens for the kube-apiserver.
	StaticTokenSecretName = "static-token"

	// VPASecretName is the name of the secret used by VPA
	VPASecretName = "vpa-tls-certs"

	// ProjectName is the key of a label on namespaces whose value holds the project name.
	ProjectName = "project.gardener.cloud/name"

	// ProjectSkipStaleCheck is the key of an annotation on a project namespace that marks the associated Project to be
	// skipped by the stale project controller. If the project has already configured stale timestamps in its status
	// then they will be reset.
	ProjectSkipStaleCheck = "project.gardener.cloud/skip-stale-check"

	// NamespaceProject is the key of an annotation on namespace whose value holds the project uid.
	NamespaceProject = "namespace.gardener.cloud/project"

	// NamespaceKeepAfterProjectDeletion is a constant for an annotation on a `Namespace` resource that states that it
	// should not be deleted if the corresponding `Project` gets deleted. Please note that all project related labels
	// from the namespace will be removed when the project is being deleted.
	NamespaceKeepAfterProjectDeletion = "namespace.gardener.cloud/keep-after-project-deletion"

	// ShootAlphaScalingAPIServerClass is a constant for an annotation on the shoot stating the initial API server class.
	// It influences the size of the initial resource requests/limits.
	// Possible values are [small, medium, large, xlarge, 2xlarge].
	// Note that this annotation is alpha and can be removed anytime without further notice. Only use it if you know
	// what you do.
	ShootAlphaScalingAPIServerClass = "alpha.kube-apiserver.scaling.shoot.gardener.cloud/class"

	// ShootExpirationTimestamp is an annotation on a Shoot resource whose value represents the time when the Shoot lifetime
	// is expired. The lifetime can be extended, but at most by the minimal value of the 'clusterLifetimeDays' property
	// of referenced quotas.
	ShootExpirationTimestamp = "shoot.gardener.cloud/expiration-timestamp"

	// ShootNoCleanup is a constant for a label on a resource indicating that the Gardener cleaner should not delete this
	// resource when cleaning a shoot during the deletion flow.
	ShootNoCleanup = "shoot.gardener.cloud/no-cleanup"

	// ShootStatus is a constant for a label on a Shoot resource indicating that the Shoot's health.
	ShootStatus = "shoot.gardener.cloud/status"

	// ShootOperationMaintain is a constant for an annotation on a Shoot indicating that the Shoot maintenance shall be executed as soon as
	// possible.
	ShootOperationMaintain = "maintain"

	// FailedShootNeedsRetryOperation is a constant for an annotation on a Shoot in a failed state indicating that a retry operation should be triggered during the next maintenance time window.
	FailedShootNeedsRetryOperation = "maintenance.shoot.gardener.cloud/needs-retry-operation"

	// ShootOperationRotateKubeconfigCredentials is a constant for an annotation on a Shoot indicating that the credentials contained in the
	// kubeconfig that is handed out to the user shall be rotated.
	ShootOperationRotateKubeconfigCredentials = "rotate-kubeconfig-credentials"

	// ShootTasks is a constant for an annotation on a Shoot which states that certain tasks should be done.
	ShootTasks = "shoot.gardener.cloud/tasks"

	// ShootTaskDeployInfrastructure is a name for a Shoot's infrastructure deployment task. It indicates that the
	// Infrastructure extension resource shall be reconciled.
	ShootTaskDeployInfrastructure = "deployInfrastructure"

	// ShootTaskRestartControlPlanePods is a name for a Shoot task which is dedicated to restart related control plane pods.
	ShootTaskRestartControlPlanePods = "restartControlPlanePods"

	// ShootTaskRestartCoreAddons is a name for a Shoot task which is dedicated to restart some core addons.
	ShootTaskRestartCoreAddons = "restartCoreAddons"

	// ShootOperationRetry is a constant for an annotation on a Shoot indicating that a failed Shoot reconciliation shall be retried.
	ShootOperationRetry = "retry"

	// ShootOperationReconcile is a constant for an annotation on a Shoot indicating that a Shoot reconciliation shall be triggered.
	ShootOperationReconcile = "reconcile"

	// ShootSyncPeriod is a constant for an annotation on a Shoot which may be used to overwrite the global Shoot controller sync period.
	// The value must be a duration. It can also be used to disable the reconciliation at all by setting it to 0m. Disabling the reconciliation
	// does only mean that the period reconciliation is disabled. However, when the Gardener is restarted/redeployed or the specification is
	// changed then the reconciliation flow will be executed.
	ShootSyncPeriod = "shoot.gardener.cloud/sync-period"

	// ShootIgnore is a constant for an annotation on a Shoot which may be used to tell the Gardener that the Shoot with this name should be
	// ignored completely. That means that the Shoot will never reach the reconciliation flow (independent of the operation (create/update/
	// delete)).
	ShootIgnore = "shoot.gardener.cloud/ignore"

	// ManagedResourceShootCoreName is the name of the shoot core managed resource.
	ManagedResourceShootCoreName = "shoot-core"

	// ManagedResourceAddonsName is the name of the addons managed resource.
	ManagedResourceAddonsName = "addons"

	// SeedSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	SeedSpecHash = "seed-spec-hash"

	// RegistrationSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	RegistrationSpecHash = "registration-spec-hash"

	// VpaAdmissionControllerName is the name of the vpa-admission-controller name.
	VpaAdmissionControllerName = "gardener.cloud:vpa:admission-controller"
	// VpaRecommenderName is the name of the vpa-recommender name.
	VpaRecommenderName = "gardener.cloud:vpa:recommender"
	// VpaUpdaterName is the name of the vpa-updater name.
	VpaUpdaterName = "gardener.cloud:vpa:updater"
	// VpaExporterName is the name of the vpa-exporter name.
	VpaExporterName = "gardener.cloud:vpa:exporter"

	// IstioNamespace is the istio-system namespace
	IstioNamespace = "istio-system"

	// ServiceAccountSigningKeySecretDataKey is the data key of a signing key Kubernetes secret.
	ServiceAccountSigningKeySecretDataKey = "signing-key"

	// AlertManagerTLS is the name of the secret resource which holds the TLS certificate for Alert Manager.
	AlertManagerTLS = "alertmanager-tls"
	// GrafanaTLS is the name of the secret resource which holds the TLS certificate for Grafana.
	GrafanaTLS = "grafana-tls"
	// PrometheusTLS is the name of the secret resource which holds the TLS certificate for Prometheus.
	PrometheusTLS = "prometheus-tls"

	// EndUserCrtValidity is the time period a user facing certificate is valid.
	EndUserCrtValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176

	// ShootDNSIngressName is a constant for the DNS resources used for the shoot ingress addon.
	ShootDNSIngressName = "ingress"

	// GardenLokiPriorityClassName is the name of the PriorityClass for the Loki in the garden namespace
	GardenLokiPriorityClassName = "garden-loki"
)
