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

	// ETCDEncryptionConfigDataName is the name of ShootState data entry holding the current key and encryption state used to encrypt shoot resources
	ETCDEncryptionConfigDataName = "etcdEncryptionConfiguration"

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

	// LokiPrefix is a constant for a prefix used for the Loki instance.
	LokiPrefix = "l"

	// KubecfgUsername is the username for the token used for the kubeconfig the shoot.
	KubecfgUsername = "system:cluster-admin"

	// KubecfgSecretName is the name of the kubecfg secret.
	KubecfgSecretName = "kubecfg"

	// KubeAPIServerHealthCheck is a key for the kube-apiserver-health-check user.
	KubeAPIServerHealthCheck = "kube-apiserver-health-check"

	// StaticTokenSecretName is the name of the secret containing static tokens for the kube-apiserver.
	StaticTokenSecretName = "static-token"

	// VPASecretName is the name of the secret used by VPA
	VPASecretName = "vpa-tls-certs"

	// ManagedResourceShootCoreName is the name of the shoot core managed resource.
	ManagedResourceShootCoreName = "shoot-core"
	// ManagedResourceAddonsName is the name of the addons managed resource.
	ManagedResourceAddonsName = "addons"

	// SeedSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	SeedSpecHash = "seed-spec-hash"

	// ControllerDeploymentHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	ControllerDeploymentHash = "deployment-hash"
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
	// LokiTLS is the name of the secret resource which holds the TLS certificate for Loki.
	LokiTLS = "loki-tls"

	// EndUserCrtValidity is the time period a user facing certificate is valid.
	EndUserCrtValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176

	// ShootDNSIngressName is a constant for the DNS resources used for the shoot ingress addon.
	ShootDNSIngressName = "ingress"

	// GardenLokiPriorityClassName is the name of the PriorityClass for the Loki in the garden namespace
	GardenLokiPriorityClassName = "garden-loki"

	// MonitoringIngressCredentials is a constant for the name of a secret containing the monitoring credentials for
	// operators monitoring for shoots.
	MonitoringIngressCredentials = "monitoring-ingress-credentials"
	// MonitoringIngressCredentialsUsers is a constant for the name of a secret containing the monitoring credentials
	// for users monitoring for shoots.
	MonitoringIngressCredentialsUsers = "monitoring-ingress-credentials-users"

	// NodeLocalIPVSAddress is the IPv4 address used by node local dns when IPVS is used.
	NodeLocalIPVSAddress = "169.254.20.10"
)
