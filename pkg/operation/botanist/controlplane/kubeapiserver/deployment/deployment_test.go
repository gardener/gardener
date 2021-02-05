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

package deployment_test

import (
	"fmt"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
)

var _ = Describe("Kube APIServer Deployment", func() {
	DescribeTable("#getResourcesForAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest, expectedCPULimit, expectedMemoryLimit string) {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit := getResourcesForAPIServer(int32(nodes), storageClass)

			Expect(cpuRequest).To(Equal(expectedCPURequest))
			Expect(memoryRequest).To(Equal(expectedMemoryRequest))
			Expect(cpuLimit).To(Equal(expectedCPULimit))
			Expect(memoryLimit).To(Equal(expectedMemoryLimit))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),
	)
})

func computeKubeAPIServerCommand(valuesProvider KubeAPIServerValuesProvider, admissionPlugins []gardencorev1beta1.AdmissionPlugin) []string {
	var (
		command                []string
		apiServerConfig        = valuesProvider.GetAPIServerConfig()
		shootKubernetesVersion = valuesProvider.GetShootKubernetesVersion()
	)

	if ok, _ := versionutils.CompareVersions(valuesProvider.GetShootKubernetesVersion(), ">=", "1.17"); ok {
		command = append(command, "/usr/local/bin/kube-apiserver")
	} else if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, ">=", "1.15"); ok {
		command = append(command, "/hyperkube", "kube-apiserver")
	} else {
		command = append(command, "/hyperkube", "apiserver")
	}

	if valuesProvider.IsKonnectivityTunnelEnabled() {
		command = append(command, "--egress-selector-config-file=/etc/kubernetes/konnectivity/egress-selector-configuration.yaml")
	}

	command = append(command,
		kubernetes.AdmissionPluginsToCommandLineParameter(admissionPlugins),
		"--admission-control-config-file=/etc/kubernetes/admission/admission-configuration.yaml",
		"--allow-privileged=true",
		"--anonymous-auth=false",
		"--audit-log-path=/var/lib/audit.log",
		"--audit-policy-file=/etc/kubernetes/audit/audit-policy.yaml",
		"--audit-log-maxsize=100",
		"--audit-log-maxbackup=5",
		"--authorization-mode=Node,RBAC",
	)

	if valuesProvider.IsSNIEnabled() {
		command = append(command, fmt.Sprintf("--advertise-address=%s", defaultShootAPIServerClusterIP))
	}
	if valuesProvider.IsBasicAuthenticationEnabled() {
		command = append(command, "--basic-auth-file=/srv/kubernetes/auth/basic_auth.csv")
	}
	command = append(command,
		"--client-ca-file=/srv/kubernetes/ca/ca.crt",
		"--enable-aggregator-routing=true",
		"--enable-bootstrap-token-auth=true",
	)

	if apiServerConfig != nil && apiServerConfig.WatchCacheSizes != nil {
		if apiServerConfig.WatchCacheSizes.Default != nil {
			command = append(command, fmt.Sprintf("--default-watch-cache-size=%d", apiServerConfig.WatchCacheSizes.Default))
		}
		if len(apiServerConfig.WatchCacheSizes.Resources) > 0 {
			var resources []string
			for _, watchResource := range apiServerConfig.WatchCacheSizes.Resources {
				group := ""
				if watchResource.APIGroup != nil && len(*watchResource.APIGroup) > 0 {
					group = *watchResource.APIGroup
				}
				resources = append(resources, fmt.Sprintf("%s%s#%d", watchResource.Resource, group, watchResource.CacheSize))
			}
			command = append(command, fmt.Sprintf("--watch-cache-sizes=%s", strings.Join(resources, ",")))
		}
	}

	command = append(command,
		"--http2-max-streams-per-connection=1000",
		"--etcd-cafile=/srv/kubernetes/etcd/ca/ca.crt",
		"--etcd-certfile=/srv/kubernetes/etcd/client/tls.crt",
		"--etcd-keyfile=/srv/kubernetes/etcd/client/tls.key",
		"--etcd-servers=https://etcd-main-client:2379",
		"--etcd-servers-overrides=/events#https://etcd-events-client:2379",
	)

	if valuesProvider.IsEtcdEncryptionEnabled() {
		command = append(command, "--encryption-provider-config=/etc/kubernetes/etcd-encryption-secret/encryption-configuration.yaml")
	}

	if apiServerConfig != nil && len(apiServerConfig.FeatureGates) > 0 {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(apiServerConfig.FeatureGates))
	}

	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, "<", "1.11"); ok {
		command = append(command, "--feature-gates=PodPriority=true")
	}
	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, "<", "1.12"); ok {
		command = append(command, "--feature-gates=TokenRequest=true")
	}
	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, "=", "1.11"); ok {
		command = append(command, "--feature-gates=TokenRequestProjection=true")
	}

	command = append(command,
		"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
		"--kubelet-client-certificate=/srv/kubernetes/apiserver-kubelet/kube-apiserver-kubelet.crt",
		"--kubelet-client-key=/srv/kubernetes/apiserver-kubelet/kube-apiserver-kubelet.key",
		"--insecure-port=0",
	)

	if apiServerConfig != nil && apiServerConfig.OIDCConfig != nil {
		if apiServerConfig.OIDCConfig.IssuerURL != nil && len(*apiServerConfig.OIDCConfig.IssuerURL) > 0 {
			command = append(command, fmt.Sprintf("--oidc-issuer-url=%s", *apiServerConfig.OIDCConfig.IssuerURL))
		}
		if apiServerConfig.OIDCConfig.ClientID != nil && len(*apiServerConfig.OIDCConfig.ClientID) > 0 {
			command = append(command, fmt.Sprintf("--oidc-client-id=%s", *apiServerConfig.OIDCConfig.ClientID))
		}
		if apiServerConfig.OIDCConfig.CABundle != nil && len(*apiServerConfig.OIDCConfig.CABundle) > 0 {
			command = append(command, "--oidc-ca-file=/srv/kubernetes/oidc/ca.crt")
		}
		if apiServerConfig.OIDCConfig.UsernameClaim != nil && len(*apiServerConfig.OIDCConfig.UsernameClaim) > 0 {
			command = append(command, fmt.Sprintf("--oidc-username-claim=%s", *apiServerConfig.OIDCConfig.UsernameClaim))
		}
		if apiServerConfig.OIDCConfig.GroupsClaim != nil && len(*apiServerConfig.OIDCConfig.GroupsClaim) > 0 {
			command = append(command, fmt.Sprintf("--oidc-groups-claim=%s", *apiServerConfig.OIDCConfig.GroupsClaim))
		}
		if apiServerConfig.OIDCConfig.UsernamePrefix != nil && len(*apiServerConfig.OIDCConfig.UsernamePrefix) > 0 {
			command = append(command, fmt.Sprintf("--oidc-username-prefix=%s", *apiServerConfig.OIDCConfig.UsernamePrefix))
		}
		if apiServerConfig.OIDCConfig.GroupsPrefix != nil && len(*apiServerConfig.OIDCConfig.GroupsPrefix) > 0 {
			command = append(command, fmt.Sprintf("--oidc-groups-prefix=%s", *apiServerConfig.OIDCConfig.GroupsPrefix))
		}
		if apiServerConfig.OIDCConfig.SigningAlgs != nil && len(apiServerConfig.OIDCConfig.SigningAlgs) > 0 {
			command = append(command, fmt.Sprintf("--oidc-signing-algs=%s", strings.Join(apiServerConfig.OIDCConfig.SigningAlgs, ",")))
		}
		if apiServerConfig.OIDCConfig.RequiredClaims != nil && len(apiServerConfig.OIDCConfig.RequiredClaims) > 0 {
			for key, value := range apiServerConfig.OIDCConfig.RequiredClaims {
				command = append(command, fmt.Sprintf("--oidc-required-claim=%s=%s", key, value))
			}
		}
	}

	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, ">=", "1.16"); ok {
		command = append(command, "--livez-grace-period=1m")
	}

	if apiServerConfig != nil && apiServerConfig.Requests != nil {
		if v := apiServerConfig.Requests.MaxNonMutatingInflight; v != nil {
			command = append(command, fmt.Sprintf("--max-requests-inflight=%d", *apiServerConfig.Requests.MaxNonMutatingInflight))
		}
		if v := apiServerConfig.Requests.MaxMutatingInflight; v != nil {
			command = append(command, fmt.Sprintf("--max-mutating-requests-inflight=%d", *apiServerConfig.Requests.MaxMutatingInflight))
		}
	}

	command = append(command,
		"--profiling=false",
		"--proxy-client-cert-file=/srv/kubernetes/aggregator/kube-aggregator.crt",
		"--proxy-client-key-file=/srv/kubernetes/aggregator/kube-aggregator.key",
		"--requestheader-client-ca-file=/srv/kubernetes/ca-front-proxy/ca.crt",
		"--requestheader-extra-headers-prefix=X-Remote-Extra-",
		"--requestheader-group-headers=X-Remote-Group",
		"--requestheader-username-headers=X-Remote-User",
	)

	if apiServerConfig != nil && len(apiServerConfig.RuntimeConfig) > 0 {
		for key, value := range apiServerConfig.RuntimeConfig {
			command = append(command, fmt.Sprintf("--runtime-config=%s=%s", key, strconv.FormatBool(value)))
		}
	}

	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, "<", "1.11"); ok {
		command = append(command, "--runtime-config=scheduling.k8s.io/v1alpha1=true")
	}

	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, "<", "1.14"); ok {
		command = append(command, "--runtime-config=admissionregistration.k8s.io/v1alpha1=true")
	}

	command = append(command,
		"--secure-port=443",
		fmt.Sprintf("--service-cluster-ip-range=%s", defaultServiceNetwork),
		"--service-account-key-file=/srv/kubernetes/service-account-key/id_rsa",
	)

	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, ">=", "1.16"); ok {
		command = append(command, "--shutdown-delay-duration=15s")
	}

	command = append(command,
		"--token-auth-file=/srv/kubernetes/token/static_tokens.csv",
		"--tls-cert-file=/srv/kubernetes/apiserver/kube-apiserver.crt",
		"--tls-private-key-file=/srv/kubernetes/apiserver/kube-apiserver.key",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
	)

	apiServerAudiences := []string{"kubernetes"}
	if apiServerConfig != nil && len(apiServerConfig.APIAudiences) > 0 {
		apiServerAudiences = apiServerConfig.APIAudiences
	}

	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, "<", "1.13"); ok {
		command = append(command, fmt.Sprintf("--service-account-api-audiences=%s", strings.Join(apiServerAudiences, ",")))
	} else {
		command = append(command, fmt.Sprintf("--api-audiences=%s", strings.Join(apiServerAudiences, ",")))
	}

	serviceAccountTokenIssuerURL := fmt.Sprintf("https://%s", defaultShootOutOfClusterAPIServerAddress)

	if apiServerConfig != nil && apiServerConfig.ServiceAccountConfig != nil && apiServerConfig.ServiceAccountConfig.Issuer != nil {
		serviceAccountTokenIssuerURL = *apiServerConfig.ServiceAccountConfig.Issuer
	}
	command = append(command, fmt.Sprintf("--service-account-issuer=%s", serviceAccountTokenIssuerURL))

	if apiServerConfig != nil && apiServerConfig.ServiceAccountConfig != nil && apiServerConfig.ServiceAccountConfig.SigningKeySecret != nil {
		command = append(command,
			"--service-account-signing-key-file=/srv/kubernetes/service-account-signing-key/signing-key",
			"--service-account-key-file=/srv/kubernetes/service-account-signing-key/signing-key")
	} else {
		command = append(command, "--service-account-signing-key-file=/srv/kubernetes/service-account-key/id_rsa")
	}

	command = append(command, "--v=2")

	return command
}

func getExpectedDeplomentFor(
	valuesProvider KubeAPIServerValuesProvider,
	command []string,
	existingDeployment appsv1.Deployment,
	checksumServiceAccountSigningKey,
	checksumConfigMapEgressSelection,
	checksumSecretOIDCCABundle,
	checksumConfigMapAuditPolicy,
	checksumConfigMapAdmissionConfig *string,
) (*appsv1.Deployment, *int32) {
	shootKubernetesVersion := valuesProvider.GetShootKubernetesVersion()

	maxSurge25 := intstr.FromString("25%")
	maxUnavailability := intstr.FromInt(0)

	deploymentReplicas := getDeploymentReplicas(existingDeployment, valuesProvider)

	toBeUpdated := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver",
			Namespace: defaultSeedNamespace,
			Labels:    getAPIServerDeploymentLabels(valuesProvider.IsSNIEnabled()),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: deploymentReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app":  "kubernetes",
				"role": "apiserver",
			}},
			Strategy: appsv1.DeploymentStrategy{
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge25,
					MaxUnavailable: &maxUnavailability,
				},
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			MinReadySeconds:      int32(30),
			RevisionHistoryLimit: pointer.Int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: utils.MergeStringMaps(getOptionalAnnotations(valuesProvider, checksumServiceAccountSigningKey, checksumConfigMapEgressSelection, checksumSecretOIDCCABundle), map[string]string{
						"checksum/configmap-audit-policy":        *checksumConfigMapAuditPolicy,
						"checksum/configmap-admission-config":    *checksumConfigMapAdmissionConfig,
						"checksum/secret-ca-front-proxy":         checksumCaFrontProxy,
						"checksum/secret-ca":                     checksumCA,
						"checksum/secret-kube-apiserver":         checksumTLSServer,
						"checksum/secret-kube-aggregator":        checksumKubeAggregator,
						"checksum/secret-kube-apiserver-kubelet": checksumKubeAPIServerKubelet,
						"checksum/secret-static-token":           checksumStaticToken,
						"checksum/secret-service-account-key":    checksumServiceAccountKey,
						"checksum/secret-ca-etcd":                checksumCAEtcd,
						"checksum/secret-etcd-client-tls":        checksumETCDClientTLS,
						"networkpolicy/konnectivity-enabled":     strconv.FormatBool(valuesProvider.IsKonnectivityTunnelEnabled()),
					}),
					Labels: utils.MergeStringMaps(getAPIServerPodLabels(), map[string]string{
						"networking.gardener.cloud/to-dns":              "allowed",
						"networking.gardener.cloud/to-public-networks":  "allowed",
						"networking.gardener.cloud/to-private-networks": "allowed",
						"networking.gardener.cloud/to-shoot-networks":   "allowed",
						"networking.gardener.cloud/from-prometheus":     "allowed",
					}),
				},
				Spec: corev1.PodSpec{
					DNSPolicy:                     "ClusterFirst",
					RestartPolicy:                 "Always",
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 1,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: "In",
													Values: []string{
														"kubernetes",
													},
												},
												{
													Key:      "role",
													Operator: "In",
													Values: []string{
														"apiserver",
													},
												},
											},
										},
									},
								},
							},
						},
					},
					PriorityClassName: "gardener-shoot-controlplane",
					Containers: []corev1.Container{
						{
							Name:            "kube-apiserver",
							Image:           apiServerImageName,
							ImagePullPolicy: "IfNotPresent",
							Command:         command,
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   computeLivenessProbePath(shootKubernetesVersion),
										Scheme: "HTTPS",
										Port:   intstr.FromInt(443),
										HTTPHeaders: []corev1.HTTPHeader{
											{
												Name:  "Authorization",
												Value: fmt.Sprintf("Bearer %s", defaultHealthCheckToken),
											},
										},
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    3,
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   computeReadinessProbePath(shootKubernetesVersion),
										Scheme: "HTTPS",
										Port:   intstr.FromInt(443),
										HTTPHeaders: []corev1.HTTPHeader{
											{
												Name:  "Authorization",
												Value: fmt.Sprintf("Bearer %s", defaultHealthCheckToken),
											},
										},
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    3,
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      15,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: "File",
							Ports: []corev1.ContainerPort{
								{
									Name:          "https",
									ContainerPort: int32(443),
									Protocol:      "TCP",
								},
							},
							Resources: getDeploymentResources(&existingDeployment, valuesProvider),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "audit-policy-config",
									MountPath: "/etc/kubernetes/audit",
								},
								{
									Name:      "ca",
									MountPath: "/srv/kubernetes/ca",
								},
								{
									Name:      "ca-etcd",
									MountPath: "/srv/kubernetes/etcd/ca",
								},
								{
									Name:      "ca-front-proxy",
									MountPath: "/srv/kubernetes/ca-front-proxy",
								},
								{
									Name:      "etcd-client-tls",
									MountPath: "/srv/kubernetes/etcd/client",
								},
								{
									Name:      "kube-apiserver",
									MountPath: "/srv/kubernetes/apiserver",
								},
								{
									Name:      "service-account-key",
									MountPath: "/srv/kubernetes/service-account-key",
								},
								{
									Name:      "static-token",
									MountPath: "/srv/kubernetes/token",
								},
								{
									Name:      "kube-apiserver-kubelet",
									MountPath: "/srv/kubernetes/apiserver-kubelet",
								},
								{
									Name:      "kube-aggregator",
									MountPath: "/srv/kubernetes/aggregator",
								},
								{
									Name:      "kube-apiserver-admission-config",
									MountPath: "/etc/kubernetes/admission",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "audit-policy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "audit-policy-config",
									},
								},
							},
						},
						{
							Name: "ca",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "ca",
								},
							},
						},
						{
							Name: "ca-etcd",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "ca-etcd",
								},
							},
						},
						{
							Name: "ca-front-proxy",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "ca-front-proxy",
								},
							},
						},
						{
							Name: "kube-apiserver",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-apiserver",
								},
							},
						},
						{
							Name: "etcd-client-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "etcd-client-tls",
								},
							},
						},
						{
							Name: "service-account-key",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "service-account-key",
								},
							},
						},
						{
							Name: "static-token",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "static-token",
								},
							},
						},
						{
							Name: "kube-apiserver-kubelet",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-apiserver-kubelet",
								},
							},
						},
						{
							Name: "kube-aggregator",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-aggregator",
								},
							},
						},
						{
							Name: "kube-apiserver-admission-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kube-apiserver-admission-config",
									},
								},
							},
						},
					}},
			},
		},
	}

	if valuesProvider.IsKonnectivityTunnelEnabled() {
		toBeUpdated = configureDeploymentForKonnectivity(toBeUpdated)
	}

	if valuesProvider.IsKonnectivityTunnelEnabled() && !valuesProvider.IsSNIEnabled() {
		toBeUpdated = configureDeploymentForKonnectivityNoSNI(toBeUpdated)
	} else if valuesProvider.IsKonnectivityTunnelEnabled() && valuesProvider.IsSNIEnabled() {
		toBeUpdated = configureDeploymentForSNIAndKonnectivity(toBeUpdated)
	} else if !valuesProvider.IsKonnectivityTunnelEnabled() {
		toBeUpdated = configureDeploymentForVPN(valuesProvider, toBeUpdated)
	}

	if valuesProvider.GetSNIValues().SNIPodMutatorEnabled {
		toBeUpdated = configureDeploymentWithSNIPodMutator(toBeUpdated)
	}

	toBeUpdated = configureDeploymentForVersion(valuesProvider, toBeUpdated)

	toBeUpdated = configureDeploymentForUserConfiguration(valuesProvider, toBeUpdated)

	return &toBeUpdated, deploymentReplicas
}

func getOptionalAnnotations(
	valuesProvider KubeAPIServerValuesProvider,
	checksumServiceAccountSigningKey,
	checksumConfigMapEgressSelection,
	checksumSecretOIDCCABundle *string) map[string]string {
	annotations := map[string]string{}

	if checksumSecretOIDCCABundle != nil {
		annotations["checksum/secret-oidc-cabundle"] = *checksumSecretOIDCCABundle
	}

	if checksumServiceAccountSigningKey != nil {
		annotations["checksum/service-account-signing-key"] = *checksumServiceAccountSigningKey
	}

	if checksumConfigMapEgressSelection != nil {
		annotations["checksum/egress-selection-config"] = *checksumConfigMapEgressSelection
	}

	if valuesProvider.IsKonnectivityTunnelEnabled() && !valuesProvider.IsSNIEnabled() {
		annotations["checksum/secret-konnectivity-server"] = checksumKonnectivityServer
	} else if valuesProvider.IsKonnectivityTunnelEnabled() && valuesProvider.IsSNIEnabled() {
		annotations["checksum/secret-konnectivity-server-client-tls"] = checksumKonnectivityServerClientTLS
	} else {
		annotations["checksum/secret-vpn-seed"] = checksumVPNSeed
		annotations["checksum/secret-vpn-seed-tlsauth"] = checksumVPNSeedTLSAuth
	}

	if valuesProvider.IsEtcdEncryptionEnabled() {
		// etcd-encryption secret name is different from the annotation key
		annotations["checksum/secret-etcd-encryption"] = checksumETCDEncryptionSecret
	}

	if valuesProvider.IsBasicAuthenticationEnabled() {
		annotations["checksum/secret-kube-apiserver-basic-auth"] = checksumBasicAuth
	}
	return annotations
}

func configureDeploymentForKonnectivity(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "egress-selection-config",
			MountPath: "/etc/kubernetes/konnectivity",
		},
	)

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "egress-selection-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "kube-apiserver-egress-selector-configuration",
					},
				},
			},
		},
	)

	return d
}

func getAPIServerPodLabels() map[string]string {
	return map[string]string{
		"gardener.cloud/role":     "controlplane",
		"garden.sapcloud.io/role": "controlplane",
		"app":                     "kubernetes",
		"role":                    "apiserver",
	}
}

func configureDeploymentForKonnectivityNoSNI(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.ServiceAccountName = "kube-apiserver"

	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "konnectivity-uds",
		MountPath: "/etc/srv/kubernetes/konnectivity-server",
		ReadOnly:  false,
	})

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "konnectivity-server-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "konnectivity-server",
				},
			},
		},
		corev1.Volume{
			Name: "konnectivity-server-kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "konnectivity-server-kubeconfig",
				},
			},
		},
		corev1.Volume{
			Name: "konnectivity-uds",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, BuildKonnectivityServerSidecar(konnectivityServerTunnelImageName, defaultSeedNamespace))
	return d
}

func BuildKonnectivityServerSidecar(tunnelImageName, seedNamespace string) corev1.Container {
	return corev1.Container{
		Name:            "konnectivity-server",
		Image:           tunnelImageName,
		ImagePullPolicy: "IfNotPresent",
		Command:         []string{"/replica-reloader"},
		Args: []string{
			fmt.Sprintf("--namespace=%s", seedNamespace),
			"--deployment-name=kube-apiserver",
			"--jitter=10s",
			"--jitter-factor=5",
			"--v=2",
			"--",
			"/proxy-server",
			"--uds-name=/etc/srv/kubernetes/konnectivity-server/konnectivity-server.socket",
			"--logtostderr=true",
			"--cluster-cert=/certs/konnectivity-server/konnectivity-server.crt",
			"--cluster-key=/certs/konnectivity-server/konnectivity-server.key",
			"--agent-namespace=kube-system",
			"--agent-service-account=konnectivity-agent",
			"--kubeconfig=/etc/srv/kubernetes/konnectivity-server-kubeconfig/kubeconfig",
			"--authentication-audience=system:konnectivity-server",
			"--keepalive-time=1h",
			"--log-file-max-size=0",
			"--delete-existing-uds-file=true",
			"--mode=http-connect",
			"--server-port=0",
			"--agent-port=8132",
			"--admin-port=8133",
			"--health-port=8134",
			"--v=2",
			"--server-count",
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Host:   "127.0.0.1",
					Path:   "/healthz",
					Scheme: "HTTP",
					Port:   intstr.FromInt(8134),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			TimeoutSeconds:      60,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("500M"),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "agentport",
				ContainerPort: 8132,
			},
			{
				Name:          "adminport",
				ContainerPort: 8133,
				HostPort:      8133,
			},
			{
				Name:          "healthport",
				ContainerPort: 8134,
				HostPort:      8134,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "konnectivity-server-certs",
				MountPath: "/certs/konnectivity-server",
				ReadOnly:  true,
			},
			{
				Name:      "konnectivity-server-kubeconfig",
				MountPath: "/etc/srv/kubernetes/konnectivity-server-kubeconfig",
				ReadOnly:  true,
			},
			{
				Name:      "konnectivity-uds",
				MountPath: "/etc/srv/kubernetes/konnectivity-server",
				ReadOnly:  false,
			},
		},
	}
}

func configureDeploymentForSNIAndKonnectivity(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      "konnectivity-server-client-tls",
			MountPath: "/etc/srv/kubernetes/konnectivity-server-client-tls",
			ReadOnly:  false,
		},
	)

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "konnectivity-server-client-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "konnectivity-server-client-tls",
				},
			},
		})
	return d
}

func configureDeploymentForVPN(valuesProvider KubeAPIServerValuesProvider, d appsv1.Deployment) appsv1.Deployment {
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "vpn-seed",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "vpn-seed",
				},
			},
		},
		corev1.Volume{
			Name: "vpn-seed-tlsauth",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "vpn-seed-tlsauth",
				},
			},
		},
		corev1.Volume{
			Name: "modules",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/lib/modules",
					Type: &directoryOrCreate,
				},
			},
		},
	)

	d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, corev1.Container{
		Name:  "set-iptable-rules",
		Image: alpineIptablesImageName,
		Command: []string{
			"/bin/sh",
			"-c",
			"iptables -A INPUT -i tun0 -p icmp -j ACCEPT && iptables -A INPUT -i tun0 -m state --state NEW -j DROP",
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
				},
			},
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "modules",
				MountPath: "/lib/modules",
			},
		},
	})

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, getVPNSidecar(valuesProvider))

	return d
}

func getVPNSidecar(valuesProvider KubeAPIServerValuesProvider) corev1.Container {
	c := corev1.Container{
		Name:            "vpn-seed",
		Image:           vpnSeedImageName,
		ImagePullPolicy: "IfNotPresent",
		Env: []corev1.EnvVar{
			{
				Name:  "MAIN_VPN_SEED",
				Value: "true",
			},
			{
				Name:  "OPENVPN_PORT",
				Value: "4314",
			},
			{
				Name:  "APISERVER_AUTH_MODE",
				Value: "client-cert",
			},
			{
				Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CA",
				Value: "/srv/secrets/vpn-seed/ca.crt",
			},
			{
				Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CRT",
				Value: "/srv/secrets/vpn-seed/tls.crt",
			},
			{
				Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_KEY",
				Value: "/srv/secrets/vpn-seed/tls.key",
			},
			{
				Name:  "SERVICE_NETWORK",
				Value: defaultServiceNetwork.String(),
			},
			{
				Name:  "POD_NETWORK",
				Value: defaultPodNetwork.String(),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "tcp-tunnel",
				ContainerPort: int32(1194),
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1000M"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
				},
			},
			Privileged: pointer.BoolPtr(true),
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "vpn-seed",
				MountPath: "/srv/secrets/vpn-seed",
				ReadOnly:  true,
			},
			{
				Name:      "vpn-seed-tlsauth",
				MountPath: "/srv/secrets/tlsauth",
				ReadOnly:  true,
			},
		},
	}

	nodeNetwork := valuesProvider.GetNodeNetwork()
	if nodeNetwork != nil && len(nodeNetwork.String()) > 0 {
		c.Env = append(c.Env,
			corev1.EnvVar{
				Name:  "NODE_NETWORK",
				Value: nodeNetwork.String(),
			})
	}

	return c
}

func configureDeploymentWithSNIPodMutator(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers,
		corev1.Container{
			Name:            "apiserver-proxy-pod-mutator",
			Image:           apiServerProxyPodMutatorWebhookImageName,
			ImagePullPolicy: "IfNotPresent",
			Args: []string{
				fmt.Sprintf("--apiserver-fqdn=%s", defaultShootOutOfClusterAPIServerAddress),
				"--host=localhost",
				"--cert-dir=/srv/kubernetes/apiserver",
				"--cert-name=kube-apiserver.crt",
				"--key-name=kube-apiserver.key",
				"--port=9443",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "kube-apiserver",
					MountPath: "/srv/kubernetes/apiserver",
					ReadOnly:  true,
				},
			},
		})

	return d
}

func configureDeploymentForVersion(valuesProvider KubeAPIServerValuesProvider, d appsv1.Deployment) appsv1.Deployment {
	if ok, _ := versionutils.CompareVersions(valuesProvider.GetShootKubernetesVersion(), "<", "1.16"); ok {
		d.Spec.Template.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"sh",
						"-c",
						"sleep 5",
					},
				},
			},
		}
	}

	if ok, _ := versionutils.CompareVersions(valuesProvider.GetShootKubernetesVersion(), ">=", "1.17"); ok {
		if valuesProvider.MountHostCADirectories() {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "fedora-rhel6-openelec-cabundle",
					MountPath: "/etc/pki/tls",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "centos-rhel7-cabundle",
					MountPath: "/etc/pki/ca-trust/extracted/pem",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "etc-ssl",
					MountPath: "/etc/ssl",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "usr-share-cacerts",
					MountPath: "/usr/share/ca-certificates",
					ReadOnly:  true,
				},
			)
		} else {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "debian-family-cabundle",
					MountPath: "/etc/ssl/certs/ca-certificates.crt",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "fedora-rhel6-cabundle",
					MountPath: "/etc/pki/tls/certs/ca-bundle.crt",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "opensuse-cabundle",
					MountPath: "/etc/ssl/ca-bundle.pem",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "openelec-cabundle",
					MountPath: "/etc/pki/tls/cacert.pem",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "centos-rhel7-cabundle",
					MountPath: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "alpine-linux-cabundle",
					MountPath: "/etc/ssl/cert.pem",
					ReadOnly:  true,
				},
			)
		}
	}

	return d
}

func configureDeploymentForUserConfiguration(valuesProvider KubeAPIServerValuesProvider, d appsv1.Deployment) appsv1.Deployment {
	if valuesProvider.IsBasicAuthenticationEnabled() {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "kube-apiserver-basic-auth",
				MountPath: "/srv/kubernetes/auth",
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "kube-apiserver-basic-auth",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "kube-apiserver-basic-auth",
					},
				},
			})
	}

	if valuesProvider.GetAPIServerConfig() != nil && valuesProvider.GetAPIServerConfig().OIDCConfig != nil && valuesProvider.GetAPIServerConfig().OIDCConfig.CABundle != nil {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "kube-apiserver-oidc-cabundle",
				MountPath: "/srv/kubernetes/oidc",
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "kube-apiserver-oidc-cabundle",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "kube-apiserver-oidc-cabundle",
					},
				}})
	}

	if valuesProvider.GetAPIServerConfig() != nil && valuesProvider.GetAPIServerConfig().ServiceAccountConfig != nil && valuesProvider.GetAPIServerConfig().ServiceAccountConfig.SigningKeySecret != nil {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "kube-apiserver-service-account-signing-key",
				MountPath: "/srv/kubernetes/service-account-signing-key",
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "kube-apiserver-service-account-signing-key",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "kube-apiserver-service-account-signing-key",
					},
				}})
	}

	// enabled of k8s version  >= 1.13
	if valuesProvider.IsEtcdEncryptionEnabled() {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "etcd-encryption-secret",
				MountPath: "/etc/kubernetes/etcd-encryption-secret",
				ReadOnly:  true,
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "etcd-encryption-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "etcd-encryption-secret",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		)
	}

	// gardenlet feature flag, so not really a User config, but external to the API server component
	if valuesProvider.MountHostCADirectories() {
		directoryOrCreate := corev1.HostPathDirectoryOrCreate
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "fedora-rhel6-openelec-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/pki/tls",
						Type: &directoryOrCreate,
					},
				},
			},
			corev1.Volume{
				Name: "centos-rhel7-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/pki/ca-trust/extracted/pem",
						Type: &directoryOrCreate,
					},
				},
			},
			corev1.Volume{
				Name: "etc-ssl",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/ssl",
						Type: &directoryOrCreate,
					},
				},
			},
			corev1.Volume{
				Name: "usr-share-cacerts",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/usr/share/ca-certificates",
						Type: &directoryOrCreate,
					},
				},
			},
		)
	} else {
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "debian-family-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/ssl/certs/ca-certificates.crt",
					},
				},
			},
			corev1.Volume{
				Name: "fedora-rhel6-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/pki/tls/certs/ca-bundle.crt",
					},
				},
			},
			corev1.Volume{
				Name: "opensuse-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/ssl/ca-bundle.pem",
					},
				},
			},
			corev1.Volume{
				Name: "openelec-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/pki/tls/cacert.pem",
					},
				},
			},
			corev1.Volume{
				Name: "centos-rhel7-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
					},
				},
			},
			corev1.Volume{
				Name: "alpine-linux-cabundle",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/ssl/cert.pem",
					},
				},
			},
		)
	}

	return d
}

func getDeploymentResources(deployment *appsv1.Deployment, valuesProvider KubeAPIServerValuesProvider) corev1.ResourceRequirements {
	if valuesProvider.DeploymentAlreadyExists() && valuesProvider.IsHvpaEnabled() {
		// Deployment is already created AND is controlled by HVPA
		// Keep the "resources" as it is.
		for k := range deployment.Spec.Template.Spec.Containers {
			v := &deployment.Spec.Template.Spec.Containers[k]
			if v.Name == "kube-apiserver" {
				apiServerResources := v.Resources.DeepCopy()
				return *apiServerResources
			}
		}
	}

	if valuesProvider.GetManagedSeedAPIServer() != nil && !valuesProvider.IsHvpaEnabled() {
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1750m"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4000m"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		}
	} else {
		var cpuRequest, memoryRequest, cpuLimit, memoryLimit string
		if valuesProvider.IsHvpaEnabled() {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(valuesProvider.GetMinimumNodeCount(), valuesProvider.GetShootAnnotations()[common.ShootAlphaScalingAPIServerClass])
		} else {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(valuesProvider.GetMaximumNodeCount(), valuesProvider.GetShootAnnotations()[common.ShootAlphaScalingAPIServerClass])
		}

		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuRequest),
				corev1.ResourceMemory: resource.MustParse(memoryRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuLimit),
				corev1.ResourceMemory: resource.MustParse(memoryLimit),
			},
		}
	}
}

func getResourcesForAPIServer(nodeCount int32, scalingClass string) (string, string, string, string) {
	var (
		validScalingClasses = sets.NewString("small", "medium", "large", "xlarge", "2xlarge")
		cpuRequest          string
		memoryRequest       string
		cpuLimit            string
		memoryLimit         string
	)

	if !validScalingClasses.Has(scalingClass) {
		switch {
		case nodeCount <= 2:
			scalingClass = "small"
		case nodeCount <= 10:
			scalingClass = "medium"
		case nodeCount <= 50:
			scalingClass = "large"
		case nodeCount <= 100:
			scalingClass = "xlarge"
		default:
			scalingClass = "2xlarge"
		}
	}

	switch {
	case scalingClass == "small":
		cpuRequest = "800m"
		memoryRequest = "800Mi"

		cpuLimit = "1000m"
		memoryLimit = "1200Mi"
	case scalingClass == "medium":
		cpuRequest = "1000m"
		memoryRequest = "1100Mi"

		cpuLimit = "1200m"
		memoryLimit = "1900Mi"
	case scalingClass == "large":
		cpuRequest = "1200m"
		memoryRequest = "1600Mi"

		cpuLimit = "1500m"
		memoryLimit = "3900Mi"
	case scalingClass == "xlarge":
		cpuRequest = "2500m"
		memoryRequest = "5200Mi"

		cpuLimit = "3000m"
		memoryLimit = "5900Mi"
	case scalingClass == "2xlarge":
		cpuRequest = "3000m"
		memoryRequest = "5200Mi"

		cpuLimit = "4000m"
		memoryLimit = "7800Mi"
	}

	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

func computeLivenessProbePath(shootKubernetesVersion string) string {
	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, ">=", "1.16"); ok {
		return "/livez"
	}
	return "/healthz"
}

func computeReadinessProbePath(shootKubernetesVersion string) string {
	if ok, _ := versionutils.CompareVersions(shootKubernetesVersion, ">=", "1.16"); ok {
		return "/readyz"
	}
	return "/healthz"
}

func getAPIServerDeploymentLabels(SNIEnabled bool) map[string]string {
	labels := map[string]string{
		"gardener.cloud/role": "controlplane",
		"app":                 "kubernetes",
		"role":                "apiserver",
	}

	if SNIEnabled {
		return utils.MergeStringMaps(labels, map[string]string{
			"core.gardener.cloud/apiserver-exposure": "gardener-managed",
		})
	}
	return labels
}

func getDeploymentReplicas(deployment appsv1.Deployment, valuesProvider KubeAPIServerValuesProvider) *int32 {
	if valuesProvider.GetManagedSeedAPIServer() != nil && !valuesProvider.IsHvpaEnabled() {
		return valuesProvider.GetManagedSeedAPIServer().Replicas
	} else {
		// is nil if deployment does not exist yet (will be defaulted)
		currentReplicas := valuesProvider.GetDeploymentReplicas()

		// As kube-apiserver HPA manages the number of replicas, we have to maintain current number of replicas
		// otherwise keep the value to default
		if currentReplicas != nil && *currentReplicas > 0 {
			return currentReplicas
		}

		// If the shoot is hibernated then we want to keep the number of replicas (scale down happens later).
		if valuesProvider.IsHibernationEnabled() && (currentReplicas == nil || *currentReplicas == 0) {
			zero := int32(0)
			return &zero
		}
	}
	defaultReplicas := int32(1)
	return &defaultReplicas
}
