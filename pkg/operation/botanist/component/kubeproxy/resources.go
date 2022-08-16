// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeproxy

import (
	_ "embed"
	"fmt"
	"strconv"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	kubeproxyconfigv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/utils/pointer"
)

const (
	// DaemonSetNamePrefix is the prefix for the names of the kube-proxy DaemonSets.
	DaemonSetNamePrefix = "kube-proxy"
	// ConfigNamePrefix is the prefix for the name of the kube-proxy ConfigMap.
	ConfigNamePrefix = "kube-proxy-config"

	containerName = "kube-proxy"
	serviceName   = "kube-proxy"

	portNameMetrics = "metrics"
	portMetrics     = 10249

	dataKeyKubeconfig         = "kubeconfig"
	dataKeyConfig             = "config.yaml"
	dataKeyConntrackFixScript = "conntrack_fix.sh"
	dataKeyCleanupScript      = "cleanup.sh"

	volumeMountPathKubeconfig         = "/var/lib/kube-proxy-kubeconfig"
	volumeMountPathConfig             = "/var/lib/kube-proxy-config"
	volumeMountPathDir                = "/var/lib/kube-proxy"
	volumeMountPathMode               = "/var/lib/kube-proxy/mode"
	volumeMountPathCleanupScript      = "/script"
	volumeMountPathConntrackFixScript = "/script"
	volumeMountPathKernelModules      = "/lib/modules"
	volumeMountPathSSLCertsHosts      = "/etc/ssl/certs"
	volumeMountPathSystemBusSocket    = "/var/run/dbus/system_bus_socket"

	volumeNameKubeconfig         = "kubeconfig"
	volumeNameConfig             = "kube-proxy-config"
	volumeNameDir                = "kube-proxy-dir"
	volumeNameMode               = "kube-proxy-mode"
	volumeNameCleanupScript      = "kube-proxy-cleanup-script"
	volumeNameConntrackFixScript = "conntrack-fix-script"
	volumeNameKernelModules      = "kernel-modules"
	volumeNameSSLCertsHosts      = "ssl-certs-hosts"
	volumeNameSystemBusSocket    = "systembussocket"

	hostPathSSLCertsHosts   = "/usr/share/ca-certificates"
	hostPathSystemBusSocket = "/var/run/dbus/system_bus_socket"
	hostPathKernelModules   = "/lib/modules"
	hostPathDir             = "/var/lib/kube-proxy"
)

var (
	//go:embed resources/conntrack-fix.sh
	conntrackFixScript string
	//go:embed resources/cleanup.sh
	cleanupScript string
)

func (k *kubeProxy) computeCentralResourcesData() (map[string][]byte, error) {
	componentConfigRaw, err := k.getRawComponentConfig()
	if err != nil {
		return nil, err
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		// This ClusterRoleBinding is similar to 'system:node-proxier' with the difference that it binds the kube-proxy's
		// ServiceAccount to the 'system:node-proxier' ClusterRole.
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:node-proxier",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:node-proxier",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{{
					Name:     portNameMetrics,
					Port:     int32(portMetrics),
					Protocol: corev1.ProtocolTCP,
				}},
				Selector: getLabels(),
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy",
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{dataKeyKubeconfig: k.values.Kubeconfig},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigNamePrefix,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{dataKeyConfig: componentConfigRaw},
		}

		configMapConntrackFixScript = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy-conntrack-fix-script",
				Namespace: metav1.NamespaceSystem,
				Labels:    utils.MergeStringMaps(getLabels(), getSystemComponentLabels()),
			},
			Data: map[string]string{dataKeyConntrackFixScript: conntrackFixScript},
		}

		configMapCleanupScript = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy-cleanup-script",
				Namespace: metav1.NamespaceSystem,
				Labels:    utils.MergeStringMaps(getLabels(), getSystemComponentLabels()),
			},
			Data: map[string]string{dataKeyCleanupScript: cleanupScript},
		}

		podSecurityPolicy *policyv1beta1.PodSecurityPolicy
		clusterRolePSP    *rbacv1.ClusterRole
		roleBindingPSP    *rbacv1.RoleBinding
	)

	utilruntime.Must(kutil.MakeUnique(secret))
	utilruntime.Must(kutil.MakeUnique(configMap))
	utilruntime.Must(kutil.MakeUnique(configMapConntrackFixScript))
	utilruntime.Must(kutil.MakeUnique(configMapCleanupScript))

	k.serviceAccount = serviceAccount
	k.secret = secret
	k.configMap = configMap
	k.configMapCleanupScript = configMapCleanupScript
	k.configMapConntrackFixScript = configMapConntrackFixScript

	if !k.values.PSPDisabled {
		podSecurityPolicy = &policyv1beta1.PodSecurityPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.kube-system.kube-proxy",
				Annotations: map[string]string{
					v1beta1constants.AnnotationSeccompAllowedProfiles: v1beta1constants.AnnotationSeccompAllowedProfilesRuntimeDefaultValue,
					v1beta1constants.AnnotationSeccompDefaultProfile:  v1beta1constants.AnnotationSeccompAllowedProfilesRuntimeDefaultValue,
				},
			},
			Spec: policyv1beta1.PodSecurityPolicySpec{
				Privileged: true,
				Volumes: []policyv1beta1.FSType{
					"hostPath",
					"secret",
					"configMap",
					"projected",
				},
				HostNetwork: true,
				HostPorts: []policyv1beta1.HostPortRange{{
					Min: portMetrics,
					Max: portMetrics,
				}},
				AllowedHostPaths: []policyv1beta1.AllowedHostPath{
					{PathPrefix: hostPathSSLCertsHosts},
					{PathPrefix: hostPathSystemBusSocket},
					{PathPrefix: hostPathKernelModules},
					{PathPrefix: hostPathDir},
				},
				AllowedCapabilities: []corev1.Capability{
					"NET_ADMIN",
				},
				RunAsUser: policyv1beta1.RunAsUserStrategyOptions{
					Rule: policyv1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: policyv1beta1.SELinuxStrategyOptions{
					Rule: policyv1beta1.SELinuxStrategyRunAsAny,
				},
				SupplementalGroups: policyv1beta1.SupplementalGroupsStrategyOptions{
					Rule: policyv1beta1.SupplementalGroupsStrategyRunAsAny,
				},
				FSGroup: policyv1beta1.FSGroupStrategyOptions{
					Rule: policyv1beta1.FSGroupStrategyRunAsAny,
				},
				ReadOnlyRootFilesystem: false,
			},
		}

		clusterRolePSP = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:psp:kube-system:kube-proxy",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"policy", "extensions"},
					ResourceNames: []string{podSecurityPolicy.Name},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
				},
			},
		}

		roleBindingPSP = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:psp:kube-proxy",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					resourcesv1alpha1.DeleteOnInvalidUpdate: "true",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRolePSP.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRoleBinding,
		service,
		secret,
		configMap,
		configMapConntrackFixScript,
		configMapCleanupScript,
		podSecurityPolicy,
		clusterRolePSP,
		roleBindingPSP,
	)
}

func (k *kubeProxy) computePoolResourcesData(pool WorkerPool) (map[string][]byte, error) {
	k8sVersionLess125, err := version.CheckVersionMeetsConstraint(pool.KubernetesVersion, "< 1.25")
	if err != nil {
		return nil, err
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		directoryOrCreate = corev1.HostPathDirectoryOrCreate
		fileOrCreate      = corev1.HostPathFileOrCreate

		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name(pool),
				Namespace: metav1.NamespaceSystem,
				Labels:    getSystemComponentLabels(),
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: getPoolLabels(pool),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getPoolLabels(pool), getSystemComponentLabels()),
					},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{
							v1beta1constants.LabelWorkerPool:              pool.Name,
							v1beta1constants.LabelWorkerKubernetesVersion: pool.KubernetesVersion,
						},
						InitContainers: []corev1.Container{{
							Name:            "cleanup",
							Image:           pool.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								fmt.Sprintf("%s/%s %s", volumeMountPathCleanupScript, dataKeyCleanupScript, volumeMountPathMode),
							},
							Env: []corev1.EnvVar{
								{
									Name:  "KUBE_PROXY_MODE",
									Value: string(k.getMode()),
								},
								{
									Name:  "EXECUTE_WORKAROUND_FOR_K8S_ISSUE_109286",
									Value: strconv.FormatBool(k8sVersionLess125),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_ADMIN"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameCleanupScript,
									MountPath: volumeMountPathCleanupScript,
								},
								{
									Name:      volumeNameKernelModules,
									MountPath: volumeMountPathKernelModules,
								},
								{
									Name:      volumeNameDir,
									MountPath: volumeMountPathDir,
								},
								{
									Name:      volumeNameMode,
									MountPath: volumeMountPathMode,
								},
								{
									Name:      volumeNameKubeconfig,
									MountPath: volumeMountPathKubeconfig,
								},
								{
									Name:      volumeNameConfig,
									MountPath: volumeMountPathConfig,
								},
							},
						}},
						PriorityClassName: "system-node-critical",
						Tolerations: []corev1.Toleration{
							{
								Effect:   corev1.TaintEffectNoSchedule,
								Operator: corev1.TolerationOpExists,
							},
							{
								Key:      "CriticalAddonsOnly",
								Operator: corev1.TolerationOpExists,
							},
							{
								Effect:   corev1.TaintEffectNoExecute,
								Operator: corev1.TolerationOpExists,
							},
						},
						HostNetwork:        true,
						ServiceAccountName: k.serviceAccount.Name,
						Containers: []corev1.Container{
							{
								Name:            containerName,
								Image:           pool.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/usr/local/bin/kube-proxy",
									fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyConfig),
									"--v=2",
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: pointer.Bool(true),
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
								Ports: []corev1.ContainerPort{{
									Name:          portNameMetrics,
									ContainerPort: portMetrics,
									HostPort:      portMetrics,
									Protocol:      corev1.ProtocolTCP,
								}},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      volumeNameKubeconfig,
										MountPath: volumeMountPathKubeconfig,
									},
									{
										Name:      volumeNameConfig,
										MountPath: volumeMountPathConfig,
									},
									{
										Name:      volumeNameSSLCertsHosts,
										MountPath: volumeMountPathSSLCertsHosts,
										ReadOnly:  true,
									},
									{
										Name:      volumeNameSystemBusSocket,
										MountPath: volumeMountPathSystemBusSocket,
									},
									{
										Name:      volumeNameKernelModules,
										MountPath: volumeMountPathKernelModules,
									},
								},
							},
							// sidecar container with fix for conntrack
							{
								Name:            "conntrack-fix",
								Image:           k.values.ImageAlpine,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									fmt.Sprintf("%s/%s", volumeMountPathConntrackFixScript, dataKeyConntrackFixScript),
								},
								SecurityContext: &corev1.SecurityContext{
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"NET_ADMIN"},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      volumeNameConntrackFixScript,
										MountPath: volumeMountPathConntrackFixScript,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: volumeNameKubeconfig,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: k.secret.Name,
									},
								},
							},
							{
								Name: volumeNameConfig,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: k.configMap.Name,
										},
									},
								},
							},
							{
								Name: volumeNameSSLCertsHosts,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathSSLCertsHosts,
									},
								},
							},
							{
								Name: volumeNameSystemBusSocket,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathSystemBusSocket,
									},
								},
							},
							{
								Name: volumeNameKernelModules,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathKernelModules,
									},
								},
							},
							{
								Name: volumeNameCleanupScript,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: k.configMapCleanupScript.Name,
										},
										DefaultMode: pointer.Int32(0777),
									},
								},
							},
							{
								Name: volumeNameDir,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathDir,
										Type: &directoryOrCreate,
									},
								},
							},
							{
								Name: volumeNameMode,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathDir + "/mode",
										Type: &fileOrCreate,
									},
								},
							},
							{
								Name: volumeNameConntrackFixScript,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: k.configMapConntrackFixScript.Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	kubernetesVersion, err := semver.NewVersion(pool.KubernetesVersion)
	if err != nil {
		return nil, err
	}
	if version.ConstraintK8sGreaterEqual119.Check(kubernetesVersion) {
		if daemonSet.Spec.Template.Spec.SecurityContext == nil {
			daemonSet.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}
		daemonSet.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	if k.values.VPAEnabled {
		daemonSet.Spec.Template.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("2048Mi"),
		}

		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name(pool),
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "DaemonSet",
					Name:       daemonSet.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("10G"),
						},
						ControlledValues: &controlledValues,
					}},
				},
			},
		}
	}

	utilruntime.Must(references.InjectAnnotations(daemonSet))

	return registry.AddAllAndSerialize(
		daemonSet,
		vpa,
	)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelProxy,
	}
}

func getSystemComponentLabels() map[string]string {
	return map[string]string{
		managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
		v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
	}
}

func getPoolLabels(pool WorkerPool) map[string]string {
	return utils.MergeStringMaps(getLabels(), map[string]string{
		"pool":    pool.Name,
		"version": pool.KubernetesVersion,
	})
}

func (k *kubeProxy) getRawComponentConfig() (string, error) {
	config := &kubeproxyconfigv1alpha1.KubeProxyConfiguration{
		ClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			Kubeconfig: volumeMountPathKubeconfig + "/" + dataKeyKubeconfig,
		},
		MetricsBindAddress: fmt.Sprintf("0.0.0.0:%d", portMetrics),
		Mode:               k.getMode(),
		Conntrack: kubeproxyconfigv1alpha1.KubeProxyConntrackConfiguration{
			MaxPerCore: pointer.Int32(524288),
		},
		FeatureGates: k.values.FeatureGates,
	}

	if !k.values.IPVSEnabled && k.values.PodNetworkCIDR != nil {
		config.ClusterCIDR = *k.values.PodNetworkCIDR
	}

	return NewConfigCodec().Encode(config)
}

func (k *kubeProxy) getMode() kubeproxyconfigv1alpha1.ProxyMode {
	if k.values.IPVSEnabled {
		return "ipvs"
	}
	return "iptables"
}
