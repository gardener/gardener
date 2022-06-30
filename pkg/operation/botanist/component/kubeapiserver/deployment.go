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
	"fmt"
	"strconv"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
)

const (
	// SecretNameVPNSeedTLSAuth is the name of the secret containing the TLS auth for the vpn-seed.
	SecretNameVPNSeedTLSAuth = "vpn-seed-tlsauth"
	secretNameLegacyVPNSeed  = "vpn-seed"

	secretNameServer                 = "kube-apiserver"
	secretNameKubeAPIServerToKubelet = "kube-apiserver-kubelet"
	secretNameKubeAggregator         = "kube-aggregator"
	secretNameHTTPProxy              = "kube-apiserver-http-proxy"

	// ContainerNameKubeAPIServer is the name of the kube-apiserver container.
	ContainerNameKubeAPIServer            = "kube-apiserver"
	containerNameVPNSeed                  = "vpn-seed"
	containerNameAPIServerProxyPodMutator = "apiserver-proxy-pod-mutator"

	volumeNameAdmissionConfiguration               = "admission-config"
	volumeNameAuditPolicy                          = "audit-policy-config"
	volumeNameBasicAuthentication                  = "basic-auth"
	volumeNameCA                                   = "ca"
	volumeNameCAClient                             = "ca-client"
	volumeNameCAEtcd                               = "ca-etcd"
	volumeNameCAFrontProxy                         = "ca-front-proxy"
	volumeNameCAVPN                                = "ca-vpn"
	volumeNameEgressSelector                       = "egress-selection-config"
	volumeNameEtcdClient                           = "etcd-client"
	volumeNameEtcdEncryptionConfig                 = "etcd-encryption-secret"
	volumeNameHTTPProxy                            = "http-proxy"
	volumeNameKubeAPIServerToKubelet               = "kubelet-client"
	volumeNameKubeAggregator                       = "kube-aggregator"
	volumeNameLibModules                           = "modules"
	volumeNameOIDCCABundle                         = "oidc-cabundle"
	volumeNameServer                               = "kube-apiserver-server"
	volumeNameServiceAccountKey                    = "service-account-key"
	volumeNameServiceAccountKeyBundle              = "service-account-key-bundle"
	volumeNameUserProvidedServiceAccountSigningKey = "service-account-signing-key"
	volumeNameStaticToken                          = "static-token"
	volumeNameVPNSeed                              = "vpn-seed"
	volumeNameVPNSeedTLSAuth                       = "vpn-seed-tlsauth"
	volumeNameFedora                               = "fedora-rhel6-openelec-cabundle"
	volumeNameCentOS                               = "centos-rhel7-cabundle"
	volumeNameEtcSSL                               = "etc-ssl"
	volumeNameUsrShareCaCerts                      = "usr-share-cacerts"

	volumeMountPathAdmissionConfiguration               = "/etc/kubernetes/admission"
	volumeMountPathAuditPolicy                          = "/etc/kubernetes/audit"
	volumeMountPathBasicAuthentication                  = "/srv/kubernetes/auth"
	volumeMountPathCA                                   = "/srv/kubernetes/ca"
	volumeMountPathCAClient                             = "/srv/kubernetes/ca-client"
	volumeMountPathCAEtcd                               = "/srv/kubernetes/etcd/ca"
	volumeMountPathCAFrontProxy                         = "/srv/kubernetes/ca-front-proxy"
	volumeMountPathCAVPN                                = "/srv/kubernetes/ca-vpn"
	volumeMountPathEgressSelector                       = "/etc/kubernetes/egress"
	volumeMountPathEtcdEncryptionConfig                 = "/etc/kubernetes/etcd-encryption-secret"
	volumeMountPathEtcdClient                           = "/srv/kubernetes/etcd/client"
	volumeMountPathHTTPProxy                            = "/etc/srv/kubernetes/envoy"
	volumeMountPathKubeAPIServerToKubelet               = "/srv/kubernetes/apiserver-kubelet"
	volumeMountPathKubeAggregator                       = "/srv/kubernetes/aggregator"
	volumeMountPathLibModules                           = "/lib/modules"
	volumeMountPathOIDCCABundle                         = "/srv/kubernetes/oidc"
	volumeMountPathServer                               = "/srv/kubernetes/apiserver"
	volumeMountPathServiceAccountKey                    = "/srv/kubernetes/service-account-key"
	volumeMountPathServiceAccountKeyBundle              = "/srv/kubernetes/service-account-key-bundle"
	volumeMountPathUserProvidedServiceAccountSigningKey = "/srv/kubernetes/service-account-signing-key"
	volumeMountPathStaticToken                          = "/srv/kubernetes/token"
	volumeMountPathVPNSeed                              = "/srv/secrets/vpn-seed"
	volumeMountPathVPNSeedTLSAuth                       = "/srv/secrets/tlsauth"
	volumeMountPathFedora                               = "/etc/pki/tls"
	volumeMountPathCentOS                               = "/etc/pki/ca-trust/extracted/pem"
	volumeMountPathEtcSSL                               = "/etc/ssl"
	volumeMountPathUsrShareCaCerts                      = "/usr/share/ca-certificates"
)

func (k *kubeAPIServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileDeployment(
	ctx context.Context,
	deployment *appsv1.Deployment,
	configMapAuditPolicy *corev1.ConfigMap,
	configMapAdmission *corev1.ConfigMap,
	configMapEgressSelector *corev1.ConfigMap,
	secretETCDEncryptionConfiguration *corev1.Secret,
	secretOIDCCABundle *corev1.Secret,
	secretUserProvidedServiceAccountSigningKey *corev1.Secret,
	secretServiceAccountKey *corev1.Secret,
	secretStaticToken *corev1.Secret,
	secretBasicAuth *corev1.Secret,
	secretServer *corev1.Secret,
	secretKubeletClient *corev1.Secret,
	secretKubeAggregator *corev1.Secret,
	secretHTTPProxy *corev1.Secret,
	secretLegacyVPNSeed *corev1.Secret,
	secretLegacyVPNSeedTLSAuth *corev1.Secret,
) error {
	var (
		maxSurge                   = intstr.FromString("25%")
		maxUnavailable             = intstr.FromInt(0)
		podAntiAffinityTopologyKey = corev1.LabelHostname
	)

	var healthCheckToken string
	if secretStaticToken != nil {
		staticToken, err := secrets.LoadStaticTokenFromCSV(SecretStaticTokenName, secretStaticToken.Data[secrets.DataKeyStaticTokenCSV])
		if err != nil {
			return err
		}

		token, err := staticToken.GetTokenForUsername(userNameHealthCheck)
		if err != nil {
			return err
		}

		healthCheckToken = token.Token
	}

	secretCACluster, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	secretCAClient, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}

	secretCAFrontProxy, found := k.secretsManager.Get(v1beta1constants.SecretNameCAFrontProxy)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAFrontProxy)
	}

	secretCAVPN, found := k.secretsManager.Get(v1beta1constants.SecretNameCAVPN)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAVPN)
	}

	secretCAETCD, found := k.secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	secretETCDClient, found := k.secretsManager.Get(etcd.SecretNameClient)
	if !found {
		return fmt.Errorf("secret %q not found", etcd.SecretNameClient)
	}

	secretServiceAccountKeyBundle, found := k.secretsManager.Get(v1beta1constants.SecretNameServiceAccountKey, secretsmanager.Bundle)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameServiceAccountKey)
	}

	if k.values.ZoneSpread {
		podAntiAffinityTopologyKey = corev1.LabelTopologyZone
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), deployment, func() error {
		deployment.Labels = GetLabels()
		deployment.Spec = appsv1.DeploymentSpec{
			MinReadySeconds:      30,
			RevisionHistoryLimit: pointer.Int32(2),
			Replicas:             k.values.Autoscaling.Replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: getLabels()},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 1,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey:   podAntiAffinityTopologyKey,
									LabelSelector: &metav1.LabelSelector{MatchLabels: getLabels()},
								},
							}},
						},
					},
					AutomountServiceAccountToken:  pointer.Bool(false),
					PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane500,
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: pointer.Int64(30),
					Containers: []corev1.Container{{
						Name:                     ContainerNameKubeAPIServer,
						Image:                    k.values.Images.KubeAPIServer,
						ImagePullPolicy:          corev1.PullIfNotPresent,
						Command:                  k.computeKubeAPIServerCommand(),
						TerminationMessagePath:   corev1.TerminationMessagePathDefault,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Ports: []corev1.ContainerPort{{
							Name:          "https",
							ContainerPort: Port,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: k.values.Autoscaling.APIServerResources,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/livez",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + healthCheckToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + healthCheckToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameAuditPolicy,
								MountPath: volumeMountPathAuditPolicy,
							},
							{
								Name:      volumeNameAdmissionConfiguration,
								MountPath: volumeMountPathAdmissionConfiguration,
							},
							{
								Name:      volumeNameCA,
								MountPath: volumeMountPathCA,
							},
							{
								Name:      volumeNameCAClient,
								MountPath: volumeMountPathCAClient,
							},
							{
								Name:      volumeNameCAEtcd,
								MountPath: volumeMountPathCAEtcd,
							},
							{
								Name:      volumeNameCAFrontProxy,
								MountPath: volumeMountPathCAFrontProxy,
							},
							{
								Name:      volumeNameEtcdClient,
								MountPath: volumeMountPathEtcdClient,
							},
							{
								Name:      volumeNameServer,
								MountPath: volumeMountPathServer,
							},
							{
								Name:      volumeNameServiceAccountKey,
								MountPath: volumeMountPathServiceAccountKey,
							},
							{
								Name:      volumeNameServiceAccountKeyBundle,
								MountPath: volumeMountPathServiceAccountKeyBundle,
							},
							{
								Name:      volumeNameStaticToken,
								MountPath: volumeMountPathStaticToken,
							},
							{
								Name:      volumeNameKubeAPIServerToKubelet,
								MountPath: volumeMountPathKubeAPIServerToKubelet,
							},
							{
								Name:      volumeNameKubeAggregator,
								MountPath: volumeMountPathKubeAggregator,
							},
							{
								Name:      volumeNameEtcdEncryptionConfig,
								MountPath: volumeMountPathEtcdEncryptionConfig,
								ReadOnly:  true,
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameAuditPolicy,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapAuditPolicy.Name,
									},
								},
							},
						},
						{
							Name: volumeNameAdmissionConfiguration,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapAdmission.Name,
									},
								},
							},
						},
						{
							Name: volumeNameCA,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCACluster.Name,
								},
							},
						},
						{
							Name: volumeNameCAClient,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCAClient.Name,
								},
							},
						},
						{
							Name: volumeNameCAEtcd,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCAETCD.Name,
								},
							},
						},
						{
							Name: volumeNameCAFrontProxy,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCAFrontProxy.Name,
								},
							},
						},
						{
							Name: volumeNameEtcdClient,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretETCDClient.Name,
								},
							},
						},
						{
							Name: volumeNameServiceAccountKey,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretServiceAccountKey.Name,
								},
							},
						},
						{
							Name: volumeNameServiceAccountKeyBundle,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretServiceAccountKeyBundle.Name,
								},
							},
						},
						{
							Name: volumeNameStaticToken,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretStaticToken.Name,
								},
							},
						},
						{
							Name: volumeNameKubeAPIServerToKubelet,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretKubeletClient.Name,
								},
							},
						},
						{
							Name: volumeNameKubeAggregator,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretKubeAggregator.Name,
								},
							},
						},
						{
							Name: volumeNameEtcdEncryptionConfig,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretETCDEncryptionConfiguration.Name,
								},
							},
						},
						{
							Name: volumeNameServer,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretServer.Name,
								},
							},
						},
					},
				},
			},
		}

		k.handleBasicAuthenticationSettings(deployment, secretBasicAuth)
		k.handleLifecycleSettings(deployment)
		k.handleHostCertVolumes(deployment)
		k.handleSNISettings(deployment)
		k.handlePodMutatorSettings(deployment)
		k.handleVPNSettings(deployment, configMapEgressSelector, secretCAVPN, secretHTTPProxy, secretCAClient, secretLegacyVPNSeed, secretLegacyVPNSeedTLSAuth)
		k.handleOIDCSettings(deployment, secretOIDCCABundle)
		k.handleServiceAccountSigningKeySettings(deployment, secretUserProvidedServiceAccountSigningKey)

		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	})
	return err
}

func (k *kubeAPIServer) computeKubeAPIServerCommand() []string {
	var out []string

	out = append(out, "/usr/local/bin/kube-apiserver")
	out = append(out, "--enable-admission-plugins="+strings.Join(k.admissionPluginNames(), ","))
	out = append(out, fmt.Sprintf("--admission-control-config-file=%s/%s", volumeMountPathAdmissionConfiguration, configMapAdmissionDataKey))
	out = append(out, "--allow-privileged=true")
	out = append(out, "--anonymous-auth="+strconv.FormatBool(k.values.AnonymousAuthenticationEnabled))
	out = append(out, "--audit-log-path=/var/lib/audit.log")
	out = append(out, fmt.Sprintf("--audit-policy-file=%s/%s", volumeMountPathAuditPolicy, configMapAuditPolicyDataKey))
	out = append(out, "--audit-log-maxsize=100")
	out = append(out, "--audit-log-maxbackup=5")
	out = append(out, "--authorization-mode=Node,RBAC")

	if len(k.values.APIAudiences) > 0 {
		out = append(out, "--api-audiences="+strings.Join(k.values.APIAudiences, ","))
	}

	out = append(out, fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateBundle))
	out = append(out, "--enable-aggregator-routing=true")
	out = append(out, "--enable-bootstrap-token-auth=true")
	out = append(out, "--http2-max-streams-per-connection=1000")
	out = append(out, fmt.Sprintf("--etcd-cafile=%s/%s", volumeMountPathCAEtcd, secrets.DataKeyCertificateBundle))
	out = append(out, fmt.Sprintf("--etcd-certfile=%s/%s", volumeMountPathEtcdClient, secrets.DataKeyCertificate))
	out = append(out, fmt.Sprintf("--etcd-keyfile=%s/%s", volumeMountPathEtcdClient, secrets.DataKeyPrivateKey))
	out = append(out, fmt.Sprintf("--etcd-servers=https://%s:%d", etcd.ServiceName(v1beta1constants.ETCDRoleMain), etcd.PortEtcdClient))
	out = append(out, fmt.Sprintf("--etcd-servers-overrides=/events#https://%s:%d", etcd.ServiceName(v1beta1constants.ETCDRoleEvents), etcd.PortEtcdClient))
	out = append(out, fmt.Sprintf("--encryption-provider-config=%s/%s", volumeMountPathEtcdEncryptionConfig, secretETCDEncryptionConfigurationDataKey))
	out = append(out, "--external-hostname="+k.values.ExternalHostname)

	if k.values.EventTTL != nil {
		out = append(out, fmt.Sprintf("--event-ttl=%s", k.values.EventTTL.Duration))
	}

	if k.values.FeatureGates != nil {
		out = append(out, kutil.FeatureGatesToCommandLineParameter(k.values.FeatureGates))
	}

	if version.ConstraintK8sLess124.Check(k.values.Version) {
		out = append(out, "--insecure-port=0")
	}

	out = append(out, "--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP")
	out = append(out, fmt.Sprintf("--kubelet-client-certificate=%s/%s", volumeMountPathKubeAPIServerToKubelet, secrets.DataKeyCertificate))
	out = append(out, fmt.Sprintf("--kubelet-client-key=%s/%s", volumeMountPathKubeAPIServerToKubelet, secrets.DataKeyPrivateKey))

	if k.values.Requests != nil {
		if k.values.Requests.MaxNonMutatingInflight != nil {
			out = append(out, fmt.Sprintf("--max-requests-inflight=%d", *k.values.Requests.MaxNonMutatingInflight))
		}

		if k.values.Requests.MaxMutatingInflight != nil {
			out = append(out, fmt.Sprintf("--max-mutating-requests-inflight=%d", *k.values.Requests.MaxMutatingInflight))
		}
	}

	out = append(out, "--profiling=false")
	out = append(out, fmt.Sprintf("--proxy-client-cert-file=%s/%s", volumeMountPathKubeAggregator, secrets.DataKeyCertificate))
	out = append(out, fmt.Sprintf("--proxy-client-key-file=%s/%s", volumeMountPathKubeAggregator, secrets.DataKeyPrivateKey))
	out = append(out, fmt.Sprintf("--requestheader-client-ca-file=%s/%s", volumeMountPathCAFrontProxy, secrets.DataKeyCertificateBundle))
	out = append(out, "--requestheader-extra-headers-prefix=X-Remote-Extra-")
	out = append(out, "--requestheader-group-headers=X-Remote-Group")
	out = append(out, "--requestheader-username-headers=X-Remote-User")

	if k.values.RuntimeConfig != nil {
		out = append(out, kutil.MapStringBoolToCommandLineParameter(k.values.RuntimeConfig, "--runtime-config="))
	}

	out = append(out, "--service-account-issuer="+k.values.ServiceAccount.Issuer)
	for _, issuer := range k.values.ServiceAccount.AcceptedIssuers {
		out = append(out, fmt.Sprintf("--service-account-issuer=%s", issuer))
	}
	if k.values.ServiceAccount.ExtendTokenExpiration != nil {
		out = append(out, fmt.Sprintf("--service-account-extend-token-expiration=%s", strconv.FormatBool(*k.values.ServiceAccount.ExtendTokenExpiration)))
	}
	if k.values.ServiceAccount.MaxTokenExpiration != nil {
		out = append(out, fmt.Sprintf("--service-account-max-token-expiration=%s", k.values.ServiceAccount.MaxTokenExpiration.Duration))
	}

	out = append(out, fmt.Sprintf("--service-cluster-ip-range=%s", k.values.VPN.ServiceNetworkCIDR))
	out = append(out, fmt.Sprintf("--secure-port=%d", Port))
	out = append(out, fmt.Sprintf("--token-auth-file=%s/%s", volumeMountPathStaticToken, secrets.DataKeyStaticTokenCSV))
	out = append(out, fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate))
	out = append(out, fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey))
	out = append(out, "--tls-cipher-suites="+strings.Join(kutil.TLSCipherSuites(k.values.Version), ","))
	out = append(out, "--v=2")

	if k.values.WatchCacheSizes != nil {
		if k.values.WatchCacheSizes.Default != nil {
			out = append(out, fmt.Sprintf("--default-watch-cache-size=%d", *k.values.WatchCacheSizes.Default))
		}

		if len(k.values.WatchCacheSizes.Resources) > 0 {
			var sizes []string

			for _, resource := range k.values.WatchCacheSizes.Resources {
				size := resource.Resource
				if resource.APIGroup != nil {
					size += "." + *resource.APIGroup
				}
				size += fmt.Sprintf("#%d", resource.CacheSize)

				sizes = append(sizes, size)
			}

			out = append(out, "--watch-cache-sizes="+strings.Join(sizes, ","))
		}
	}

	return out
}

func (k *kubeAPIServer) admissionPluginNames() []string {
	var out []string

	for _, plugin := range k.values.AdmissionPlugins {
		out = append(out, plugin.Name)
	}

	return out
}

func (k *kubeAPIServer) handleHostCertVolumes(deployment *appsv1.Deployment) {
	directoryOrCreate := corev1.HostPathDirectoryOrCreate

	// locations are taken from
	// https://github.com/golang/go/blob/1bb247a469e306c57a5e0eaba788efb8b3b1acef/src/crypto/x509/root_linux.go#L7-L15
	// we cannot be sure on which Node OS the Seed Cluster is running so, it's safer to mount them all
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
		{
			Name:      volumeNameFedora,
			MountPath: volumeMountPathFedora,
			ReadOnly:  true,
		},
		{
			Name:      volumeNameCentOS,
			MountPath: volumeMountPathCentOS,
			ReadOnly:  true,
		},
		{
			Name:      volumeNameEtcSSL,
			MountPath: volumeMountPathEtcSSL,
			ReadOnly:  true,
		},
		{
			Name:      volumeNameUsrShareCaCerts,
			MountPath: volumeMountPathUsrShareCaCerts,
			ReadOnly:  true,
		},
	}...)
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: volumeNameFedora,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathFedora,
					Type: &directoryOrCreate,
				},
			},
		},
		{
			Name: volumeNameCentOS,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathCentOS,
					Type: &directoryOrCreate,
				},
			},
		},
		{
			Name: volumeNameEtcSSL,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathEtcSSL,
					Type: &directoryOrCreate,
				},
			},
		},
		{
			Name: volumeNameUsrShareCaCerts,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathUsrShareCaCerts,
					Type: &directoryOrCreate,
				},
			},
		},
	}...)
}

func (k *kubeAPIServer) handleBasicAuthenticationSettings(deployment *appsv1.Deployment, secret *corev1.Secret) {
	if !k.values.BasicAuthenticationEnabled {
		return
	}

	deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--basic-auth-file=%s/%s", volumeMountPathBasicAuthentication, secrets.DataKeyCSV))
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
		{
			Name:      volumeNameBasicAuthentication,
			MountPath: volumeMountPathBasicAuthentication,
		},
	}...)
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: volumeNameBasicAuthentication,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		},
	}...)
}

func (k *kubeAPIServer) handleSNISettings(deployment *appsv1.Deployment) {
	if !k.values.SNI.Enabled {
		return
	}

	deployment.Labels[v1beta1constants.LabelAPIServerExposure] = v1beta1constants.LabelAPIServerExposureGardenerManaged
	deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--advertise-address=%s", k.values.SNI.AdvertiseAddress))
}

func (k *kubeAPIServer) handleLifecycleSettings(deployment *appsv1.Deployment) {
	deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--livez-grace-period=1m")
	deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--shutdown-delay-duration=15s")
}

func (k *kubeAPIServer) handleVPNSettings(
	deployment *appsv1.Deployment,
	configMapEgressSelector *corev1.ConfigMap,
	vpnCASecret *corev1.Secret,
	secretHTTPProxy *corev1.Secret,
	secretLegacyVPNCAClient *corev1.Secret,
	secretLegacyVPNSeed *corev1.Secret,
	secretLegacyVPNSeedTLSAuth *corev1.Secret,
) {
	if !k.values.VPN.ReversedVPNEnabled {
		deployment.Spec.Template.Labels[v1beta1constants.LabelNetworkPolicyToShootNetworks] = v1beta1constants.LabelNetworkPolicyAllowed
		deployment.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "set-iptable-rules",
			Image: k.values.Images.AlpineIPTables,
			Command: []string{
				"/bin/sh",
				"-c",
				"iptables -A INPUT -i tun0 -p icmp -j ACCEPT && iptables -A INPUT -i tun0 -m state --state NEW -j DROP",
			},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
				Privileged: pointer.Bool(true),
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      volumeNameLibModules,
				MountPath: volumeMountPathLibModules,
			}},
		}}
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameLibModules,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/lib/modules"},
			},
		})

		vpnSeedContainer := corev1.Container{
			Name:            containerNameVPNSeed,
			Image:           k.values.Images.VPNSeed,
			ImagePullPolicy: corev1.PullIfNotPresent,
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
					Value: volumeMountPathCA + "/" + secrets.DataKeyCertificateBundle,
				},
				{
					Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CRT",
					Value: volumeMountPathVPNSeed + "/" + secrets.DataKeyCertificate,
				},
				{
					Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_KEY",
					Value: volumeMountPathVPNSeed + "/" + secrets.DataKeyPrivateKey,
				},
				{
					Name:  "SERVICE_NETWORK",
					Value: k.values.VPN.ServiceNetworkCIDR,
				},
				{
					Name:  "POD_NETWORK",
					Value: k.values.VPN.PodNetworkCIDR,
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
				Privileged: pointer.Bool(true),
			},
			TerminationMessagePath:   corev1.TerminationMessagePathDefault,
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeNameVPNSeed,
					MountPath: volumeMountPathVPNSeed,
				},
				{
					Name:      volumeNameVPNSeedTLSAuth,
					MountPath: volumeMountPathVPNSeedTLSAuth,
				},
				{
					Name:      volumeNameCA,
					MountPath: volumeMountPathCA,
				},
			},
		}

		if k.values.VPN.NodeNetworkCIDR != nil {
			vpnSeedContainer.Env = append(vpnSeedContainer.Env, corev1.EnvVar{
				Name:  "NODE_NETWORK",
				Value: *k.values.VPN.NodeNetworkCIDR,
			})
		}

		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, vpnSeedContainer)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: volumeNameVPNSeed,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: pointer.Int32(420),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretLegacyVPNCAClient.Name,
									},
									Items: []corev1.KeyToPath{{
										Key:  secrets.DataKeyCertificateBundle,
										Path: secrets.DataKeyCertificateCA,
									}},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretLegacyVPNSeed.Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  secrets.DataKeyCertificate,
											Path: secrets.DataKeyCertificate,
										},
										{
											Key:  secrets.DataKeyPrivateKey,
											Path: secrets.DataKeyPrivateKey,
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: volumeNameVPNSeedTLSAuth,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: secretLegacyVPNSeedTLSAuth.Name},
				},
			},
		}...)
	} else {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--egress-selector-config-file=%s/%s", volumeMountPathEgressSelector, configMapEgressSelectorDataKey))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
			{
				Name:      volumeNameCAVPN,
				MountPath: volumeMountPathCAVPN,
			},
			{
				Name:      volumeNameHTTPProxy,
				MountPath: volumeMountPathHTTPProxy,
			},
			{
				Name:      volumeNameEgressSelector,
				MountPath: volumeMountPathEgressSelector,
			},
		}...)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: volumeNameCAVPN,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: vpnCASecret.Name,
					},
				},
			},
			{
				Name: volumeNameHTTPProxy,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretHTTPProxy.Name,
					},
				},
			},
			{
				Name: volumeNameEgressSelector,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapEgressSelector.Name,
						},
					},
				},
			},
		}...)
	}
}

func (k *kubeAPIServer) handleOIDCSettings(deployment *appsv1.Deployment, secretOIDCCABundle *corev1.Secret) {
	if k.values.OIDC == nil {
		return
	}

	if k.values.OIDC.CABundle != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--oidc-ca-file=%s/%s", volumeMountPathOIDCCABundle, secretOIDCCABundleDataKeyCaCrt))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
			{
				Name:      volumeNameOIDCCABundle,
				MountPath: volumeMountPathOIDCCABundle,
			},
		}...)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: volumeNameOIDCCABundle,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretOIDCCABundle.Name,
					},
				},
			},
		}...)
	}

	if v := k.values.OIDC.IssuerURL; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-issuer-url="+*v)
	}

	if v := k.values.OIDC.ClientID; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-client-id="+*v)
	}

	if v := k.values.OIDC.UsernameClaim; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-username-claim="+*v)
	}

	if v := k.values.OIDC.GroupsClaim; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-groups-claim="+*v)
	}

	if v := k.values.OIDC.UsernamePrefix; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-username-prefix="+*v)
	}

	if v := k.values.OIDC.GroupsPrefix; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-groups-prefix="+*v)
	}

	if k.values.OIDC.SigningAlgs != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-signing-algs="+strings.Join(k.values.OIDC.SigningAlgs, ","))
	}

	for key, value := range k.values.OIDC.RequiredClaims {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, "--oidc-required-claim="+fmt.Sprintf("%s=%s", key, value))
	}
}

func (k *kubeAPIServer) handleServiceAccountSigningKeySettings(deployment *appsv1.Deployment, secretServiceAccountSigningKey *corev1.Secret) {
	if k.values.ServiceAccount.SigningKey != nil {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--service-account-signing-key-file=%s/%s", volumeMountPathUserProvidedServiceAccountSigningKey, SecretServiceAccountSigningKeyDataKeySigningKey))
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--service-account-key-file=%s/%s", volumeMountPathUserProvidedServiceAccountSigningKey, SecretServiceAccountSigningKeyDataKeySigningKey))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
			{
				Name:      volumeNameUserProvidedServiceAccountSigningKey,
				MountPath: volumeMountPathUserProvidedServiceAccountSigningKey,
			},
		}...)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: volumeNameUserProvidedServiceAccountSigningKey,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretServiceAccountSigningKey.Name,
					},
				},
			},
		}...)
	} else {
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--service-account-signing-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey))
		deployment.Spec.Template.Spec.Containers[0].Command = append(deployment.Spec.Template.Spec.Containers[0].Command, fmt.Sprintf("--service-account-key-file=%s/%s", volumeMountPathServiceAccountKeyBundle, secrets.DataKeyPrivateKeyBundle))
	}
}

func (k *kubeAPIServer) handlePodMutatorSettings(deployment *appsv1.Deployment) {
	if k.values.SNI.PodMutatorEnabled {
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
			Name:  containerNameAPIServerProxyPodMutator,
			Image: k.values.Images.APIServerProxyPodWebhook,
			Args: []string{
				"--apiserver-fqdn=" + k.values.SNI.APIServerFQDN,
				"--host=localhost",
				"--port=9443",
				"--cert-dir=" + volumeMountPathServer,
				"--cert-name=" + secrets.DataKeyCertificate,
				"--key-name=" + secrets.DataKeyPrivateKey,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("128M"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("500M"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      volumeNameServer,
				MountPath: volumeMountPathServer,
			}},
		})
	}
}
