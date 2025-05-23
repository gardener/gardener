// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	"fmt"

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
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	portNameServer  = "https"
	portNameMetrics = "metrics"
	portNameHealthz = "healthz"

	portServer  = 10443
	portMetrics = 8080
	portHealthz = 8081

	volumeNameTLS                   = "gardener-discovery-server-tls"
	volumeMountPathTLS              = "/var/run/secrets/gardener.cloud/gardener-discovery-server/tls"
	volumeNameWorkloadIdentity      = "garden-workload-identity"
	volumeMountPathWorkloadIdentity = "/etc/gardener-discovery-server/garden/workload-identity"
)

func (g *gardenerDiscoveryServer) deployment(
	secretNameGenericTokenKubeconfig string,
	secretNameVirtualGardenAccess string,
	secretNameTLS string,
	secretNameWorkloadIdentity string,
) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: g.namespace,
			Labels: utils.MergeStringMaps(labels(), map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
			}),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(labels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("virtual-garden-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            v1beta1constants.PriorityClassNameGardenSystem200,
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
						RunAsGroup:   ptr.To[int64](65532),
						FSGroup:      ptr.To[int64](65532),
					},
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           g.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--tls-cert-file=" + volumeMountPathTLS + "/" + secretsutils.DataKeyCertificate,
								"--tls-private-key-file=" + volumeMountPathTLS + "/" + secretsutils.DataKeyPrivateKey,
								"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
								fmt.Sprintf("--workload-identity-openid-configuration-file=%s/%s", volumeMountPathWorkloadIdentity, openIDConfigDataKey),
								fmt.Sprintf("--workload-identity-jwks-file=%s/%s", volumeMountPathWorkloadIdentity, jwksDataKey),
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          portNameServer,
									ContainerPort: portServer,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          portNameMetrics,
									ContainerPort: portMetrics,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          portNameHealthz,
									ContainerPort: portHealthz,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromString(portNameHealthz),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
								SuccessThreshold:    1,
								PeriodSeconds:       20,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromString(portNameHealthz),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameTLS,
									MountPath: volumeMountPathTLS,
									ReadOnly:  true,
								},
								{
									Name:      volumeNameWorkloadIdentity,
									MountPath: volumeMountPathWorkloadIdentity,
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameTLS,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameTLS,
									DefaultMode: ptr.To[int32](0400),
								},
							},
						},
						{
							Name: volumeNameWorkloadIdentity,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameWorkloadIdentity,
									DefaultMode: ptr.To[int32](0400),
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
