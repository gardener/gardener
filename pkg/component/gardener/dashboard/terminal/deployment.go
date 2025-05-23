// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	volumeMountPathConfig = "/etc/terminal-controller-manager"
	volumeNameConfig      = "config"
	volumeNameServerCert  = "server-cert"
)

func (t *terminal) deployment(
	secretNameGenericTokenKubeconfig string,
	secretNameVirtualGardenAccess string,
	secretNameServerCert string,
	configMapName string,
) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
			Labels: utils.MergeStringMaps(getLabels(), map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
			}),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector:             &metav1.LabelSelector{MatchLabels: getLabels()},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(getLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("virtual-garden-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            v1beta1constants.PriorityClassNameGardenSystem200,
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
					},
					TerminationGracePeriodSeconds: ptr.To(int64(10)),
					Containers: []corev1.Container{{
						Name:            name,
						Image:           t.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args: []string{
							"--config-file=" + volumeMountPathConfig + "/" + dataKeyConfig,
							"--zap-devel=false",
						},
						Env: []corev1.EnvVar{{
							Name:  "KUBECONFIG",
							Value: gardenerutils.PathGenericKubeconfig,
						}},
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          portNameAdmission,
								ContainerPort: portAdmission,
								Protocol:      corev1.ProtocolTCP,
							},
							{
								Name:          portNameMetrics,
								ContainerPort: portMetrics,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.FromInt32(portProbes),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 15,
							TimeoutSeconds:      5,
							FailureThreshold:    6,
							SuccessThreshold:    1,
							PeriodSeconds:       20,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Port:   intstr.FromInt32(portProbes),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 5,
							TimeoutSeconds:      5,
							FailureThreshold:    6,
							SuccessThreshold:    1,
							PeriodSeconds:       10,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameConfig,
								MountPath: volumeMountPathConfig,
								ReadOnly:  true,
							},
							{
								Name:      volumeNameServerCert,
								MountPath: "/tmp/k8s-webhook-server/serving-certs",
								ReadOnly:  true,
							},
							{
								Name:      volumeNameServerCert,
								MountPath: "/tmp/k8s-metrics-server/serving-certs",
								ReadOnly:  true,
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameConfig,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
								},
							},
						},
						{
							Name: volumeNameServerCert,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameServerCert,
								},
							},
						},
					},
				},
			},
		},
	}

	utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, secretNameGenericTokenKubeconfig, secretNameVirtualGardenAccess))
	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}
