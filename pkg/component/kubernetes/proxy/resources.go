// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	_ "embed"
	"fmt"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	kubeproxyconfigv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	daemonSetNamePrefix = "kube-proxy"
	// ConfigNamePrefix is the prefix for the name of the kube-proxy ConfigMap.
	ConfigNamePrefix = "kube-proxy-config"

	containerName             = "kube-proxy"
	containerNameConntrackFix = "conntrack-fix"
	serviceName               = "kube-proxy"

	portNameMetrics = "metrics"
	portMetrics     = 10249
	portHealthz     = 10256

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
	volumeMountPathXtablesLock        = "/run/xtables.lock"

	volumeNameKubeconfig         = "kubeconfig"
	volumeNameConfig             = "kube-proxy-config"
	volumeNameDir                = "kube-proxy-dir"
	volumeNameMode               = "kube-proxy-mode"
	volumeNameCleanupScript      = "kube-proxy-cleanup-script"
	volumeNameConntrackFixScript = "conntrack-fix-script"
	volumeNameKernelModules      = "kernel-modules"
	volumeNameSSLCertsHosts      = "ssl-certs-hosts"
	volumeNameXtablesLock        = "xtables-lock"

	hostPathSSLCertsHosts = "/usr/share/ca-certificates"
	hostPathKernelModules = "/lib/modules"
	hostPathDir           = "/var/lib/kube-proxy"
	hostPathXtablesLock   = "/run/xtables.lock"
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
			AutomountServiceAccountToken: ptr.To(false),
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
				Labels:    GetLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{{
					Name:     portNameMetrics,
					Port:     int32(portMetrics),
					Protocol: corev1.ProtocolTCP,
				}},
				Selector: GetLabels(),
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
				Labels:    utils.MergeStringMaps(GetLabels(), getSystemComponentLabels()),
			},
			Data: map[string]string{dataKeyConfig: componentConfigRaw},
		}

		configMapConntrackFixScript = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy-conntrack-fix-script",
				Namespace: metav1.NamespaceSystem,
				Labels:    utils.MergeStringMaps(GetLabels(), getSystemComponentLabels()),
			},
			Data: map[string]string{dataKeyConntrackFixScript: conntrackFixScript},
		}

		configMapCleanupScript = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-proxy-cleanup-script",
				Namespace: metav1.NamespaceSystem,
				Labels:    utils.MergeStringMaps(GetLabels(), getSystemComponentLabels()),
			},
			Data: map[string]string{dataKeyCleanupScript: cleanupScript},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(secret))
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	utilruntime.Must(kubernetesutils.MakeUnique(configMapConntrackFixScript))
	utilruntime.Must(kubernetesutils.MakeUnique(configMapCleanupScript))

	k.serviceAccount = serviceAccount
	k.secret = secret
	k.configMap = configMap
	k.configMapCleanupScript = configMapCleanupScript
	k.configMapConntrackFixScript = configMapConntrackFixScript

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRoleBinding,
		service,
		secret,
		configMap,
		configMapConntrackFixScript,
		configMapCleanupScript,
	)
}

func (k *kubeProxy) computePoolResourcesData(pool WorkerPool) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		directoryOrCreate  = corev1.HostPathDirectoryOrCreate
		fileOrCreate       = corev1.HostPathFileOrCreate
		k8sGreaterEqual128 = versionutils.ConstraintK8sGreaterEqual128.Check(pool.KubernetesVersion)
		k8sGreaterEqual129 = versionutils.ConstraintK8sGreaterEqual129.Check(pool.KubernetesVersion)

		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name(pool, ptr.To(false)),
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(
					getSystemComponentLabels(),
					map[string]string{v1beta1constants.LabelNodeCriticalComponent: "true"},
				),
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: getPoolLabels(pool),
				},
				RevisionHistoryLimit: ptr.To[int32](2),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(
							getPoolLabels(pool),
							getSystemComponentLabels(),
							map[string]string{v1beta1constants.LabelNodeCriticalComponent: "true"},
						),
					},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{
							v1beta1constants.LabelWorkerPool:              pool.Name,
							v1beta1constants.LabelWorkerKubernetesVersion: pool.KubernetesVersion.String(),
						},
						InitContainers:    k.getInitContainers(pool.KubernetesVersion, pool.Image),
						PriorityClassName: "system-node-critical",
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Tolerations: []corev1.Toleration{
							{
								Effect:   corev1.TaintEffectNoSchedule,
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
							k.getKubeProxyContainer(k8sGreaterEqual129, k8sGreaterEqual128, pool.Image, false),
							{
								// sidecar container with fix for conntrack
								Name:            containerNameConntrackFix,
								Image:           k.values.ImageAlpine,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									fmt.Sprintf("%s/%s", volumeMountPathConntrackFixScript, dataKeyConntrackFixScript),
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
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
										DefaultMode: ptr.To[int32](0777),
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
							{
								Name: volumeNameXtablesLock,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathXtablesLock,
										Type: &fileOrCreate,
									},
								},
							},
						},
					},
				},
			},
		}
	)

	if k.values.VPAEnabled {
		daemonSet.Spec.Template.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("2048Mi"),
		}
		if k8sGreaterEqual129 {
			daemonSet.Spec.Template.Spec.InitContainers[1].Resources.Limits = corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			}
		}
	}

	utilruntime.Must(references.InjectAnnotations(daemonSet))

	return registry.AddAllAndSerialize(daemonSet)
}

func (k *kubeProxy) computePoolResourcesDataForMajorMinorVersionOnly(pool WorkerPool) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	if k.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name(pool, ptr.To(true)),
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "DaemonSet",
					Name:       name(pool, ptr.To(false)),
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: containerName,
							MaxAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("10G"),
							},
							ControlledValues: &controlledValues,
						},
						{
							ContainerName: containerNameConntrackFix,
							Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						},
					},
				},
			},
		}
	}

	return registry.AddAllAndSerialize(vpa)
}

// GetLabels returns the default labels for kube-proxy resources.
func GetLabels() map[string]string {
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
	return utils.MergeStringMaps(GetLabels(), map[string]string{
		"pool":    pool.Name,
		"version": pool.KubernetesVersion.String(),
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
			MaxPerCore: ptr.To[int32](524288),
		},
		FeatureGates: k.values.FeatureGates,
	}

	if !k.values.IPVSEnabled && len(k.values.PodNetworkCIDRs) > 0 {
		if err := netutils.CheckDualStackForKubeComponents(k.values.PodNetworkCIDRs, "pod"); err != nil {
			return "", err
		}

		config.ClusterCIDR = netutils.JoinByComma(k.values.PodNetworkCIDRs)
	}

	return NewConfigCodec().Encode(config)
}

func (k *kubeProxy) getMode() kubeproxyconfigv1alpha1.ProxyMode {
	if k.values.IPVSEnabled {
		return "ipvs"
	}
	return "iptables"
}

func (k *kubeProxy) getInitContainers(kubernetesVersion *semver.Version, image string) []corev1.Container {
	initContainers := []corev1.Container{
		{
			Name:            "cleanup",
			Image:           image,
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
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: ptr.To(true),
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
		},
	}

	k8sGreaterEqual129 := versionutils.ConstraintK8sGreaterEqual129.Check(kubernetesVersion)
	if k8sGreaterEqual129 {
		k8sGreaterEqual128 := versionutils.ConstraintK8sGreaterEqual128.Check(kubernetesVersion)
		initContainers = append(initContainers, k.getKubeProxyContainer(k8sGreaterEqual129, k8sGreaterEqual128, image, true))
	}

	return initContainers
}

func (k *kubeProxy) getKubeProxyContainer(k8sGreaterEqual129, k8sGreaterEqual128 bool, image string, init bool) corev1.Container {
	container := corev1.Container{
		Name:            containerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/usr/local/bin/kube-proxy",
			fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyConfig),
			"--v=2",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN", "SYS_RESOURCE"},
			},
		},
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
				Name:      volumeNameKernelModules,
				MountPath: volumeMountPathKernelModules,
			},
			{
				Name:      volumeNameXtablesLock,
				MountPath: volumeMountPathXtablesLock,
			},
		},
	}

	if !k8sGreaterEqual129 || init {
		container.SecurityContext = &corev1.SecurityContext{
			Privileged: ptr.To(true),
		}
	}
	if init {
		container.Name += "-init"
		container.Command = append(container.Command, "--init-only")
	} else {
		container.Ports = []corev1.ContainerPort{{
			Name:          portNameMetrics,
			ContainerPort: portMetrics,
			HostPort:      portMetrics,
			Protocol:      corev1.ProtocolTCP,
		}}

		urlPath := "/healthz"
		if k8sGreaterEqual128 {
			urlPath = "/livez"
		}
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   urlPath,
					Port:   intstr.FromInt32(portHealthz),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			SuccessThreshold:    1,
			FailureThreshold:    2,
		}
	}

	return container
}
