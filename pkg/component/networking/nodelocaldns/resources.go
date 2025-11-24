// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns

import (
	"errors"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (n *nodeLocalDNS) computeResourcesData() (*corev1.ServiceAccount, *corev1.ConfigMap, *corev1.Service, error) {
	if n.getHealthAddress() == "" {
		return nil, nil, nil, errors.New("empty IPVSAddress")
	}

	var (
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					labelKey: nodelocaldnsconstants.LabelValue,
				},
			},
			Data: map[string]string{
				configDataKey: domain + `:53 {
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + n.getHealthAddress() + `:` + strconv.Itoa(livenessProbePort) + `
    import custom/*.override
    errors
    cache {
            success 9984 30
            denial 9984 5
    }
    reload
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
.:53 {
    loop
    bind ` + n.bindIP() + `
    forward . ` + n.upstreamDNSAddress() + ` {
            ` + n.forceTcpToUpstreamDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    import custom/*.override
    errors
    cache 30
    reload
    }
`,
			},
		}
	)

	if n.values.CustomDNSServerInNodeLocalDNS {
		configMap.Data[configDataKey] = configMap.Data[configDataKey] + "import generated-config/custom-server-block.server\n"
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					"k8s-app": "kube-dns-upstream",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"k8s-app": "kube-dns"},
				Ports: []corev1.ServicePort{
					{
						Name:       "dns",
						Port:       int32(portServiceServer),
						TargetPort: intstr.FromInt32(portServer),
						Protocol:   corev1.ProtocolUDP,
					},
					{
						Name:       "dns-tcp",
						Port:       int32(portServiceServer),
						TargetPort: intstr.FromInt32(portServer),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
	)

	return serviceAccount, configMap, service, nil
}

func (n *nodeLocalDNS) computePoolResourcesData(serviceAccount *corev1.ServiceAccount, configMap *corev1.ConfigMap, service *corev1.Service) (clientObjects []client.Object) {
	var (
		maxUnavailable       = intstr.FromString("10%")
		hostPathFileOrCreate = corev1.HostPathFileOrCreate
		vpa                  *vpaautoscalingv1.VerticalPodAutoscaler
		daemonSet            *appsv1.DaemonSet
	)

	clientObjects = []client.Object{serviceAccount, configMap, service}
	for _, worker := range n.values.Workers {
		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns-" + worker.Name,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					labelKey:                                    nodelocaldnsconstants.LabelValue,
					v1beta1constants.GardenRole:                 v1beta1constants.GardenRoleSystemComponent,
					managedresources.LabelKeyOrigin:             managedresources.LabelValueGardener,
					v1beta1constants.LabelNodeCriticalComponent: "true",
				},
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					RollingUpdate: &appsv1.RollingUpdateDaemonSet{
						MaxUnavailable: &maxUnavailable,
					},
				},
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						labelKey: nodelocaldnsconstants.LabelValue,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							labelKey:                                    nodelocaldnsconstants.LabelValue,
							v1beta1constants.LabelNetworkPolicyToDNS:    "allowed",
							v1beta1constants.LabelNodeCriticalComponent: "true",
						},
						Annotations: map[string]string{
							"prometheus.io/port":   strconv.Itoa(prometheusPort),
							"prometheus.io/scrape": strconv.FormatBool(prometheusScrape),
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-node-critical",
						ServiceAccountName: serviceAccount.Name,
						HostNetwork:        true,
						DNSPolicy:          corev1.DNSDefault,
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Tolerations: []corev1.Toleration{
							{
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoExecute,
							},
							{
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoSchedule,
							},
						},
						NodeSelector: map[string]string{
							v1beta1constants.LabelNodeLocalDNS: "true",
							v1beta1constants.LabelWorkerPool:   worker.Name,
						},
						Containers: []corev1.Container{
							{
								Name:  containerName,
								Image: n.values.Image,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("25m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("200Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"NET_ADMIN"},
									},
								},
								Args: []string{
									"-localip",
									n.containerArg(),
									"-conf",
									"/etc/Corefile",
									"-upstreamsvc",
									serviceName,
									"-health-port",
									strconv.Itoa(livenessProbePort),
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: int32(53),
										Name:          "dns",
										Protocol:      corev1.ProtocolUDP,
									},
									{
										ContainerPort: int32(53),
										Name:          "dns-tcp",
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusPort),
										Name:          metricsPortName,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusErrorPort),
										Name:          errorMetricsPortName,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Host: n.getIPVSAddress(),
											Path: "/health",
											Port: intstr.FromInt32(livenessProbePort),
										},
									},
									InitialDelaySeconds: int32(60),
									TimeoutSeconds:      int32(5),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: "/run/xtables.lock",
										Name:      "xtables-lock",
										ReadOnly:  false,
									},
									{
										MountPath: "/etc/coredns",
										Name:      "config-volume",
									},
									{
										MountPath: "/etc/kube-dns",
										Name:      "kube-dns-config",
									},
									{
										Name:      volumeMountNameCustomConfig,
										MountPath: volumeMountPathCustomConfig,
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "xtables-lock",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/run/xtables.lock",
										Type: &hostPathFileOrCreate,
									},
								},
							},
							{
								Name: "kube-dns-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-dns",
										},
										Optional: ptr.To(true),
									},
								},
							},
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  configDataKey,
												Path: "Corefile.base",
											},
										},
									},
								},
							},
							{
								Name: volumeMountNameCustomConfig,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: coredns.CustomConfigMapName,
										},
										DefaultMode: ptr.To[int32](420),
										Optional:    ptr.To(true),
									},
								},
							},
						},
					},
				},
			},
		}

		if n.values.CustomDNSServerInNodeLocalDNS {
			daemonSet.Spec.Template.Spec.InitContainers = append(daemonSet.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  sideCarName,
				Image: n.values.CorednsConfigAdapterImage,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("10Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
					RunAsUser:                ptr.To[int64](65532),
					RunAsGroup:               ptr.To[int64](65532),
				},
				Args: []string{
					"-inputDir=" + volumeMountPathCustomConfig,
					"-outputDir=" + volumeMountPathGeneratedConfig,
					"-bind=bind " + n.bindIP(),
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      volumeMountNameCustomConfig,
						MountPath: volumeMountPathCustomConfig,
						ReadOnly:  true,
					},
					{
						MountPath: volumeMountPathGeneratedConfig,
						Name:      volumeMountNameGeneratedConfig,
					},
				},
				RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
			})

			daemonSet.Spec.Template.Spec.Volumes = append(daemonSet.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeMountNameGeneratedConfig,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})

			daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(daemonSet.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				MountPath: volumeMountPathGeneratedConfig,
				Name:      volumeMountNameGeneratedConfig,
			})
		}

		utilruntime.Must(references.InjectAnnotations(daemonSet))
		clientObjects = append(clientObjects, daemonSet)

		if n.values.VPAEnabled {
			vpaUpdateMode := vpaautoscalingv1.UpdateModeRecreate
			vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-local-dns-" + worker.Name,
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
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName:    containerName,
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							},
						},
					},
				},
			}
			if n.values.CustomDNSServerInNodeLocalDNS {
				vpa.Spec.ResourcePolicy.ContainerPolicies = append(vpa.Spec.ResourcePolicy.ContainerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
					ContainerName: sideCarName,
					Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
				})
			}
			clientObjects = append(clientObjects, vpa)
		}
	}
	return clientObjects
}
