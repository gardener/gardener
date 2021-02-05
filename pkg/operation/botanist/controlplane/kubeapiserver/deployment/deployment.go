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
	"fmt"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/konnectivity"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (k *kubeAPIServer) deployKubeAPIServerDeployment(
	ctx context.Context,
	command []string,
	checksumServiceAccountSigningKey,
	checksumConfigMapEgressSelection,
	checksumConfigMapAuditPolicy,
	checksumSecretOIDCCABundle,
	checksumConfigMapAdmissionConfig *string) error {
	foundDeployment := true
	deployment := k.emptyDeployment()
	if err := k.seedClient.Client().Get(ctx, kutil.Key(k.seedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		foundDeployment = false
	}

	existingDeployment := *deployment
	k.deploymentReplicas = k.getDeploymentReplicas(existingDeployment)
	maxSurge25 := intstr.FromString("25%")
	maxUnavailability := intstr.FromInt(0)

	// build deployment to be created / updated
	toBeUpdated := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: k.seedNamespace,
			Labels:    k.getAPIServerDeploymentLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: k.deploymentReplicas,
			Selector: &metav1.LabelSelector{MatchLabels: getAPIServerDeploymentSelectorLabels()},
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
					Annotations: utils.MergeStringMaps(k.getOptionalAnnotations(checksumServiceAccountSigningKey, checksumConfigMapEgressSelection, checksumSecretOIDCCABundle), map[string]string{
						"checksum/configmap-audit-policy":                                      *checksumConfigMapAuditPolicy,
						"checksum/configmap-admission-config":                                  *checksumConfigMapAdmissionConfig,
						fmt.Sprintf("checksum/secret-%s", k.secrets.CAFrontProxy.Name):         k.secrets.CAFrontProxy.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.CA.Name):                   k.secrets.CA.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.TLSServer.Name):            k.secrets.TLSServer.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.KubeAggregator.Name):       k.secrets.KubeAggregator.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.KubeAPIServerKubelet.Name): k.secrets.KubeAPIServerKubelet.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.StaticToken.Name):          k.secrets.StaticToken.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.ServiceAccountKey.Name):    k.secrets.ServiceAccountKey.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.EtcdCA.Name):               k.secrets.EtcdCA.Checksum,
						fmt.Sprintf("checksum/secret-%s", k.secrets.EtcdClientTLS.Name):        k.secrets.EtcdClientTLS.Checksum,
						"networkpolicy/konnectivity-enabled":                                   strconv.FormatBool(k.konnectivityTunnelEnabled),
					}),
					Labels: utils.MergeStringMaps(getAPIServerPodLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
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
													Key:      v1beta1constants.LabelApp,
													Operator: metav1.LabelSelectorOpIn,
													Values: []string{
														v1beta1constants.LabelKubernetes,
													},
												},
												{
													Key:      v1beta1constants.LabelRole,
													Operator: metav1.LabelSelectorOpIn,
													Values: []string{
														labelRole,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane,
					Containers: []corev1.Container{
						{
							Name:            containerNameKubeAPIServer,
							Image:           k.images.KubeAPIServerImageName,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         command,
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   k.computeLivenessProbePath(),
										Scheme: corev1.URISchemeHTTPS,
										Port:   intstr.FromInt(443),
										HTTPHeaders: []corev1.HTTPHeader{
											{
												Name:  "Authorization",
												Value: fmt.Sprintf("Bearer %s", k.healthCheckToken),
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
										Path:   k.computeReadinessProbePath(),
										Scheme: corev1.URISchemeHTTPS,
										Port:   intstr.FromInt(443),
										HTTPHeaders: []corev1.HTTPHeader{
											{
												Name:  "Authorization",
												Value: fmt.Sprintf("Bearer %s", k.healthCheckToken),
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
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							Ports: []corev1.ContainerPort{
								{
									Name:          portNameHTTPS,
									ContainerPort: int32(443),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: k.getDeploymentResources(&existingDeployment, foundDeployment),
							VolumeMounts: []corev1.VolumeMount{
								{Name: volumeMountNameAuditPolicyConfig,
									MountPath: volumeMountPathAuditPolicyConfig,
								},
								{
									Name:      volumeMountNameCA,
									MountPath: volumeMountPathCA,
								},
								{
									Name:      volumeMountNameCAEtcd,
									MountPath: volumeMountPathETCDCA,
								},
								{
									Name:      volumeMountNameCAFrontProxy,
									MountPath: volumeMountPathCAFrontProxy,
								},
								{
									Name:      volumeMountNameEtcdClientTLS,
									MountPath: volumeMountPathETCDClient,
								},
								{
									Name:      volumeMountNameTLSServer,
									MountPath: volumeMountPathTLS,
								},
								{
									Name:      volumeMountNameServiceAccountKey,
									MountPath: volumeMountPathServiceAccountKey,
								},
								{
									Name:      volumeMountNameStaticToken,
									MountPath: volumeMountPathStaticTokenAuth,
								},
								{
									Name:      volumeMountNameKubeAPIServerKubelet,
									MountPath: volumeMountPathKubeletSecret,
								},
								{
									Name:      volumeMountNameKubeAggregator,
									MountPath: volumeMountPathKubeAggregator,
								},
								{
									Name:      volumeMountNameAdmissionConfig,
									MountPath: volumeMountPathAdmissionPluginConfig,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeMountNameAuditPolicyConfig,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cmNameAuditPolicyConfig,
									},
								},
							},
						},
						{
							Name: volumeMountNameCA,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.CA.Name,
								},
							},
						},
						{
							Name: volumeMountNameCAEtcd,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.EtcdCA.Name,
								},
							},
						},
						{
							Name: volumeMountNameCAFrontProxy,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.CAFrontProxy.Name,
								},
							},
						},
						{
							Name: volumeMountNameTLSServer,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.TLSServer.Name,
								},
							},
						},
						{
							Name: volumeMountNameEtcdClientTLS,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.EtcdClientTLS.Name,
								},
							},
						},
						{
							Name: volumeMountNameServiceAccountKey,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.ServiceAccountKey.Name,
								},
							},
						},
						{
							Name: volumeMountNameStaticToken,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.StaticToken.Name,
								},
							},
						},
						{
							Name: volumeMountNameKubeAPIServerKubelet,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.KubeAPIServerKubelet.Name,
								},
							},
						},
						{
							Name: volumeMountNameKubeAggregator,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: k.secrets.KubeAggregator.Name,
								},
							},
						},
						{
							Name: volumeMountNameAdmissionConfig,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cmNameAPIServerAdmissionConfig,
									},
								},
							},
						},
					}},
			},
		},
	}

	if k.konnectivityTunnelEnabled {
		toBeUpdated = k.configureDeploymentForKonnectivity(toBeUpdated)
	}

	if k.konnectivityTunnelEnabled && !k.sniValues.SNIEnabled {
		toBeUpdated = k.configureDeploymentForKonnectivityNoSNI(toBeUpdated)
	} else if k.konnectivityTunnelEnabled && k.sniValues.SNIEnabled {
		toBeUpdated = k.configureDeploymentForSNIAndKonnectivity(toBeUpdated)
	} else if !k.konnectivityTunnelEnabled {
		toBeUpdated = k.configureDeploymentForVPN(toBeUpdated)
	}

	if k.sniValues.SNIPodMutatorEnabled {
		toBeUpdated = k.configureDeploymentWithSNIPodMutator(toBeUpdated)
	}

	toBeUpdated = k.configureDeploymentForVersion(toBeUpdated)

	toBeUpdated = k.configureDeploymentForUserConfiguration(toBeUpdated)

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), deployment, func() error {
		deployment.Spec = toBeUpdated.Spec
		deployment.Labels = toBeUpdated.Labels
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (k *kubeAPIServer) getOptionalAnnotations(
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

	if k.konnectivityTunnelEnabled && !k.sniValues.SNIEnabled {
		annotations[fmt.Sprintf("checksum/secret-%s", k.secrets.KonnectivityServerCerts.Name)] = k.secrets.KonnectivityServerCerts.Checksum
	} else if k.konnectivityTunnelEnabled && k.sniValues.SNIEnabled {
		annotations["checksum/secret-"+k.secrets.KonnectivityServerClientTLS.Name] = k.secrets.KonnectivityServerClientTLS.Checksum
	} else {
		annotations[fmt.Sprintf("checksum/secret-%s", k.secrets.VpnSeed.Name)] = k.secrets.VpnSeed.Checksum
		annotations[fmt.Sprintf("checksum/secret-%s", k.secrets.VpnSeedTLSAuth.Name)] = k.secrets.VpnSeedTLSAuth.Checksum
	}

	if k.etcdEncryptionEnabled {
		// etcd-encryption secret name is different from the annotation key
		annotations["checksum/secret-etcd-encryption"] = k.secrets.EtcdEncryption.Checksum
	}

	if k.basicAuthenticationEnabled {
		annotations[fmt.Sprintf("checksum/secret-%s", k.secrets.BasicAuth.Name)] = k.secrets.BasicAuth.Checksum
	}
	return annotations
}

func (k *kubeAPIServer) configureDeploymentForKonnectivity(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      volumeMountNameEgressSelectionConfig,
			MountPath: volumeMountPathKonnectivityEgressSelector,
		},
	)

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeMountNameEgressSelectionConfig,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmNameKonnectivityEgressSelector,
					},
				},
			},
		},
	)

	return d
}

func (k *kubeAPIServer) configureDeploymentForKonnectivityNoSNI(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.ServiceAccountName = serviceAccountName

	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeMountNameKonnectivityUDS,
		MountPath: volumeMountPathKonnectivityUDS,
		ReadOnly:  false,
	})

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeMountNameKonnectivityServerCerts,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k.secrets.KonnectivityServerCerts.Name,
				},
			},
		},
		corev1.Volume{
			Name: volumeMountNameKonnectivityServerKubeconfig,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k.secrets.KonnectivityServerKubeconfig.Name,
				},
			},
		},
		corev1.Volume{
			Name: volumeMountNameKonnectivityUDS,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, GetKonnectivityServerSidecar(k.images.KonnectivityServerTunnelImageName, k.seedNamespace))
	return d
}

func GetKonnectivityServerSidecar(tunnelImageName, seedNamespace string) corev1.Container {
	return corev1.Container{
		Name:            containerNameKonnectivityServer,
		Image:           tunnelImageName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/replica-reloader"},
		Args: []string{
			fmt.Sprintf("--namespace=%s", seedNamespace),
			fmt.Sprintf("--deployment-name=%s", v1beta1constants.DeploymentNameKubeAPIServer),
			"--jitter=10s",
			"--jitter-factor=5",
			"--v=2",
			"--",
			"/proxy-server",
			fmt.Sprintf("--uds-name=%s/%s", volumeMountPathKonnectivityUDS, konnectivityUDSName),
			"--logtostderr=true",
			fmt.Sprintf("--cluster-cert=%s/%s.crt", volumeMountPathKonnectivityServerCerts, konnectivity.SecretNameServerTLS),
			fmt.Sprintf("--cluster-key=%s/%s.key", volumeMountPathKonnectivityServerCerts, konnectivity.SecretNameServerTLS),
			fmt.Sprintf("--agent-namespace=%s", metav1.NamespaceSystem),
			fmt.Sprintf("--agent-service-account=%s", konnectivity.AgentName),
			fmt.Sprintf("--kubeconfig=%s/kubeconfig", volumeMountPathKonnectivityServerKubeconfig),
			"--authentication-audience=system:konnectivity-server",
			"--keepalive-time=1h",
			"--log-file-max-size=0",
			"--delete-existing-uds-file=true",
			"--mode=http-connect",
			// the server port should always be 0 when using UDS
			"--server-port=0",
			fmt.Sprintf("--agent-port=%d", konnectivity.ServerAgentPort),
			fmt.Sprintf("--admin-port=%d", konnectivity.ServerAdminPort),
			fmt.Sprintf("--health-port=%d", konnectivity.ServerHealthPort),
			"--v=2",
			// the last argument should be server-count - the reloader injects the actual count after it
			"--server-count",
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Host:   "127.0.0.1",
					Path:   "/healthz",
					Scheme: corev1.URISchemeHTTP,
					Port:   intstr.FromInt(int(konnectivity.ServerHealthPort)),
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
				ContainerPort: konnectivity.ServerAgentPort,
			},
			{
				Name:          "adminport",
				ContainerPort: konnectivity.ServerAdminPort,
				HostPort:      konnectivity.ServerAdminPort,
			},
			{
				Name:          "healthport",
				ContainerPort: konnectivity.ServerHealthPort,
				HostPort:      konnectivity.ServerHealthPort,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeMountNameKonnectivityServerCerts,
				MountPath: volumeMountPathKonnectivityServerCerts,
				ReadOnly:  true,
			},
			{
				Name:      volumeMountNameKonnectivityServerKubeconfig,
				MountPath: volumeMountPathKonnectivityServerKubeconfig,
				ReadOnly:  true,
			},
			{
				Name:      volumeMountNameKonnectivityUDS,
				MountPath: volumeMountPathKonnectivityUDS,
				ReadOnly:  false,
			},
		},
	}
}

func (k *kubeAPIServer) configureDeploymentForSNIAndKonnectivity(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      volumeMountNameKonnectivityClientTLS,
			MountPath: volumeMountPathKonnectivityClientTLS,
			ReadOnly:  false,
		},
	)

	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeMountNameKonnectivityClientTLS,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k.secrets.KonnectivityServerClientTLS.Name,
				},
			},
		})
	return d
}

func (k *kubeAPIServer) configureDeploymentForVPN(d appsv1.Deployment) appsv1.Deployment {
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: volumeMountNameVPNSeed,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k.secrets.VpnSeed.Name,
				},
			},
		},
		corev1.Volume{
			Name: volumeMountNameVPNSeedTLSAuth,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k.secrets.VpnSeedTLSAuth.Name,
				},
			},
		},
		corev1.Volume{
			Name: volumeMountNameVPNModules,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathVPNModules,
					Type: &directoryOrCreate,
				},
			},
		},
	)

	d.Spec.Template.Spec.InitContainers = append(d.Spec.Template.Spec.InitContainers, corev1.Container{
		Name:  "set-iptable-rules",
		Image: k.images.AlpineIptablesImageName,
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
				Name:      volumeMountNameVPNModules,
				MountPath: volumeMountPathVPNModules,
			},
		},
	})

	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, k.getVPNSidecar())

	return d
}

func (k *kubeAPIServer) getVPNSidecar() corev1.Container {
	c := corev1.Container{
		Name:            containerNameVPNSeed,
		Image:           k.images.VPNSeedImageName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name:  "MAIN_VPN_SEED",
				Value: "true",
			},
			{
				Name:  "OPENVPN_PORT",
				Value: vpnPort,
			},
			{
				Name:  "APISERVER_AUTH_MODE",
				Value: "client-cert",
			},
			{
				Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CA",
				Value: fmt.Sprintf("%s/%s", volumeMountPathVPNSeed, secrets.DataKeyCertificateCA),
			},
			{
				Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CRT",
				Value: fmt.Sprintf("%s/%s", volumeMountPathVPNSeed, secrets.DataKeyCertificate),
			},
			{
				Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_KEY",
				Value: fmt.Sprintf("%s/%s", volumeMountPathVPNSeed, secrets.DataKeyPrivateKey),
			},
			{
				Name:  "SERVICE_NETWORK",
				Value: k.serviceNetwork.String(),
			},
			{
				Name:  "POD_NETWORK",
				Value: k.podNetwork.String(),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "tcp-tunnel",
				ContainerPort: vpnClientPort,
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
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeMountNameVPNSeed,
				MountPath: volumeMountPathVPNSeed,
				ReadOnly:  true,
			},
			{
				Name:      volumeMountNameVPNSeedTLSAuth,
				MountPath: volumeMountPathVPNSeedTLSAuth,
				ReadOnly:  true,
			},
		},
	}

	if k.nodeNetwork != nil && len(k.nodeNetwork.String()) > 0 {
		c.Env = append(c.Env,
			corev1.EnvVar{
				Name:  "NODE_NETWORK",
				Value: k.nodeNetwork.String(),
			})
	}

	return c
}

func (k *kubeAPIServer) configureDeploymentWithSNIPodMutator(d appsv1.Deployment) appsv1.Deployment {
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers,
		corev1.Container{
			Name:            containerNameApiserverProxyPodMutator,
			Image:           k.images.ApiServerProxyPodMutatorWebhookImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				fmt.Sprintf("--apiserver-fqdn=%s", k.shootOutOfClusterAPIServerAddress),
				"--host=localhost",
				fmt.Sprintf("--cert-dir=%s", volumeMountPathTLS),
				"--cert-name=kube-apiserver.crt",
				"--key-name=kube-apiserver.key",
				"--port=9443",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      volumeMountNameTLSServer,
					MountPath: volumeMountPathTLS,
					ReadOnly:  true,
				},
			},
		})

	return d
}

func (k *kubeAPIServer) configureDeploymentForVersion(d appsv1.Deployment) appsv1.Deployment {
	if versionConstraintK8sSmaller116.Check(k.shootKubernetesVersion) {
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

	if versionConstraintK8sGreaterEqual117.Check(k.shootKubernetesVersion) {
		if k.mountHostCADirectories {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleFedoraRHEL6Openelec,
					MountPath: volumeMountPathCABundleFedoraRHEL6Openelec,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleCentOSRHEl7,
					MountPath: volumeMountPathCABundleCentOSRHEl7Dir,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleEtcSSL,
					MountPath: volumeMountPathCABundleEtcSSL,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleUsrShareCacerts,
					MountPath: volumeMountPathCABundleUsrShareCacerts,
					ReadOnly:  true,
				},
			)
		} else {
			d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleDebianFamily,
					MountPath: volumeMountPathCABundleDebianFamily,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleFedoraRHEL6,
					MountPath: volumeMountPathCABundleFedoraRHEL6,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleOpensuse,
					MountPath: volumeMountPathCABundleOpensuse,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleOpenelec,
					MountPath: volumeMountPathCABundleOpenelec,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleCentOSRHEl7,
					MountPath: volumeMountPathCABundleCentOSRHEL7File,
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      volumeMountNameCABundleAlpine,
					MountPath: volumeMountPathCABundleAlpine,
					ReadOnly:  true,
				},
			)
		}
	}

	return d
}

func (k *kubeAPIServer) configureDeploymentForUserConfiguration(d appsv1.Deployment) appsv1.Deployment {
	if k.basicAuthenticationEnabled {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeMountNameBasicAuth,
				MountPath: volumeMountPathBasicAuth,
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeMountNameBasicAuth,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: k.secrets.BasicAuth.Name,
					},
				},
			})
	}

	if k.config != nil && k.config.OIDCConfig != nil && k.config.OIDCConfig.CABundle != nil {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeMountNameOIDCBundle,
				MountPath: volumeMountPathOIDCCABundle,
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeMountNameOIDCBundle,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretNameOIDC,
					},
				}})
	}

	if k.config != nil && k.config.ServiceAccountConfig != nil && k.config.ServiceAccountConfig.SigningKeySecret != nil {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeMountNameServiceAccountSigningKey,
				MountPath: volumeMountPathServiceAccountSigning,
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeMountNameServiceAccountSigningKey,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretNameServiceAccountSigningKey,
					},
				}})
	}

	// enabled if k8s version >= 1.13
	if k.etcdEncryptionEnabled {
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      volumeMountNameEtcdEncryption,
				MountPath: volumeMountPathETCDEncryptionSecret,
				ReadOnly:  true,
			})

		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeMountNameEtcdEncryption,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  k.secrets.EtcdEncryption.Name,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		)
	}

	// gardenlet feature flag, so not really a user config, but external to the API server component
	if k.mountHostCADirectories {
		directoryOrCreate := corev1.HostPathDirectoryOrCreate
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeMountNameCABundleFedoraRHEL6Openelec,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleFedoraRHEL6Openelec,
						Type: &directoryOrCreate,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleCentOSRHEl7,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleCentOSRHEl7Dir,
						Type: &directoryOrCreate,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleEtcSSL,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleEtcSSL,
						Type: &directoryOrCreate,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleUsrShareCacerts,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleUsrShareCacerts,
						Type: &directoryOrCreate,
					},
				},
			},
		)
	} else {
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: volumeMountNameCABundleDebianFamily,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleDebianFamily,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleFedoraRHEL6,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleFedoraRHEL6,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleOpensuse,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleOpensuse,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleOpenelec,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleOpenelec,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleCentOSRHEl7,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleCentOSRHEL7File,
					},
				},
			},
			corev1.Volume{
				Name: volumeMountNameCABundleAlpine,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: volumeMountPathCABundleAlpine,
					},
				},
			},
		)
	}

	return d
}

func (k *kubeAPIServer) getDeploymentReplicas(deployment appsv1.Deployment) *int32 {
	if k.managedSeed != nil && !k.hvpaEnabled {
		return k.managedSeed.Replicas
	} else {
		// is nil if deployment does not exist yet (will be defaulted)
		currentReplicas := deployment.Spec.Replicas

		// As kube-apiserver HPA manages the number of replicas, we have to maintain current number of replicas
		// otherwise keep the value to default
		if currentReplicas != nil && *currentReplicas > 0 {
			return currentReplicas
		}

		// If the shoot is hibernated then we want to keep the number of replicas (scale down happens later).
		if k.hibernationEnabled && (currentReplicas == nil || *currentReplicas == 0) {
			zero := int32(0)
			return &zero
		}
	}
	defaultReplicas := int32(defaultAPIServerReplicas)
	return &defaultReplicas
}

func (k *kubeAPIServer) getDeploymentResources(deployment *appsv1.Deployment, foundDeployment bool) corev1.ResourceRequirements {
	if foundDeployment && k.hvpaEnabled {
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

	if k.managedSeed != nil && !k.hvpaEnabled {
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
		if k.hvpaEnabled {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(k.minNodeCount, k.shootAnnotations[common.ShootAlphaScalingAPIServerClass])
		} else {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(k.maxNodeCount, k.shootAnnotations[common.ShootAlphaScalingAPIServerClass])
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

// getResourcesForAPIServer returns the cpu and memory requirements for API server based on nodeCount
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

func (k *kubeAPIServer) computeLivenessProbePath() string {
	if versionConstraintK8sGreaterEqual116.Check(k.shootKubernetesVersion) {
		return "/livez"
	}
	return "/healthz"
}

func (k *kubeAPIServer) computeReadinessProbePath() string {
	if versionConstraintK8sGreaterEqual116.Check(k.shootKubernetesVersion) {
		return "/readyz"
	}
	return "/healthz"
}

func (k *kubeAPIServer) computeKubeAPIServerCommand(admissionPlugins []gardencorev1beta1.AdmissionPlugin) []string {
	var command []string

	if versionConstraintK8sGreaterEqual117.Check(k.shootKubernetesVersion) {
		command = append(command, "/usr/local/bin/kube-apiserver")
	} else if versionConstraintK8sGreaterEqual115.Check(k.shootKubernetesVersion) {
		command = append(command, "/hyperkube", "kube-apiserver")
	} else {
		command = append(command, "/hyperkube", "apiserver")
	}

	if k.konnectivityTunnelEnabled {
		command = append(command, fmt.Sprintf("--egress-selector-config-file=%s/%s", volumeMountPathKonnectivityEgressSelector, fileNameKonnectivityEgressSelector))
	}

	command = append(command,
		kubernetes.AdmissionPluginsToCommandLineParameter(admissionPlugins),
		fmt.Sprintf("--admission-control-config-file=%s/%s", volumeMountPathAdmissionPluginConfig, fileNameAdmissionPluginConfiguration),
		"--allow-privileged=true",
		"--anonymous-auth=false",
		"--audit-log-path=/var/lib/audit.log",
		fmt.Sprintf("--audit-policy-file=%s/%s", volumeMountPathAuditPolicyConfig, fileNameAuditPolicyConfig),
		"--audit-log-maxsize=100",
		"--audit-log-maxbackup=5",
		"--authorization-mode=Node,RBAC",
	)

	if k.sniValues.SNIEnabled {
		command = append(command, fmt.Sprintf("--advertise-address=%s", k.sniValues.shootAPIServerClusterIP))
	}
	if k.basicAuthenticationEnabled {
		command = append(command, fmt.Sprintf("--basic-auth-file=%s/%s", volumeMountPathBasicAuth, secrets.DataKeyCSV))
	}
	command = append(command,
		fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathCA, secrets.DataKeyCertificateCA),
		"--enable-aggregator-routing=true",
		"--enable-bootstrap-token-auth=true",
	)

	if k.config != nil && k.config.WatchCacheSizes != nil {
		if k.config.WatchCacheSizes.Default != nil {
			command = append(command, fmt.Sprintf("--default-watch-cache-size=%d", k.config.WatchCacheSizes.Default))
		}
		if len(k.config.WatchCacheSizes.Resources) > 0 {
			var resources []string
			for _, watchResource := range k.config.WatchCacheSizes.Resources {
				group := ""
				if watchResource.APIGroup != nil && len(*watchResource.APIGroup) > 0 {
					group = *watchResource.APIGroup
				}
				resources = append(resources, fmt.Sprintf("%s%s#%d", watchResource.Resource, group, watchResource.CacheSize))
			}
			// format: --watch-cache-sizes=secrets#500,deployments.apps#500
			// see: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/
			command = append(command, fmt.Sprintf("--watch-cache-sizes=%s", strings.Join(resources, ",")))
		}
	}

	command = append(command,
		"--http2-max-streams-per-connection=1000",
		fmt.Sprintf("--etcd-cafile=%s/%s", volumeMountPathETCDCA, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--etcd-certfile=%s/%s", volumeMountPathETCDClient, secrets.DataKeyCertificate),
		fmt.Sprintf("--etcd-keyfile=%s/%s", volumeMountPathETCDClient, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--etcd-servers=https://etcd-main-client:%d", etcd.PortEtcdClient),
		fmt.Sprintf("--etcd-servers-overrides=/events#https://etcd-events-client:%d", etcd.PortEtcdClient),
	)

	if k.etcdEncryptionEnabled {
		command = append(command, fmt.Sprintf("--encryption-provider-config=%s/%s", volumeMountPathETCDEncryptionSecret, common.EtcdEncryptionSecretFileName))
	}

	if k.config != nil && len(k.config.FeatureGates) > 0 {
		command = append(command, kutil.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}

	if versionConstraintK8sSmaller111.Check(k.shootKubernetesVersion) {
		command = append(command, "--feature-gates=PodPriority=true")
	}
	if versionConstraintK8sSmaller112.Check(k.shootKubernetesVersion) {
		command = append(command, "--feature-gates=TokenRequest=true")
	}
	if versionConstraintK8sEqual111.Check(k.shootKubernetesVersion) {
		command = append(command, "--feature-gates=TokenRequestProjection=true")
	}

	command = append(command,
		"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
		fmt.Sprintf("--kubelet-client-certificate=%s/kube-apiserver-kubelet.crt", volumeMountPathKubeletSecret),
		fmt.Sprintf("--kubelet-client-key=%s/kube-apiserver-kubelet.key", volumeMountPathKubeletSecret),
		"--insecure-port=0",
	)

	if k.config != nil && k.config.OIDCConfig != nil {
		if k.config.OIDCConfig.IssuerURL != nil && len(*k.config.OIDCConfig.IssuerURL) > 0 {
			command = append(command, fmt.Sprintf("--oidc-issuer-url=%s", *k.config.OIDCConfig.IssuerURL))
		}
		if k.config.OIDCConfig.ClientID != nil && len(*k.config.OIDCConfig.ClientID) > 0 {
			command = append(command, fmt.Sprintf("--oidc-client-id=%s", *k.config.OIDCConfig.ClientID))
		}
		if k.config.OIDCConfig.CABundle != nil && len(*k.config.OIDCConfig.CABundle) > 0 {
			command = append(command, fmt.Sprintf("--oidc-ca-file=%s/%s", volumeMountPathOIDCCABundle, fileNameSecretOIDCCert))
		}
		if k.config.OIDCConfig.UsernameClaim != nil && len(*k.config.OIDCConfig.UsernameClaim) > 0 {
			command = append(command, fmt.Sprintf("--oidc-username-claim=%s", *k.config.OIDCConfig.UsernameClaim))
		}
		if k.config.OIDCConfig.GroupsClaim != nil && len(*k.config.OIDCConfig.GroupsClaim) > 0 {
			command = append(command, fmt.Sprintf("--oidc-groups-claim=%s", *k.config.OIDCConfig.GroupsClaim))
		}
		if k.config.OIDCConfig.UsernamePrefix != nil && len(*k.config.OIDCConfig.UsernamePrefix) > 0 {
			command = append(command, fmt.Sprintf("--oidc-username-prefix=%s", *k.config.OIDCConfig.UsernamePrefix))
		}
		if k.config.OIDCConfig.GroupsPrefix != nil && len(*k.config.OIDCConfig.GroupsPrefix) > 0 {
			command = append(command, fmt.Sprintf("--oidc-groups-prefix=%s", *k.config.OIDCConfig.GroupsPrefix))
		}
		if k.config.OIDCConfig.SigningAlgs != nil && len(k.config.OIDCConfig.SigningAlgs) > 0 {
			command = append(command, fmt.Sprintf("--oidc-signing-algs=%s", strings.Join(k.config.OIDCConfig.SigningAlgs, ",")))
		}
		if k.config.OIDCConfig.RequiredClaims != nil && len(k.config.OIDCConfig.RequiredClaims) > 0 {
			for key, value := range k.config.OIDCConfig.RequiredClaims {
				command = append(command, fmt.Sprintf("--oidc-required-claim=%s=%s", key, value))
			}
		}
	}

	if versionConstraintK8sGreaterEqual116.Check(k.shootKubernetesVersion) {
		command = append(command, "--livez-grace-period=1m")
	}

	if k.config != nil && k.config.Requests != nil {
		if v := k.config.Requests.MaxNonMutatingInflight; v != nil {
			command = append(command, fmt.Sprintf("--max-requests-inflight=%d", *k.config.Requests.MaxNonMutatingInflight))
		}
		if v := k.config.Requests.MaxMutatingInflight; v != nil {
			command = append(command, fmt.Sprintf("--max-mutating-requests-inflight=%d", *k.config.Requests.MaxMutatingInflight))
		}
	}

	command = append(command,
		"--profiling=false",
		fmt.Sprintf("--proxy-client-cert-file=%s/kube-aggregator.crt", volumeMountPathKubeAggregator),
		fmt.Sprintf("--proxy-client-key-file=%s/kube-aggregator.key", volumeMountPathKubeAggregator),
		fmt.Sprintf("--requestheader-client-ca-file=%s/%s", volumeMountPathCAFrontProxy, secrets.DataKeyCertificateCA),
		"--requestheader-extra-headers-prefix=X-Remote-Extra-",
		"--requestheader-group-headers=X-Remote-Group",
		"--requestheader-username-headers=X-Remote-User",
	)

	if k.config != nil && len(k.config.RuntimeConfig) > 0 {
		for key, value := range k.config.RuntimeConfig {
			command = append(command, fmt.Sprintf("--runtime-config=%s=%s", key, strconv.FormatBool(value)))
		}
	}

	if versionConstraintK8sSmaller111.Check(k.shootKubernetesVersion) {
		command = append(command, "--runtime-config=scheduling.k8s.io/v1alpha1=true")
	}

	if versionConstraintK8sSmaller114.Check(k.shootKubernetesVersion) {
		command = append(command, "--runtime-config=admissionregistration.k8s.io/v1alpha1=true")
	}

	command = append(command,
		"--secure-port=443",
		fmt.Sprintf("--service-cluster-ip-range=%s", k.serviceNetwork.String()),
		fmt.Sprintf("--service-account-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey),
	)

	if versionConstraintK8sGreaterEqual116.Check(k.shootKubernetesVersion) {
		command = append(command, "--shutdown-delay-duration=15s")
	}

	command = append(command,
		fmt.Sprintf("--token-auth-file=%s/%s", volumeMountPathStaticTokenAuth, secrets.DataKeyStaticTokenCSV),
		fmt.Sprintf("--tls-cert-file=%s/kube-apiserver.crt", volumeMountPathTLS),
		fmt.Sprintf("--tls-private-key-file=%s/kube-apiserver.key", volumeMountPathTLS),
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
	)

	apiServerAudiences := []string{"kubernetes"}
	if k.config != nil && len(k.config.APIAudiences) > 0 {
		apiServerAudiences = k.config.APIAudiences
	}

	if versionConstraintK8sSmaller113.Check(k.shootKubernetesVersion) {
		command = append(command, fmt.Sprintf("--service-account-api-audiences=%s", strings.Join(apiServerAudiences, ",")))
	} else {
		command = append(command, fmt.Sprintf("--api-audiences=%s", strings.Join(apiServerAudiences, ",")))
	}

	serviceAccountTokenIssuerURL := fmt.Sprintf("https://%s", k.shootOutOfClusterAPIServerAddress)

	if k.config != nil && k.config.ServiceAccountConfig != nil && k.config.ServiceAccountConfig.Issuer != nil {
		serviceAccountTokenIssuerURL = *k.config.ServiceAccountConfig.Issuer
	}
	command = append(command, fmt.Sprintf("--service-account-issuer=%s", serviceAccountTokenIssuerURL))

	if k.config != nil && k.config.ServiceAccountConfig != nil && k.config.ServiceAccountConfig.SigningKeySecret != nil {
		command = append(command,
			fmt.Sprintf("--service-account-signing-key-file=%s/%s", volumeMountPathServiceAccountSigning, fileNameServiceAccountSigning),
			fmt.Sprintf("--service-account-key-file=%s/%s", volumeMountPathServiceAccountSigning, fileNameServiceAccountSigning))
	} else {
		command = append(command, fmt.Sprintf("--service-account-signing-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey))
	}

	command = append(command, "--v=2")

	return command
}

func getAPIServerPodLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole:           v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:             v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole:            labelRole,
	}
}

func getAPIServerDeploymentSelectorLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: labelRole,
	}
}

func (k *kubeAPIServer) getAPIServerDeploymentLabels() map[string]string {
	labels := map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole:  labelRole,
	}

	if k.sniValues.SNIEnabled {
		return utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.LabelAPIServerExposure: v1beta1constants.LabelAPIServerExposureGardenerManaged,
		})
	}
	return labels
}

func (k *kubeAPIServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.seedNamespace}}
}
